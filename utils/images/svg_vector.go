package images

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"image/color"
	"io"
	"math"
	"regexp"
	"strconv"
	"strings"

	"github.com/srwiley/oksvg"
	"github.com/srwiley/rasterx"
	"golang.org/x/image/colornames"
)

// ErrUnsupportedSVG means the entire artwork must use a raster fallback. Never
// emit the partially collected paths when ReadVectorSVG returns an error.
var ErrUnsupportedSVG = errors.New("unsupported vector SVG")

// VectorSVG is a backend-independent scene in SVG user coordinates. It deliberately
// represents only features that can be translated without dropping artwork.
type VectorSVG struct {
	X, Y, Width, Height float64
	Paths               []VectorSVGPath
}

type VectorSVGPath struct {
	Path      rasterx.Path
	Transform rasterx.Matrix2D
	VectorSVGStyle
}

// VectorSVGStyle uses straight-alpha colors; nil paint means no fill/stroke.
// Cap and Join use the SVG/PDF order: butt/round/square and miter/round/bevel.
type VectorSVGStyle struct {
	Fill, Stroke               *color.NRGBA
	FillOpacity, StrokeOpacity float64
	StrokeWidth, MiterLimit    float64
	Cap, Join                  int
	EvenOdd                    bool
	Dash                       []float64
	DashOffset                 float64
}

type svgVectorState struct {
	tag       string
	transform rasterx.Matrix2D
	style     VectorSVGStyle
}

var svgVectorNumber = regexp.MustCompile(`[+-]?(?:[0-9]+\.?[0-9]*|\.[0-9]+)(?:[eE][+-]?[0-9]+)?`)

// ReadVectorSVG accepts paths, basic shapes, groups, solid paints, fill/stroke
// opacity and affine transforms. CSS rules, references, clipping, gradients,
// text, images and group/element opacity require whole-image raster fallback.
func ReadVectorSVG(data []byte) (*VectorSVG, error) {
	decoder := xml.NewDecoder(bytes.NewReader(data))
	scene := new(VectorSVG)
	var stack []svgVectorState
	seenRoot := false
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("parse SVG XML: %w", err)
		}
		switch token := token.(type) {
		case xml.StartElement:
			if len(stack) >= 256 || len(scene.Paths) >= 10000 {
				return nil, fmt.Errorf("%w: scene complexity limit", ErrUnsupportedSVG)
			}
			if token.Name.Space != "" && token.Name.Space != "http://www.w3.org/2000/svg" {
				return nil, fmt.Errorf("%w: foreign element namespace", ErrUnsupportedSVG)
			}
			state := svgVectorState{tag: token.Name.Local, transform: rasterx.Identity, style: VectorSVGStyle{
				Fill: &color.NRGBA{A: 255}, FillOpacity: 1, StrokeOpacity: 1, StrokeWidth: 1, MiterLimit: 4,
			}}
			if len(stack) == 0 {
				if seenRoot || state.tag != "svg" {
					return nil, fmt.Errorf("%w: expected one SVG root", ErrUnsupportedSVG)
				}
				seenRoot = true
			} else {
				parent := stack[len(stack)-1]
				if parent.tag != "svg" && parent.tag != "g" {
					return nil, fmt.Errorf("%w: nested shape content", ErrUnsupportedSVG)
				}
				state.style, state.transform = parent.style, parent.transform
				if state.tag == "title" || state.tag == "desc" || state.tag == "metadata" {
					if err := decoder.Skip(); err != nil {
						return nil, fmt.Errorf("skip SVG metadata: %w", err)
					}
					continue
				}
				if state.tag == "svg" {
					return nil, fmt.Errorf("%w: nested SVG viewport", ErrUnsupportedSVG)
				}
			}
			geometry, err := readSVGVectorAttributes(token, &state, scene)
			if err != nil {
				return nil, err
			}
			if state.tag != "svg" && state.tag != "g" {
				path, err := compileSVGVectorShape(state.tag, geometry)
				if err != nil {
					return nil, err
				}
				scene.Paths = append(scene.Paths, VectorSVGPath{Path: path, Transform: state.transform, VectorSVGStyle: state.style})
			}
			stack = append(stack, state)
		case xml.EndElement:
			if len(stack) == 0 {
				return nil, errors.New("unbalanced SVG elements")
			}
			stack = stack[:len(stack)-1]
		case xml.CharData:
			if strings.TrimSpace(string(token)) != "" {
				return nil, fmt.Errorf("%w: text content", ErrUnsupportedSVG)
			}
		case xml.Directive:
			return nil, fmt.Errorf("%w: XML directive", ErrUnsupportedSVG)
		case xml.ProcInst:
			if token.Target != "xml" {
				return nil, fmt.Errorf("%w: processing instruction", ErrUnsupportedSVG)
			}
		}
	}
	if !seenRoot || len(stack) != 0 || scene.Width <= 0 || scene.Height <= 0 {
		return nil, fmt.Errorf("%w: missing positive viewBox", ErrUnsupportedSVG)
	}
	return scene, nil
}

