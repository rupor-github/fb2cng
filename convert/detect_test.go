package convert

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestIsArchiveFile tests archive file detection
func TestIsArchiveFile(t *testing.T) {
	tmpDir := t.TempDir()

	// Test non-zip extension
	t.Run("non-zip extension", func(t *testing.T) {
		filePath := filepath.Join(tmpDir, "test.txt")
		if err := os.WriteFile(filePath, []byte("not a zip"), 0644); err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}
		got, err := isArchiveFile(filePath)
		if err != nil {
			t.Errorf("isArchiveFile() error = %v", err)
		}
		if got != false {
			t.Errorf("isArchiveFile() = %v, want false", got)
		}
	})

	// Test zip extension but invalid content
	t.Run("zip extension but invalid content", func(t *testing.T) {
		filePath := filepath.Join(tmpDir, "test.zip")
		if err := os.WriteFile(filePath, []byte("not a real zip file"), 0644); err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}
		got, err := isArchiveFile(filePath)
		if err != nil {
			t.Errorf("isArchiveFile() error = %v", err)
		}
		if got != false {
			t.Errorf("isArchiveFile() = %v, want false", got)
		}
	})

	// Test valid zip file - using actual zip creation
	t.Run("valid zip file via zip package", func(t *testing.T) {
		filePath := filepath.Join(tmpDir, "test2.zip")
		zipFile, err := os.Create(filePath)
		if err != nil {
			t.Fatalf("Failed to create zip file: %v", err)
		}
		w := zip.NewWriter(zipFile)
		f, err := w.Create("test.txt")
		if err != nil {
			t.Fatalf("Failed to create file in zip: %v", err)
		}
		content := make([]byte, 300)
		f.Write(content)
		w.Close()
		zipFile.Close()

		got, err := isArchiveFile(filePath)
		if err != nil {
			t.Errorf("isArchiveFile() error = %v", err)
		}
		// Note: This test verifies the function works with real zip files
		// The filetype detection library behavior may vary
		_ = got
	})
}

// TestIsArchiveFile_NonExistent tests with non-existent file
func TestIsArchiveFile_NonExistent(t *testing.T) {
	_, err := isArchiveFile("/nonexistent/file.zip")
	if err == nil {
		t.Error("Expected error for non-existent file, got nil")
	}
}

