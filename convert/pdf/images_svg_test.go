package pdf

import (
	"bytes"
	"context"
	"image"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest"
	"go.uber.org/zap/zaptest/observer"

	"fbc/common"
	"fbc/config"
	"fbc/content"
	"fbc/convert/pdf/docwriter"
	"fbc/fb2"
)

func TestPDFSVGForm(t *testing.T) {
	resource, err := makePDFSVGForm([]byte(`<svg viewBox="10 20 2048 1024">
<g transform="translate(10 20)"><path d="M0 0 Q10 20 20 0 C25 0 30 10 35 0 Z"
fill="red" fill-opacity="0.5" stroke="blue" stroke-width="2" stroke-linecap="round"
stroke-linejoin="bevel" stroke-dasharray="1 2" stroke-dashoffset="3" fill-rule="evenodd"/></g></svg>`))
	if err != nil {
		t.Fatal(err)
	}
	dict := docwriter.Format(resource.Dict)
	for _, want := range []string{
		"/Subtype /Form", "/BBox [0 0 1 1]", "/Resources", "/ExtGState", "/ca 0.5", "/CA 1",
	} {
		if !strings.Contains(dict, want) {
			t.Fatalf("Form dictionary %q is missing %q", dict, want)
		}
	}
	for _, want := range []string{
		"0.00048828125 0 0 -0.0009765625 0 1 cm", "1 0 0 rg", "0 0 1 RG", "/GS1 gs",
		"2 w", "1 J", "2 j", "[1 2 ] 3 d", "0 0 m", " c\n", "h\nB*\n",
	} {
		if !strings.Contains(string(resource.Data), want) {
			t.Fatalf("Form content %q is missing %q", resource.Data, want)
		}
	}
}

func TestPDFSVGResourcePolicy(t *testing.T) {
	for _, tt := range []struct {
		name, attributes, body string
		force, remove          bool
		wantSubtype            docwriter.Name
	}{
		{"vector default", "", `<path d="M1 2L10 15"/>`, false, false, "Form"},
		{"forced raster", "", `<path d="M1 2L10 15"/>`, true, false, "Image"},
		{"remove transparency", "", `<path d="M1 2L10 15"/>`, false, true, "Image"},
		{"unsupported clip", `clip-path="url(#clip)"`, `<path d="M1 2L10 15"/>`, false, false, "Image"},
		{"unsupported group opacity", "", `<g opacity="0.5"><path d="M1 2L10 15"/></g>`, false, false, "Image"},
		{"gradient fallback", "", `<defs><linearGradient id="g"><stop offset="0" stop-color="red"/>
<stop offset="1" stop-color="blue"/></linearGradient></defs><rect width="40" height="20" fill="url(#g)"/>`, false, false, "Image"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			img := &fb2.BookImage{MimeType: "image/svg+xml", Data: []byte(
				`<svg viewBox="0 0 40 20" ` + tt.attributes + `><path d="M0 0L1 1"/>` + tt.body + `</svg>`)}
			img.Filename = "images/art.svg"
			core, logs := observer.New(zapcore.DebugLevel)
			doc := pdfDocumentSpec{
				Images: fb2.BookImages{"svg": img}, Log: zap.New(core),
				ImageConfig: config.ImagesConfig{RasterizeSVG: tt.force, RemoveTransparency: tt.remove},
			}
			pages := []pdfPage{{Images: []pdfPageImage{{ImageID: "svg"}, {ImageID: "svg"}}}}
			nextID := 7
			resources, err := preparePDFImageResources(doc, pages, &nextID)
			if err != nil {
				t.Fatal(err)
			}
			if len(resources) != 1 || nextID != 8 || resources["svg"].Dict["Subtype"] != tt.wantSubtype {
				t.Fatalf("unexpected resources: %+v; next ID = %d", resources, nextID)
			}
			assignPDFImageResourceNames(pages, resources)
			if pages[0].Images[0].Name != "Im1" || pages[0].Images[1].Name != "Im1" {
				t.Fatal("repeated artwork must reuse one named resource")
			}
			entries := logs.All()
			if tt.wantSubtype == "Form" {
				if len(entries) != 0 {
					t.Fatalf("native vector output unexpectedly logged fallback: %+v", entries)
				}
				return
			}
			if len(entries) != 1 || entries[0].Level != zapcore.DebugLevel {
				t.Fatalf("want one debug message per resource, got %+v", entries)
			}
			fields := entries[0].ContextMap()
			if fields["id"] != "svg" || fields["filename"] != "images/art.svg" {
				t.Fatalf("missing resource identity: %+v", fields)
			}
			if tt.force || tt.remove {
				if entries[0].Message != "Rasterizing SVG as requested by configuration" ||
					fields["rasterize_svg"] != tt.force || fields["remove_transparency"] != tt.remove || fields["error"] != nil {
					t.Fatalf("unexpected configuration log: %+v", entries[0])
				}
			} else {
				reason, _ := fields["error"].(string)
				if entries[0].Message != "Falling back to SVG rasterization for PDF output" ||
					!strings.Contains(reason, "unsupported vector SVG") {
					t.Fatalf("missing fallback reason: %+v", entries[0])
				}
			}
		})
	}
}

