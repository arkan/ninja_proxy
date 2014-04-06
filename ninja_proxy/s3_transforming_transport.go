package ninja_proxy

import(
  "bufio"
  "bytes"
  "fmt"
  "io/ioutil"
  "net/http"
  "github.com/Arkan/ninja_proxy/s3"
  "time"
)

type S3TransformingTransport struct {
  Transport     http.RoundTripper
  Client        *http.Client
  BucketName    string
  AwsKeyId      string
  AwsSecretKey  string
}

func (t *S3TransformingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
  if req.URL.Fragment == "" {
    fmt.Println("fetching remote URL: %v", req.URL)
    return t.Transport.RoundTrip(req)
  }

  fmt.Println("req.URL = ", req.URL)
  fmt.Println("req.URL.Fragment = ", req.URL.Fragment)
  fmt.Println("req.URL.path = ", req.URL.Path)
  auth := s3.Auth{AccessKey: t.AwsKeyId, SecretKey: t.AwsSecretKey}
  fmt.Println("auth: ", auth)
  path := req.URL.Path
  fmt.Println("Path: ", path)
  resp, err2 := s3.GetS3File(t.Client, path, t.BucketName, make(http.Header), auth)

  if err2 != nil {
    fmt.Println("err = ", err2)
    return nil, err2
  }

  b, err := ioutil.ReadAll(resp.Body)
  if err != nil {
    return nil, err
  }

  opt := ParseOptions(req.URL.Fragment)
  img, err := Transform(b, opt)
  if err != nil {
    img = b
  }

  // replay response with transformed image and updated content length
  buf := new(bytes.Buffer)
  fmt.Println("Status = ", resp.Status)
  // fmt.Fprintf(buf, "%s %s\n", resp.Proto, resp.Status)
  fmt.Fprintf(buf, "HTTP/1.1 200 OK\n")
  fmt.Fprintf(buf, "Expires: %s\n", time.Now().Add(time.Hour*24*365*10).In(time.UTC).Format(time.RFC1123))
  fmt.Fprintf(buf, "Cache-Control: public, max-age=315576000\n")
  resp.Header.WriteSubset(buf, map[string]bool{"Content-Length": true, "Transfer-Encoding": true})
  fmt.Fprintf(buf, "Content-Length: %d\n\n", len(img))
  buf.Write(img)

  return http.ReadResponse(bufio.NewReader(buf), req)
}
