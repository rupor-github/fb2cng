package pdf

import (
	"bytes"
	"fmt"
	"image/color"
	"strconv"
	"strings"

	"github.com/srwiley/rasterx"
	"golang.org/x/image/math/fixed"

	"fbc/convert/pdf/docwriter"
	imgutil "fbc/utils/images"
)

// makePDFSVGForm uses a unit-square bounding box so the existing image placement
// matrix also places Forms correctly. The scene retains its SVG user coordinates.
func makePDFSVGForm(data []byte) (pdfImageResource, error) {
	scene, err := imgutil.ReadVectorSVG(data)
	if err != nil {
		return pdfImageResource{}, err
	}
	var stream bytes.Buffer
	states := docwriter.Dict{}
	normalize := rasterx.Matrix2D{
		A: 1 / scene.Width, D: -1 / scene.Height,
		E: -scene.X / scene.Width, F: 1 + scene.Y/scene.Height,
	}
	for i, path := range scene.Paths {
		if len(path.Path) == 0 {
			continue
		}
		stream.WriteString("q\n")
		matrix := normalize.Mult(path.Transform)
		fmt.Fprintf(&stream, "%s %s %s %s %s %s cm\n",
			pdfSVGNumber(matrix.A), pdfSVGNumber(matrix.B), pdfSVGNumber(matrix.C),
			pdfSVGNumber(matrix.D), pdfSVGNumber(matrix.E), pdfSVGNumber(matrix.F))
		fill := path.Fill != nil && path.FillOpacity > 0
		stroke := path.Stroke != nil && path.StrokeWidth > 0 && path.StrokeOpacity > 0
		if path.FillOpacity != 1 || path.StrokeOpacity != 1 {
			name := fmt.Sprintf("GS%d", i+1)
			states[name] = docwriter.Dict{
				"Type": docwriter.Name("ExtGState"),
				"ca":   docwriter.Number(path.FillOpacity),
				"CA":   docwriter.Number(path.StrokeOpacity),
			}
			fmt.Fprintf(&stream, "/%s gs\n", name)
		}
		if fill {
			writePDFSVGColor(&stream, *path.Fill, "rg")
		}
		if stroke {
			writePDFSVGColor(&stream, *path.Stroke, "RG")
			fmt.Fprintf(&stream, "%s w\n%d J\n%d j\n%s M\n",
				pdfSVGNumber(path.StrokeWidth), path.Cap, path.Join, pdfSVGNumber(path.MiterLimit))
			stream.WriteByte('[')
			for _, dash := range path.Dash {
				fmt.Fprintf(&stream, "%s ", pdfSVGNumber(dash))
			}
			fmt.Fprintf(&stream, "] %s d\n", pdfSVGNumber(path.DashOffset))
		}
		writePDFSVGPath(&stream, path.Path)
		switch {
		case fill && stroke && path.EvenOdd:
			stream.WriteString("B*\n")
		case fill && stroke:
			stream.WriteString("B\n")
		case fill && path.EvenOdd:
			stream.WriteString("f*\n")
		case fill:
			stream.WriteString("f\n")
		case stroke:
			stream.WriteString("S\n")
		default:
			stream.WriteString("n\n")
		}
		stream.WriteString("Q\n")
	}
	resources := docwriter.Dict{}
	if len(states) > 0 {
		resources["ExtGState"] = states
	}
	return pdfImageResource{
		Dict: docwriter.Dict{
			"Type":      docwriter.Name("XObject"),
			"Subtype":   docwriter.Name("Form"),
			"FormType":  docwriter.Integer(1),
			"BBox":      docwriter.Array{docwriter.Integer(0), docwriter.Integer(0), docwriter.Integer(1), docwriter.Integer(1)},
			"Resources": resources,
		},
		Data: stream.Bytes(),
	}, nil
}

func writePDFSVGColor(stream *bytes.Buffer, paint color.NRGBA, operator string) {
	fmt.Fprintf(stream, "%s %s %s %s\n",
		pdfSVGNumber(float64(paint.R)/255), pdfSVGNumber(float64(paint.G)/255), pdfSVGNumber(float64(paint.B)/255), operator)
}

// PDF numbers cannot contain exponents. Four decimal places (the document's
// usual formatter) are insufficient for unit-square normalization of large SVGs.
func pdfSVGNumber(value float64) string {
	text := strconv.FormatFloat(value, 'f', 12, 64)
	text = strings.TrimRight(strings.TrimRight(text, "0"), ".")
	if text == "-0" {
		return "0"
	}
	return text
}

func writePDFSVGPath(stream *bytes.Buffer, path rasterx.Path) {
	var x, y, startX, startY float64
	for i := 0; i < len(path); {
		command := rasterx.PathCommand(path[i])
		i++
		number := func(offset int) float64 { return float64(path[i+offset]) / float64(fixed.I(1)) }
		switch command {
		case rasterx.PathMoveTo:
			x, y = number(0), number(1)
			startX, startY = x, y
			fmt.Fprintf(stream, "%s %s m\n", pdfSVGNumber(x), pdfSVGNumber(y))
			i += 2
		case rasterx.PathLineTo:
			x, y = number(0), number(1)
			fmt.Fprintf(stream, "%s %s l\n", pdfSVGNumber(x), pdfSVGNumber(y))
			i += 2
		case rasterx.PathQuadTo:
			cx, cy, endX, endY := number(0), number(1), number(2), number(3)
			fmt.Fprintf(stream, "%s %s %s %s %s %s c\n",
				pdfSVGNumber(x+2*(cx-x)/3), pdfSVGNumber(y+2*(cy-y)/3),
				pdfSVGNumber(endX+2*(cx-endX)/3), pdfSVGNumber(endY+2*(cy-endY)/3), pdfSVGNumber(endX), pdfSVGNumber(endY))
			x, y = endX, endY
			i += 4
		case rasterx.PathCubicTo:
			fmt.Fprintf(stream, "%s %s %s %s %s %s c\n",
				pdfSVGNumber(number(0)), pdfSVGNumber(number(1)), pdfSVGNumber(number(2)),
				pdfSVGNumber(number(3)), pdfSVGNumber(number(4)), pdfSVGNumber(number(5)))
			x, y = number(4), number(5)
			i += 6
		case rasterx.PathClose:
			stream.WriteString("h\n")
			x, y = startX, startY
		}
	}
}
