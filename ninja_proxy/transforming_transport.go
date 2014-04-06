package ninja_proxy

import(
  "bufio"
  "bytes"
  "fmt"
  "io/ioutil"
  "net/http"
)

type TransformingTransport struct {
  Transport http.RoundTripper
  Client *http.Client
}

func (t *TransformingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
  if req.URL.Fragment == "" {
    fmt.Println("fetching remote URL: %v", req.URL)
    return t.Transport.RoundTrip(req)
  }

  u := *req.URL
  u.Fragment = ""
  resp, err := t.Client.Get(u.String())

  defer resp.Body.Close()
  b, err := ioutil.ReadAll(resp.Body)
  if err != nil {
    return nil, err
  }

  opt := ParseOptions(req.URL.Fragment)
  img, err := Transform(b, opt)
  if err != nil {
    img = b
  }

  buf := new(bytes.Buffer)
  fmt.Fprintf(buf, "%s %s\n", resp.Proto, resp.Status)
  resp.Header.WriteSubset(buf, map[string]bool{"Content-Length": true})
  fmt.Fprintf(buf, "Content-Length: %d\n\n", len(img))
  buf.Write(img)

  return http.ReadResponse(bufio.NewReader(buf), req)
}
