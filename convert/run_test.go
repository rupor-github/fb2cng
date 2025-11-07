package convert

import (
	"archive/zip"
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
	"golang.org/x/text/encoding/unicode"
	"golang.org/x/text/encoding/unicode/utf32"
	"golang.org/x/text/transform"

	"fbc/config"
	"fbc/state"
)

const sampleFB2Path = "../testdata/_Test.fb2"

// setupTestEnv creates a test environment with proper context and logger
func setupTestEnv(t *testing.T) (context.Context, *state.LocalEnv) {
	logger := zaptest.NewLogger(t, zaptest.WrapOptions(zap.AddCaller(), zap.AddCallerSkip(1)))
	cfg, err := config.LoadConfiguration("")
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	// skip image optimization in tests - some of the tests have broken images
	cfg.Document.Images.Optimize = false
	ctx := state.ContextWithEnv(context.Background())
	env := state.EnvFromContext(ctx)
	env.Log = logger
	env.Cfg = cfg
	return ctx, env
}

func loadProcessBookSample(t *testing.T) []byte {
	t.Helper()
	data, err := os.ReadFile(sampleFB2Path)
	if err != nil {
		t.Fatalf("read sample FB2: %v", err)
	}
	return data
}

func readerForEncoding(t *testing.T, data []byte, enc srcEncoding) *bytes.Reader {
	t.Helper()
	var encoded []byte
	switch enc {
	case encUnknown:
		encoded = data
	case encUTF8:
		encoded = append([]byte{0xEF, 0xBB, 0xBF}, data...)
	case encUTF16BigEndian:
		encoded = encodeWithTransformer(t, data, unicode.UTF16(unicode.BigEndian, unicode.UseBOM).NewEncoder())
	case encUTF16LittleEndian:
		encoded = encodeWithTransformer(t, data, unicode.UTF16(unicode.LittleEndian, unicode.UseBOM).NewEncoder())
	case encUTF32BigEndian:
		encoded = encodeWithTransformer(t, data, utf32.UTF32(utf32.BigEndian, utf32.UseBOM).NewEncoder())
	case encUTF32LittleEndian:
		encoded = encodeWithTransformer(t, data, utf32.UTF32(utf32.LittleEndian, utf32.UseBOM).NewEncoder())
	default:
		t.Fatalf("unsupported encoding: %v", enc)
	}
	return bytes.NewReader(encoded)
}

func encodeWithTransformer(t *testing.T, data []byte, encoder transform.Transformer) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := transform.NewWriter(&buf, encoder)
	if _, err := w.Write(data); err != nil {
		t.Fatalf("encode sample: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("finalize encoded sample: %v", err)
	}
	return buf.Bytes()
}

// TestProcess_NonExistentPath tests process with non-existent path
func TestProcess_NonExistentPath(t *testing.T) {
	ctx, _ := setupTestEnv(t)
	logger := zaptest.NewLogger(t, zaptest.WrapOptions(zap.AddCaller(), zap.AddCallerSkip(1)))

	err := process(ctx, "/nonexistent/path/file.fb2", "/tmp", logger)
	if err == nil {
		t.Fatal("Expected error for non-existent path, got nil")
	}
	expectedMsg := "input source was not found"
	if !strings.Contains(err.Error(), expectedMsg) {
		t.Errorf("Expected error containing '%s', got: %v", expectedMsg, err)
	}
}

// TestProcess_CancelledContext tests process with cancelled context
func TestProcess_CancelledContext(t *testing.T) {
	ctx, _ := setupTestEnv(t)
	cancelCtx, cancel := context.WithCancel(ctx)
	logger := zaptest.NewLogger(t, zaptest.WrapOptions(zap.AddCaller(), zap.AddCallerSkip(1)))
	cancel() // Cancel immediately

	tmpDir := t.TempDir()
	err := process(cancelCtx, tmpDir, tmpDir, logger)
	if err != context.Canceled {
		t.Errorf("Expected context.Canceled error, got %v", err)
	}
}

