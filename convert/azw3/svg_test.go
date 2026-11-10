package azw3

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/beevik/etree"
	"go.uber.org/zap"

	"fbc/convert/publication"
	"fbc/fb2"
)

func svgTestPublication(t *testing.T) *publication.Publication {
	t.Helper()
	pub, _ := resourceTestPublication(t)
	pub.Images["a"] = &fb2.BookImage{Filename: "images/art.svg", MimeType: "image/svg+xml", Data: []byte(fallbackSVG)}
	pub.Images["alias"] = &fb2.BookImage{Filename: "images/alias.svg", MimeType: "image/svg+xml", Data: []byte(fallbackSVG)}
	pub.Chapters[0].Doc.FindElement("//img").CreateAttr("src", "images/art.svg")
	pub.Stylesheet.Resources[0].MimeType = "image/svg+xml"
	pub.Stylesheet.Resources[0].Data = []byte(strings.ReplaceAll(strings.ReplaceAll(fallbackSVG, "black", "blue"),
		"</svg>", `<text x="0" y="10">Привет, café</text></svg>`))
	return pub
}

func compiledFlow(t *testing.T, compiled *textLayout, number uint32) []byte {
	t.Helper()
	if number >= compiled.FlowCount {
		t.Fatalf("missing flow %d of %d", number, compiled.FlowCount)
	}
	start := binary.BigEndian.Uint32(compiled.FDST[12+8*number:])
	end := binary.BigEndian.Uint32(compiled.FDST[16+8*number:])
	return compiled.Raw[start:end]
}

func TestCompileNativeSVGFlows(t *testing.T) {
	pub := svgTestPublication(t)
	before := snapshotPublication(t, pub)
	compiled, err := compileText(context.Background(), pub)
	if err != nil {
		t.Fatal(err)
	}
	if compiled.FlowCount != 4 || len(compiled.Resources) != 2 {
		t.Fatalf("got %d flows/%d embeds, want 4/2", compiled.FlowCount, len(compiled.Resources))
	}
	ref := compiled.References["images/art.svg"]
	if ref.Kind != resourceSVG || ref.Number != 2 || compiled.References["images/alias.svg"] != ref {
		t.Errorf("SVG flow namespace/deduplication is wrong: %+v", compiled.References)
	}
	if compiled.References["images/z.jpg"].Number != 2 || compiled.References["fonts/bookerly.ttf"].Number != 1 {
		t.Error("SVG flow allocation changed the independent embed namespace")
	}
	if !bytes.Equal(compiledFlow(t, compiled, 2), []byte(fallbackSVG)) ||
		!bytes.Equal(compiledFlow(t, compiled, 3), pub.Stylesheet.Resources[0].Data) {
		t.Error("self-contained SVG payloads changed during native compilation")
	}
	if !bytes.Contains(compiled.Documents[0].Data, []byte(`src="kindle:flow:0002?mime=image/svg+xml"`)) ||
		!bytes.Contains(compiled.Documents[0].Data, []byte(`url(kindle:flow:0003?mime=image/svg+xml)`)) ||
		!bytes.Contains(compiledFlow(t, compiled, 1), []byte(`url("kindle:flow:0003?mime=image/svg+xml")`)) {
		t.Error("SVG XHTML, inline CSS or stylesheet references were not lowered to flows")
	}
	styled := compiled.Positions[linkTarget{Filename: "chapter.xhtml", ID: "styled"}]
	if styled.Position != uint32(bytes.Index(compiled.Documents[0].Data, []byte(`<div id="styled"`))) {
		t.Error("SVG URI lowering invalidated a frozen reading position")
	}
	again, err := compileText(context.Background(), pub)
	if err != nil || !reflect.DeepEqual(compiled, again) {
		t.Error("native SVG compilation is not deterministic")
	}
	if !reflect.DeepEqual(before, snapshotPublication(t, pub)) {
		t.Error("SVG compilation changed source markup")
	}
}

func TestNativeSVGWithoutStylesheet(t *testing.T) {
	doc := etree.NewDocument()
	if err := doc.ReadFromString(`<html><body><img src="art.svg"/></body></html>`); err != nil {
		t.Fatal(err)
	}
	pub := &publication.Publication{
		Chapters: []publication.Chapter{{Filename: "book.xhtml", Doc: doc}},
		Images:   fb2.BookImages{"art": {Filename: "art.svg", MimeType: "image/svg+xml", Data: []byte(fallbackSVG)}},
	}
	compiled, err := compileText(context.Background(), pub)
	if err != nil {
		t.Fatal(err)
	}
	if compiled.FlowCount != 3 || string(compiledFlow(t, compiled, 1)) != "\n" || len(compiled.Resources) != 0 {
		t.Error("SVG needs a stable flow namespace even without a CSS link or embed records")
	}
}