func readSVGVectorAttributes(element xml.StartElement, state *svgVectorState, scene *VectorSVG) (map[string]string, error) {
	geometry := make(map[string]string)
	var inlineStyle string
	for _, attr := range element.Attr {
		key, value := attr.Name.Local, strings.TrimSpace(attr.Value)
		if key == "xmlns" || attr.Name.Space == "xmlns" ||
			(attr.Name.Space == "http://www.w3.org/XML/1998/namespace" && (key == "space" || key == "lang")) {
			continue
		}
		if attr.Name.Space != "" {
			return nil, fmt.Errorf("%w: namespaced attribute %s", ErrUnsupportedSVG, key)
		}
		switch key {
		case "id", "version":
		case "style":
			inlineStyle = value
		case "transform":
			matrix, err := parseSVGVectorTransform(value)
			if err != nil {
				return nil, err
			}
			state.transform = state.transform.Mult(matrix)
			if !validSVGVectorMatrix(state.transform) {
				return nil, fmt.Errorf("%w: excessive inherited transform", ErrUnsupportedSVG)
			}
		case "viewBox":
			values, err := svgVectorNumbers(value)
			if state.tag != "svg" || err != nil || len(values) != 4 || values[2] < 1e-6 || values[3] < 1e-6 {
				return nil, fmt.Errorf("%w: invalid viewBox", ErrUnsupportedSVG)
			}
			scene.X, scene.Y, scene.Width, scene.Height = values[0], values[1], values[2], values[3]
		case "preserveAspectRatio":
			if state.tag != "svg" || (value != "xMidYMid meet" && value != "xMidYMid" && value != "none") {
				return nil, fmt.Errorf("%w: preserveAspectRatio", ErrUnsupportedSVG)
			}
		default:
			if svgVectorGeometryAttribute(state.tag, key) {
				geometry[key] = value
			} else if err := setSVGVectorStyle(&state.style, key, value); err != nil {
				return nil, err
			}
		}
	}
	// Inline CSS takes precedence over presentation attributes regardless of XML order.
	for declaration := range strings.SplitSeq(inlineStyle, ";") {
		if strings.TrimSpace(declaration) == "" {
			continue
		}
		key, value, ok := strings.Cut(declaration, ":")
		if !ok {
			return nil, fmt.Errorf("%w: invalid inline style", ErrUnsupportedSVG)
		}
		if err := setSVGVectorStyle(&state.style, strings.TrimSpace(key), strings.TrimSpace(value)); err != nil {
			return nil, err
		}
	}
	return geometry, nil
}