func TestPDFSVGFallbackLoggingDisabled(t *testing.T) {
	core, logs := observer.New(zapcore.InfoLevel)
	img := &fb2.BookImage{MimeType: "image/svg+xml", Data: []byte(
		`<svg viewBox="0 0 40 20"><g opacity="0.5"><rect width="20" height="20"/></g></svg>`)}
	resource, err := makePDFImageResource(img, config.ImagesConfig{}, zap.New(core))
	if err != nil {
		t.Fatal(err)
	}
	if resource.Dict["Subtype"] != docwriter.Name("Image") || logs.Len() != 0 {
		t.Fatalf("fallback must not warn at normal verbosity: %+v", logs.All())
	}
}

func TestPDFBuiltinSVGResources(t *testing.T) {
	for _, tt := range []struct{ name, file string }{
		{"default cover", "../default-cover.svg"},
		{"missing image", "../../fb2/placeholders/not-found.svg"},
		{"broken image", "../../fb2/placeholders/broken.svg"},
		{"substituted broken image", ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			bo := fb2.BinaryObject{ID: "art", ContentType: "image/svg+xml"}
			if tt.file == "" {
				bo.ContentType, bo.Data = "image/png", []byte("invalid PNG")
			} else {
				var err error
				bo.Data, err = os.ReadFile(tt.file)
				if err != nil {
					t.Fatal(err)
				}
			}
			var width, height float64
			for _, policy := range []struct {
				name          string
				force, remove bool
				subtype       docwriter.Name
			}{
				{"native", false, false, "Form"},
				{"forced raster", true, false, "Image"},
				{"remove transparency", false, true, "Image"},
			} {
				t.Run(policy.name, func(t *testing.T) {
					cfg := &config.ImagesConfig{
						RasterizeSVG: policy.force, RemoveTransparency: policy.remove,
						Screen: config.ScreenConfig{Width: 300, Height: 400},
					}
					img := bo.PrepareImage(common.OutputFmtPdf, tt.name == "default cover", cfg, zaptest.NewLogger(t))
					doc := pdfDocumentSpec{ScreenDPI: 72, Images: fb2.BookImages{"art": img}, ImageConfig: *cfg}
					w, h := naturalPDFImageSize(doc, img)
					if width == 0 {
						width, height = w, h
					} else if w != width || h != height {
						t.Fatalf("policy changed layout dimensions from %gx%g to %gx%g", width, height, w, h)
					}
					core, logs := observer.New(zap.DebugLevel)
					doc.Log = zap.New(core)
					nextID := 1
					resources, err := preparePDFImageResources(doc, []pdfPage{{Images: []pdfPageImage{{ImageID: "art"}}}}, &nextID)
					if err != nil {
						t.Fatal(err)
					}
					if resources["art"].Dict["Subtype"] != policy.subtype {
						t.Fatalf("want %s resource, got %+v", policy.subtype, resources["art"].Dict)
					}
					if logs.FilterMessage("Falling back to SVG rasterization for PDF output").Len() != 0 {
						t.Fatal("built-in artwork must not require unsupported-feature fallback")
					}
				})
			}
		})
	}
}

