package kfx

import (
	"fmt"
	"image/color"
	"math"
	"strconv"

	"github.com/amazon-ion/ion-go/ion"
	"github.com/srwiley/rasterx"
	"go.uber.org/zap"
	"golang.org/x/image/math/fixed"

	"fbc/common"
	"fbc/config"
	"fbc/fb2"
	imgutil "fbc/utils/images"
)

// KVG is inline structured content, not an external SVG media resource. Keep its
// user-space viewport separate from the image's already-resolved layout box.
type kvgArtwork struct {
	Width, Height float64
	Shapes        []any
	Source        *fb2.BookImage
	ImageID       string
	Location      string
}

func compileKVGArtwork(data []byte) (*kvgArtwork, error) {
	scene, err := imgutil.ReadVectorSVG(data)
	if err != nil {
		return nil, err
	}
	if math.Round(scene.Width) <= 0 || math.Round(scene.Height) <= 0 {
		return nil, fmt.Errorf("%w: KVG viewport rounds to zero", imgutil.ErrUnsupportedSVG)
	}
	art := &kvgArtwork{Width: scene.Width, Height: scene.Height}
	for _, path := range scene.Paths {
		// The verified KVG shape representation has no fill-rule property.
		if path.EvenOdd && path.Fill != nil {
			return nil, fmt.Errorf("%w: KVG even-odd fill", imgutil.ErrUnsupportedSVG)
		}
		shape := NewStruct().SetSymbol(SymType, SymShape).
			SetSymbol(SymFillColor, SymNone).SetList(SymPath, kvgPath(path.Path))
		if path.Fill != nil && path.FillOpacity > 0 {
			// KVG treats a zero alpha byte on fill_color as opaque RGB, not clear.
			// Use the explicit none symbol and separate fill opacity instead.
			shape.SetInt(SymFillColor, kvgPaint(path.Fill, 1)).Set(SymFillOpacity, path.FillOpacity)
		}
		if path.Stroke != nil && path.StrokeWidth > 0 {
			shape.SetInt(SymStrokeColor, kvgPaint(path.Stroke, path.StrokeOpacity)).Set(SymStrokeWidth, path.StrokeWidth)
			// Native stroke fields were verified in KPV against PDF-rendered
			// references, independently of KFXInput's conservative symbol list.
			switch path.Cap {
			case 1:
				shape.SetSymbol(SymStrokeLinecap, SymRound)
			case 2:
				shape.SetSymbol(SymStrokeLinecap, SymSquare)
			}
			switch path.Join {
			case 0:
				if path.MiterLimit != 4 {
					shape.Set(SymStrokeMiterLimit, path.MiterLimit)
				}
			case 1:
				shape.SetSymbol(SymStrokeLinejoin, SymRound)
			case 2:
				shape.SetSymbol(SymStrokeLinejoin, SymBevel)
			}
			if len(path.Dash) > 0 {
				dash := make([]any, len(path.Dash))
				for i, value := range path.Dash {
					dash[i] = value
				}
				shape.SetList(SymStrokeDasharray, dash)
				if path.DashOffset != 0 {
					shape.Set(SymStrokeDashOffset, path.DashOffset)
				}
			}
		}
		m := path.Transform
		m.E, m.F = m.E-scene.X, m.F-scene.Y
		if (m.B != 0 || m.C != 0) &&
			(math.Abs(m.A-m.D) > 1e-9 || math.Abs(m.B+m.C) > 1e-9 || math.Abs(m.A*m.A+m.B*m.B-1) > 1e-9) {
			return nil, fmt.Errorf("%w: unverified KVG affine transform", imgutil.ErrUnsupportedSVG)
		}
		if m != rasterx.Identity {
			// Match AffineTransform.getMatrix() in Amazon's KVG serializer.
			shape.SetList(SymTransform, []any{m.A, m.B, m.C, m.D, m.E, m.F})
		}
		art.Shapes = append(art.Shapes, shape)
	}
	// KPV rejects Ion floats in shape data, including paths and stroke widths.
	// The SDK's Java double values do not imply Ion Float wire values. Normalize
	// all numeric shape fields here so future properties cannot bypass this rule.
	for _, shape := range art.Shapes {
		if _, err := kvgDecimalValues(shape); err != nil {
			return nil, fmt.Errorf("encode KVG shape numbers: %w", err)
		}
	}
	return art, nil
}