func svgVectorGeometryAttribute(tag, key string) bool {
	var keys string
	switch tag {
	case "svg":
		keys = " width height " // Layout follows viewBox, as in the existing rasterizer.
	case "path":
		keys = " d "
	case "rect":
		keys = " x y width height rx ry "
	case "circle":
		keys = " cx cy r "
	case "ellipse":
		keys = " cx cy rx ry "
	case "line":
		keys = " x1 y1 x2 y2 "
	case "polygon", "polyline":
		keys = " points "
	}
	return strings.Contains(keys, " "+key+" ")
}

func setSVGVectorStyle(style *VectorSVGStyle, key, value string) error {
	var err error
	switch key {
	case "fill":
		style.Fill, err = parseSVGVectorPaint(value)
	case "stroke":
		style.Stroke, err = parseSVGVectorPaint(value)
	case "fill-opacity", "stroke-opacity":
		var opacity float64
		opacity, err = svgVectorNumberValue(strings.TrimSuffix(value, "%"))
		if strings.HasSuffix(value, "%") {
			opacity /= 100
		}
		opacity = min(max(opacity, 0), 1)
		if key == "fill-opacity" {
			style.FillOpacity = opacity
		} else {
			style.StrokeOpacity = opacity
		}
	case "stroke-width", "stroke-miterlimit", "stroke-dashoffset":
		var number float64
		number, err = svgVectorNumberValue(strings.TrimSuffix(value, "px"))
		switch key {
		case "stroke-width":
			style.StrokeWidth = number
			if number < 0 {
				err = errors.New("negative stroke width")
			}
		case "stroke-miterlimit":
			style.MiterLimit = number
			if number < 1 {
				err = errors.New("invalid miter limit")
			}
		case "stroke-dashoffset":
			style.DashOffset = number
		}
	case "stroke-linecap":
		switch value {
		case "butt":
			style.Cap = 0
		case "round":
			style.Cap = 1
		case "square":
			style.Cap = 2
		default:
			err = errors.New("unsupported line cap")
		}
	case "stroke-linejoin":
		switch value {
		case "miter":
			style.Join = 0
		case "round":
			style.Join = 1
		case "bevel":
			style.Join = 2
		default:
			err = errors.New("unsupported line join")
		}
	case "fill-rule":
		if value != "evenodd" && value != "nonzero" {
			err = errors.New("unsupported fill rule")
		}
		style.EvenOdd = value == "evenodd"
	case "stroke-dasharray":
		style.Dash = nil
		if value != "none" {
			style.Dash, err = svgVectorNumbers(value)
			positive := false
			for _, dash := range style.Dash {
				positive = positive || dash > 0
				if dash < 0 {
					err = errors.New("negative dash length")
				}
			}
			if !positive {
				style.Dash = nil
			}
		}
	case "opacity":
		// Applying element/group opacity per paint is incorrect for overlapping
		// children or fill+stroke; defer until compositing groups are implemented.
		var opacity float64
		opacity, err = svgVectorNumberValue(value)
		if opacity != 1 {
			err = errors.New("element/group opacity requires compositing")
		}
	default:
		err = errors.New("unsupported property")
	}
	if err != nil {
		return fmt.Errorf("%w: %s=%q: %v", ErrUnsupportedSVG, key, value, err)
	}
	return nil
}

func parseSVGVectorPaint(value string) (*color.NRGBA, error) {
	if value == "none" {
		return nil, nil
	}
	if paint, ok := colornames.Map[strings.ToLower(value)]; ok {
		return &color.NRGBA{R: paint.R, G: paint.G, B: paint.B, A: paint.A}, nil
	}
	// Keep the parser conservative; notably oksvg's color parser substitutes black
	// for url() paints, which must never be mistaken for supported vector artwork.
	if !strings.HasPrefix(value, "#") {
		return nil, errors.New("paint is not a named or hexadecimal solid color")
	}
	r, g, b, err := oksvg.ParseSVGColorNum(value)
	if err != nil {
		return nil, err
	}
	return &color.NRGBA{R: r, G: g, B: b, A: 255}, nil
}