func TestGeneratePDFSVGVectorAndRaster(t *testing.T) {
	for _, tt := range []struct {
		name          string
		force, remove bool
		want          string
	}{
		{"native", false, false, "/Subtype /Form"},
		{"fallback", false, false, "/Subtype /Image"},
		{"force raster", true, false, "/Subtype /Image"},
		{"remove alpha", false, true, "/Subtype /Image"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.DocumentConfig{Images: config.ImagesConfig{
				Screen: config.ScreenConfig{Width: 100, Height: 100, DPI: 72}, RasterizeSVG: tt.force, RemoveTransparency: tt.remove,
			}}
			c := &content.Content{SrcName: "book.fb2", CoverID: "svg", Book: &fb2.FictionBook{}, ImagesIndex: fb2.BookImages{"svg": {
				MimeType: "image/svg+xml", Data: []byte(`<svg viewBox="0 0 40 20"><rect width="20" height="20" fill="red"/></svg>`),
			}}}
			if tt.name == "fallback" {
				c.ImagesIndex["svg"].Data = []byte(`<svg viewBox="0 0 40 20"><g opacity="0.5"><rect width="20" height="20"/></g></svg>`)
			}
			file := filepath.Join(t.TempDir(), "book.pdf")
			core, logs := observer.New(zapcore.DebugLevel)
			if err := Generate(context.Background(), c, file, "", cfg, zap.New(core)); err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Contains(data, []byte(tt.want)) || !bytes.Contains(data, []byte("/Im1 Do")) {
				t.Fatalf("generated PDF is missing resource/placement %q", tt.want)
			}
			fallbacks := logs.FilterMessage("Falling back to SVG rasterization for PDF output").All()
			if tt.name == "fallback" {
				if len(fallbacks) != 1 || fallbacks[0].ContextMap()["id"] != "svg" {
					t.Fatalf("Generate did not forward logger to SVG fallback: %+v", fallbacks)
				}
			} else if len(fallbacks) != 0 {
				t.Fatalf("unexpected fallback log: %+v", fallbacks)
			}
		})
	}
}

func TestPDFSVGVectorVisualParity(t *testing.T) {
	renderer, err := exec.LookPath("pdftoppm")
	if err != nil {
		t.Skip("pdftoppm is required for vector/raster visual comparisons")
	}
	type visualCase struct {
		name, body, reference string
		maxMeanErrorPercent   uint64
	}
	tests := []visualCase{
		{"fill and opacity", `<rect x="20" y="10" width="120" height="60" fill="red" fill-opacity="0.5"/>`, "", 0},
		{"transformed strokes", `<g transform="translate(40 20) scale(1.2)"><path d="M0 0L60 40L100 0"
fill="none" stroke="blue" stroke-width="4" stroke-linecap="round" stroke-linejoin="round"/></g>`,
			// oksvg's single-argument scale uses a zero Y scale and does not scale
			// stroke width. Expand the reference geometry instead of reproducing it.
			`<path d="M40 20L112 68L160 20" fill="none" stroke="blue" stroke-width="4.8"
stroke-linecap="round" stroke-linejoin="round"/>`, 0},
		{"curves", `<path d="M10 60 Q60 0 110 60 C140 90 180 20 190 60" fill="none" stroke="black" stroke-width="3"/>`, "", 0},
		{"ellipse", `<ellipse cx="100" cy="50" rx="60" ry="30" fill="#00ff00"/>`, "", 0},
		{"rounded rectangle", `<rect x="20" y="10" width="120" height="60" rx="10" ry="10" fill="blue"/>`, "", 0},
		{"even odd", `<path d="M10 10H190V90H10Z M60 30H140V70H60Z" fill="red" fill-rule="evenodd"/>`,
			// oksvg ignores fill-rule; use four rectangles as the reference hole.
			`<g fill="red"><rect x="10" y="10" width="180" height="20"/><rect x="10" y="70" width="180" height="20"/>
<rect x="10" y="30" width="50" height="40"/><rect x="140" y="30" width="50" height="40"/></g>`, 0},
		{"dashed stroke", `<path d="M10 50H190" fill="none" stroke="black" stroke-width="4" stroke-dasharray="10 5"/>`, "", 0},
	}
	for i := range tests {
		tests[i].body = `<svg viewBox="0 0 200 100">` + tests[i].body + `</svg>`
		if tests[i].reference != "" {
			tests[i].reference = `<svg viewBox="0 0 200 100">` + tests[i].reference + `</svg>`
		}
	}
	for _, file := range []string{"../default-cover.svg", "../../fb2/placeholders/broken.svg", "../../fb2/placeholders/not-found.svg"} {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		tests = append(tests, visualCase{name: filepath.Base(file), body: string(data), maxMeanErrorPercent: 2})
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			img := &fb2.BookImage{MimeType: "image/svg+xml", Data: []byte(tt.body)}
			if _, err := makePDFSVGForm(img.Data); err != nil {
				t.Fatalf("visual test must exercise the vector path: %v", err)
			}
			width, height := pdfImagePixelSize(img)
			doc := pdfDocumentSpec{
				PageWidth: float64(width), PageHeight: float64(height), ScreenWidthPx: width, ScreenHeightPx: height, ScreenDPI: 72,
				Images: fb2.BookImages{"svg": img}, CoverID: "svg",
			}
			var rendered [2]image.Image
			for i := range rendered {
				doc.ImageConfig.RasterizeSVG = i == 1
				if i == 1 && tt.reference != "" {
					doc.Images = fb2.BookImages{"svg": {MimeType: "image/svg+xml", Data: []byte(tt.reference)}}
				}
				data, err := buildPDFDocument(doc)
				if err != nil {
					t.Fatal(err)
				}
				rendered[i] = renderPDFSVGTest(t, renderer, data)
			}
			bounds := rendered[0].Bounds()
			if bounds != rendered[1].Bounds() {
				t.Fatal("vector/raster page dimensions differ")
			}
			different := 0
			var totalDelta uint64
			for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
				for x := bounds.Min.X; x < bounds.Max.X; x++ {
					r1, g1, b1, _ := rendered[0].At(x, y).RGBA()
					r2, g2, b2, _ := rendered[1].At(x, y).RGBA()
					delta := func(a, b uint32) uint32 { return max(a, b) - min(a, b) }
					totalDelta += uint64(delta(r1, r2)) + uint64(delta(g1, g2)) + uint64(delta(b1, b2))
					if max(delta(r1, r2), delta(g1, g2), delta(b1, b2)) > 0x2000 {
						different++
					}
				}
			}
			// Dense artwork has many boundary pixels affected by resampling and
			// different antialiasers. Allow those differences only when the mean
			// absolute channel error stays below its explicit tolerance.
			pixels := bounds.Dx() * bounds.Dy()
			if different*100 > pixels*3 && totalDelta*100 > uint64(pixels)*3*0xffff*tt.maxMeanErrorPercent {
				t.Fatalf("%d pixels differ substantially between vector and raster output", different)
			}
		})
	}
}

