package ninja_proxy

import (
  "fmt"
  "io"
  "net/http"
  "net/url"
  "strings"
  "time"
  "strconv"
  "github.com/golang/glog"
  "github.com/gregjones/httpcache"
  "reflect"
)

// Proxy serves image requests.
type Proxy struct {
  Client *http.Client // client used to fetch remote URLs
  Cache  Cache

  // Whitelist specifies a list of remote hosts that images can be proxied from.  An empty list means all hosts are allowed.
  Whitelist []string

  MaxWidth  int
  MaxHeight int
}

// NewProxy constructs a new proxy.  The provided http Client will be used to
// fetch remote URLs.  If nil is provided, http.DefaultClient will be used.
func NewProxy(transport http.RoundTripper, cache Cache, options map[string]string) *Proxy {
  if transport == nil {
    transport = http.DefaultTransport
  }
  if cache == nil {
    cache = NopCache
  }

  client := new(http.Client)
  if BucketName := options["BucketName"]; BucketName == "" {
    fmt.Println("  - Normal mode")
    client.Transport = &httpcache.Transport{
      Transport:           &TransformingTransport{transport, client},
      Cache:               cache,
      MarkCachedResponses: true,
    }
  } else {
    fmt.Println("  - S3 mode")
    fmt.Println("  - Bucket: ", options["BucketName"])
    client.Transport = &httpcache.Transport{
      Transport:           &S3TransformingTransport{
        Transport:      transport,
        Client:         client,
        BucketName:     options["BucketName"],
        AwsKeyId:       options["AwsKeyId"],
        AwsSecretKey:   options["AwsSecretKey"],
      },
      Cache:               cache,
      MarkCachedResponses: true,
    }
  }

  MaxHeight, _ := strconv.Atoi(options["MaxHeight"])
  MaxWidth, _ := strconv.Atoi(options["MaxWidth"])
  var Whitelist []string = nil

  if _whiteList := options["Whitelist"]; _whiteList != "" {
    Whitelist = strings.Split(_whiteList, ",")
  }

  fmt.Println("  - MaxHeight: ", MaxHeight)
  fmt.Println("  - MaxWidth: ", MaxWidth)
  fmt.Println("  - Whitelist: ", Whitelist)

  return &Proxy{
    Client:     client,
    Cache:      cache,
    MaxHeight:  MaxHeight,
    MaxWidth:   MaxWidth,
    Whitelist:  Whitelist,
  }
}

// ServeHTTP handles image requests.
func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
  req, err := NewRequest(r)
  if err != nil {
    glog.Errorf("invalid request URL: %v", err)
    http.Error(w, fmt.Sprintf("invalid request URL: %v", err), http.StatusBadRequest)
    return
  }

  fmt.Println("request: ", req)

  if p.MaxWidth > 0 && int(req.Options.Width) > p.MaxWidth {
    req.Options.Width = float64(p.MaxWidth)
  }
  if p.MaxHeight > 0 && int(req.Options.Height) > p.MaxHeight {
    req.Options.Height = float64(p.MaxHeight)
  }

  if !p.allowed(req.URL) {
    glog.Errorf("remote URL is not for an allowed host: %v", req.URL)
    http.Error(w, fmt.Sprintf("remote URL is not for an allowed host: %v", req.URL), http.StatusBadRequest)
    return
  }

  u := req.URL.String()
  if req.Options != nil && !reflect.DeepEqual(req.Options, emptyOptions) {
    u += "#" + req.Options.String()
  }
  resp, err := p.Client.Get(u)
  if err != nil {
    glog.Errorf("error fetching remote image: %v", err)
    http.Error(w, fmt.Sprintf("Error fetching remote image: %v", err), http.StatusInternalServerError)
    return
  }

  if resp.StatusCode != http.StatusOK {
    http.Error(w, fmt.Sprintf("Remote URL %q returned status: %v", req.URL, resp.Status), http.StatusInternalServerError)
    return
  }

  w.Header().Add("Last-Modified", resp.Header.Get("Last-Modified"))
  w.Header().Add("Expires", resp.Header.Get("Expires"))
  w.Header().Add("Etag", resp.Header.Get("Etag"))

  if is304 := check304(w, r, resp); is304 {
    w.WriteHeader(http.StatusNotModified)
    return
  }

  w.Header().Add("Content-Length", resp.Header.Get("Content-Length"))
  defer resp.Body.Close()
  io.Copy(w, resp.Body)
}

// allowed returns whether the specified URL is on the whitelist of remote hosts.
func (p *Proxy) allowed(u *url.URL) bool {
  if len(p.Whitelist) == 0 {
    return true
  }

  for _, host := range p.Whitelist {
    if u.Host == host {
      return true
    }
    if strings.HasPrefix(host, "*.") && strings.HasSuffix(u.Host, host[2:]) {
      return true
    }
  }

  return false
}

func check304(w http.ResponseWriter, req *http.Request, resp *http.Response) bool {
  etag := resp.Header.Get("Etag")
  if etag != "" && etag == req.Header.Get("If-None-Match") {
    return true
  }

  lastModified, err := time.Parse(time.RFC1123, resp.Header.Get("Last-Modified"))
  if err != nil {
    return false
  }
  ifModSince, err := time.Parse(time.RFC1123, req.Header.Get("If-Modified-Since"))
  if err != nil {
    return false
  }
  if lastModified.Before(ifModSince) {
    return true
  }

  return false
}
