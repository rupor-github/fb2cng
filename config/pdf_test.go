package config

import (
	"math"
	"os"
	"path/filepath"
	"testing"
)

func TestPDFPageFrameConfiguration(t *testing.T) {
	for _, test := range []struct {
		name, yaml, want string
		points           float64
		invalid          bool
	}{
		{name: "default", yaml: "document: {}", want: "0"},
		{name: "disabled", yaml: "document:\n  pdf:\n    page_frame: 0", want: "0"},
		{name: "points", yaml: "document:\n  pdf:\n    page_frame: 0.5pt", want: "0.5pt", points: 0.5},
		{name: "pixels", yaml: "document:\n  pdf:\n    page_frame: 1px", want: "1px", points: 0.24},
		{
			name: "pixels at custom DPI", want: "1px", points: 0.48,
			yaml: "document:\n  pdf:\n    page_frame: 1px\n  images:\n    screen:\n      dpi: 150",
		},
		{name: "zero points", yaml: "document:\n  pdf:\n    page_frame: 0pt", want: "0pt"},
		{name: "zero pixels", yaml: "document:\n  pdf:\n    page_frame: 0px", want: "0px"},
		{name: "negative", yaml: "document:\n  pdf:\n    page_frame: -1pt", invalid: true},
		{name: "missing unit", yaml: "document:\n  pdf:\n    page_frame: 0.5", invalid: true},
		{name: "unsupported unit", yaml: "document:\n  pdf:\n    page_frame: 1em", invalid: true},
		{name: "empty", yaml: "document:\n  pdf:\n    page_frame: ''", invalid: true},
		{name: "whitespace", yaml: "document:\n  pdf:\n    page_frame: ' '", invalid: true},
		{name: "NaN", yaml: "document:\n  pdf:\n    page_frame: NaNpt", invalid: true},
		{name: "infinity", yaml: "document:\n  pdf:\n    page_frame: Infpx", invalid: true},
		{name: "overflow", yaml: "document:\n  pdf:\n    page_frame: 1e309pt", invalid: true},
		{name: "missing number", yaml: "document:\n  pdf:\n    page_frame: pt", invalid: true},
		{name: "too wide points", yaml: "document:\n  pdf:\n    page_frame: 303.36pt", invalid: true},
		{name: "too wide pixels", yaml: "document:\n  pdf:\n    page_frame: 1264px", invalid: true},
		{name: "unknown field", yaml: "document:\n  pdf:\n    frame: 1pt", invalid: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.yaml")
			if err := os.WriteFile(path, []byte(test.yaml+"\n"), 0600); err != nil {
				t.Fatal(err)
			}
			cfg, err := LoadConfiguration(path)
			if test.invalid {
				if err == nil {
					t.Fatal("accepted invalid configuration")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if cfg.Document.PDF.PageFrame != test.want {
				t.Fatalf("page_frame = %q, want %q", cfg.Document.PDF.PageFrame, test.want)
			}
			points, err := cfg.Document.PDF.PageFramePoints(cfg.Document.Images.Screen)
			if err != nil || math.Abs(points-test.points) > 1e-9 {
				t.Fatalf("PageFramePoints() = %v, %v; want %v", points, err, test.points)
			}
			dumped, err := Dump(cfg)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, dumped, 0600); err != nil {
				t.Fatal(err)
			}
			roundTrip, err := LoadConfiguration(path)
			if err != nil {
				t.Fatal(err)
			}
			if roundTrip.Document.PDF != cfg.Document.PDF {
				t.Fatalf("round trip changed PDF config: %#v", roundTrip.Document.PDF)
			}
		})
	}
}

func TestPDFPageFramePointsZeroValue(t *testing.T) {
	points, err := (PDFConfig{}).PageFramePoints(ScreenConfig{})
	if err != nil || points != 0 {
		t.Fatalf("zero-value PageFramePoints() = %v, %v; want disabled", points, err)
	}
}
