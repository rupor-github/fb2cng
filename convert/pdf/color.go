package pdf

import (
	"fmt"
	"strconv"
	"strings"

	"fbc/convert/pdf/docwriter"
	"fbc/css"
)

type pdfColor struct {
	R float64
	G float64
	B float64
}

func (c pdfColor) contentOperator() string {
	return docwriter.FormatNumber(c.clampedR()) + " " + docwriter.FormatNumber(c.clampedG()) + " " + docwriter.FormatNumber(c.clampedB()) + " rg"
}

func (c pdfColor) strokeOperator() string {
	return docwriter.FormatNumber(c.clampedR()) + " " + docwriter.FormatNumber(c.clampedG()) + " " + docwriter.FormatNumber(c.clampedB()) + " RG"
}

func (c pdfColor) String() string {
	return fmt.Sprintf("#%02x%02x%02x", int(c.clampedR()*255+0.5), int(c.clampedG()*255+0.5), int(c.clampedB()*255+0.5))
}

func (c pdfColor) clampedR() float64 { return min(max(c.R, 0), 1) }

func (c pdfColor) clampedG() float64 { return min(max(c.G, 0), 1) }

func (c pdfColor) clampedB() float64 { return min(max(c.B, 0), 1) }

func pdfCSSColor(value css.Value) (pdfColor, bool) {
	raw := strings.TrimSpace(formatCSSValue(value))
	if raw == "" {
		return pdfColor{}, false
	}
	lower := strings.ToLower(raw)
	if lower == "currentcolor" || lower == "inherit" || lower == "transparent" {
		return pdfColor{}, false
	}
	if color, ok := pdfHexColor(lower); ok {
		return color, true
	}
	if color, ok := pdfNamedColor(lower); ok {
		return color, true
	}
	if color, ok := pdfRGBFunctionColor(lower); ok {
		return color, true
	}
	return pdfColor{}, false
}

func pdfHexColor(value string) (pdfColor, bool) {
	value = strings.TrimPrefix(value, "#")
	switch len(value) {
	case 3:
		r, okR := parseHexByte(strings.Repeat(value[0:1], 2))
		g, okG := parseHexByte(strings.Repeat(value[1:2], 2))
		b, okB := parseHexByte(strings.Repeat(value[2:3], 2))
		return rgbBytesColor(r, g, b), okR && okG && okB
	case 6:
		r, okR := parseHexByte(value[0:2])
		g, okG := parseHexByte(value[2:4])
		b, okB := parseHexByte(value[4:6])
		return rgbBytesColor(r, g, b), okR && okG && okB
	default:
		return pdfColor{}, false
	}
}

func parseHexByte(value string) (int64, bool) {
	parsed, err := strconv.ParseInt(value, 16, 16)
	return parsed, err == nil
}

func rgbBytesColor(r, g, b int64) pdfColor {
	return pdfColor{R: float64(r) / 255, G: float64(g) / 255, B: float64(b) / 255}
}

func pdfNamedColor(name string) (pdfColor, bool) {
	switch name {
	case "black":
		return pdfColor{}, true
	case "white":
		return pdfColor{R: 1, G: 1, B: 1}, true
	case "red":
		return pdfColor{R: 1}, true
	case "green":
		return pdfColor{G: 0.5}, true
	case "blue":
		return pdfColor{B: 1}, true
	case "gray", "grey":
		return pdfColor{R: 0.5, G: 0.5, B: 0.5}, true
	default:
		return pdfColor{}, false
	}
}

func pdfRGBFunctionColor(value string) (pdfColor, bool) {
	if !strings.HasPrefix(value, "rgb(") || !strings.HasSuffix(value, ")") {
		return pdfColor{}, false
	}
	inner := strings.TrimSuffix(strings.TrimPrefix(value, "rgb("), ")")
	parts := strings.Split(inner, ",")
	if len(parts) != 3 {
		return pdfColor{}, false
	}
	components := [3]float64{}
	for i, part := range parts {
		component, ok := pdfRGBComponent(strings.TrimSpace(part))
		if !ok {
			return pdfColor{}, false
		}
		components[i] = component
	}
	return pdfColor{R: components[0], G: components[1], B: components[2]}, true
}

func pdfRGBComponent(value string) (float64, bool) {
	if before, ok := strings.CutSuffix(value, "%"); ok {
		parsed, err := strconv.ParseFloat(before, 64)
		if err != nil {
			return 0, false
		}
		return min(max(parsed/100, 0), 1), true
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0, false
	}
	return min(max(parsed/255, 0), 1), true
}
