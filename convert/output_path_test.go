package convert

import (
	"path/filepath"
	"testing"

	"github.com/beevik/etree"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"

	"fbc/config"
	"fbc/fb2"
	"fbc/state"
)

func setupTestEnvForOutputPath(t *testing.T, noDirs bool, transliterate bool, format config.OutputFmt, template string) *state.LocalEnv {
	t.Helper()
	logger := zaptest.NewLogger(t, zaptest.WrapOptions(zap.AddCaller(), zap.AddCallerSkip(1)))
	cfg, err := config.LoadConfiguration("")
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	cfg.Document.FileNameTransliterate = transliterate
	cfg.Document.OutputNameTemplate = template

	env := &state.LocalEnv{
		Log:          logger,
		Cfg:          cfg,
		NoDirs:       noDirs,
		OutputFormat: format,
	}
	return env
}

func setupTestContentForPath(t *testing.T) *Content {
	t.Helper()
	doc := etree.NewDocument()
	return &Content{
		doc:     doc,
		srcName: "testbook.fb2",
		book: &fb2.FictionBook{
			Description: fb2.Description{
				TitleInfo: fb2.TitleInfo{
					BookTitle: fb2.TextField{Value: "Test Book"},
					Lang:      "en",
					Authors: []fb2.Author{
						{FirstName: "John", LastName: "Doe"},
					},
				},
				DocumentInfo: fb2.DocumentInfo{
					ID: "test-book-id",
				},
			},
		},
	}
}

func TestBuildOutputPath_SimpleCase_NoDirs(t *testing.T) {
	c := setupTestContentForPath(t)
	env := setupTestEnvForOutputPath(t, true, false, config.OutputFmtEpub3, "")

	result := c.buildOutputPath("books/author/book.fb2", "/output", env)
	expected := filepath.Join("/output", "book.epub")

	if result != expected {
		t.Errorf("buildOutputPath() = %q, want %q", result, expected)
	}
}

func TestBuildOutputPath_SimpleCase_WithDirs(t *testing.T) {
	c := setupTestContentForPath(t)
	env := setupTestEnvForOutputPath(t, false, false, config.OutputFmtEpub3, "")

	result := c.buildOutputPath("books/author/book.fb2", "/output", env)
	expected := filepath.Join("/output", "books", "author", "book.epub")

	if result != expected {
		t.Errorf("buildOutputPath() = %q, want %q", result, expected)
	}
}

func TestBuildOutputPath_DifferentFormats(t *testing.T) {
	tests := []struct {
		name   string
		format config.OutputFmt
		ext    string
	}{
		{"EPUB2", config.OutputFmtEpub2, ".epub"},
		{"EPUB3", config.OutputFmtEpub3, ".epub"},
		{"KEPUB", config.OutputFmtKepub, ".kepub.epub"},
		{"KFX", config.OutputFmtKfx, ".kfx"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := setupTestContentForPath(t)
			env := setupTestEnvForOutputPath(t, true, false, tt.format, "")

			result := c.buildOutputPath("book.fb2", "/output", env)
			expected := filepath.Join("/output", "book"+tt.ext)

			if result != expected {
				t.Errorf("buildOutputPath() = %q, want %q", result, expected)
			}
		})
	}
}

func TestBuildOutputPath_Transliterate(t *testing.T) {
	c := setupTestContentForPath(t)
	env := setupTestEnvForOutputPath(t, true, true, config.OutputFmtEpub3, "")

	result := c.buildOutputPath("Книга.fb2", "/output", env)
	expected := filepath.Join("/output", "kniga.epub")

	if result != expected {
		t.Errorf("buildOutputPath() = %q, want %q", result, expected)
	}
}

func TestDetermineOutputDir_NoDirs(t *testing.T) {
	c := setupTestContentForPath(t)
	env := setupTestEnvForOutputPath(t, true, false, config.OutputFmtEpub3, "")

	result := c.determineOutputDir("books/author/book.fb2", "/output", env)
	expected := "/output"

	if result != expected {
		t.Errorf("determineOutputDir() = %q, want %q", result, expected)
	}
}

func TestDetermineOutputDir_WithDirs(t *testing.T) {
	c := setupTestContentForPath(t)
	env := setupTestEnvForOutputPath(t, false, false, config.OutputFmtEpub3, "")

	result := c.determineOutputDir("books/author/book.fb2", "/output", env)
	expected := filepath.Join("/output", "books", "author")

	if result != expected {
		t.Errorf("determineOutputDir() = %q, want %q", result, expected)
	}
}