func svgVectorNumberValue(value string) (float64, error) {
	number, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0, err
	}
	if math.IsNaN(number) || math.IsInf(number, 0) || math.Abs(number) > 1e6 {
		return 0, errors.New("non-finite or excessive SVG coordinate")
	}
	return number, nil
}

func svgVectorNumbers(value string) ([]float64, error) {
	var numbers []float64
	for value = strings.TrimSpace(value); value != ""; value = strings.TrimLeft(value, " \t\r\n,") {
		match := svgVectorNumber.FindStringIndex(value)
		if match == nil || match[0] != 0 {
			return nil, errors.New("invalid SVG number list")
		}
		number, err := svgVectorNumberValue(value[:match[1]])
		if err != nil {
			return nil, err
		}
		numbers = append(numbers, number)
		value = value[match[1]:]
	}
	return numbers, nil
}

func parseSVGVectorTransform(value string) (rasterx.Matrix2D, error) {
	matrix := rasterx.Identity
	for value = strings.TrimSpace(value); value != ""; value = strings.TrimLeft(value, " \t\r\n,") {
		name, rest, ok := strings.Cut(value, "(")
		if !ok {
			return matrix, fmt.Errorf("%w: invalid transform", ErrUnsupportedSVG)
		}
		args, rest, ok := strings.Cut(rest, ")")
		numbers, err := svgVectorNumbers(args)
		if !ok || err != nil {
			return matrix, fmt.Errorf("%w: invalid transform arguments", ErrUnsupportedSVG)
		}
		local := rasterx.Identity
		switch strings.TrimSpace(name) {
		case "matrix":
			if len(numbers) != 6 {
				return matrix, fmt.Errorf("%w: matrix arguments", ErrUnsupportedSVG)
			}
			local = rasterx.Matrix2D{A: numbers[0], B: numbers[1], C: numbers[2], D: numbers[3], E: numbers[4], F: numbers[5]}
		case "translate", "scale":
			if len(numbers) < 1 || len(numbers) > 2 {
				return matrix, fmt.Errorf("%w: translate/scale arguments", ErrUnsupportedSVG)
			}
			y := 0.0
			if strings.TrimSpace(name) == "scale" {
				y = numbers[0]
			}
			if len(numbers) == 2 {
				y = numbers[1]
			}
			if strings.TrimSpace(name) == "scale" {
				local = local.Scale(numbers[0], y)
			} else {
				local = local.Translate(numbers[0], y)
			}
		case "rotate":
			if len(numbers) != 1 && len(numbers) != 3 {
				return matrix, fmt.Errorf("%w: rotate arguments", ErrUnsupportedSVG)
			}
			if len(numbers) == 3 {
				local = local.Translate(numbers[1], numbers[2])
			}
			local = local.Rotate(numbers[0] * math.Pi / 180)
			if len(numbers) == 3 {
				local = local.Translate(-numbers[1], -numbers[2])
			}
		case "skewX", "skewY":
			if len(numbers) != 1 {
				return matrix, fmt.Errorf("%w: skew arguments", ErrUnsupportedSVG)
			}
			if strings.TrimSpace(name) == "skewX" {
				local = local.SkewX(numbers[0] * math.Pi / 180)
			} else {
				local = local.SkewY(numbers[0] * math.Pi / 180)
			}
		default:
			return matrix, fmt.Errorf("%w: transform %q", ErrUnsupportedSVG, name)
		}
		matrix = matrix.Mult(local)
		if !validSVGVectorMatrix(matrix) {
			return matrix, fmt.Errorf("%w: singular or excessive transform", ErrUnsupportedSVG)
		}
		value = rest
	}
	return matrix, nil
}

func validSVGVectorMatrix(matrix rasterx.Matrix2D) bool {
	for _, value := range []float64{matrix.A, matrix.B, matrix.C, matrix.D, matrix.E, matrix.F} {
		if math.IsNaN(value) || math.IsInf(value, 0) || math.Abs(value) > 1e6 {
			return false
		}
	}
	return math.Abs(matrix.A*matrix.D-matrix.B*matrix.C) >= 1e-12
}

