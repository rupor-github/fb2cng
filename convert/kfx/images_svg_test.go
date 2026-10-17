package kfx

import (
	"encoding/base64"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/amazon-ion/ion-go/ion"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	"fbc/common"
	"fbc/config"
	"fbc/content"
	"fbc/fb2"
	"fbc/state"
	imgutil "fbc/utils/images"
)

func TestCompileKVGArtwork(t *testing.T) {
	art, err := compileKVGArtwork([]byte(`<svg viewBox="10 20 40 20"><g transform="translate(10 20) scale(2)">
<path d="M1 2 Q3 4 5 6 C7 8 9 10 11 12 Z" fill="red" fill-opacity="0.5"
stroke="blue" stroke-opacity="0.25" stroke-width="2" stroke-dasharray="1 2"/></g></svg>`))
	if err != nil {
		t.Fatal(err)
	}
	if art.Width != 40 || art.Height != 20 || len(art.Shapes) != 1 {
		t.Fatalf("unexpected artwork: %+v", art)
	}
	shape := art.Shapes[0].(StructValue)
	opacity, ok := shape[SymFillOpacity].(*ion.Decimal)
	if shape[SymFillColor] != int64(0xffff0000) || !ok || decimalToFloat64(opacity) != 0.5 ||
		shape[SymStrokeColor] != int64(0x3f0000ff) {
		t.Fatalf("incorrect straight-alpha paints: %+v", shape)
	}
	assertKVGDecimals(t, shape[SymPath], []float64{0, 1, 2, 2, 3, 4, 5, 6, 3, 7, 8, 9, 10, 11, 12, 4})
	assertKVGDecimals(t, shape[SymTransform], []float64{2, 0, 0, 2, 0, 0})
}

func assertKVGDecimals(t *testing.T, value any, want []float64) {
	t.Helper()
	vector, ok := value.([]any)
	if !ok || len(vector) != len(want) {
		t.Fatalf("incorrect KVG numeric vector: %v, want %v", value, want)
	}
	for i, number := range vector {
		decimal, ok := number.(*ion.Decimal)
		if !ok || decimalToFloat64(decimal) != want[i] {
			t.Fatalf("KVG vector[%d] = %T(%v), want Ion decimal %v", i, number, number, want[i])
		}
	}
}

func TestCompileKVGArtworkRejectsWholeScene(t *testing.T) {
	for _, body := range []string{
		`<rect width="10" height="10" fill-rule="evenodd"/>`,
		`<g opacity="0.5"><rect width="10" height="10"/></g>`,
		`<text>Unsupported</text>`,
		`<g transform="skewX(15)"><rect width="10" height="10"/></g>`,
	} {
		t.Run(body, func(t *testing.T) {
			art, err := compileKVGArtwork([]byte(`<svg viewBox="0 0 40 20"><path d="M0 0L1 1"/>` + body + `</svg>`))
			if art != nil || !errors.Is(err, imgutil.ErrUnsupportedSVG) {
				t.Fatalf("unsupported scene must not emit partial artwork: %+v, %v", art, err)
			}
		})
	}
}

