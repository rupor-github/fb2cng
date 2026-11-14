package images

import (
	"image"
	"image/color"
	"image/draw"
)

// GrayscaleLuma uses Go's JFIF/BT.601 weights without linear-light conversion.
// For transparent images, convert unassociated RGB and retain alpha rather
// than converting premultiplied colors to image.Gray and losing transparency.
func GrayscaleLuma(img image.Image) image.Image {
	switch img.(type) {
	case *image.Gray, *image.Gray16:
		return img
	}
	opaque := false
	if source, ok := img.(interface{ Opaque() bool }); ok {
		opaque = source.Opaque()
	}
	bounds := img.Bounds()
	switch img.(type) {
	case *image.RGBA64, *image.NRGBA64:
		if opaque {
			gray := image.NewGray16(bounds)
			draw.Draw(gray, bounds, img, bounds.Min, draw.Src)
			return gray
		}
		dst := image.NewNRGBA64(bounds)
		for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
			for x := bounds.Min.X; x < bounds.Max.X; x++ {
				c := color.NRGBA64Model.Convert(img.At(x, y)).(color.NRGBA64)
				gray := color.Gray16Model.Convert(color.RGBA64{R: c.R, G: c.G, B: c.B, A: 0xffff}).(color.Gray16)
				dst.SetNRGBA64(x, y, color.NRGBA64{R: gray.Y, G: gray.Y, B: gray.Y, A: c.A})
			}
		}
		return dst
	}
	if opaque {
		gray := image.NewGray(bounds)
		draw.Draw(gray, bounds, img, bounds.Min, draw.Src)
		return gray
	}
	dst := image.NewNRGBA(bounds)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			c := color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA)
			gray := color.GrayModel.Convert(color.RGBA{R: c.R, G: c.G, B: c.B, A: 255}).(color.Gray)
			dst.SetNRGBA(x, y, color.NRGBA{R: gray.Y, G: gray.Y, B: gray.Y, A: c.A})
		}
	}
	return dst
}

// IsGrayscale reports whether img is grayscale (all pixels have R==G==B).
// NOTE: This function may be slow for large images, if speed is a problem it
// could be optimized.
func IsGrayscale(img image.Image) bool {
	switch img.(type) {
	case *image.Gray, *image.Gray16:
		return true
	}

	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			c := color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA)
			if c.R != c.G || c.G != c.B {
				return false
			}
		}
	}
	return true
}