func TestBuildDefaultFileName(t *testing.T) {
	tests := []struct {
		name          string
		src           string
		transliterate bool
		format        config.OutputFmt
		expected      string
	}{
		{"simple epub", "book.fb2", false, config.OutputFmtEpub3, "book.epub"},
		{"with path", "path/to/book.fb2", false, config.OutputFmtEpub3, "book.epub"},
		{"kepub format", "book.fb2", false, config.OutputFmtKepub, "book.kepub.epub"},
		{"kfx format", "book.fb2", false, config.OutputFmtKfx, "book.kfx"},
		{"transliterate", "Книга.fb2", true, config.OutputFmtEpub3, "kniga.epub"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := setupTestContentForPath(t)
			env := setupTestEnvForOutputPath(t, true, tt.transliterate, tt.format, "")

			result := c.buildDefaultFileName(tt.src, env)
			if result != tt.expected {
				t.Errorf("buildDefaultFileName() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestGetFileExtension(t *testing.T) {
	tests := []struct {
		name   string
		format config.OutputFmt
		want   string
	}{
		{"EPUB2", config.OutputFmtEpub2, ".epub"},
		{"EPUB3", config.OutputFmtEpub3, ".epub"},
		{"KEPUB", config.OutputFmtKepub, ".kepub.epub"},
		{"KFX", config.OutputFmtKfx, ".kfx"},
	}

	c := setupTestContentForPath(t)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := c.getFileExtension(tt.format)
			if result != tt.want {
				t.Errorf("getFileExtension() = %q, want %q", result, tt.want)
			}
		})
	}
}

func TestGetFileExtension_Panic(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("getFileExtension() did not panic for unsupported format")
		}
	}()

	c := setupTestContentForPath(t)
	_ = c.getFileExtension(config.OutputFmt(999))
}

func TestSplitAndCleanPath(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected []string
	}{
		{"simple path", "author/book", []string{"author", "book"}},
		{"single segment", "book", []string{"book"}},
		{"with trailing slash", "author/book/", []string{"author", "book"}},
		{"three levels", "genre/author/book", []string{"genre", "author", "book"}},
		{"empty path", "", []string{}},
	}

	c := setupTestContentForPath(t)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := c.splitAndCleanPath(tt.path)
			if len(result) != len(tt.expected) {
				t.Errorf("splitAndCleanPath() length = %d, want %d", len(result), len(tt.expected))
				return
			}
			for i := range result {
				if result[i] != tt.expected[i] {
					t.Errorf("splitAndCleanPath()[%d] = %q, want %q", i, result[i], tt.expected[i])
				}
			}
		})
	}
}

func TestCleanPathSegment(t *testing.T) {
	tests := []struct {
		name          string
		segment       string
		transliterate bool
		expected      string
	}{
		{"simple segment", "author", false, "author"},
		{"with spaces", "My Book", false, "My Book"},
		{"transliterate cyrillic", "Автор", true, "avtor"},
		{"special chars", "book:name", false, "bookname"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := setupTestContentForPath(t)
			env := setupTestEnvForOutputPath(t, true, tt.transliterate, config.OutputFmtEpub3, "")

			result := c.cleanPathSegment(tt.segment, env)
			if result != tt.expected {
				t.Errorf("cleanPathSegment() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestBuildPathFromTemplate(t *testing.T) {
	tests := []struct {
		name          string
		outDir        string
		expandedName  string
		transliterate bool
		format        config.OutputFmt
		expected      string
	}{
		{
			"simple template",
			"/output",
			"author/book",
			false,
			config.OutputFmtEpub3,
			filepath.Join("/output", "author", "book.epub"),
		},
		{
			"single level",
			"/output",
			"book",
			false,
			config.OutputFmtEpub3,
			filepath.Join("/output", "book.epub"),
		},
		{
			"with transliterate",
			"/output",
			"Автор/Книга",
			true,
			config.OutputFmtEpub3,
			filepath.Join("/output", "avtor", "kniga.epub"),
		},
		{
			"kepub format",
			"/output",
			"author/book",
			false,
			config.OutputFmtKepub,
			filepath.Join("/output", "author", "book.kepub.epub"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := setupTestContentForPath(t)
			env := setupTestEnvForOutputPath(t, true, tt.transliterate, tt.format, "")

			result := c.buildPathFromTemplate(tt.outDir, tt.expandedName, env)
			if result != tt.expected {
				t.Errorf("buildPathFromTemplate() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestBuildPathFromTemplate_EmptyPath(t *testing.T) {
	c := setupTestContentForPath(t)
	env := setupTestEnvForOutputPath(t, true, false, config.OutputFmtEpub3, "")

	result := c.buildPathFromTemplate("/output", "", env)
	expected := "/output"

	if result != expected {
		t.Errorf("buildPathFromTemplate() with empty path = %q, want %q", result, expected)
	}
}