func TestKVGExtendedStrokeEncoding(t *testing.T) {
	for _, tt := range []struct {
		name, attributes string
		field            KFXSymbol
		symbol           KFXSymbol
		number           float64
	}{
		{"square caps", `stroke-linecap="square"`, SymStrokeLinecap, SymSquare, 0},
		{"bevel joins", `stroke-linejoin="bevel"`, SymStrokeLinejoin, SymBevel, 0},
		{"low miter limit", `stroke-miterlimit="1"`, SymStrokeMiterLimit, 0, 1},
		{"high miter limit", `stroke-miterlimit="10"`, SymStrokeMiterLimit, 0, 10},
		{"fractional miter limit", `stroke-miterlimit="2.5"`, SymStrokeMiterLimit, 0, 2.5},
		{"positive dash offset", `stroke-dashoffset="10"`, SymStrokeDashOffset, 0, 10},
		{"negative dash offset", `stroke-dashoffset="-10"`, SymStrokeDashOffset, 0, -10},
		{"wrapped dash offset", `stroke-dashoffset="70"`, SymStrokeDashOffset, 0, 70},
		{"fractional dash offset", `stroke-dashoffset="-0.25"`, SymStrokeDashOffset, 0, -0.25},
	} {
		t.Run(tt.name, func(t *testing.T) {
			art, err := compileKVGArtwork([]byte(`<svg viewBox="0 0 40 20"><path d="M1 15L20 2L39 15"
fill="none" stroke="blue" stroke-width="2" stroke-dasharray="4 2" ` + tt.attributes + `/></svg>`))
			if err != nil {
				t.Fatal(err)
			}
			shape := art.Shapes[0].(StructValue)
			if tt.symbol != 0 {
				if shape[tt.field] != SymbolValue(tt.symbol) {
					t.Fatalf("stroke field %s = %v, want %s", tt.field, shape[tt.field], tt.symbol)
				}
			} else {
				decimal, ok := shape[tt.field].(*ion.Decimal)
				if !ok || decimalToFloat64(decimal) != tt.number {
					t.Fatalf("stroke field %s = %T(%v), want Ion decimal %v", tt.field, shape[tt.field], shape[tt.field], tt.number)
				}
			}
		})
	}
	art, err := compileKVGArtwork([]byte(`<svg viewBox="0 0 40 20"><path d="M1 2L30 15" stroke="black"
stroke-linejoin="round" stroke-miterlimit="10" stroke-dashoffset="5"/></svg>`))
	if err != nil {
		t.Fatal(err)
	}
	shape := art.Shapes[0].(StructValue)
	if shape[SymStrokeMiterLimit] != nil || shape[SymStrokeDashOffset] != nil {
		t.Fatal("miter limit on round joins and offset without dashes must not affect the representation")
	}
}

func TestKVGRejectsZeroRoundedViewport(t *testing.T) {
	art, err := compileKVGArtwork([]byte(`<svg viewBox="0 0 0.1 20"><path d="M0 0L1 1"/></svg>`))
	if art != nil || !errors.Is(err, imgutil.ErrUnsupportedSVG) {
		t.Fatalf("unrepresentable viewport must use raster fallback: %+v, %v", art, err)
	}
}

func TestKVGNoFill(t *testing.T) {
	for _, attributes := range []string{`fill="none"`, `fill="red" fill-opacity="0"`} {
		t.Run(attributes, func(t *testing.T) {
			art, err := compileKVGArtwork([]byte(`<svg viewBox="0 0 20 20"><path d="M0 0L10 10L20 0" ` +
				attributes + ` stroke="blue"/></svg>`))
			if err != nil {
				t.Fatal(err)
			}
			if art.Shapes[0].(StructValue)[SymFillColor] != SymbolValue(SymNone) {
				t.Fatal("transparent/no fill must not use a zero-alpha numeric fill (which KVG treats as opaque)")
			}
		})
	}
}

func TestKVGNativeTransformOrder(t *testing.T) {
	art, err := compileKVGArtwork([]byte(`<svg viewBox="0 0 40 20">
<path transform="matrix(0 1 -1 0 0 0)" d="M1 2L3 4"/></svg>`))
	if err != nil {
		t.Fatal(err)
	}
	// Amazon's SDK uses Java AffineTransform's [a,b,c,d,e,f] order. A
	// third-party SVG reconstruction swaps b/c and is not a native-wire oracle.
	assertKVGDecimals(t, art.Shapes[0].(StructValue)[SymTransform], []float64{0, 1, -1, 0, 0, 0})
}