func TestPrepareNativeSVGPolicy(t *testing.T) {
	for _, tt := range []struct {
		name         string
		force, alpha bool
		cover        bool
		native       bool
	}{
		{"native", false, false, false, true},
		{"forced raster", true, false, false, false},
		{"remove transparency", false, true, false, false},
		{"cover compatibility", false, false, true, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			pub := svgTestPublication(t)
			pub.Images["a"].BuiltinVignette = true
			cfg := imageTestConfig()
			cfg.Screen.Width = 120
			cfg.RasterizeSVG, cfg.RemoveTransparency = tt.force, tt.alpha
			if tt.cover {
				pub.CoverID = "a"
			}
			prepared, err := preparePublicationImages(context.Background(), pub, cfg, zap.NewNop())
			if err != nil {
				t.Fatal(err)
			}
			got := prepared.Images["a"]
			if tt.native {
				if got.MimeType != "image/svg+xml" || got.Dim.Width != 60 || got.Dim.Height != 30 {
					t.Fatal("native SVG lost its screen/scale policy")
				}
				doc := etree.NewDocument()
				if err := doc.ReadFromBytes(got.Data); err != nil {
					t.Fatal(err)
				}
				if doc.Root().SelectAttrValue("width", "") != "60" || doc.Root().SelectAttrValue("height", "") != "30" ||
					doc.Root().SelectAttrValue("viewBox", "") != "0 0 40 20" {
					t.Error("native dimensions must preserve vector geometry and CSS intrinsic sizing")
				}
				if doc.FindElement("//path").SelectAttrValue("stroke-width", "") != "1" {
					t.Error("native SVG must retain authored strokes, not bake in raster stroke compensation")
				}
			} else if got.MimeType != "image/jpeg" {
				t.Fatal("explicit raster/cover policy must still produce JPEG")
			}
			if !bytes.Equal(pub.Images["a"].Data, []byte(fallbackSVG)) {
				t.Error("native sizing mutated source SVG bytes")
			}
		})
	}
}

func TestNativeSVGBrokenSourceRetention(t *testing.T) {
	for _, force := range []bool{false, true} {
		t.Run(fmt.Sprintf("force=%t", force), func(t *testing.T) {
			pub := svgTestPublication(t)
			broken := []byte(`<svg xmlns="http://www.w3.org/2000/svg"><path></svg>`)
			pub.Images["a"].Data = broken
			cfg := imageTestConfig()
			cfg.UseBroken, cfg.RasterizeSVG = true, force
			prepared, err := preparePublicationImages(context.Background(), pub, cfg, zap.NewNop())
			if err != nil {
				t.Fatal(err)
			}
			compiled, err := compileText(context.Background(), prepared)
			if err != nil {
				t.Fatal(err)
			}
			ref := compiled.References["images/art.svg"]
			if ref.Kind != resourceSVG || !bytes.Equal(compiledFlow(t, compiled, ref.Number), broken) {
				t.Error("use_broken SVG was repaired, substituted or rejected instead of preserved")
			}
			if os.Getenv("AZW3_KINDLEUNPACK_TEST") == "1" {
				name := fmt.Sprintf("publication-broken-svg-force-%t", force)
				data := assembleTextProbe(t, compiled, nil, nil, "standard")
				writeProbeArtifacts(t, map[string][]byte{name + ".azw3": data})
				work := unpackProbe(t, name, data)
				filename := filepath.Join(work, "unpacked", "mobi8", "OEBPS", "Images", fmt.Sprintf("svgimg%04d.svg", ref.Number))
				got, err := os.ReadFile(filename)
				if err != nil || !bytes.Equal(got, broken) {
					t.Fatalf("independent decoding did not preserve the malformed SVG bytes: %v", err)
				}
			}
		})
	}
}

func TestSVGDependencyLowering(t *testing.T) {
	svg := []byte(`<svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink">` +
		`<defs><linearGradient id="paint"/></defs><style>.art { fill: url(#paint); background: URL (pixel.png); }</style>` +
		`<image xlink:href="pixel.png"/><use href="#paint"/><path fill="url(#paint)"/></svg>`)
	pub := &publication.Publication{
		Images: fb2.BookImages{
			"one": {Filename: "one/art.svg", MimeType: "image/svg+xml", Data: svg},
			"two": {Filename: "two/art.svg", MimeType: "image/svg+xml", Data: svg},
		},
		Stylesheet: publication.Stylesheet{Resources: []fb2.StylesheetResource{
			{Filename: "one/pixel.png", MimeType: "image/png", Data: []byte("first")},
			{Filename: "two/pixel.png", MimeType: "image/png", Data: []byte("second")},
		}},
	}
	resources, err := compileResources(context.Background(), pub)
	if err != nil {
		t.Fatal(err)
	}
	if len(resources.Flows) != 2 {
		t.Fatal("equal SVG bytes in different directories must not conflate relative dependencies")
	}
	for i, flow := range resources.Flows {
		uri := fmt.Sprintf("kindle:embed:%04d?mime=image/png", i+1)
		if !bytes.Contains(flow.Data, []byte(`xlink:href="`+uri+`"`)) ||
			!bytes.Contains(flow.Data, []byte(`background: url(`+uri+`)`)) ||
			!bytes.Contains(flow.Data, []byte(`href="#paint"`)) || !bytes.Contains(flow.Data, []byte(`fill="url(#paint)"`)) {
			t.Errorf("SVG dependencies/internal paint references changed incorrectly: %s", flow.Data)
		}
	}
	if !bytes.Equal(pub.Images["one"].Data, svg) {
		t.Error("dependency lowering changed the source SVG")
	}
	pub.Stylesheet.Resources = nil
	if _, err := compileResources(context.Background(), pub); err == nil || !strings.Contains(err.Error(), "unresolved publication resource") {
		t.Errorf("missing SVG dependency accepted: %v", err)
	}
}