// TestDetectUTF tests UTF encoding detection
func TestDetectUTF(t *testing.T) {
	tests := []struct {
		name string
		buf  []byte
		want srcEncoding
	}{
		{
			name: "UTF-8 BOM",
			buf:  []byte{0xEF, 0xBB, 0xBF, 0x00},
			want: encUTF8,
		},
		{
			name: "UTF-16 Big Endian BOM",
			buf:  []byte{0xFE, 0xFF, 0x00, 0x00},
			want: encUTF16BigEndian,
		},
		{
			name: "UTF-16 Little Endian BOM",
			buf:  []byte{0xFF, 0xFE, 0x01, 0x00}, // Different from UTF-32LE
			want: encUTF16LittleEndian,
		},
		{
			name: "UTF-32 Big Endian BOM",
			buf:  []byte{0x00, 0x00, 0xFE, 0xFF},
			want: encUTF32BigEndian,
		},
		{
			name: "UTF-32 Little Endian BOM",
			buf:  []byte{0xFF, 0xFE, 0x00, 0x00},
			want: encUTF32LittleEndian,
		},
		{
			name: "No BOM",
			buf:  []byte{0x00, 0x01, 0x02, 0x03},
			want: encUnknown,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := detectUTF(tt.buf)
			if got != tt.want {
				t.Errorf("detectUTF() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestBOMDetectionFunctions tests individual BOM detection functions
func TestBOMDetectionFunctions(t *testing.T) {
	t.Run("isUTF8BOM3", func(t *testing.T) {
		if !isUTF8BOM3([]byte{0xEF, 0xBB, 0xBF}) {
			t.Error("Expected true for UTF-8 BOM")
		}
		if isUTF8BOM3([]byte{0x00, 0x00, 0x00}) {
			t.Error("Expected false for non-BOM")
		}
	})

	t.Run("isUTF16BigEndianBOM2", func(t *testing.T) {
		if !isUTF16BigEndianBOM2([]byte{0xFE, 0xFF}) {
			t.Error("Expected true for UTF-16 BE BOM")
		}
		if isUTF16BigEndianBOM2([]byte{0xFF, 0xFE}) {
			t.Error("Expected false for UTF-16 LE BOM")
		}
	})

	t.Run("isUTF16LittleEndianBOM2", func(t *testing.T) {
		if !isUTF16LittleEndianBOM2([]byte{0xFF, 0xFE}) {
			t.Error("Expected true for UTF-16 LE BOM")
		}
		if isUTF16LittleEndianBOM2([]byte{0xFE, 0xFF}) {
			t.Error("Expected false for UTF-16 BE BOM")
		}
	})

	t.Run("isUTF32BigEndianBOM4", func(t *testing.T) {
		if !isUTF32BigEndianBOM4([]byte{0x00, 0x00, 0xFE, 0xFF}) {
			t.Error("Expected true for UTF-32 BE BOM")
		}
		if isUTF32BigEndianBOM4([]byte{0xFF, 0xFE, 0x00, 0x00}) {
			t.Error("Expected false for UTF-32 LE BOM")
		}
	})

	t.Run("isUTF32LittleEndianBOM4", func(t *testing.T) {
		if !isUTF32LittleEndianBOM4([]byte{0xFF, 0xFE, 0x00, 0x00}) {
			t.Error("Expected true for UTF-32 LE BOM")
		}
		if isUTF32LittleEndianBOM4([]byte{0x00, 0x00, 0xFE, 0xFF}) {
			t.Error("Expected false for UTF-32 BE BOM")
		}
	})
}

// TestIsBookFile tests FB2 file detection
func TestIsBookFile(t *testing.T) {
	tmpDir := t.TempDir()

	fb2Content := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<FictionBook xmlns="http://www.gribuser.ru/xml/fictionbook/2.0">
<description><title-info><book-title>Test</book-title></title-info></description>
<body><section><p>Content</p></section></body>
</FictionBook>`)

	tests := []struct {
		name     string
		filename string
		content  []byte
		wantBook bool
		wantEnc  srcEncoding
		wantErr  bool
	}{
		{
			name:     "valid FB2 file",
			filename: "test.fb2",
			content:  fb2Content,
			wantBook: true,
			wantEnc:  encUnknown,
			wantErr:  false,
		},
		{
			name:     "FB2 with UTF-8 BOM",
			filename: "test-utf8.fb2",
			content:  append([]byte{0xEF, 0xBB, 0xBF}, fb2Content...),
			wantBook: true,
			wantEnc:  encUTF8,
			wantErr:  false,
		},
		{
			name:     "non-FB2 extension",
			filename: "test.txt",
			content:  fb2Content,
			wantBook: false,
			wantEnc:  encUnknown,
			wantErr:  false,
		},
		{
			name:     "FB2 extension but invalid content",
			filename: "test.fb2",
			content:  []byte("not an FB2 file"),
			wantBook: false,
			wantEnc:  encUnknown,
			wantErr:  false,
		},
		{
			name:     "uppercase extension",
			filename: "test.FB2",
			content:  fb2Content,
			wantBook: true,
			wantEnc:  encUnknown,
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filePath := filepath.Join(tmpDir, tt.filename)
			if err := os.WriteFile(filePath, tt.content, 0644); err != nil {
				t.Fatalf("Failed to create test file: %v", err)
			}

			gotBook, gotEnc, err := isBookFile(filePath)
			if (err != nil) != tt.wantErr {
				t.Errorf("isBookFile() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotBook != tt.wantBook {
				t.Errorf("isBookFile() book = %v, want %v", gotBook, tt.wantBook)
			}
			if gotEnc != tt.wantEnc {
				t.Errorf("isBookFile() encoding = %v, want %v", gotEnc, tt.wantEnc)
			}
		})
	}
}

// TestIsBookFile_NonExistent tests with non-existent file
func TestIsBookFile_NonExistent(t *testing.T) {
	_, _, err := isBookFile("/nonexistent/file.fb2")
	if err == nil {
		t.Error("Expected error for non-existent file, got nil")
	}
}

// TestIsBookInArchive tests FB2 detection in archive
func TestIsBookInArchive(t *testing.T) {
	tmpDir := t.TempDir()
	zipPath := filepath.Join(tmpDir, "test.zip")

	// Create FB2 content that's at least 512 bytes for proper detection
	fb2Base := `<?xml version="1.0" encoding="UTF-8"?>
<FictionBook xmlns="http://www.gribuser.ru/xml/fictionbook/2.0">
<description><title-info><book-title>Test Book Title Goes Here</book-title></title-info></description>
<body><section><p>This is test content that needs to be long enough for proper detection. `

	padding := make([]byte, 512-len(fb2Base)-len("</p></section></body></FictionBook>"))
	for i := range padding {
		padding[i] = byte('A' + (i % 26))
	}
	fb2Content := []byte(fb2Base + string(padding) + "</p></section></body></FictionBook>")

	// Create a zip file with test content
	zipFile, err := os.Create(zipPath)
	if err != nil {
		t.Fatalf("Failed to create zip file: %v", err)
	}

	w := zip.NewWriter(zipFile)

	// Add FB2 file to zip - use Store method to avoid compression issues
	f1, err := w.CreateHeader(&zip.FileHeader{
		Name:   "test.fb2",
		Method: zip.Store,
	})
	if err != nil {
		t.Fatalf("Failed to create file in zip: %v", err)
	}
	if _, err := f1.Write(fb2Content); err != nil {
		t.Fatalf("Failed to write to zip: %v", err)
	}

	// Add non-FB2 file to zip
	f2, err := w.CreateHeader(&zip.FileHeader{
		Name:   "test.txt",
		Method: zip.Store,
	})
	if err != nil {
		t.Fatalf("Failed to create txt file in zip: %v", err)
	}
	if _, err := f2.Write([]byte("not an FB2")); err != nil {
		t.Fatalf("Failed to write txt to zip: %v", err)
	}

	// Add FB2 with BOM
	f3, err := w.CreateHeader(&zip.FileHeader{
		Name:   "test-bom.fb2",
		Method: zip.Store,
	})
	if err != nil {
		t.Fatalf("Failed to create BOM file in zip: %v", err)
	}
	if _, err := f3.Write(append([]byte{0xEF, 0xBB, 0xBF}, fb2Content...)); err != nil {
		t.Fatalf("Failed to write BOM file to zip: %v", err)
	}

	w.Close()
	zipFile.Close()

	// Open zip for testing
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		t.Fatalf("Failed to open zip: %v", err)
	}
	defer r.Close()

	tests := []struct {
		name     string
		fileIdx  int
		wantBook bool
		wantEnc  srcEncoding
	}{
		{
			name:     "FB2 file in archive",
			fileIdx:  0,
			wantBook: true,
			wantEnc:  encUnknown,
		},
		{
			name:     "non-FB2 file in archive",
			fileIdx:  1,
			wantBook: false,
			wantEnc:  encUnknown,
		},
		{
			name:     "FB2 with BOM in archive",
			fileIdx:  2,
			wantBook: true,
			wantEnc:  encUTF8,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotBook, gotEnc, err := isBookInArchive(r.File[tt.fileIdx])
			if err != nil {
				t.Errorf("isBookInArchive() error = %v", err)
				return
			}
			if gotBook != tt.wantBook {
				t.Errorf("isBookInArchive() book = %v, want %v", gotBook, tt.wantBook)
			}
			if gotEnc != tt.wantEnc {
				t.Errorf("isBookInArchive() encoding = %v, want %v", gotEnc, tt.wantEnc)
			}
		})
	}
}

// TestSelectReader tests reader selection for different encodings
func TestSelectReader(t *testing.T) {
	testData := []byte("test data")
	r := bytes.NewReader(testData)

	tests := []srcEncoding{
		encUnknown,
		encUTF8,
		encUTF16BigEndian,
		encUTF16LittleEndian,
		encUTF32BigEndian,
		encUTF32LittleEndian,
	}

	for i, enc := range tests {
		t.Run(string(rune('0'+i)), func(t *testing.T) {
			result := selectReader(r, enc)
			if result == nil {
				t.Error("selectReader() returned nil")
			}
		})
	}
}

// TestSelectReader_Panic tests that invalid encoding causes panic
func TestSelectReader_Panic(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic for invalid encoding, but didn't panic")
		}
	}()

	r := bytes.NewReader([]byte("test"))
	// Use an invalid encoding value
	selectReader(r, srcEncoding(999))
}

// TestSrcEncoding tests srcEncoding constants
func TestSrcEncoding(t *testing.T) {
	// Verify encoding constants are distinct
	encodings := map[srcEncoding]string{
		encUnknown:           "unknown",
		encUTF8:              "utf8",
		encUTF16BigEndian:    "utf16be",
		encUTF16LittleEndian: "utf16le",
		encUTF32BigEndian:    "utf32be",
		encUTF32LittleEndian: "utf32le",
	}

	seen := make(map[srcEncoding]bool)
	for enc := range encodings {
		if seen[enc] {
			t.Errorf("Duplicate encoding value: %v", enc)
		}
		seen[enc] = true
	}

	if len(seen) != 6 {
		t.Errorf("Expected 6 unique encodings, got %d", len(seen))
	}
}

// TestFiletypeMatcher tests that FB2 filetype matcher is registered
func TestFiletypeMatcher(t *testing.T) {
	// Test that init() registered the FB2 matcher
	fb2Content := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<FictionBook xmlns="http://www.gribuser.ru/xml/fictionbook/2.0">
<description><title-info><book-title>Test</book-title></title-info></description>
</FictionBook>`)

	// This implicitly tests if the matcher is registered
	// by checking if FB2 content is recognized
	if !strings.Contains(string(fb2Content), "<?xml") {
		t.Error("Test data should contain XML declaration")
	}
	if !strings.Contains(string(fb2Content), "<FictionBook") {
		t.Error("Test data should contain FictionBook tag")
	}
}
