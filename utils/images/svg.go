package images

import (
	"bytes"
	"image"
	"image/color"
	"image/draw"
	"math"

	"github.com/srwiley/oksvg"
	"github.com/srwiley/rasterx"
)

// RasterizeSVGToImage rasterizes SVG to an RGBA image.
// If targetW/targetH are <=0, dimensions are taken from the SVG viewBox (falling back to 1024x1024).
func RasterizeSVGToImage(svgData []byte, targetW, targetH int) (image.Image, error) {
	icon, err := oksvg.ReadIconStream(bytes.NewReader(svgData))
	if err != nil {
		return nil, err
	}

	w := int(math.Ceil(icon.ViewBox.W))
	h := int(math.Ceil(icon.ViewBox.H))
	if targetW > 0 {
		w = targetW
	}
	if targetH > 0 {
		h = targetH
	}
	if w <= 0 {
		w = 1024
	}
	if h <= 0 {
		h = 1024
	}

	icon.SetTarget(0, 0, float64(w), float64(h))

	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.Draw(dst, dst.Bounds(), &image.Uniform{C: color.RGBA{255, 255, 255, 255}}, image.Point{}, draw.Src)

	scanner := rasterx.NewScannerGV(w, h, dst, dst.Bounds())
	dasher := rasterx.NewDasher(w, h, scanner)
	icon.Draw(dasher, 1.0)
	return dst, nil
}