func TestKVGNativeWireTypes(t *testing.T) {
	art, err := compileKVGArtwork([]byte(`<svg viewBox="0 0 40 20"><path d="M1 2Q3 4 5 6Z"
fill="red" fill-opacity="0.5" stroke="blue" stroke-width="1.2" stroke-dasharray="1 2"
stroke-linecap="round" stroke-linejoin="round" transform="translate(2 3)"/>
<path d="M1 15L20 2L39 15" fill="none" stroke="red" stroke-linecap="square"
stroke-miterlimit="10" stroke-dasharray="4 2" stroke-dashoffset="-3.5"/></svg>`))
	if err != nil {
		t.Fatal(err)
	}
	entry := NewContentEntry(ContentRef{EID: 7, Type: SymImage, ResourceName: "e1"})
	fragment := &Fragment{FType: SymStoryline, FIDName: "l1", Value: entry}
	fragments := NewFragmentList()
	if err := fragments.Add(fragment); err != nil {
		t.Fatal(err)
	}
	lowerKVGArtwork(fragments, imageResourceInfoByID{"art": {ResourceName: "e1", Vector: art}},
		&config.ImagesConfig{}, zap.NewNop())
	if entry[SymKVGContentType] != SymbolValue(SymText) {
		t.Fatal("KVG content must carry the text-content marker emitted by Amazon's SVG adapter")
	}
	shape := art.Shapes[0].(StructValue)
	if shape[SymStrokeLinecap] != SymbolValue(SymRound) || shape[SymStrokeLinejoin] != SymbolValue(SymRound) {
		t.Fatal("round caps/joins must use the KPV-tested native symbol fields")
	}
	// Test serialized Ion types, not just numerically equivalent Go values.
	// KFXInput accepts dimension structs for these viewport fields, but the
	// native KVG renderer expects scalar integers, as emitted by Amazon's SDK.
	data, err := NewContainer().serializeFragmentValue(fragment)
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := DecodeIon(GetIonProlog(), data, &decoded); err != nil {
		t.Fatal(err)
	}
	for field, want := range map[string]int{"$66": 40, "$67": 20} {
		if got, ok := decoded[field].(int); !ok || got != want {
			t.Fatalf("KVG %s must be scalar Ion int %d, got %T(%v)", field, want, decoded[field], decoded[field])
		}
	}
	shapes := decoded["$250"].([]any)
	path := shapes[0].(map[string]any)["$249"].([]any)
	for i, value := range path {
		if _, ok := value.(*ion.Decimal); !ok {
			t.Fatalf("KVG path[%d] must be Ion decimal (including opcodes), got %T", i, value)
		}
	}
	var rejectFloats func(any)
	rejectFloats = func(value any) {
		switch value := value.(type) {
		case float64, *float64:
			t.Fatalf("KVG shape contains an Ion float instead of a decimal: %v", value)
		case map[string]any:
			for _, child := range value {
				rejectFloats(child)
			}
		case []any:
			for _, child := range value {
				rejectFloats(child)
			}
		}
	}
	rejectFloats(shapes)
	extended := shapes[1].(map[string]any)
	for field, want := range map[string]float64{"$530": 10, "$532": -3.5} {
		decimal, ok := extended[field].(*ion.Decimal)
		if !ok || decimalToFloat64(decimal) != want {
			t.Fatalf("serialized KVG %s must be Ion decimal %v, got %T(%v)", field, want, extended[field], extended[field])
		}
	}
}

func TestCompileKVGBuiltinVignettes(t *testing.T) {
	files, err := filepath.Glob("../../state/vignettes/*.svg")
	if err != nil || len(files) != 8 {
		t.Fatalf("expected all eight built-in vignettes, got %v: %v", files, err)
	}
	for _, file := range files {
		t.Run(filepath.Base(file), func(t *testing.T) {
			data, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			art, err := compileKVGArtwork(data)
			if err != nil || len(art.Shapes) == 0 {
				t.Fatalf("built-in vignette requires raster fallback: %+v, %v", art, err)
			}
		})
	}
}

