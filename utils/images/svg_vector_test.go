package images

import (
	"errors"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSVGVectorTransforms(t *testing.T) {
	for _, tt := range []struct {
		transform string
		x, y      float64
	}{
		{"translate(3)", 4, 2},
		{"scale(2)", 2, 4},
		{"scale(2,3)", 2, 6},
		{"rotate(90)", -2, 1},
		{"rotate(90 1 1)", 0, 1},
		{"skewX(45)", 3, 2},
		{"skewY(45)", 1, 3},
		{"matrix(1 2 3 4 5 6)", 12, 16},
	} {
		t.Run(tt.transform, func(t *testing.T) {
			matrix, err := parseSVGVectorTransform(tt.transform)
			if err != nil {
				t.Fatal(err)
			}
			x, y := matrix.Transform(1, 2)
			if math.Abs(x-tt.x) > 1e-9 || math.Abs(y-tt.y) > 1e-9 {
				t.Fatalf("transformed point = %v/%v, want %v/%v", x, y, tt.x, tt.y)
			}
		})
	}
}

func FuzzReadVectorSVG(f *testing.F) {
	for _, seed := range []string{
		`<svg viewBox="0 0 40 20"><path d="M1 2L3 4Z"/></svg>`,
		`<svg viewBox="0 0 40 20"><circle r="10"/></svg>`,
		`<svg viewBox="0 0 40 20"><path d="M1 2A3 4 0 0 1 5 6"/></svg>`,
		`<svg viewBox="0 0 40 20"><g transform="scale(2)"><rect width="10" height="10"/></g></svg>`,
	} {
		f.Add([]byte(seed))
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 16384 {
			t.Skip()
		}
		scene, err := ReadVectorSVG(data)
		if err != nil && scene != nil {
			t.Fatal("partial vector scene returned on failure")
		}
	})
}

func TestReadVectorSVG(t *testing.T) {
	for _, tt := range []struct {
		name, body string
		paths      int
	}{
		{"path", `<path d="M1 2h4v3z"/>`, 1},
		{"curves", `<path d="M1 2 Q3 4 5 6 T9 10 C11 12 13 14 15 16 S17 18 19 20"/>`, 1},
		{"arc", `<path d="M5 5 A10 10 0 0 1 20 20"/>`, 1},
		{"rect", `<rect width="10" height="20" rx="2"/>`, 1},
		{"circle", `<circle cx="10" cy="10" r="4"/>`, 1},
		{"ellipse", `<ellipse cx="10" cy="10" rx="4" ry="2"/>`, 1},
		{"line", `<line x1="1" y1="2" x2="10" y2="20"/>`, 1},
		{"polygon", `<polygon points="1,2 3,4 5,6"/>`, 1},
		{"two-point polyline", `<polyline points="1,2 3,4"/>`, 1},
		{"groups", `<g fill="red"><g stroke="blue"><path d="M1 2L3 4"/></g></g>`, 1},
		{"metadata", `<title>Title</title><desc>Description</desc><metadata><other/></metadata><path d="M1 2L3 4"/>`, 1},
		{"empty", ``, 0},
	} {
		t.Run(tt.name, func(t *testing.T) {
			scene, err := ReadVectorSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="10 20 40 80">` + tt.body + `</svg>`))
			if err != nil {
				t.Fatal(err)
			}
			if scene.X != 10 || scene.Y != 20 || scene.Width != 40 || scene.Height != 80 || len(scene.Paths) != tt.paths {
				t.Fatalf("unexpected scene: %+v", scene)
			}
			for _, path := range scene.Paths {
				if len(path.Path) == 0 {
					t.Fatal("geometry was silently dropped")
				}
			}
		})
	}
}

func TestReadVectorSVGStyleAndTransforms(t *testing.T) {
	scene, err := ReadVectorSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 40 20" fill="red">
<g transform="translate(10 5) scale(2)" stroke="blue" fill-opacity="0.5">
<path d="M1 2L3 4" fill="red" style="fill:#00ff00;stroke-width:2px;stroke-linecap:round;stroke-linejoin:bevel;
stroke-miterlimit:5;stroke-dasharray:1,2,3;stroke-dashoffset:-1;fill-rule:evenodd;stroke-opacity:25%"/>
<path d="M2 3L4 5" fill="none"/>
</g></svg>`))
	if err != nil {
		t.Fatal(err)
	}
	path := scene.Paths[0]
	x, y := path.Transform.Transform(1, 2)
	if x != 12 || y != 9 {
		t.Fatalf("transformed point = %v/%v, want 12/9", x, y)
	}
	if path.Fill.R != 0 || path.Fill.G != 255 || path.Stroke.B != 255 || path.FillOpacity != 0.5 || path.StrokeOpacity != 0.25 {
		t.Fatalf("unexpected paints: %+v", path.VectorSVGStyle)
	}
	if path.StrokeWidth != 2 || path.Cap != 1 || path.Join != 2 || path.MiterLimit != 5 || !path.EvenOdd ||
		len(path.Dash) != 3 || path.DashOffset != -1 {
		t.Fatalf("unexpected stroke: %+v", path.VectorSVGStyle)
	}
	if scene.Paths[1].Fill != nil || scene.Paths[1].StrokeWidth != 1 || scene.Paths[1].FillOpacity != 0.5 {
		t.Fatal("sibling style leaked or group inheritance was lost")
	}
}

func TestReadVectorSVGUnsupportedRejectsEntireScene(t *testing.T) {
	for _, tt := range []struct{ name, body string }{
		{"gradient", `<defs><linearGradient id="g"/></defs><path d="M1 2L3 4" fill="url(#g)"/>`},
		{"unresolved paint", `<path d="M1 2L3 4" fill="url(#missing)"/>`},
		{"text", `<text x="1" y="2">Text</text>`},
		{"image", `<image href="data:image/png;base64,AA=="/>`},
		{"clip", `<path d="M1 2L3 4" clip-path="url(#clip)"/>`},
		{"filter", `<path d="M1 2L3 4" filter="url(#f)"/>`},
		{"mask", `<path d="M1 2L3 4" mask="url(#m)"/>`},
		{"class", `<path class="art" d="M1 2L3 4"/>`},
		{"CSS", `<style>path{fill:red}</style>`},
		{"group opacity", `<g opacity="0.5"><path d="M1 2L3 4"/></g>`},
		{"element opacity", `<path d="M1 2L3 4" opacity="0.5"/>`},
		{"currentColor", `<path d="M1 2L3 4" fill="currentColor"/>`},
		{"vector effect", `<path d="M1 2L3 4" vector-effect="non-scaling-stroke"/>`},
		{"visibility", `<g visibility="hidden"><path d="M1 2L3 4"/></g>`},
		{"animation", `<animate attributeName="fill"/>`},
		{"nested viewport", `<svg viewBox="0 0 10 10"/>`},
		{"foreign element", `<path xmlns="urn:other" d="M1 2L3 4"/>`},
		{"foreign attribute", `<path xmlns:x="urn:other" x:fill="red" d="M1 2L3 4"/>`},
		{"negative radius", `<circle r="-1"/>`},
		{"percentage geometry", `<rect width="50%" height="20"/>`},
		{"excessive coordinate", `<path d="M1e30 2L3 4"/>`},
		{"invalid transform", `<g transform="scale(NaN)"/>`},
		{"singular transform", `<g transform="scale(0)"/>`},
		{"unknown property", `<path style="made-up:yes" d="M1 2L3 4"/>`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			data := []byte(`<svg viewBox="0 0 40 20"><path d="M0 0L1 1"/>` + tt.body + `</svg>`)
			scene, err := ReadVectorSVG(data)
			if !errors.Is(err, ErrUnsupportedSVG) || scene != nil {
				t.Fatalf("scene = %+v, error = %v; want whole-image fallback", scene, err)
			}
		})
	}
}

