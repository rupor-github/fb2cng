package images

import (
	"bytes"
	"image"
	"image/color"
	"image/draw"
	"math"
	"regexp"
	"strconv"

	"github.com/srwiley/oksvg"
	"github.com/srwiley/rasterx"
)

const defaultSVGSize = 2048 // Default size to use when SVG viewBox has no size to match KP3

// KindleSVGStrokeWidthFactor is the multiplier for stroke-width values when
// rasterizing SVGs for Kindle. Kindle devices render at higher resolution,
// so strokes need to be thicker to remain visible.
const KindleSVGStrokeWidthFactor = 8.0

// strokeWidthRe matches stroke-width attributes and properties in SVG.
// Captures the numeric value for replacement.
var strokeWidthRe = regexp.MustCompile(`(stroke-width\s*[=:]\s*["']?)(\d+(?:\.\d+)?)(["']?)`)

// ScaleSVGStrokeWidth multiplies all stroke-width values in SVG data by the given factor.
// Returns the modified SVG data. If factor is <= 0 or 1, returns the original data unchanged.
func ScaleSVGStrokeWidth(svgData []byte, factor float64) []byte {
	if factor <= 0 || factor == 1.0 {
		return svgData
	}

	return strokeWidthRe.ReplaceAllFunc(svgData, func(match []byte) []byte {
		submatches := strokeWidthRe.FindSubmatch(match)
		if len(submatches) < 4 {
			return match
		}

		prefix := submatches[1]   // "stroke-width=" or "stroke-width:"
		valueStr := submatches[2] // numeric value
		suffix := submatches[3]   // closing quote if any

		value, err := strconv.ParseFloat(string(valueStr), 64)
		if err != nil {
			return match
		}

		newValue := value * factor
		// Format with minimal precision needed
		newValueStr := strconv.FormatFloat(newValue, 'f', -1, 64)

		return append(append(prefix, newValueStr...), suffix...)
	})
}

// RasterizeSVGToImage rasterizes SVG to an RGBA image.
//
// Rules:
//   - if targetW == 0 && targetH == 0: use SVG viewBox dimensions (fallback to 1024x1024)
//   - if only one of targetW/targetH is > 0: scale by that dimension keeping aspect ratio
//   - if both targetW and targetH are > 0: fit into that box keeping aspect ratio
//   - if strokeWidthFactor > 0 and != 1: multiply all stroke-width values before rasterizing
func RasterizeSVGToImage(svgData []byte, targetW, targetH int, strokeWidthFactor float64) (image.Image, error) {
	// Scale stroke widths if factor is specified
	if strokeWidthFactor > 0 && strokeWidthFactor != 1.0 {
		svgData = ScaleSVGStrokeWidth(svgData, strokeWidthFactor)
	}

	icon, err := oksvg.ReadIconStream(bytes.NewReader(svgData))
	if err != nil {
		return nil, err
	}

	intrW := int(math.Ceil(icon.ViewBox.W))
	intrH := int(math.Ceil(icon.ViewBox.H))
	if intrW <= 0 {
		intrW = defaultSVGSize
	}
	if intrH <= 0 {
		intrH = defaultSVGSize
	}

	w, h := intrW, intrH
	if targetW <= 0 && targetH <= 0 {
		// Keep intrinsic size.
	} else if targetW > 0 && targetH <= 0 {
		w = targetW
		h = int(math.Round(float64(w) * float64(intrH) / float64(intrW)))
	} else if targetH > 0 && targetW <= 0 {
		h = targetH
		w = int(math.Round(float64(h) * float64(intrW) / float64(intrH)))
	} else {
		scaleW := float64(targetW) / float64(intrW)
		scaleH := float64(targetH) / float64(intrH)
		scale := math.Min(scaleW, scaleH)
		w = int(math.Round(float64(intrW) * scale))
		h = int(math.Round(float64(intrH) * scale))
	}
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}

	icon.SetTarget(0, 0, float64(w), float64(h))

	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.Draw(dst, dst.Bounds(), &image.Uniform{C: color.RGBA{255, 255, 255, 255}}, image.Point{}, draw.Src)

	scanner := rasterx.NewScannerGV(w, h, dst, dst.Bounds())
	dasher := rasterx.NewDasher(w, h, scanner)
	icon.Draw(dasher, 1.0)
	return dst, nil
}
