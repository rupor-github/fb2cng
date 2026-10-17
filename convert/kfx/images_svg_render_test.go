package kfx

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"go.uber.org/zap"

	"fbc/common"
	"fbc/config"
	"fbc/content"
	"fbc/convert/pdf"
	"fbc/fb2"
	"fbc/misc"
	"fbc/state"
	imgutil "fbc/utils/images"
)

// These probes deliberately bypass the production capability gate. A book that
// decodes successfully is not evidence of correct rendering: compare its native
// KVG image with the adjacent PDF-rendered reference in Kindle Previewer first.
// FBC_KVG_RENDER_PROBE_DIR must name an existing, empty output directory.
func TestKVGStrokeRenderProbes(t *testing.T) {
	dir := os.Getenv("FBC_KVG_RENDER_PROBE_DIR")
	if dir == "" {
		t.Skip("set FBC_KVG_RENDER_PROBE_DIR to generate native KVG/PDF-reference render probes")
	}
	renderer, err := exec.LookPath("pdftoppm")
	if err != nil {
		t.Fatal("pdftoppm is required for independent PDF-rendered references")
	}
	coverSVG, err := os.ReadFile("../default-cover.svg")
	if err != nil {
		t.Fatal(err)
	}
	cover, err := imgutil.RasterizeSVGToImage(coverSVG, 300, 400, 0)
	if err != nil {
		t.Fatal(err)
	}
	coverJPEG, err := imgutil.EncodeJPEGWithDPI(cover, 100, imgutil.DpiPxPerInch, 72, 72)
	if err != nil {
		t.Fatal(err)
	}
	for _, probe := range []struct {
		name, description, body string
		height                  int
	}{
		{"square-caps", "Rows: RED butt, BLUE square, GREEN round. Square ends extend beyond the endpoint guides; check diagonals too.", `
<path d="M100 20V220 M260 20V220 M360 20V220 M500 20V220" stroke="#cccccc" fill="none"/>
<path d="M100 60H260 M360 40L500 80" stroke="#cc0000" stroke-width="24" fill="none" stroke-linecap="butt"/>
<path d="M100 120H260 M360 100L500 140" stroke="#0044cc" stroke-width="24" fill="none" stroke-linecap="square"/>
<path d="M100 180H260 M360 160L500 200" stroke="#008844" stroke-width="24" fill="none" stroke-linecap="round"/>`, 240},
		{"bevel-joins", "Columns: RED miter, BLUE round, GREEN bevel. Compare the tips: pointed, curved, and flat-cut respectively.", `
<path d="M50 220L100 80L150 220" stroke="#cc0000" stroke-width="24" fill="none" stroke-linejoin="miter"/>
<path d="M250 220L300 80L350 220" stroke="#0044cc" stroke-width="24" fill="none" stroke-linejoin="round"/>
<path d="M450 220L500 80L550 220" stroke="#008844" stroke-width="24" fill="none" stroke-linejoin="bevel"/>`, 280},
		{"miter-limits", "Columns: RED limit 1, BLUE limit 4, GREEN limit 10. " +
			"Upper joins: blue/green pointed, red clipped. Lower joins: only green pointed.", `
<path d="M50 130L100 50L150 130 M80 360L100 220L120 360" stroke="#cc0000" stroke-width="20"
 fill="none" stroke-linejoin="miter" stroke-miterlimit="1"/>
<path d="M250 130L300 50L350 130 M280 360L300 220L320 360" stroke="#0044cc" stroke-width="20"
 fill="none" stroke-linejoin="miter" stroke-miterlimit="4"/>
<path d="M450 130L500 50L550 130 M480 360L500 220L520 360" stroke="#008844" stroke-width="20"
 fill="none" stroke-linejoin="miter" stroke-miterlimit="10"/>`, 400},
		{"dash-offsets", "Rows: RED offset 0, BLUE +10, GREEN -10, PURPLE +70. Blue and purple must match; green must shift in the opposite direction.", `
<path d="M80 20V280 M120 20V280 M140 20V280 M180 20V280 M200 20V280" stroke="#cccccc" fill="none"/>
<path d="M80 50H520" stroke="#cc0000" stroke-width="12" fill="none" stroke-dasharray="40 20" stroke-dashoffset="0"/>
<path d="M80 120H520" stroke="#0044cc" stroke-width="12" fill="none" stroke-dasharray="40 20" stroke-dashoffset="10"/>
<path d="M80 190H520" stroke="#008844" stroke-width="12" fill="none" stroke-dasharray="40 20" stroke-dashoffset="-10"/>
<path d="M80 260H520" stroke="#8800aa" stroke-width="12" fill="none" stroke-dasharray="40 20" stroke-dashoffset="70"/>`, 300},
	} {
		t.Run(probe.name, func(t *testing.T) {
			svg := []byte(fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 600 %d">
<rect width="600" height="%d" fill="white"/>%s</svg>`, probe.height, probe.height, probe.body))
			scene, err := imgutil.ReadVectorSVG(svg)
			if err != nil {
				t.Fatal(err)
			}
			// Keep probes usable after individual features graduate into production.
			production, err := compileKVGArtwork(svg)
			if err != nil && !errors.Is(err, imgutil.ErrUnsupportedSVG) {
				t.Fatalf("unexpected SVG compilation error: %v", err)
			}
			base := string(svg)
			for _, replacement := range [][2]string{
				{`stroke-linecap="square"`, `stroke-linecap="butt"`},
				{`stroke-linejoin="bevel"`, `stroke-linejoin="miter"`},
				{`stroke-miterlimit="1"`, `stroke-miterlimit="4"`},
				{`stroke-miterlimit="10"`, `stroke-miterlimit="4"`},
				{`stroke-dashoffset="10"`, `stroke-dashoffset="0"`},
				{`stroke-dashoffset="-10"`, `stroke-dashoffset="0"`},
				{`stroke-dashoffset="70"`, `stroke-dashoffset="0"`},
			} {
				base = strings.ReplaceAll(base, replacement[0], replacement[1])
			}
			writeKVGRenderProbe(t, filepath.Join(dir, probe.name+".svg"), svg)
			reference := renderKVGProbePDFReference(t, renderer, dir, probe.name, svg, probe.height)
			cfg, err := config.LoadConfiguration("")
			if err != nil {
				t.Fatal(err)
			}
			cfg.Document.Images.Optimize = false
			cfg.Document.Images.JPEGQuality = 100
			cfg.Document.Images.Screen.Width, cfg.Document.Images.Screen.Height = 600, 900
			ctx := state.ContextWithEnv(t.Context())
			env := state.EnvFromContext(ctx)
			env.Cfg, env.Log = cfg, zap.NewNop()
			env.DefaultStyle, err = os.ReadFile("../default.css")
			if err != nil {
				t.Fatal(err)
			}
			book := fmt.Sprintf(`<FictionBook xmlns="http://www.gribuser.ru/xml/fictionbook/2.0" xmlns:l="http://www.w3.org/1999/xlink">
<description><title-info><book-title>KVG render probe: %s</book-title><lang>en</lang>
<coverpage><image l:href="#cover"/></coverpage></title-info><document-info><id>render-probe-%s</id></document-info></description>
<body><section><title><p>%s</p></title><p>%s</p><p>NATIVE KVG - compare with the reference below.</p><image l:href="#native"/>
<p>PDF REFERENCE - this image was rendered from the original SVG by the PDF backend and Poppler.</p><image l:href="#reference"/>
<p>END OF PROBE - this text must remain visible.</p></section></body>
<binary id="cover" content-type="image/jpeg">%s</binary><binary id="native" content-type="image/svg+xml">%s</binary>
<binary id="reference" content-type="image/png">%s</binary></FictionBook>`,
				probe.name, probe.name, probe.name, probe.description,
				base64.StdEncoding.EncodeToString(coverJPEG), base64.StdEncoding.EncodeToString([]byte(base)),
				base64.StdEncoding.EncodeToString(reference))
			c, err := content.Prepare(ctx, strings.NewReader(book), probe.name+".fb2", common.OutputFmtAzw8, env.Log)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := os.RemoveAll(c.WorkDir); err != nil {
					t.Error(err)
				}
			})
			container := NewContainer()
			container.ContainerID = "CR!" + hashToAlphanumeric(c.Book.Description.DocumentInfo.ID, 28)
			container.GeneratorApp, container.GeneratorPkg = misc.GetAppName(), misc.GetVersion()
			if err := buildFragments(ctx, container, c, &cfg.Document, env.Log); err != nil {
				t.Fatal(err)
			}
			if len(container.Fragments.GetByType(SymRawMedia)) != 2 {
				t.Fatal("only the cover and PDF-reference image should be raster media")
			}
			// Inject only the feature under test, after normal layout and position
			// generation. Shapes and every new numeric property remain Ion decimals.
			count := 0
			var visit func(any)
			visit = func(value any) {
				switch value := value.(type) {
				case StructValue:
					if value[SymType] == SymbolValue(SymKVG) {
						shapes := value[SymShapeList].([]any)
						if len(shapes) != len(scene.Paths) {
							t.Fatal("probe geometry differs from the original SVG")
						}
						for i, item := range shapes {
							shape, style := item.(StructValue), scene.Paths[i].VectorSVGStyle
							if style.Cap == 2 {
								shape.SetSymbol(KFXSymbol(77), KFXSymbol(341)) // square
								count++
							}
							if style.Join == 2 {
								shape.SetSymbol(KFXSymbol(529), KFXSymbol(536)) // bevel
								count++
							}
							if style.MiterLimit != 4 {
								shape.Set(KFXSymbol(530), style.MiterLimit)
								count++
							}
							if style.DashOffset != 0 {
								shape.Set(KFXSymbol(532), style.DashOffset)
								count++
							}
							if _, err := kvgDecimalValues(shape); err != nil {
								t.Fatal(err)
							}
						}
						if production != nil && !reflect.DeepEqual(production.Shapes, shapes) {
							t.Fatal("production encoding differs from the independently injected render-test fields")
						}
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
			for _, fragment := range container.Fragments.All() {
				visit(fragment.Value)
			}
			if count == 0 {
				t.Fatal("probe contains no experimental native fields")
			}
			data, err := container.WriteContainer()
			if err != nil {
				t.Fatal(err)
			}
			if err := ValidateContainer(data); err != nil {
				t.Fatalf("invalid probe container: %v", err)
			}
			writeKVGRenderProbe(t, filepath.Join(dir, "KVG-probe-"+probe.name+".azw8"), data)
			t.Logf("wrote %s probe with %d experimental field(s) and a PDF-rendered visual reference", probe.name, count)
		})
	}
}

func renderKVGProbePDFReference(t *testing.T, renderer, dir, name string, svg []byte, height int) []byte {
	t.Helper()
	file := filepath.Join(dir, name+"-reference.pdf")
	if _, err := os.Stat(file); !os.IsNotExist(err) {
		t.Fatalf("reference path must not already exist: %s (%v)", file, err)
	}
	c := &content.Content{SrcName: name + ".svg", CoverID: "art", Book: &fb2.FictionBook{},
		ImagesIndex: fb2.BookImages{"art": {MimeType: "image/svg+xml", Data: svg}}}
	cfg := &config.DocumentConfig{Images: config.ImagesConfig{Screen: config.ScreenConfig{Width: 600, Height: height, DPI: 72}}}
	if err := pdf.Generate(t.Context(), c, file, "", cfg, zap.NewNop()); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(data, []byte("/Subtype /Form")) || bytes.Contains(data, []byte("/Subtype /Image")) {
		t.Fatal("reference must use native PDF vectors, not the SVG rasterizer")
	}
	base := filepath.Join(dir, name+"-reference")
	cmd := exec.CommandContext(t.Context(), renderer, "-f", "1", "-singlefile", "-scale-to", "600", "-png", file, base)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("render PDF reference: %v: %s", err, output)
	}
	data, err = os.ReadFile(base + ".png")
	if err != nil {
		t.Fatal(err)
	}
	dimensions, err := png.DecodeConfig(bytes.NewReader(data))
	if err != nil || dimensions.Width != 600 || dimensions.Height != height {
		t.Fatalf("reference image must match the native viewport: %+v, %v", dimensions, err)
	}
	return data
}

func writeKVGRenderProbe(t *testing.T, path string, data []byte) {
	t.Helper()
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		t.Fatal(err)
	}
	_, writeErr := file.Write(data)
	if err := errors.Join(writeErr, file.Close()); err != nil {
		t.Fatal(err)
	}
}