func TestReadVectorSVGInvalidDocument(t *testing.T) {
	for _, data := range []string{
		``, `<svg>`, `<svg viewBox="0 0 40 20"/><svg viewBox="0 0 40 20"/>`,
		`<svg viewBox="0 0 0 20"/>`, `<svg viewBox="0 0 NaN 20"/>`, `<svg viewBox="0 0 40 20"><g></svg>`,
		`<?xml-stylesheet href="art.css"?><svg viewBox="0 0 40 20"/>`,
	} {
		t.Run(data, func(t *testing.T) {
			if scene, err := ReadVectorSVG([]byte(data)); err == nil || scene != nil {
				t.Fatalf("invalid SVG accepted: %+v, %v", scene, err)
			}
		})
	}
}

func TestReadVectorSVGBuiltinVignettes(t *testing.T) {
	files, err := filepath.Glob("../../state/vignettes/*.svg")
	if err != nil || len(files) == 0 {
		t.Fatalf("find built-in SVGs: %v", err)
	}
	for _, file := range files {
		t.Run(filepath.Base(file), func(t *testing.T) {
			data, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			if scene, err := ReadVectorSVG(data); err != nil || len(scene.Paths) == 0 {
				t.Fatalf("built-in SVG is not supported: %v", err)
			}
		})
	}
}

func TestSVGImageSizeMatchesRasterization(t *testing.T) {
	for _, viewBox := range []string{"0 0 40 20", "10 20 40.1 20.5", "0 0 100000 200000", ""} {
		t.Run(viewBox, func(t *testing.T) {
			data := []byte(`<svg viewBox="` + viewBox + `"/>`)
			if viewBox == "" {
				data = []byte(`<svg/>`)
			}
			w, h, err := SVGImageSize(data, 0, 0)
			if err != nil {
				t.Fatal(err)
			}
			// Do not allocate an 8192x8192 buffer just to test dimension extraction.
			wantW, wantH := svgImageDimensions(40, 20, 0, 0)
			switch {
			case strings.Contains(viewBox, "40.1"):
				wantW, wantH = 41, 21
			case strings.Contains(viewBox, "100000"):
				wantW, wantH = 4096, 8192
			case viewBox == "":
				wantW, wantH = 2048, 2048
			}
			if w != wantW || h != wantH {
				t.Fatalf("dimensions = %dx%d, want %dx%d", w, h, wantW, wantH)
			}
		})
	}
}