func TestKindlePlaceholderKVGResources(t *testing.T) {
	for _, input := range []struct{ name, file, mime string }{
		{"missing", "../../fb2/placeholders/not-found.svg", "image/svg+xml"},
		{"broken", "../../fb2/placeholders/broken.svg", "image/svg+xml"},
		{"invalid raster", "", "image/png"},
		{"invalid SVG", "", "image/svg+xml"},
	} {
		t.Run(input.name, func(t *testing.T) {
			data := []byte("invalid image")
			if input.name == "invalid SVG" {
				data = []byte(`<svg><path d="invalid"`)
			}
			if input.file != "" {
				var err error
				data, err = os.ReadFile(input.file)
				if err != nil {
					t.Fatal(err)
				}
			}
			for _, policy := range []struct {
				name                 string
				force, remove, cover bool
			}{
				{"native", false, false, false},
				{"forced raster", true, false, false},
				{"opaque", false, true, false},
				{"cover", false, false, true},
			} {
				t.Run(policy.name, func(t *testing.T) {
					cfg := &config.ImagesConfig{RasterizeSVG: policy.force, RemoveTransparency: policy.remove,
						JPEGQuality: 90, Screen: config.ScreenConfig{Width: 8000, Height: 1200}, ScaleFactor: 1}
					bo := fb2.BinaryObject{ID: "art", ContentType: input.mime, Data: data}
					core, logs := observer.New(zap.DebugLevel)
					log := zap.New(core)
					img := bo.PrepareImage(common.OutputFmtKfx, policy.cover, cfg, log)
					coverID := ""
					if policy.cover {
						coverID = "art"
					}
					external, raw, info := buildImageResourceFragments(fb2.BookImages{"art": img}, coverID, cfg, log)
					resource := info["art"]
					if resource.Width != 200 || resource.Height != 200 {
						t.Fatalf("placeholder layout dimensions changed: %+v", resource)
					}
					if policy.force || policy.remove || policy.cover {
						if resource.Vector != nil || len(raw) != 1 || len(external) != 1 ||
							external[0].Value.(StructValue)[SymFormat] != SymbolValue(SymFormatJPG) {
							t.Fatal("explicit raster/cover policy must keep JPEG placeholders")
						}
						return
					}
					if resource.Vector == nil || len(raw) != 0 || len(external) != 0 {
						t.Fatal("placeholder must use native KVG, not raw media")
					}
					caps, joins := 0, 0
					for _, item := range resource.Vector.Shapes {
						shape := item.(StructValue)
						if shape[SymStrokeLinecap] == SymbolValue(SymRound) {
							caps++
						}
						if shape[SymStrokeLinejoin] == SymbolValue(SymRound) {
							joins++
						}
					}
					if caps == 0 || (input.name != "missing" && joins == 0) {
						t.Fatal("native placeholder lost its authored round caps/joins")
					}
					if logs.FilterMessage("Falling back to SVG rasterization for Kindle output").Len() != 0 {
						t.Fatal("embedded placeholders must not require unsupported-feature fallback")
					}
				})
			}
		})
	}
}

func TestKindleSVGResourcePolicy(t *testing.T) {
	for _, rasterFormat := range []common.RasterFormat{common.RasterFormatAuto, common.RasterFormatJpeg} {
		t.Run(rasterFormat.String(), func(t *testing.T) {
			for _, tt := range []struct {
				name, body                 string
				force, remove, cover, want bool
			}{
				{"native", `<rect width="20" height="20"/>`, false, false, false, true},
				{"force raster", `<rect width="20" height="20"/>`, true, false, false, false},
				{"remove transparency", `<rect width="20" height="20"/>`, false, true, false, false},
				{"cover stays JPEG", `<rect width="20" height="20"/>`, false, false, true, false},
				{"unsupported opacity", `<g opacity="0.5"><rect width="20" height="20"/></g>`, false, false, false, false},
			} {
				t.Run(tt.name, func(t *testing.T) {
					cfg := &config.ImagesConfig{RasterizeSVG: tt.force, RemoveTransparency: tt.remove, JPEGQuality: 90,
						Screen: config.ScreenConfig{Width: 80, Height: 120}, ScaleFactor: 0.5, RasterFormat: rasterFormat}
					bo := fb2.BinaryObject{ID: "art", ContentType: "image/svg+xml", Data: []byte(`<svg viewBox="0 0 40 20">` + tt.body + `</svg>`)}
					core, logs := observer.New(zap.DebugLevel)
					log := zap.New(core)
					img := bo.PrepareImage(common.OutputFmtKfx, tt.cover, cfg, log)
					img.Filename = "images/art.svg"
					coverID := ""
					if tt.cover {
						coverID = "art"
					}
					external, raw, info := buildImageResourceFragments(fb2.BookImages{"art": img}, coverID, cfg, log)
					if (info["art"].Vector != nil) != tt.want {
						t.Fatalf("wrong representation: %+v", info["art"])
					}
					if !tt.cover && (info["art"].Width != 40 || info["art"].Height != 20) {
						t.Fatalf("representation changed logical dimensions: %+v", info["art"])
					}
					if tt.want {
						if len(external) != 0 || len(raw) != 0 {
							t.Fatal("native artwork must not create external media")
						}
					} else if len(raw) != 1 || external[0].Value.(StructValue)[SymFormat] != SymbolValue(SymFormatJPG) {
						t.Fatal("raster fallback must use the existing JPEG policy")
					}
					fallback := logs.FilterMessage("Falling back to SVG rasterization for Kindle output").All()
					if tt.name == "unsupported opacity" {
						if len(fallback) != 1 || fallback[0].ContextMap()["id"] != "art" || fallback[0].ContextMap()["error"] == nil {
							t.Fatalf("missing fallback identity/reason: %+v", fallback)
						}
					} else if len(fallback) != 0 {
						t.Fatal("configuration or cover policy must not be logged as unsupported artwork")
					}
					requested := logs.FilterMessage("Rasterizing SVG as requested by configuration").All()
					if tt.force || tt.remove {
						if len(requested) != 1 || requested[0].ContextMap()["rasterize_svg"] != tt.force ||
							requested[0].ContextMap()["remove_transparency"] != tt.remove {
							t.Fatalf("missing configuration rasterization reason: %+v", requested)
						}
					} else if len(requested) != 0 {
						t.Fatal("fallback must not claim that configuration requested rasterization")
					}
				})
			}
		})
	}
}

