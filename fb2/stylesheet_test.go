package fb2

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.uber.org/zap/zaptest"
)

func TestParseStylesheetResources(t *testing.T) {
	tests := []struct {
		name string
		css  string
		want int
	}{
		{
			name: "font-face with url",
			css: `@font-face {
				font-family: 'MyFont';
				src: url('#font-id') format('woff2');
			}`,
			want: 1,
		},
		{
			name: "multiple url variations",
			css: `p { background: url(#bg-img); }
			      a { background: url('#bg-img2'); }
			      div { background: url("#bg-img3"); }`,
			want: 3,
		},
		{
			name: "import statement",
			css:  `@import url("other.css");`,
			want: 1,
		},
		{
			name: "data url",
			css:  `@font-face { font-family: 'Test'; src: url('data:font/woff2;base64,ABC'); }`,
			want: 1,
		},
		{
			name: "url with spaces",
			css:  `@font-face { font-family: 'Test'; src: url( '#font-id' ); }`,
			want: 1,
		},
		{
			name: "no urls",
			css:  `body { color: red; font-size: 12pt; }`,
			want: 0,
		},
		{
			name: "duplicate urls",
			css: `
				@font-face { font-family: 'Test'; src: url('#font-id'); }
				p { background: url('#font-id'); }
			`,
			want: 1, // Should deduplicate
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			refs := parseStylesheetResources(tt.css)
			if len(refs) != tt.want {
				t.Errorf("got %d refs, want %d", len(refs), tt.want)
			}
		})
	}
}

