package ninja_proxy

import (
  "bytes"
  "image"
  "image/gif"
  "image/jpeg"
  "image/png"
  "reflect"
  "github.com/disintegration/imaging"
)

// Transform the provided image.
func Transform(img []byte, opt *Options) ([]byte, error) {
  if opt == nil || reflect.DeepEqual(opt, emptyOptions) {
    // bail if no transformation was requested
    return img, nil
  }

  // decode image
  m, format, err := image.Decode(bytes.NewReader(img))
  if err != nil {
    return nil, err
  }

  // convert percentage width and height values to absolute values
  imgW := m.Bounds().Max.X - m.Bounds().Min.X
  imgH := m.Bounds().Max.Y - m.Bounds().Min.Y
  var h, w int
  if opt.Width > 0 && opt.Width < 1 {
    w = int(float64(imgW) * opt.Width)
  } else {
    w = int(opt.Width)
  }
  if opt.Height > 0 && opt.Height < 1 {
    h = int(float64(imgH) * opt.Height)
  } else {
    h = int(opt.Height)
  }

  // never resize larger than the original image
  if w > imgW {
    w = imgW
  }
  if h > imgH {
    h = imgH
  }

  // resize
  if w != 0 || h != 0 {
    if opt.Fit {
      m = imaging.Fit(m, w, h, imaging.Lanczos)
    } else {
      if w == 0 || h == 0 {
        m = imaging.Resize(m, w, h, imaging.Lanczos)
      } else {
        m = imaging.Thumbnail(m, w, h, imaging.Lanczos)
      }
    }
  }

  // flip
  if opt.FlipVertical {
    m = imaging.FlipV(m)
  }
  if opt.FlipHorizontal {
    m = imaging.FlipH(m)
  }

  // rotate
  switch opt.Rotate {
  case 90:
    m = imaging.Rotate90(m)
    break
  case 180:
    m = imaging.Rotate180(m)
    break
  case 270:
    m = imaging.Rotate270(m)
    break
  }

  // encode image
  buf := new(bytes.Buffer)
  switch format {
  case "gif":
    gif.Encode(buf, m, nil)
    break
  case "jpeg":
    jpeg.Encode(buf, m, &jpeg.Options{95})
    break
  case "png":
    png.Encode(buf, m)
    break
  }

  return buf.Bytes(), nil
}