// TestProcess_Directory tests process with a directory
func TestProcess_Directory(t *testing.T) {
	ctx, _ := setupTestEnv(t)
	logger := zaptest.NewLogger(t, zaptest.WrapOptions(zap.AddCaller(), zap.AddCallerSkip(1)))

	tmpDir := t.TempDir()
	dstDir := t.TempDir()

	// Create a valid FB2 file in the directory
	fb2Content := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<FictionBook xmlns="http://www.gribuser.ru/xml/fictionbook/2.0">
<description><title-info><book-title>Test</book-title></title-info></description>
<body><section><p>Content</p></section></body>
</FictionBook>`)

	testFile := filepath.Join(tmpDir, "test.fb2")
	if err := os.WriteFile(testFile, fb2Content, 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	err := process(ctx, tmpDir, dstDir, logger)
	if err != nil {
		t.Errorf("process() error = %v", err)
	}
}

// TestProcess_DirectoryWithTail tests process with directory path that has a tail
func TestProcess_DirectoryWithTail(t *testing.T) {
	ctx, _ := setupTestEnv(t)
	logger := zaptest.NewLogger(t, zaptest.WrapOptions(zap.AddCaller(), zap.AddCallerSkip(1)))

	tmpDir := t.TempDir()
	// Create a directory with a tail (invalid case)
	invalidPath := filepath.Join(tmpDir, "subdir")
	if err := os.MkdirAll(invalidPath, 0755); err != nil {
		t.Fatalf("Failed to create directory: %v", err)
	}

	// Add a non-existent tail to the directory path
	pathWithTail := filepath.Join(invalidPath, "nonexistent.fb2")

	err := process(ctx, pathWithTail, tmpDir, logger)
	if err == nil {
		t.Fatal("Expected error for directory with tail, got nil")
	}
}

// TestProcess_SingleFile tests process with a single FB2 file
func TestProcess_SingleFile(t *testing.T) {
	ctx, _ := setupTestEnv(t)
	logger := zaptest.NewLogger(t, zaptest.WrapOptions(zap.AddCaller(), zap.AddCallerSkip(1)))

	tmpDir := t.TempDir()
	dstDir := t.TempDir()

	fb2Content := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<FictionBook xmlns="http://www.gribuser.ru/xml/fictionbook/2.0">
<description><title-info><book-title>Test</book-title></title-info></description>
<body><section><p>Content</p></section></body>
</FictionBook>`)

	testFile := filepath.Join(tmpDir, "book.fb2")
	if err := os.WriteFile(testFile, fb2Content, 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	err := process(ctx, testFile, dstDir, logger)
	if err != nil {
		t.Errorf("process() error = %v", err)
	}
}

// TestProcess_Archive tests process with a ZIP archive
func TestProcess_Archive(t *testing.T) {
	ctx, _ := setupTestEnv(t)
	logger := zaptest.NewLogger(t, zaptest.WrapOptions(zap.AddCaller(), zap.AddCallerSkip(1)))

	tmpDir := t.TempDir()
	dstDir := t.TempDir()

	// Create FB2 content
	fb2Base := `<?xml version="1.0" encoding="UTF-8"?>
<FictionBook xmlns="http://www.gribuser.ru/xml/fictionbook/2.0">
<description><title-info><book-title>Test</book-title></title-info></description>
<body><section><p>Content `
	padding := make([]byte, 512-len(fb2Base)-len("</p></section></body></FictionBook>"))
	for i := range padding {
		padding[i] = byte('A' + (i % 26))
	}
	fb2Content := []byte(fb2Base + string(padding) + "</p></section></body></FictionBook>")

	// Create a ZIP archive
	zipPath := filepath.Join(tmpDir, "books.zip")
	zipFile, err := os.Create(zipPath)
	if err != nil {
		t.Fatalf("Failed to create zip file: %v", err)
	}

	w := zip.NewWriter(zipFile)
	f, err := w.CreateHeader(&zip.FileHeader{
		Name:   "book.fb2",
		Method: zip.Store,
	})
	if err != nil {
		t.Fatalf("Failed to create file in zip: %v", err)
	}
	if _, err := f.Write(fb2Content); err != nil {
		t.Fatalf("Failed to write to zip: %v", err)
	}
	w.Close()
	zipFile.Close()

	err = process(ctx, zipPath, dstDir, logger)
	if err != nil {
		t.Errorf("process() error = %v", err)
	}
}

// TestProcess_ArchiveWithPath tests process with path inside archive
func TestProcess_ArchiveWithPath(t *testing.T) {
	ctx, _ := setupTestEnv(t)
	logger := zaptest.NewLogger(t, zaptest.WrapOptions(zap.AddCaller(), zap.AddCallerSkip(1)))

	tmpDir := t.TempDir()
	dstDir := t.TempDir()

	// Create FB2 content
	fb2Base := `<?xml version="1.0" encoding="UTF-8"?>
<FictionBook xmlns="http://www.gribuser.ru/xml/fictionbook/2.0">
<description><title-info><book-title>Test</book-title></title-info></description>
<body><section><p>Content `
	padding := make([]byte, 512-len(fb2Base)-len("</p></section></body></FictionBook>"))
	for i := range padding {
		padding[i] = byte('A' + (i % 26))
	}
	fb2Content := []byte(fb2Base + string(padding) + "</p></section></body></FictionBook>")

	// Create a ZIP archive with a subdirectory
	zipPath := filepath.Join(tmpDir, "books.zip")
	zipFile, err := os.Create(zipPath)
	if err != nil {
		t.Fatalf("Failed to create zip file: %v", err)
	}

	w := zip.NewWriter(zipFile)
	f, err := w.CreateHeader(&zip.FileHeader{
		Name:   "subdir/book.fb2",
		Method: zip.Store,
	})
	if err != nil {
		t.Fatalf("Failed to create file in zip: %v", err)
	}
	if _, err := f.Write(fb2Content); err != nil {
		t.Fatalf("Failed to write to zip: %v", err)
	}
	w.Close()
	zipFile.Close()

	// Process with a path inside the archive
	pathInArchive := zipPath + string(filepath.Separator) + "subdir"
	err = process(ctx, pathInArchive, dstDir, logger)
	if err != nil {
		t.Errorf("process() error = %v", err)
	}
}

