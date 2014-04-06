package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
  "os"
  "github.com/gregjones/httpcache"
  "github.com/gregjones/httpcache/diskcache"
  "github.com/Arkan/ninja_proxy/ninja_proxy"
)

var VERSION = 0.1
var addr        = flag.String("addr",       "localhost:8080", "TCP address to listen on")
var whitelist   = flag.String("whitelist",  "",               "comma separated list of allowed remote hosts")
var cacheDir    = flag.String("cacheDir",   "",               "directory to use for file cache")
var bucketName  = flag.String("bucketName", "",               "bucket name")
var version     = flag.Bool  ("version",    false,            "print version information")
var MaxWidth    = flag.String("maxWidth",   "2000",           "max image width")
var MaxHeight   = flag.String("maxHeight",  "2000",           "max image height")
var CacheMode   = flag.String("cacheMode",  "none",           "Cache mode - none/memory/disk")

func main(){
  flag.Parse()

  if *version {
    fmt.Printf("ImageProcessing: %v\n", VERSION)
    return
  }

  if *bucketName != "" && (os.Getenv("AWS_ACCESS_KEY_ID") == "" || os.Getenv("AWS_SECRET_ACCESS_KEY") == "") {
    fmt.Printf("Error: AWS_ACCESS_KEY_ID and AWS_SECRET_ACCESS_KEY should be set\n")
    return
  }

  fmt.Printf("ImageProcessing (version %v)\n", VERSION)

  var c httpcache.Cache
  if *CacheMode  == "disk" && *cacheDir != "" {
    c = diskcache.New(*cacheDir)
    fmt.Printf("  - Cache: DiskCache (%s)\n", *cacheDir)
  } else if *CacheMode == "memory" {
    c = httpcache.NewMemoryCache()
    fmt.Printf("  - Cache: MemoryCache\n")
  } else {
    c = nil
    fmt.Printf("  - Cache: None\n")
  }

  options := map[string]string{
    "MaxWidth":               *MaxWidth,
    "MaxHeight":              *MaxHeight,
    "Whitelist":              *whitelist,
    "BucketName":             *bucketName,
    "AwsKeyId":               os.Getenv("AWS_ACCESS_KEY_ID"),
    "AwsSecretKey":           os.Getenv("AWS_SECRET_ACCESS_KEY"),
  }

  p := ninja_proxy.NewProxy(nil, c, options)

  server := &http.Server{
    Addr:     *addr,
    Handler:  p,
  }

  fmt.Printf("\nListening on %s\n", server.Addr)

  err := server.ListenAndServe()
  if err != nil {
    log.Fatal("ListenAndServe: ", err)
  }
}