func TestPDFSVGFormTransparency(t *testing.T) {
	renderer, err := exec.LookPath("pdftoppm")
	if err != nil {
		t.Skip("pdftoppm is required to verify SVG compositing")
	}
	form, err := makePDFSVGForm([]byte(`<svg viewBox="10 20 40 20">
<rect x="10" y="20" width="20" height="20" fill="red" fill-opacity="0.5"/></svg>`))
	if err != nil {
		t.Fatal(err)
	}
	writer := docwriter.NewWriter("1.4")
	objects := map[int]docwriter.Dict{
		1: {"Type": docwriter.Name("Catalog"), "Pages": docwriter.Ref{ObjectNumber: 2}},
		2: {"Type": docwriter.Name("Pages"), "Kids": docwriter.Array{docwriter.Ref{ObjectNumber: 3}}, "Count": docwriter.Integer(1)},
		3: {
			"Type": docwriter.Name("Page"), "Parent": docwriter.Ref{ObjectNumber: 2}, "Contents": docwriter.Ref{ObjectNumber: 4},
			"MediaBox":  docwriter.Array{docwriter.Integer(0), docwriter.Integer(0), docwriter.Integer(200), docwriter.Integer(100)},
			"Resources": docwriter.Dict{"XObject": docwriter.Dict{"Art": docwriter.Ref{ObjectNumber: 5}}},
		},
	}
	for id := 1; id <= 3; id++ {
		if err := writer.Object(id, objects[id]); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.StreamObject(4, nil, []byte("0 0 1 rg 0 0 200 100 re f\nq 200 0 0 100 0 0 cm /Art Do Q")); err != nil {
		t.Fatal(err)
	}
	if err := writer.StreamObject(5, form.Dict, form.Data); err != nil {
		t.Fatal(err)
	}
	data, err := writer.Finish(docwriter.Trailer{Root: docwriter.Ref{ObjectNumber: 1}})
	if err != nil {
		t.Fatal(err)
	}
	img := renderPDFSVGTest(t, renderer, data)
	r, g, b, _ := img.At(100, 100).RGBA()
	if r < 0x7e00 || r > 0x8100 || g != 0 || b < 0x7e00 || b > 0x8100 {
		t.Fatalf("red over blue = %d/%d/%d, want equal red/blue", r, g, b)
	}
	r, g, b, _ = img.At(300, 100).RGBA()
	if r != 0 || g != 0 || b != 0xffff {
		t.Fatalf("transparent background = %d/%d/%d, want untouched blue", r, g, b)
	}
}

func renderPDFSVGTest(t *testing.T, renderer string, data []byte) image.Image {
	t.Helper()
	base := filepath.Join(t.TempDir(), "render")
	if err := os.WriteFile(base+".pdf", data, 0600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.CommandContext(t.Context(), renderer, "-singlefile", "-scale-to", "400", "-png", base+".pdf", base)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("render PDF: %v: %s", err, output)
	}
	file, err := os.Open(base + ".png")
	if err != nil {
		t.Fatal(err)
	}
	img, err := png.Decode(file)
	if closeErr := file.Close(); closeErr != nil {
		t.Fatal(closeErr)
	}
	if err != nil {
		t.Fatal(err)
	}
	return img
}