func TestBuildFragmentsKVGPositionsAndAltText(t *testing.T) {
	svg := `<svg viewBox="0 0 40 20"><rect width="20" height="20" fill="red"/></svg>`
	xml := `<FictionBook xmlns="http://www.gribuser.ru/xml/fictionbook/2.0" xmlns:l="http://www.w3.org/1999/xlink">
<description><title-info><book-title>KVG test</book-title><lang>en</lang></title-info>
<document-info><id>kvg-test</id></document-info></description><body><section><image l:href="#art"/>
<p>Before <image l:href="#art"/> after.</p><table><tr><td>Cell <image l:href="#art"/></td></tr></table>
<image l:href="#art" alt="Accessible artwork"/><image l:href="#art" alt="Repeated alt"/></section></body>
<binary id="art" content-type="image/svg+xml">` + base64.StdEncoding.EncodeToString([]byte(svg)) + `</binary></FictionBook>`
	var baseline any
	for _, force := range []bool{false, true} {
		name := "native"
		if force {
			name = "raster"
		}
		t.Run(name, func(t *testing.T) {
			cfg, err := config.LoadConfiguration("")
			if err != nil {
				t.Fatal(err)
			}
			cfg.Document.Images.RasterizeSVG = force
			cfg.Document.Images.Screen.Width = 80
			ctx := state.ContextWithEnv(t.Context())
			env := state.EnvFromContext(ctx)
			env.Cfg, env.Log = cfg, zap.NewNop()
			c, err := content.Prepare(ctx, strings.NewReader(xml), "book.fb2", common.OutputFmtKfx, env.Log)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := os.RemoveAll(c.WorkDir); err != nil {
					t.Error(err)
				}
			})
			container := NewContainer()
			if err := buildFragments(ctx, container, c, &cfg.Document, env.Log); err != nil {
				t.Fatal(err)
			}
			if len(container.Fragments.GetByType(SymRawMedia)) != 1 {
				t.Fatal("alt text must reuse one raster resource while other uses remain vector")
			}
			var native int
			var visit func(any)
			visit = func(value any) {
				switch value := value.(type) {
				case StructValue:
					if value[SymType] == SymbolValue(SymKVG) {
						native++
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
			for _, fragment := range container.Fragments.All() {
				visit(fragment.Value)
			}
			if (!force && native != 3) || (force && native != 0) {
				t.Fatalf("unexpected KVG entry count: %d", native)
			}
			positions := container.Fragments.GetByType(SymPositionIdMap)[0].Value
			if !force {
				baseline = positions
			} else if !reflect.DeepEqual(baseline, positions) {
				t.Fatal("KVG changed mixed/table/block position accounting")
			}
			if _, err := container.WriteContainer(); err != nil {
				t.Fatalf("serialize KVG content: %v", err)
			}
		})
	}
}
