package s3

// Code based on https://github.com/crowdmob/goamz

import(
  "encoding/xml"
  "fmt"
  "io"
  "io/ioutil"
  "log"
  "errors"
  "net/http"
  "net/http/httputil"
  "net/url"
  "strconv"
  "strings"
  "time"
)

type Auth struct {
  AccessKey, SecretKey string
  token                string
  expiration           time.Time
}

type request struct {
  method   string
  bucket   string
  path     string
  params   url.Values
  headers  http.Header
  baseurl  string
  payload  io.Reader
  prepared bool
  auth     Auth
}

func (req *request) url() (*url.URL, error) {
  u, err := url.Parse(req.baseurl)
  if err != nil {
    return nil, fmt.Errorf("bad S3 endpoint URL %q: %v", req.baseurl, err)
  }
  u.RawQuery = req.params.Encode()
  u.Path = req.path
  return u, nil
}

var (
  S3BucketEndpoint ="https://${bucket}.s3.amazonaws.com"
  S3Endpoint = "https://ec2.eu-west-1.amazonaws.com"
)


func GetS3File(client *http.Client, path string, bucket string, headers map[string][]string, auth Auth) (*http.Response, error) {
  req := &request{
    bucket:  bucket,
    path:    path,
    headers: headers,
    auth:    auth,
  }

  if err := prepare(req); err != nil {
    return nil, err
  }

  return run(client, req, nil)
}


func prepare(req *request) error {
  var signpath = req.path

  if !req.prepared {
    req.prepared = true
    if req.method == "" {
      req.method = "GET"
    }
    // Copy so they can be mutated without affecting on retries.
    params := make(url.Values)
    headers := make(http.Header)
    for k, v := range req.params {
      params[k] = v
    }
    for k, v := range req.headers {
      headers[k] = v
    }
    req.params = params
    req.headers = headers
    if !strings.HasPrefix(req.path, "/") {
      req.path = "/" + req.path
    }
    signpath = req.path
    if req.bucket != "" {
      req.baseurl = S3BucketEndpoint
      if req.baseurl == "" {
        // Use the path method to address the bucket.
        req.baseurl = S3Endpoint
        req.path = "/" + req.bucket + req.path
      } else {
        // Just in case, prevent injection.
        if strings.IndexAny(req.bucket, "/:@") >= 0 {
          return fmt.Errorf("bad S3 bucket: %q", req.bucket)
        }
        req.baseurl = strings.Replace(req.baseurl, "${bucket}", req.bucket, -1)
      }
      signpath = "/" + req.bucket + signpath
    }
  }
  // Always sign again as it's not clear how far the
  // server has handled a previous attempt.
  u, err := url.Parse(req.baseurl)
  if err != nil {
    return fmt.Errorf("bad S3 endpoint URL %q: %v", req.baseurl, err)
  }
  reqSignpathSpaceFix := (&url.URL{Path: signpath}).String()
  req.headers["Host"] = []string{u.Host}
  req.headers["Date"] = []string{time.Now().In(time.UTC).Format(time.RFC1123)}
  // if s3.Auth.Token() != "" {
  //   req.headers["X-Amz-Security-Token"] = []string{s3.Auth.Token()}
  // }
  Sign(req.auth, req.method, reqSignpathSpaceFix, req.params, req.headers)
  return nil
}

func run(c *http.Client, req *request, resp interface{}) (*http.Response, error) {
  log.Printf("Running S3 request: %#v", req)

  u, err := req.url()
  if err != nil {
    return nil, err
  }

  hreq := http.Request{
    URL:        u,
    Method:     req.method,
    ProtoMajor: 1,
    ProtoMinor: 1,
    Close:      true,
    Header:     req.headers,
  }

  if v, ok := req.headers["Content-Length"]; ok {
    hreq.ContentLength, _ = strconv.ParseInt(v[0], 10, 64)
    delete(req.headers, "Content-Length")
  }
  if req.payload != nil {
    hreq.Body = ioutil.NopCloser(req.payload)
  }

  hresp, err := c.Do(&hreq)
  if err != nil {
    return nil, err
  }

  dump, _ := httputil.DumpResponse(hresp, false)
  log.Printf("} -> %s\n", dump)

  if hresp.StatusCode != 200 && hresp.StatusCode != 204 {
    return nil, errors.New("ERROR" )
  }
  if resp != nil {
    err = xml.NewDecoder(hresp.Body).Decode(resp)
    hresp.Body.Close()
    log.Printf("goamz.s3> decoded xml into %#v", resp)
  }
  return hresp, err
}
