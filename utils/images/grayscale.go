package images

import (
	"image"
	"image/color"
)

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