func kvgDecimalValues(value any) (any, error) {
	switch value := value.(type) {
	case float64:
		return ion.ParseDecimal(strconv.FormatFloat(value, 'f', -1, 64))
	case StructValue:
		for field, child := range value {
			converted, err := kvgDecimalValues(child)
			if err != nil {
				return nil, err
			}
			value[field] = converted
		}
	case []any:
		for i, child := range value {
			converted, err := kvgDecimalValues(child)
			if err != nil {
				return nil, err
			}
			value[i] = converted
		}
	}
	return value, nil
}

func kvgPaint(paint *color.NRGBA, opacity float64) int64 {
	if paint == nil {
		return 0
	}
	// KFX's packed alpha uses (byte+1)/256, with endpoint values reserved
	// for fully transparent/opaque colors, rather than the SVG byte/255 rule.
	alpha := uint32(min(max(int(math.Round(float64(paint.A)/255*opacity*256))-1, 0), 255))
	return int64(alpha<<24 | uint32(paint.R)<<16 | uint32(paint.G)<<8 | uint32(paint.B))
}

func kvgPath(path rasterx.Path) []any {
	var result []any
	for i := 0; i < len(path); {
		command := rasterx.PathCommand(path[i])
		i++
		var code, count int
		switch command {
		case rasterx.PathMoveTo:
			code, count = 0, 2
		case rasterx.PathLineTo:
			code, count = 1, 2
		case rasterx.PathQuadTo:
			code, count = 2, 4
		case rasterx.PathCubicTo:
			code, count = 3, 6
		case rasterx.PathClose:
			code = 4
		}
		// Opcodes and coordinates share a numeric vector; compileKVGArtwork
		// converts every value to the native reader's required Ion decimal type.
		result = append(result, float64(code))
		for j := range count {
			result = append(result, float64(path[i+j])/float64(fixed.I(1)))
		}
		i += count
	}
	return result
}

func rasterizeKindleSVG(id string, img *fb2.BookImage, cover bool, cfg *config.ImagesConfig, log *zap.Logger) *fb2.BookImage {
	bo := fb2.BinaryObject{ID: id, ContentType: "image/svg+xml", Data: img.Data, BuiltinVignette: img.BuiltinVignette}
	return bo.RasterizeSVG(common.OutputFmtKfx, cover, cfg, log)
}

// Lower artwork after layout/margin processing, but before position collection.
// Every image-emitting path (including mixed content and tables) then shares the
// same translation. Nonempty alt text keeps the image representation because the
// verified KVG schema has no equivalent accessibility field.
func lowerKVGArtwork(
	fragments *FragmentList, info imageResourceInfoByID, cfg *config.ImagesConfig, log *zap.Logger,
) ([]*Fragment, []*Fragment) {
	artByName := make(map[string]*kvgArtwork)
	for _, image := range info {
		if image.Vector != nil {
			artByName[image.ResourceName] = image.Vector
		}
	}
	var external, raw []*Fragment
	rasterized := make(map[string]bool)
	var visit func(any)
	visit = func(value any) {
		switch value := value.(type) {
		case StructValue:
			name, _ := value[SymResourceName].(SymbolByNameValue)
			if art := artByName[string(name)]; art != nil && value[SymType] == SymbolValue(SymImage) {
				if alt, _ := value[SymAltText].(string); alt != "" {
					if !rasterized[string(name)] {
						log.Debug("Falling back to SVG rasterization for Kindle output",
							zap.String("id", art.ImageID), zap.String("filename", art.Source.Filename), zap.String("reason", "KVG alt text"))
						img := rasterizeKindleSVG(art.ImageID, art.Source, false, cfg, log)
						format := imageFormatSymbol(img.MimeType)
						external = append(external, &Fragment{FType: SymExtResource, FIDName: string(name),
							Value: NewExternalResource(art.Location, format, formatSymbolToMIME(format),
								int64(img.Dim.Width), int64(img.Dim.Height)).Set(SymResourceName, name)})
						raw = append(raw, &Fragment{FType: SymRawMedia, FIDName: art.Location, Value: RawValue(img.Data)})
						rasterized[string(name)] = true
					}
					return
				}
				delete(value, SymResourceName)
				delete(value, SymAltText)
				value.SetSymbol(SymType, SymKVG).
					SetSymbol(SymKVGContentType, SymText).
					// These are viewport scalars, not CSS dimension properties. Native
					// readers do not accept {value, unit} structs here.
					SetInt(SymContainerWidth, int64(math.Round(art.Width))).
					SetInt(SymContainerHeight, int64(math.Round(art.Height))).SetList(SymShapeList, art.Shapes)
				return
			}
			for _, child := range value {
				visit(child)
			}
		case []any:
			for _, child := range value {
				visit(child)
			}
		}
	}
	for _, fragment := range fragments.All() {
		visit(fragment.Value)
	}
	return external, raw
}