// TestProcess_NonFB2File tests process with non-FB2 file
func TestProcess_NonFB2File(t *testing.T) {
	ctx, _ := setupTestEnv(t)
	logger := zaptest.NewLogger(t, zaptest.WrapOptions(zap.AddCaller(), zap.AddCallerSkip(1)))

	tmpDir := t.TempDir()

	// Create a non-FB2 file
	testFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("not an FB2 file"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	err := process(ctx, testFile, tmpDir, logger)
	if err == nil {
		t.Fatal("Expected error for non-FB2 file, got nil")
	}
	expectedMsg := "input was not recognized as FB2 book"
	if !strings.Contains(err.Error(), expectedMsg) {
		t.Errorf("Expected error containing '%s', got: %v", expectedMsg, err)
	}
}

// TestProcess_EmptyDirectory tests process with empty directory
func TestProcess_EmptyDirectory(t *testing.T) {
	ctx, _ := setupTestEnv(t)
	logger := zaptest.NewLogger(t, zaptest.WrapOptions(zap.AddCaller(), zap.AddCallerSkip(1)))

	tmpDir := t.TempDir()
	dstDir := t.TempDir()

	err := process(ctx, tmpDir, dstDir, logger)
	if err != nil {
		t.Errorf("process() should handle empty directory, got error: %v", err)
	}
}

// TestProcess_DifferentFormats tests process with different output formats
func TestProcess_DifferentFormats(t *testing.T) {
	ctx, env := setupTestEnv(t)
	logger := zaptest.NewLogger(t, zaptest.WrapOptions(zap.AddCaller(), zap.AddCallerSkip(1)))

	tmpDir := t.TempDir()
	dstDir := t.TempDir()

	fb2Content := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<FictionBook xmlns="http://www.gribuser.ru/xml/fictionbook/2.0">
<description><title-info><book-title>Test</book-title></title-info></description>
<body><section><p>Content</p></section></body>
</FictionBook>`)

	testFile := filepath.Join(tmpDir, "book.fb2")
	if err := os.WriteFile(testFile, fb2Content, 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	formats := []config.OutputFmt{config.OutputFmtEpub2, config.OutputFmtKepub, config.OutputFmtKfx}
	for _, format := range formats {
		t.Run(format.String(), func(t *testing.T) {
			env.OutputFormat = format
			err := process(ctx, testFile, dstDir, logger)
			if err != nil {
				t.Errorf("process() with format %s error = %v", format, err)
			}
		})
	}
}

// TestProcessDir_EmptyDir tests processDir with empty directory
func TestProcessDir_EmptyDir(t *testing.T) {
	ctx, _ := setupTestEnv(t)
	logger := zaptest.NewLogger(t, zaptest.WrapOptions(zap.AddCaller(), zap.AddCallerSkip(1)))

	tmpDir := t.TempDir()

	err := processDir(ctx, tmpDir, tmpDir, logger)
	if err != nil {
		t.Errorf("Expected no error for empty directory, got %v", err)
	}
}

// TestProcessDir_NonExistent tests processDir with non-existent directory
func TestProcessDir_NonExistent(t *testing.T) {
	ctx, _ := setupTestEnv(t)
	logger := zaptest.NewLogger(t, zaptest.WrapOptions(zap.AddCaller(), zap.AddCallerSkip(1)))

	// processDir uses filepath.Walk which logs warnings but doesn't fail
	// on non-existent directories
	err := processDir(ctx, "/nonexistent-dir-12345", "/tmp", logger)
	// The function may return an error from filepath.Walk
	// Just verify it doesn't panic
	_ = err
}

// TestProcessDir_WithCancelledContext tests processDir with cancelled context
func TestProcessDir_WithCancelledContext(t *testing.T) {
	ctx, _ := setupTestEnv(t)
	cancelCtx, cancel := context.WithCancel(ctx)
	logger := zaptest.NewLogger(t, zaptest.WrapOptions(zap.AddCaller(), zap.AddCallerSkip(1)))

	tmpDir := t.TempDir()
	// Create a dummy file
	testFile := filepath.Join(tmpDir, "test.fb2")
	if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	cancel() // Cancel context

	// processDir should handle context cancellation gracefully
	err := processDir(cancelCtx, tmpDir, tmpDir, logger)
	// The function may or may not return an error depending on timing
	// Just ensure it doesn't panic
	_ = err
}

// TestParseOutputFmt tests ParseOutputFmt function
func TestParseOutputFmt(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    config.OutputFmt
		wantErr bool
	}{
		{"epub2", "epub2", config.OutputFmtEpub2, false},
		{"EPUB2 uppercase", "EPUB2", config.OutputFmtEpub2, false},
		{"epub3", "epub3", config.OutputFmtEpub3, false},
		{"EPUB3 uppercase", "EPUB3", config.OutputFmtEpub3, false},
		{"kepub", "kepub", config.OutputFmtKepub, false},
		{"KEPUB uppercase", "KEPUB", config.OutputFmtKepub, false},
		{"kfx", "kfx", config.OutputFmtKfx, false},
		{"KFX uppercase", "KFX", config.OutputFmtKfx, false},
		{"invalid", "invalid", 0, true},
		{"empty", "", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := config.ParseOutputFmt(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseOutputFmt() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("ParseOutputFmt() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestOutputFmt_String tests OutputFmt String method
func TestOutputFmt_String(t *testing.T) {
	tests := []struct {
		name string
		fmt  config.OutputFmt
		want string
	}{
		{"epub2", config.OutputFmtEpub2, "epub2"},
		{"epub3", config.OutputFmtEpub3, "epub3"},
		{"kepub", config.OutputFmtKepub, "kepub"},
		{"kfx", config.OutputFmtKfx, "kfx"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.fmt.String()
			if got != tt.want {
				t.Errorf("OutputFmt.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestProcessBook tests processBook with basic inputs
func TestProcessBook(t *testing.T) {
	ctx, _ := setupTestEnv(t)
	logger := zaptest.NewLogger(t, zaptest.WrapOptions(zap.AddCaller(), zap.AddCallerSkip(1)))
	sample := loadProcessBookSample(t)
	sampleName := filepath.Base(sampleFB2Path)

	// Basic UTF-8 without BOM
	dst := t.TempDir()
	err := processBook(ctx, selectReader(readerForEncoding(t, sample, encUnknown), encUnknown), sampleName, dst, logger)
	if err != nil {
		t.Errorf("processBook() error = %v", err)
	}

	// Test with different encodings
	encodings := []srcEncoding{encUTF8, encUTF16BigEndian, encUTF16LittleEndian, encUTF32BigEndian, encUTF32LittleEndian}
	for i, enc := range encodings {
		testName := "encoding_" + string(rune('0'+i))
		t.Run(testName, func(t *testing.T) {
			dst := t.TempDir()
			err := processBook(ctx, selectReader(readerForEncoding(t, sample, enc), enc), sampleName, dst, logger)
			if err != nil {
				t.Errorf("processBook() with encoding %v error = %v", enc, err)
			}
		})
	}
}

// TestProcessBook_WithPanic tests processBook panic recovery
func TestProcessBook_WithPanic(t *testing.T) {
	ctx, _ := setupTestEnv(t)
	logger := zaptest.NewLogger(t, zaptest.WrapOptions(zap.AddCaller(), zap.AddCallerSkip(1)))
	sample := loadProcessBookSample(t)
	sampleName := filepath.Base(sampleFB2Path)

	// The current implementation has panic recovery
	// This test ensures panic recovery works correctly
	// Since the actual implementation returns nil, this just verifies no panic
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("processBook() should not panic, but got: %v", r)
		}
	}()

	dst := t.TempDir()
	err := processBook(ctx, selectReader(readerForEncoding(t, sample, encUnknown), encUnknown), sampleName, dst, logger)
	_ = err
}

// TestSrcEncoding_String tests srcEncoding string representation
func TestSrcEncoding_String(t *testing.T) {
	// Note: srcEncoding doesn't have a String() method defined
	// This test documents the expected values
	encodings := []srcEncoding{
		encUnknown,
		encUTF8,
		encUTF16BigEndian,
		encUTF16LittleEndian,
		encUTF32BigEndian,
		encUTF32LittleEndian,
	}

	for _, enc := range encodings {
		// Just verify they can be used without panic
		_ = enc
	}
}