func TestSanitizeResourceFilename(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want string
	}{
		{
			name: "fragment id",
			url:  "#font-id",
			want: "font-id",
		},
		{
			name: "simple filename",
			url:  "myfont.woff2",
			want: "myfont.woff2",
		},
		{
			name: "path with filename",
			url:  "fonts/myfont.woff2",
			want: "myfont.woff2",
		},
		{
			name: "path traversal",
			url:  "../../../etc/passwd",
			want: "passwd",
		},
		{
			name: "absolute path",
			url:  "/usr/share/fonts/font.ttf",
			want: "font.ttf",
		},
		{
			name: "empty",
			url:  "",
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitizeResourceFilename(tt.url)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestMimeToExtension(t *testing.T) {
	tests := []struct {
		mime string
		want string
	}{
		{"font/woff", ".woff"},
		{"font/woff2", ".woff2"},
		{"font/ttf", ".ttf"},
		{"application/x-font-ttf", ".ttf"},
		{"font/otf", ".otf"},
		{"application/font-woff", ".woff"},
		{"image/svg+xml", ".svg"},
		{"application/vnd.ms-fontobject", ".eot"},
		{"unknown/type", ""},
	}

	for _, tt := range tests {
		t.Run(tt.mime, func(t *testing.T) {
			got := mimeToExtension(tt.mime)
			if got != tt.want {
				t.Errorf("mimeToExtension(%q) = %q, want %q", tt.mime, got, tt.want)
			}
		})
	}
}

func TestNormalizeStylesheets(t *testing.T) {
	log := zaptest.NewLogger(t)

	t.Run("resolves font reference from binary", func(t *testing.T) {
		book := &FictionBook{
			Stylesheets: []Stylesheet{
				{
					Type: "text/css",
					Data: `@font-face { font-family: 'Test'; src: url('#font1'); }`,
				},
			},
			Binaries: []BinaryObject{
				{
					ID:          "font1",
					ContentType: "font/woff2",
					Data:        []byte("fake font data"),
				},
			},
		}

		result := book.NormalizeStylesheets("", nil, log)

		if len(result.Stylesheets) != 1 {
			t.Fatalf("expected 1 stylesheet, got %d", len(result.Stylesheets))
		}

		sheet := result.Stylesheets[0]
		if len(sheet.Resources) != 1 {
			t.Fatalf("expected 1 resource, got %d", len(sheet.Resources))
		}

		resource := sheet.Resources[0]
		if resource.ResolvedID != "font1" {
			t.Errorf("expected resolved ID 'font1', got %q", resource.ResolvedID)
		}
		if resource.MimeType != "font/woff2" {
			t.Errorf("expected MIME 'font/woff2', got %q", resource.MimeType)
		}
		if resource.OriginalURL != "#font1" {
			t.Errorf("expected original URL '#font1', got %q", resource.OriginalURL)
		}
		if string(resource.Data) != "fake font data" {
			t.Errorf("expected data 'fake font data', got %q", string(resource.Data))
		}
		if resource.Loaded {
			t.Errorf("expected Loaded=false for binary resource")
		}
	})

	t.Run("skips non-css stylesheets", func(t *testing.T) {
		book := &FictionBook{
			Stylesheets: []Stylesheet{
				{
					Type: "text/xsl",
					Data: `<xsl:stylesheet/>`,
				},
			},
		}

		result := book.NormalizeStylesheets("", nil, log)
		if len(result.Stylesheets[0].Resources) != 0 {
			t.Errorf("expected no resources for non-CSS stylesheet")
		}
	})

	t.Run("warns on missing binary", func(t *testing.T) {
		book := &FictionBook{
			Stylesheets: []Stylesheet{
				{
					Type: "text/css",
					Data: `@font-face { font-family: 'Test'; src: url('#missing'); }`,
				},
			},
			Binaries: []BinaryObject{},
		}

		result := book.NormalizeStylesheets("", nil, log)
		if len(result.Stylesheets[0].Resources) != 0 {
			t.Errorf("expected no resources for missing binary")
		}
	})

	t.Run("handles multiple resources", func(t *testing.T) {
		book := &FictionBook{
			Stylesheets: []Stylesheet{
				{
					Type: "text/css",
					Data: `
						@font-face { src: url('#font1'); }
						@font-face { src: url('#font2'); }
					`,
				},
			},
			Binaries: []BinaryObject{
				{ID: "font1", ContentType: "font/woff", Data: []byte("data1")},
				{ID: "font2", ContentType: "font/woff2", Data: []byte("data2")},
			},
		}

		result := book.NormalizeStylesheets("", nil, log)
		if len(result.Stylesheets[0].Resources) != 2 {
			t.Fatalf("expected 2 resources, got %d", len(result.Stylesheets[0].Resources))
		}
	})

	t.Run("generates filename from ID when needed", func(t *testing.T) {
		book := &FictionBook{
			Stylesheets: []Stylesheet{
				{
					Type: "text/css",
					Data: `@font-face { font-family: 'Test'; src: url('#my-font'); }`,
				},
			},
			Binaries: []BinaryObject{
				{ID: "my-font", ContentType: "font/woff2", Data: []byte("data")},
			},
		}

		result := book.NormalizeStylesheets("", nil, log)
		resource := result.Stylesheets[0].Resources[0]
		if resource.Filename != "fonts/my-font.woff2" {
			t.Errorf("expected filename 'fonts/my-font.woff2', got %q", resource.Filename)
		}
	})

	t.Run("loads external font file", func(t *testing.T) {
		// Create a temporary font file
		tmpDir := t.TempDir()
		fontPath := filepath.Join(tmpDir, "testfont.woff2")
		// Use proper WOFF2 magic bytes "wOF2" followed by additional data
		fontData := []byte{0x77, 0x4F, 0x46, 0x32, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
		if err := os.WriteFile(fontPath, fontData, 0644); err != nil {
			t.Fatalf("failed to create test font file: %v", err)
		}

		// Create FB2 file in same directory
		fb2Path := filepath.Join(tmpDir, "test.fb2")

		book := &FictionBook{
			Stylesheets: []Stylesheet{
				{
					Type: "text/css",
					Data: `@font-face { src: url('testfont.woff2'); }`,
				},
			},
			Binaries: []BinaryObject{},
		}

		result := book.NormalizeStylesheets(fb2Path, nil, log)

		if len(result.Stylesheets[0].Resources) != 1 {
			t.Fatalf("expected 1 loaded resource, got %d", len(result.Stylesheets[0].Resources))
		}

		resource := result.Stylesheets[0].Resources[0]
		if !resource.Loaded {
			t.Error("expected Loaded=true for file resource")
		}
		if resource.MimeType != "font/woff2" {
			t.Errorf("expected MIME 'font/woff2', got %q", resource.MimeType)
		}
		if string(resource.Data) != string(fontData) {
			t.Errorf("loaded data doesn't match file data")
		}
		if resource.Filename != "fonts/testfont.woff2" {
			t.Errorf("expected filename 'fonts/testfont.woff2', got %q", resource.Filename)
		}

		// Verify binary was added
		if len(result.Binaries) != 1 {
			t.Fatalf("expected 1 binary added, got %d", len(result.Binaries))
		}
		if result.Binaries[0].ContentType != "font/woff2" {
			t.Errorf("binary has wrong content type: %q", result.Binaries[0].ContentType)
		}
	})

	t.Run("loads font from subdirectory", func(t *testing.T) {
		tmpDir := t.TempDir()
		fontsDir := filepath.Join(tmpDir, "fonts")
		if err := os.MkdirAll(fontsDir, 0755); err != nil {
			t.Fatalf("failed to create fonts directory: %v", err)
		}

		fontPath := filepath.Join(fontsDir, "myfont.ttf")
		// Use proper TTF magic bytes (0x00, 0x01, 0x00, 0x00) followed by additional data
		fontData := []byte{0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
		if err := os.WriteFile(fontPath, fontData, 0644); err != nil {
			t.Fatalf("failed to create test font file: %v", err)
		}

		fb2Path := filepath.Join(tmpDir, "test.fb2")

		book := &FictionBook{
			Stylesheets: []Stylesheet{
				{
					Type: "text/css",
					Data: `@font-face { src: url('fonts/myfont.ttf'); }`,
				},
			},
		}

		result := book.NormalizeStylesheets(fb2Path, nil, log)

		if len(result.Stylesheets[0].Resources) != 1 {
			t.Fatalf("expected 1 loaded resource, got %d", len(result.Stylesheets[0].Resources))
		}

		resource := result.Stylesheets[0].Resources[0]
		if !resource.Loaded {
			t.Error("expected Loaded=true for file resource")
		}
		if resource.MimeType != "font/ttf" {
			t.Errorf("expected MIME 'font/ttf', got %q", resource.MimeType)
		}
	})

	t.Run("prepends default stylesheet when provided", func(t *testing.T) {
		defaultCSS := []byte(`/* Default CSS */ body { margin: 0; }`)

		book := &FictionBook{
			Stylesheets: []Stylesheet{
				{
					Type: "text/css",
					Data: `/* FB2 CSS */ p { text-indent: 1em; }`,
				},
			},
		}

		result := book.NormalizeStylesheets("", defaultCSS, log)

		if len(result.Stylesheets) != 2 {
			t.Fatalf("expected 2 stylesheets, got %d", len(result.Stylesheets))
		}

		// First should be default
		if result.Stylesheets[0].Data != string(defaultCSS) {
			t.Errorf("first stylesheet should be default CSS")
		}
		if result.Stylesheets[0].Type != "text/css" {
			t.Errorf("default stylesheet should have type text/css")
		}

		// Second should be original FB2 stylesheet
		if !strings.Contains(result.Stylesheets[1].Data, "FB2 CSS") {
			t.Errorf("second stylesheet should be original FB2 stylesheet")
		}
	})

	t.Run("skips default stylesheet when nil", func(t *testing.T) {
		book := &FictionBook{
			Stylesheets: []Stylesheet{
				{
					Type: "text/css",
					Data: `p { text-indent: 1em; }`,
				},
			},
		}

		result := book.NormalizeStylesheets("", nil, log)

		if len(result.Stylesheets) != 1 {
			t.Fatalf("expected 1 stylesheet, got %d", len(result.Stylesheets))
		}
	})

	t.Run("skips default stylesheet when empty", func(t *testing.T) {
		book := &FictionBook{
			Stylesheets: []Stylesheet{
				{
					Type: "text/css",
					Data: `p { text-indent: 1em; }`,
				},
			},
		}

		result := book.NormalizeStylesheets("", []byte{}, log)

		if len(result.Stylesheets) != 1 {
			t.Fatalf("expected 1 stylesheet, got %d", len(result.Stylesheets))
		}
	})

	t.Run("default stylesheet resources resolve from current directory", func(t *testing.T) {
		// Create a font file in current directory (simulated with tmpDir)
		tmpDir := t.TempDir()

		// Create fonts subdirectory in "current" directory
		fontsDir := filepath.Join(tmpDir, "fonts")
		if err := os.MkdirAll(fontsDir, 0755); err != nil {
			t.Fatalf("failed to create fonts directory: %v", err)
		}

		fontPath := filepath.Join(fontsDir, "default.woff2")
		// Use proper WOFF2 magic bytes "wOF2"
		fontData := []byte{0x77, 0x4F, 0x46, 0x32, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
		if err := os.WriteFile(fontPath, fontData, 0644); err != nil {
			t.Fatalf("failed to create font file: %v", err)
		}

		// Create FB2 file in a completely different directory
		fb2Dir := filepath.Join(tmpDir, "books", "subdir")
		if err := os.MkdirAll(fb2Dir, 0755); err != nil {
			t.Fatalf("failed to create fb2 directory: %v", err)
		}
		fb2Path := filepath.Join(fb2Dir, "test.fb2")

		// Change to the directory where fonts are
		oldWd, err := os.Getwd()
		if err != nil {
			t.Fatalf("failed to get working directory: %v", err)
		}
		defer os.Chdir(oldWd)

		if err := os.Chdir(tmpDir); err != nil {
			t.Fatalf("failed to change directory: %v", err)
		}

		// Default stylesheet with relative font reference
		defaultCSS := []byte(`@font-face { src: url('fonts/default.woff2'); }`)

		book := &FictionBook{
			Stylesheets: []Stylesheet{
				{
					Type: "text/css",
					Data: `/* FB2 embedded */`,
				},
			},
		}

		result := book.NormalizeStylesheets(fb2Path, defaultCSS, log)

		if len(result.Stylesheets) != 2 {
			t.Fatalf("expected 2 stylesheets, got %d", len(result.Stylesheets))
		}

		// First stylesheet should be default with loaded resource
		defaultSheet := result.Stylesheets[0]
		if len(defaultSheet.Resources) != 1 {
			t.Fatalf("expected 1 resource in default stylesheet, got %d", len(defaultSheet.Resources))
		}

		resource := defaultSheet.Resources[0]
		if !resource.Loaded {
			t.Error("expected Loaded=true for default stylesheet resource")
		}
		if string(resource.Data) != string(fontData) {
			t.Errorf("resource data mismatch: got %q, want %q", string(resource.Data), string(fontData))
		}
		if resource.Filename != "fonts/default.woff2" {
			t.Errorf("expected filename 'fonts/default.woff2', got %q", resource.Filename)
		}
	})

	t.Run("FB2 embedded stylesheet resources resolve from source directory", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create fonts directory next to FB2 file
		fb2Dir := filepath.Join(tmpDir, "books")
		fontsDir := filepath.Join(fb2Dir, "fonts")
		if err := os.MkdirAll(fontsDir, 0755); err != nil {
			t.Fatalf("failed to create fonts directory: %v", err)
		}

		fontPath := filepath.Join(fontsDir, "embedded.woff2")
		// Use proper WOFF2 magic bytes "wOF2"
		fontData := []byte{0x77, 0x4F, 0x46, 0x32, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
		if err := os.WriteFile(fontPath, fontData, 0644); err != nil {
			t.Fatalf("failed to create font file: %v", err)
		}

		fb2Path := filepath.Join(fb2Dir, "test.fb2")

		// Run from a different directory
		otherDir := filepath.Join(tmpDir, "other")
		if err := os.MkdirAll(otherDir, 0755); err != nil {
			t.Fatalf("failed to create other directory: %v", err)
		}

		oldWd, err := os.Getwd()
		if err != nil {
			t.Fatalf("failed to get working directory: %v", err)
		}
		defer os.Chdir(oldWd)

		if err := os.Chdir(otherDir); err != nil {
			t.Fatalf("failed to change directory: %v", err)
		}

		// FB2 embedded stylesheet with relative font reference
		book := &FictionBook{
			Stylesheets: []Stylesheet{
				{
					Type: "text/css",
					Data: `@font-face { src: url('fonts/embedded.woff2'); }`,
				},
			},
		}

		result := book.NormalizeStylesheets(fb2Path, nil, log)

		if len(result.Stylesheets) != 1 {
			t.Fatalf("expected 1 stylesheet, got %d", len(result.Stylesheets))
		}

		sheet := result.Stylesheets[0]
		if len(sheet.Resources) != 1 {
			t.Fatalf("expected 1 resource, got %d", len(sheet.Resources))
		}

		resource := sheet.Resources[0]
		if !resource.Loaded {
			t.Error("expected Loaded=true for FB2 embedded resource")
		}
		if string(resource.Data) != string(fontData) {
			t.Errorf("resource data mismatch: got %q, want %q", string(resource.Data), string(fontData))
		}
	})
	t.Run("rejects path traversal via url()", func(t *testing.T) {
		// Create a "sensitive" file outside the source directory
		tmpDir := t.TempDir()
		sensitiveDir := filepath.Join(tmpDir, "sensitive")
		if err := os.MkdirAll(sensitiveDir, 0755); err != nil {
			t.Fatalf("failed to create sensitive directory: %v", err)
		}
		sensitiveFile := filepath.Join(sensitiveDir, "secret.txt")
		if err := os.WriteFile(sensitiveFile, []byte("top secret data"), 0644); err != nil {
			t.Fatalf("failed to create sensitive file: %v", err)
		}

		// Create book directory (separate from sensitive directory)
		bookDir := filepath.Join(tmpDir, "books", "subdir")
		if err := os.MkdirAll(bookDir, 0755); err != nil {
			t.Fatalf("failed to create book directory: %v", err)
		}
		fb2Path := filepath.Join(bookDir, "test.fb2")

		// CSS tries to traverse up and read the sensitive file
		book := &FictionBook{
			Stylesheets: []Stylesheet{
				{
					Type: "text/css",
					Data: `@font-face { src: url('../../sensitive/secret.txt'); }`,
				},
			},
		}

		result := book.NormalizeStylesheets(fb2Path, nil, log)

		// The path traversal should be rejected — no resources loaded
		if len(result.Stylesheets[0].Resources) != 0 {
			t.Errorf("expected 0 resources (path traversal should be rejected), got %d", len(result.Stylesheets[0].Resources))
			for _, r := range result.Stylesheets[0].Resources {
				t.Logf("  loaded: %q (data=%q)", r.OriginalURL, string(r.Data))
			}
		}
	})

	t.Run("rejects absolute path via url()", func(t *testing.T) {
		// Create a file at a known absolute path
		tmpDir := t.TempDir()
		absFile := filepath.Join(tmpDir, "secret.txt")
		if err := os.WriteFile(absFile, []byte("secret via absolute path"), 0644); err != nil {
			t.Fatalf("failed to create file: %v", err)
		}

		bookDir := filepath.Join(tmpDir, "books")
		if err := os.MkdirAll(bookDir, 0755); err != nil {
			t.Fatalf("failed to create book directory: %v", err)
		}
		fb2Path := filepath.Join(bookDir, "test.fb2")

		// CSS references an absolute path
		book := &FictionBook{
			Stylesheets: []Stylesheet{
				{
					Type: "text/css",
					Data: `@font-face { src: url('` + absFile + `'); }`,
				},
			},
		}

		result := book.NormalizeStylesheets(fb2Path, nil, log)

		// Absolute paths should be rejected
		if len(result.Stylesheets[0].Resources) != 0 {
			t.Errorf("expected 0 resources (absolute path should be rejected), got %d", len(result.Stylesheets[0].Resources))
		}
	})
}

func TestParseSectionPageBreaks(t *testing.T) {
	tests := []struct {
		name string
		css  string
		want map[int]bool
	}{
		{
			name: "simple page-break-before always",
			css:  `.section-title-h2 { page-break-before: always; }`,
			want: map[int]bool{2: true},
		},
		{
			name: "multiple depths",
			css: `.section-title-h2 { page-break-before: always; }
			      .section-title-h3 { page-break-before: always; }`,
			want: map[int]bool{2: true, 3: true},
		},
		{
			name: "no page-break means false",
			css:  `.section-title-h3 { margin-top: 2em; }`,
			want: map[int]bool{3: false},
		},
		{
			name: "mixed — some have break some do not",
			css: `.section-title-h2 { page-break-before: always; }
			      .section-title-h4 { margin-top: 1em; }`,
			want: map[int]bool{2: true, 4: false},
		},
		{
			name: "grouped selector",
			css:  `.section-title-h3, .something-else { page-break-before: always; }`,
			want: map[int]bool{3: true},
		},
		{
			name: "extra whitespace around property",
			css:  `.section-title-h5 {  page-break-before :  always ; color: red; }`,
			want: map[int]bool{5: true},
		},
		{
			name: "depth 6 boundary",
			css:  `.section-title-h6 { page-break-before: always; }`,
			want: map[int]bool{6: true},
		},
		{
			name: "no matching rules",
			css:  `body { margin: 0; } .chapter-title { page-break-before: always; }`,
			want: map[int]bool{},
		},
		{
			name: "ignores depth outside 2-6",
			css:  `.section-title-h1 { page-break-before: always; } .section-title-h7 { page-break-before: always; }`,
			want: map[int]bool{},
		},
		{
			name: "case insensitive property matching",
			css:  `.section-title-h2 { PAGE-BREAK-BEFORE: ALWAYS; }`,
			want: map[int]bool{2: true},
		},
		{
			name: "later rule overrides earlier",
			css: `.section-title-h2 { page-break-before: always; }
			      .section-title-h2 { margin-top: 1em; }`,
			want: map[int]bool{2: false},
		},
		{
			name: "empty css",
			css:  ``,
			want: map[int]bool{},
		},
		{
			name: "real-world default.css pattern",
			css: `.section-title {
				page-break-inside: avoid;
				page-break-after: avoid;
				margin: 2em 0 1em 0;
			}
			.section-title-h2 {
				page-break-before: always;
			}`,
			want: map[int]bool{2: true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseSectionPageBreaks(tt.css)
			if len(got) != len(tt.want) {
				t.Fatalf("got %d entries %v, want %d entries %v", len(got), got, len(tt.want), tt.want)
			}
			for depth, wantBreak := range tt.want {
				if gotBreak, ok := got[depth]; !ok {
					t.Errorf("missing depth %d in result", depth)
				} else if gotBreak != wantBreak {
					t.Errorf("depth %d: got %v, want %v", depth, gotBreak, wantBreak)
				}
			}
		})
	}
}

func TestSectionNeedsBreak(t *testing.T) {
	t.Run("returns true for depth with page break", func(t *testing.T) {
		book := &FictionBook{}
		book.sectionPageBreaks = map[int]bool{2: true, 3: true}

		if !book.SectionNeedsBreak(2) {
			t.Error("expected SectionNeedsBreak(2) = true")
		}
		if !book.SectionNeedsBreak(3) {
			t.Error("expected SectionNeedsBreak(3) = true")
		}
	})

	t.Run("returns false for depth without page break", func(t *testing.T) {
		book := &FictionBook{}
		book.sectionPageBreaks = map[int]bool{2: true}

		if book.SectionNeedsBreak(4) {
			t.Error("expected SectionNeedsBreak(4) = false")
		}
	})

	t.Run("returns false for depth explicitly set to false", func(t *testing.T) {
		book := &FictionBook{}
		book.sectionPageBreaks = map[int]bool{3: false}

		if book.SectionNeedsBreak(3) {
			t.Error("expected SectionNeedsBreak(3) = false")
		}
	})

	t.Run("returns false for depth 1", func(t *testing.T) {
		book := &FictionBook{}
		book.sectionPageBreaks = map[int]bool{2: true, 3: true}

		if book.SectionNeedsBreak(1) {
			t.Error("expected SectionNeedsBreak(1) = false — depth 1 is chapter level")
		}
	})

	t.Run("clamps depth above 6", func(t *testing.T) {
		book := &FictionBook{}
		book.sectionPageBreaks = map[int]bool{6: true}

		if !book.SectionNeedsBreak(7) {
			t.Error("expected SectionNeedsBreak(7) = true (clamped to 6)")
		}
		if !book.SectionNeedsBreak(100) {
			t.Error("expected SectionNeedsBreak(100) = true (clamped to 6)")
		}
	})

	t.Run("nil map returns false", func(t *testing.T) {
		book := &FictionBook{}

		if book.SectionNeedsBreak(2) {
			t.Error("expected SectionNeedsBreak(2) = false with nil map")
		}
	})

	t.Run("populated via NormalizeStylesheets", func(t *testing.T) {
		log := zaptest.NewLogger(t)
		book := &FictionBook{
			Stylesheets: []Stylesheet{
				{
					Type: "text/css",
					Data: `.section-title-h2 { page-break-before: always; }`,
				},
			},
		}

		result := book.NormalizeStylesheets("", nil, log)

		if !result.SectionNeedsBreak(2) {
			t.Error("expected SectionNeedsBreak(2) = true after NormalizeStylesheets")
		}
		if result.SectionNeedsBreak(3) {
			t.Error("expected SectionNeedsBreak(3) = false — not specified in CSS")
		}
	})

	t.Run("user CSS overrides default CSS", func(t *testing.T) {
		log := zaptest.NewLogger(t)
		defaultCSS := []byte(`.section-title-h2 { page-break-before: always; }`)

		book := &FictionBook{
			Stylesheets: []Stylesheet{
				{
					Type: "text/css",
					Data: `.section-title-h2 { margin-top: 2em; }`, // no page-break-before
				},
			},
		}

		result := book.NormalizeStylesheets("", defaultCSS, log)

		if result.SectionNeedsBreak(2) {
			t.Error("expected SectionNeedsBreak(2) = false — user CSS should override default")
		}
	})
}

func TestParseBodyTitlePageBreak(t *testing.T) {
	tests := []struct {
		name      string
		css       string
		wantValue bool
		wantFound bool
	}{
		{
			name:      "page-break-before always",
			css:       `.body-title { page-break-before: always; }`,
			wantValue: true,
			wantFound: true,
		},
		{
			name:      "no body-title rule",
			css:       `.section-title { page-break-before: always; }`,
			wantValue: false,
			wantFound: false,
		},
		{
			name:      "body-title without page-break-before",
			css:       `.body-title { margin-top: 2em; }`,
			wantValue: false,
			wantFound: false,
		},
		{
			name:      "page-break-before avoid",
			css:       `.body-title { page-break-before: avoid; }`,
			wantValue: false,
			wantFound: true,
		},
		{
			name:      "case insensitive",
			css:       `.body-title { PAGE-BREAK-BEFORE: ALWAYS; }`,
			wantValue: true,
			wantFound: true,
		},
		{
			name:      "empty CSS",
			css:       ``,
			wantValue: false,
			wantFound: false,
		},
		{
			name: "real-world default.css pattern",
			css: `.body-title {
				page-break-inside: avoid;
				page-break-after: avoid;
				page-break-before: always;
				margin: 2em 0 1em 0;
			}`,
			wantValue: true,
			wantFound: true,
		},
		{
			name: "later rule without property does not override",
			css: `.body-title { page-break-before: always; }
			      .body-title { margin-top: 1em; }`,
			wantValue: true,
			wantFound: true,
		},
		{
			name: "later rule with property overrides",
			css: `.body-title { page-break-before: always; }
			      .body-title { page-break-before: avoid; }`,
			wantValue: false,
			wantFound: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotValue, gotFound := parseBodyTitlePageBreak(tt.css)
			if gotFound != tt.wantFound {
				t.Errorf("found = %v, want %v", gotFound, tt.wantFound)
			}
			if gotValue != tt.wantValue {
				t.Errorf("value = %v, want %v", gotValue, tt.wantValue)
			}
		})
	}
}

func TestBodyTitleNeedsBreak(t *testing.T) {
	t.Run("returns false by default", func(t *testing.T) {
		book := &FictionBook{}
		if book.BodyTitleNeedsBreak() {
			t.Error("expected BodyTitleNeedsBreak() = false by default")
		}
	})

	t.Run("returns true when set", func(t *testing.T) {
		book := &FictionBook{}
		book.SetBodyTitlePageBreak(true)
		if !book.BodyTitleNeedsBreak() {
			t.Error("expected BodyTitleNeedsBreak() = true after SetBodyTitlePageBreak(true)")
		}
	})

	t.Run("returns false when explicitly unset", func(t *testing.T) {
		book := &FictionBook{}
		book.SetBodyTitlePageBreak(true)
		book.SetBodyTitlePageBreak(false)
		if book.BodyTitleNeedsBreak() {
			t.Error("expected BodyTitleNeedsBreak() = false after SetBodyTitlePageBreak(false)")
		}
	})

	t.Run("populated via NormalizeStylesheets", func(t *testing.T) {
		log := zaptest.NewLogger(t)
		book := &FictionBook{
			Stylesheets: []Stylesheet{
				{
					Type: "text/css",
					Data: `.body-title { page-break-before: always; }`,
				},
			},
		}

		result := book.NormalizeStylesheets("", nil, log)

		if !result.BodyTitleNeedsBreak() {
			t.Error("expected BodyTitleNeedsBreak() = true after NormalizeStylesheets")
		}
	})

	t.Run("user CSS overrides default CSS", func(t *testing.T) {
		log := zaptest.NewLogger(t)
		defaultCSS := []byte(`.body-title { page-break-before: always; }`)

		book := &FictionBook{
			Stylesheets: []Stylesheet{
				{
					Type: "text/css",
					Data: `.body-title { page-break-before: avoid; }`,
				},
			},
		}

		result := book.NormalizeStylesheets("", defaultCSS, log)

		if result.BodyTitleNeedsBreak() {
			t.Error("expected BodyTitleNeedsBreak() = false — user CSS should override default")
		}
	})

	t.Run("user CSS without body-title preserves default", func(t *testing.T) {
		log := zaptest.NewLogger(t)
		defaultCSS := []byte(`.body-title { page-break-before: always; }`)

		book := &FictionBook{
			Stylesheets: []Stylesheet{
				{
					Type: "text/css",
					Data: `.section-title { margin-top: 2em; }`,
				},
			},
		}

		result := book.NormalizeStylesheets("", defaultCSS, log)

		if !result.BodyTitleNeedsBreak() {
			t.Error("expected BodyTitleNeedsBreak() = true — user CSS doesn't mention body-title, default should apply")
		}
	})
}