func TestNativeSVGKindleUnpack(t *testing.T) {
	pub := svgTestPublication(t)
	compiled, err := compileText(context.Background(), pub)
	if err != nil {
		t.Fatal(err)
	}
	data := assembleTextProbe(t, compiled, nil, nil, "standard")
	writeProbeArtifacts(t, map[string][]byte{"publication-native-svg.azw3": data})
	work := unpackProbe(t, "publication-native-svg", data)
	base := filepath.Join(work, "unpacked", "mobi8", "OEBPS")
	for number, want := range map[int][]byte{2: pub.Images["a"].Data, 3: pub.Stylesheet.Resources[0].Data} {
		got, err := os.ReadFile(filepath.Join(base, "Images", fmt.Sprintf("svgimg%04d.svg", number)))
		if err != nil || !bytes.Equal(got, want) {
			t.Fatalf("SVG flow %d changed during independent reconstruction: %v", number, err)
		}
	}
	xhtml, err := os.ReadFile(filepath.Join(base, "Text", "part0000.xhtml"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(xhtml, []byte(`src="../Images/svgimg0002.svg"`)) ||
		!bytes.Contains(xhtml, []byte(`url(kindle:flow:0003?mime=image/svg+xml)`)) {
		t.Errorf("native SVG XHTML references were not reconstructed: %s", xhtml)
	}
	// KindleUnpack 0.83 does not lower SVG-flow URLs in inline style attributes.
	// KindleGen 2.9 emits the same unquoted URI there and exhibits the same decoder
	// omission. Keep checking the exact wire form; do not alter the decoder.
	css, err := os.ReadFile(filepath.Join(base, "Styles", "style0001.css"))
	if err != nil || !bytes.Contains(css, []byte(`svgimg0003.svg`)) {
		t.Fatalf("native SVG CSS reference was not reconstructed: %v\n%s", err, css)
	}
	rawFiles, err := filepath.Glob(filepath.Join(work, "unpacked", "mobi8", "*.rawml"))
	if err != nil || len(rawFiles) != 1 {
		t.Fatal("missing independently decoded rawML")
	}
	raw, err := os.ReadFile(rawFiles[0])
	if err != nil || !bytes.Equal(raw, compiled.Raw) {
		t.Fatal("independent decoding changed native SVG flows or their frozen text coordinates")
	}
}

func TestNativeSVGEmbeddedImageKindleUnpack(t *testing.T) {
	pub := svgTestPublication(t)
	pub.Images["a"].Data = []byte(`<svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" ` +
		`viewBox="0 0 40 20"><image xlink:href="z.jpg" width="40" height="20"/></svg>`)
	compiled, err := compileText(context.Background(), pub)
	if err != nil {
		t.Fatal(err)
	}
	data := assembleTextProbe(t, compiled, nil, nil, "standard")
	writeProbeArtifacts(t, map[string][]byte{"publication-svg-embedded-image.azw3": data})
	work := unpackProbe(t, "publication-svg-embedded-image", data)
	base := filepath.Join(work, "unpacked", "mobi8", "OEBPS")
	xhtml, err := os.ReadFile(filepath.Join(base, "Text", "part0000.xhtml"))
	if err != nil {
		t.Fatal(err)
	}
	doc := etree.NewDocument()
	if err := doc.ReadFromBytes(xhtml); err != nil {
		t.Fatal(err)
	}
	image := doc.FindElement("//svg/image")
	if image == nil {
		t.Fatalf("KindleUnpack did not reconstruct SVG with its embedded raster: %s", xhtml)
	}
	href := image.SelectAttrValue("xlink:href", "")
	if !strings.HasPrefix(href, "../Images/") {
		t.Fatalf("unresolved SVG raster link: %q", href)
	}
	payload, err := os.ReadFile(filepath.Join(base, "Text", filepath.FromSlash(href)))
	if err != nil || !bytes.Equal(payload, pub.Images["z"].Data) {
		t.Fatalf("SVG embedded raster bytes changed: %v", err)
	}
}