func compileSVGVectorShape(tag string, geometry map[string]string) (rasterx.Path, error) {
	switch tag {
	case "path", "rect", "circle", "ellipse", "line", "polygon", "polyline":
	default:
		return nil, fmt.Errorf("%w: element %s", ErrUnsupportedSVG, tag)
	}
	for key, value := range geometry {
		if key == "d" || key == "points" {
			continue
		}
		number, err := svgVectorNumberValue(strings.TrimSuffix(value, "px"))
		if err != nil || (number < 0 && (key == "width" || key == "height" || key == "rx" || key == "ry" || key == "r")) {
			return nil, fmt.Errorf("%w: invalid geometry %s", ErrUnsupportedSVG, key)
		}
		geometry[key] = strconv.FormatFloat(number, 'f', -1, 64)
	}
	if tag == "rect" {
		// SVG copies a sole rounded-corner radius to the other axis. oksvg's
		// geometry helper does not, so make that inheritance explicit.
		if rx, ok := geometry["rx"]; ok {
			if _, ok := geometry["ry"]; !ok {
				geometry["ry"] = rx
			}
		} else if ry, ok := geometry["ry"]; ok {
			geometry["rx"] = ry
		}
	}
	if tag == "polygon" || tag == "polyline" {
		points, err := svgVectorNumbers(geometry["points"])
		if err != nil || len(points)%2 != 0 {
			return nil, fmt.Errorf("%w: invalid polygon points", ErrUnsupportedSVG)
		}
		var path strings.Builder
		for i := 0; i < len(points); i += 2 {
			command := "L"
			if i == 0 {
				command = "M"
			}
			fmt.Fprintf(&path, "%s%s %s ", command,
				strconv.FormatFloat(points[i], 'f', -1, 64), strconv.FormatFloat(points[i+1], 'f', -1, 64))
		}
		if tag == "polygon" && len(points) > 0 {
			path.WriteByte('Z')
		}
		tag, geometry = "path", map[string]string{"d": path.String()}
	}
	// Compile isolated geometry with the existing path parser. Presentation and
	// transforms are handled above rather than relying on oksvg's private styles.
	var buf bytes.Buffer
	encoder := xml.NewEncoder(&buf)
	root := xml.StartElement{Name: xml.Name{Local: "svg"}}
	shape := xml.StartElement{Name: xml.Name{Local: tag}}
	for key, value := range geometry {
		if key != "d" && key != "points" {
			if _, err := svgVectorNumberValue(strings.TrimSuffix(value, "px")); err != nil {
				return nil, fmt.Errorf("%w: geometry %s: %v", ErrUnsupportedSVG, key, err)
			}
		} else {
			for _, number := range svgVectorNumber.FindAllString(value, -1) {
				if _, err := svgVectorNumberValue(number); err != nil {
					return nil, fmt.Errorf("%w: path coordinate: %v", ErrUnsupportedSVG, err)
				}
			}
		}
		shape.Attr = append(shape.Attr, xml.Attr{Name: xml.Name{Local: key}, Value: value})
	}
	for _, token := range []xml.Token{root, shape, shape.End(), root.End()} {
		if err := encoder.EncodeToken(token); err != nil {
			return nil, fmt.Errorf("encode SVG geometry: %w", err)
		}
	}
	if err := encoder.Flush(); err != nil {
		return nil, fmt.Errorf("flush SVG geometry: %w", err)
	}
	icon, err := oksvg.ReadIconStream(&buf, oksvg.StrictErrorMode)
	if err != nil {
		return nil, fmt.Errorf("%w: compile %s: %v", ErrUnsupportedSVG, tag, err)
	}
	var path rasterx.Path
	for _, shape := range icon.SVGPaths {
		path = append(path, shape.Path...)
	}
	return path, nil
}
