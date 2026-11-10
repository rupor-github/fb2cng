package epub

import (
	"archive/zip"
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/beevik/etree"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
	"golang.org/x/text/language"

	"fbc/common"
	"fbc/config"
	"fbc/content"
	"fbc/convert/publication"
	"fbc/fb2"
	"fbc/state"
)

func setupTestLogger(t *testing.T) *zap.Logger {
	return zaptest.NewLogger(t, zaptest.WrapOptions(zap.AddCaller()))
}

func setupTestContext(t *testing.T) (context.Context, *state.LocalEnv, *zap.Logger) {
	logger := setupTestLogger(t)
	cfg, err := config.LoadConfiguration("")
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	ctx := state.ContextWithEnv(context.Background())
	env := state.EnvFromContext(ctx)
	env.Log = logger
	env.Cfg = cfg
	return ctx, env, logger
}

func TestWriteMimetype(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	err := writeMimetype(zw)
	if err != nil {
		t.Fatalf("writeMimetype() error = %v", err)
	}

	if err := zw.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}

	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatalf("open zip: %v", err)
	}

	if len(zr.File) != 1 {
		t.Fatalf("Expected 1 file, got %d", len(zr.File))
	}

	f := zr.File[0]
	if f.Name != "mimetype" {
		t.Errorf("Filename = %v, want mimetype", f.Name)
	}

	if f.Method != zip.Store {
		t.Errorf("Compression method = %v, want Store (0)", f.Method)
	}

	rc, err := f.Open()
	if err != nil {
		t.Fatalf("open file: %v", err)
	}
	defer rc.Close()

	content, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}

	if string(content) != mimetypeContent {
		t.Errorf("Content = %v, want %v", string(content), mimetypeContent)
	}
}

func TestWriteContainer(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	err := writeContainer(zw)
	if err != nil {
		t.Fatalf("writeContainer() error = %v", err)
	}

	if err := zw.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}

	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatalf("open zip: %v", err)
	}

	if len(zr.File) != 1 {
		t.Fatalf("Expected 1 file, got %d", len(zr.File))
	}

	f := zr.File[0]
	if f.Name != "META-INF/container.xml" {
		t.Errorf("Filename = %v, want META-INF/container.xml", f.Name)
	}

	rc, err := f.Open()
	if err != nil {
		t.Fatalf("open file: %v", err)
	}
	defer rc.Close()

	content, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}

	doc := etree.NewDocument()
	if err := doc.ReadFromBytes(content); err != nil {
		t.Fatalf("parse XML: %v", err)
	}

	container := doc.SelectElement("container")
	if container == nil {
		t.Fatal("Missing <container> element")
	}

	rootfiles := container.SelectElement("rootfiles")
	if rootfiles == nil {
		t.Fatal("Missing <rootfiles> element")
	}

	rootfile := rootfiles.SelectElement("rootfile")
	if rootfile == nil {
		t.Fatal("Missing <rootfile> element")
	}

	fullPath := rootfile.SelectAttrValue("full-path", "")
	if !strings.Contains(fullPath, "content.opf") {
		t.Errorf("full-path = %v, should contain content.opf", fullPath)
	}
}

func TestCopyFile(t *testing.T) {
	tmpDir := t.TempDir()

	srcPath := filepath.Join(tmpDir, "source.txt")
	dstPath := filepath.Join(tmpDir, "dest.txt")

	testContent := "test file content"
	if err := os.WriteFile(srcPath, []byte(testContent), 0644); err != nil {
		t.Fatalf("write source file: %v", err)
	}

	err := copyFile(srcPath, dstPath)
	if err != nil {
		t.Fatalf("copyFile() error = %v", err)
	}

	content, err := os.ReadFile(dstPath)
	if err != nil {
		t.Fatalf("read dest file: %v", err)
	}

	if string(content) != testContent {
		t.Errorf("Content = %v, want %v", string(content), testContent)
	}
}

func TestCopyFile_NonExistentSource(t *testing.T) {
	tmpDir := t.TempDir()
	srcPath := filepath.Join(tmpDir, "nonexistent.txt")
	dstPath := filepath.Join(tmpDir, "dest.txt")

	err := copyFile(srcPath, dstPath)
	if err == nil {
		t.Error("copyFile() should return error for non-existent source")
	}
}

func TestWriteXMLToZip(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	doc := etree.NewDocument()
	doc.CreateProcInst("xml", `version="1.0"`)
	root := doc.CreateElement("test")
	root.CreateElement("child").SetText("content")

	err := writeXMLToZip(zw, "test.xml", doc)
	if err != nil {
		t.Fatalf("writeXMLToZip() error = %v", err)
	}

	if err := zw.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}

	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatalf("open zip: %v", err)
	}

	if len(zr.File) != 1 {
		t.Fatalf("Expected 1 file, got %d", len(zr.File))
	}

	f := zr.File[0]
	if f.Name != "test.xml" {
		t.Errorf("Filename = %v, want test.xml", f.Name)
	}

	rc, err := f.Open()
	if err != nil {
		t.Fatalf("open file: %v", err)
	}
	defer rc.Close()

	content, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}

	readDoc := etree.NewDocument()
	if err := readDoc.ReadFromBytes(content); err != nil {
		t.Fatalf("parse XML: %v", err)
	}

	child := readDoc.FindElement("//child")
	if child == nil || child.Text() != "content" {
		t.Errorf("Child element text = %v, want 'content'", child.Text())
	}
}

func TestWriteDataToZip(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	testData := []byte("test data content")
	err := writeDataToZip(zw, "data.bin", testData)
	if err != nil {
		t.Fatalf("writeDataToZip() error = %v", err)
	}

	if err := zw.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}

	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatalf("open zip: %v", err)
	}

	if len(zr.File) != 1 {
		t.Fatalf("Expected 1 file, got %d", len(zr.File))
	}

	f := zr.File[0]
	rc, err := f.Open()
	if err != nil {
		t.Fatalf("open file: %v", err)
	}
	defer rc.Close()

	content, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}

	if !bytes.Equal(content, testData) {
		t.Errorf("Content = %v, want %v", content, testData)
	}
}

func TestGenerate_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Load real test FB2 file
	testFB2Path := "../../testdata/_Test.fb2"
	fb2File, err := os.Open(testFB2Path)
	if err != nil {
		t.Fatalf("Failed to open test FB2: %v", err)
	}
	defer fb2File.Close()

	ctx, env, log := setupTestContext(t)
	env.Overwrite = true
	tmpDir := t.TempDir()

	// Prepare content using the same function as the main converter
	c, err := content.Prepare(ctx, fb2File, testFB2Path, common.OutputFmtEpub2, log)
	if err != nil {
		t.Fatalf("Failed to prepare content: %v", err)
	}
	// WorkDir is already set by Prepare
	t.Logf("WorkDir from Prepare: %s", c.WorkDir)
	t.Logf("Test tmpDir: %s", tmpDir)

	outputPath := filepath.Join(tmpDir, "test.epub")
	t.Logf("Output path: %s", outputPath)
	cfg := env.Cfg.Document

	err = Generate(ctx, c, outputPath, "", &cfg, log)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	// List files in tmpDir for debugging
	files, _ := os.ReadDir(tmpDir)
	t.Logf("Files in tmpDir: %d", len(files))
	for _, f := range files {
		t.Logf("  - %s", f.Name())
	}

	// Verify output file exists
	if _, err := os.Stat(outputPath); err != nil {
		if os.IsNotExist(err) {
			t.Errorf("Output file was not created at %s", outputPath)
			// Check if it was created in workdir instead
			workdirPath := filepath.Join(c.WorkDir, "book.epub.tmp")
			if _, err2 := os.Stat(workdirPath); err2 == nil {
				t.Logf("File exists in workdir: %s", workdirPath)
			}
		} else {
			t.Fatalf("Error checking output file: %v", err)
		}
	}

	// Verify it's a valid zip
	zr, err := zip.OpenReader(outputPath)
	if err != nil {
		t.Fatalf("Failed to open output as zip: %v", err)
	}
	defer zr.Close()

	// Check for required files
	requiredFiles := []string{
		"mimetype",
		"META-INF/container.xml",
		"OEBPS/content.opf",
	}

	foundFiles := make(map[string]bool)
	for _, f := range zr.File {
		foundFiles[f.Name] = true
	}

	for _, required := range requiredFiles {
		if !foundFiles[required] {
			t.Errorf("Required file missing: %s", required)
		}
	}

	// Verify some content from the test file
	if !strings.Contains(c.Book.Description.TitleInfo.BookTitle.Value, "Тестовая книга") {
		t.Error("Expected Russian test book title")
	}
}

func TestGenerate_WithFootnotes(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Load real test FB2 file which has footnotes
	testFB2Path := "../../testdata/_Test.fb2"
	fb2File, err := os.Open(testFB2Path)
	if err != nil {
		t.Fatalf("Failed to open test FB2: %v", err)
	}
	defer fb2File.Close()

	ctx, env, log := setupTestContext(t)
	env.Overwrite = true
	tmpDir := t.TempDir()

	// Prepare content using the same function as the main converter
	c, err := content.Prepare(ctx, fb2File, testFB2Path, common.OutputFmtEpub2, log)
	if err != nil {
		t.Fatalf("Failed to prepare content: %v", err)
	}
	// WorkDir is set by Prepare, but we want output in tmpDir

	// Verify the test file actually has footnotes
	hasFootnotes := false
	for _, body := range c.Book.Bodies {
		if body.Footnotes() {
			hasFootnotes = true
			break
		}
	}
	if !hasFootnotes {
		t.Skip("Test FB2 file doesn't have footnotes")
	}

	outputPath := filepath.Join(tmpDir, "test-footnotes.epub")
	cfg := env.Cfg.Document

	err = Generate(ctx, c, outputPath, "", &cfg, log)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	// Verify file was created
	if _, err := os.Stat(outputPath); os.IsNotExist(err) {
		t.Fatalf("Output file was not created at %s", outputPath)
	}

	// Open and verify the epub
	zr, err := zip.OpenReader(outputPath)
	if err != nil {
		t.Fatalf("Failed to open output: %v", err)
	}
	defer zr.Close()

	// Find and verify footnote chapter
	var footnoteDoc *etree.Document
	for _, f := range zr.File {
		if strings.Contains(f.Name, ".xhtml") && strings.Contains(f.Name, "index") {
			rc, err := f.Open()
			if err != nil {
				continue
			}
			content, _ := io.ReadAll(rc)
			rc.Close()

			doc := etree.NewDocument()
			if err := doc.ReadFromBytes(content); err != nil {
				continue
			}

			// Check if this is the footnote document
			divs := doc.FindElements("//div[@class='footnote-body']")
			if len(divs) > 0 {
				footnoteDoc = doc
				break
			}
		}
	}

	if footnoteDoc != nil {
		// Verify unique IDs for all footnote bodies
		bodyDivs := footnoteDoc.FindElements("//div[@class='footnote-body']")

		ids := make(map[string]bool)
		for _, div := range bodyDivs {
			id := div.SelectAttrValue("id", "")
			if id == "" {
				t.Error("Footnote body div has no ID")
				continue
			}
			if ids[id] {
				t.Errorf("Duplicate footnote body ID: %s", id)
			}
			ids[id] = true
		}

		t.Logf("Found %d unique footnote body IDs", len(ids))
	} else {
		t.Log("No footnote chapter found (might not have separate footnote bodies)")
	}
}

func TestGenerate_OverwriteProtection(t *testing.T) {
	ctx, env, log := setupTestContext(t)
	env.Overwrite = false // Disable overwrite
	tmpDir := t.TempDir()

	c := &content.Content{
		Book: &fb2.FictionBook{
			Description: fb2.Description{
				TitleInfo: fb2.TitleInfo{
					BookTitle: fb2.TextField{Value: "Test"},
					Lang:      language.Make("en"),
				},
			},
			Bodies: []fb2.Body{{Kind: fb2.BodyMain}},
		},
		OutputFormat: common.OutputFmtEpub2,
		ImagesIndex:  make(fb2.BookImages),
		WorkDir:      tmpDir,
	}

	outputPath := filepath.Join(tmpDir, "existing.epub")

	// Create existing file
	if err := os.WriteFile(outputPath, []byte("existing"), 0644); err != nil {
		t.Fatalf("Create existing file: %v", err)
	}

	cfg := &config.DocumentConfig{}
	err := Generate(ctx, c, outputPath, "", cfg, log)

	// Note: Generate() does not check env.Overwrite - that check is done in convert/run.go
	// This test verifies that Generate can overwrite files when called directly
	if err != nil {
		t.Errorf("Generate() error = %v", err)
	}
}

func TestGenerate_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, _, log := setupTestContext(t)
	tmpDir := t.TempDir()

	c := &content.Content{
		Book:         &fb2.FictionBook{},
		OutputFormat: common.OutputFmtEpub2,
		ImagesIndex:  make(fb2.BookImages),
		WorkDir:      tmpDir,
	}

	outputPath := filepath.Join(tmpDir, "test.epub")
	cfg := &config.DocumentConfig{}

	err := Generate(ctx, c, outputPath, "", cfg, log)
	if err == nil {
		t.Error("Generate() should fail with cancelled context")
	}
}

func TestCopyZipWithoutDataDescriptors(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a test zip file with data descriptors
	srcPath := filepath.Join(tmpDir, "source.zip")
	dstPath := filepath.Join(tmpDir, "dest.zip")

	// Create source zip
	srcFile, err := os.Create(srcPath)
	if err != nil {
		t.Fatalf("create source: %v", err)
	}

	zw := zip.NewWriter(srcFile)
	w, err := zw.Create("test.txt")
	if err != nil {
		t.Fatalf("create entry: %v", err)
	}
	_, err = w.Write([]byte("test content"))
	if err != nil {
		t.Fatalf("write content: %v", err)
	}
	zw.Close()
	srcFile.Close()

	// Copy without data descriptors
	err = copyZipWithoutDataDescriptors(srcPath, dstPath)
	if err != nil {
		t.Fatalf("copyZipWithoutDataDescriptors() error = %v", err)
	}

	// Verify destination exists and is valid
	if _, err := os.Stat(dstPath); os.IsNotExist(err) {
		t.Error("Destination file not created")
	}

	// Read and verify content
	zr, err := zip.OpenReader(dstPath)
	if err != nil {
		t.Fatalf("open dest zip: %v", err)
	}
	defer zr.Close()

	if len(zr.File) != 1 {
		t.Errorf("Expected 1 file in dest zip, got %d", len(zr.File))
	}
}

func TestCopyZipWithoutDataDescriptors_NonExistentSource(t *testing.T) {
	tmpDir := t.TempDir()
	srcPath := filepath.Join(tmpDir, "nonexistent.zip")
	dstPath := filepath.Join(tmpDir, "dest.zip")

	err := copyZipWithoutDataDescriptors(srcPath, dstPath)
	if err == nil {
		t.Error("Expected error for non-existent source")
	}
}

func TestWriteStylesheet(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	c := &content.Content{
		Book: &fb2.FictionBook{
			Stylesheets: []fb2.Stylesheet{
				{Type: "text/css", Data: "body { font-family: serif; }"},
				{Type: "text/css", Data: "/* custom */"}},
		},
		OutputFormat: common.OutputFmtEpub2,
	}

	sheet, err := publication.AssembleStylesheet(c.Book.Stylesheets, c.OutputFormat)
	if err != nil {
		t.Fatal(err)
	}
	err = writeStylesheet(zw, sheet)
	if err != nil {
		t.Fatalf("writeStylesheet() error = %v", err)
	}

	if err := zw.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}

	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatalf("open zip: %v", err)
	}

	var foundCSS bool
	for _, f := range zr.File {
		if strings.HasSuffix(f.Name, ".css") {
			foundCSS = true
			rc, err := f.Open()
			if err != nil {
				t.Fatalf("open css: %v", err)
			}
			content, _ := io.ReadAll(rc)
			rc.Close()

			if !strings.Contains(string(content), "font-family") {
				t.Error("CSS should contain font-family")
			}
		}
	}

	if !foundCSS {
		t.Error("CSS file not found in zip")
	}
}

func TestWriteStylesheetWithResources(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	// Create stylesheet with font resource
	c := &content.Content{
		Book: &fb2.FictionBook{
			Stylesheets: []fb2.Stylesheet{
				{Type: "text/css", Data: "body { font-family: serif; }"},
				{
					Type: "text/css",
					Data: "@font-face { font-family: 'MyFont'; src: url('#myfont'); }",
					Resources: []fb2.StylesheetResource{
						{
							OriginalURL: "#myfont",
							ResolvedID:  "myfont",
							MimeType:    "font/woff2",
							Data:        []byte("fake font data"),
							Filename:    "fonts/myfont.woff2", // Full path with directory
						},
					},
				},
			},
		},
		OutputFormat: common.OutputFmtEpub2,
	}

	sheet, err := publication.AssembleStylesheet(c.Book.Stylesheets, c.OutputFormat)
	if err != nil {
		t.Fatal(err)
	}
	err = writeStylesheet(zw, sheet)
	if err != nil {
		t.Fatalf("writeStylesheet() error = %v", err)
	}

	if err := zw.Close(); err != nil {
		t.Fatalf("close zip error = %v", err)
	}

	// Verify the zip contains both stylesheet and font
	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatalf("zip reader error = %v", err)
	}

	foundCSS := false
	foundFont := false

	for _, f := range zr.File {
		if f.Name == "OEBPS/stylesheet.css" {
			foundCSS = true
			// Read and verify CSS was rewritten
			rc, err := f.Open()
			if err != nil {
				t.Fatalf("open css error = %v", err)
			}
			cssData, _ := io.ReadAll(rc)
			rc.Close()
			cssStr := string(cssData)

			// Should contain rewritten URL
			if !strings.Contains(cssStr, `url("fonts/myfont.woff2")`) {
				t.Errorf("CSS should contain rewritten URL, got: %s", cssStr)
			}
			// Should not contain only original URL (it's OK if both exist during transition)
			if strings.Contains(cssStr, `url('#myfont')`) && !strings.Contains(cssStr, `url("fonts/myfont.woff2")`) {
				t.Errorf("CSS should have rewritten original URL")
			}
		}
		if f.Name == "OEBPS/fonts/myfont.woff2" {
			foundFont = true
			// Verify font data
			rc, err := f.Open()
			if err != nil {
				t.Fatalf("open font error = %v", err)
			}
			fontData, _ := io.ReadAll(rc)
			rc.Close()
			if string(fontData) != "fake font data" {
				t.Errorf("font data mismatch")
			}
		}
	}

	if !foundCSS {
		t.Error("stylesheet.css not found in zip")
	}
	if !foundFont {
		t.Error("font resource not found in zip")
	}
}

func TestWriteStylesheetWithNonFontResource(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	// Create stylesheet with non-font resource (e.g., SVG image)
	c := &content.Content{
		Book: &fb2.FictionBook{
			Stylesheets: []fb2.Stylesheet{
				{Type: "text/css", Data: "body { margin: 0; }"},
				{
					Type: "text/css",
					Data: "background: url('#mysvg');",
					Resources: []fb2.StylesheetResource{
						{
							OriginalURL: "#mysvg",
							ResolvedID:  "mysvg",
							MimeType:    "image/svg+xml",
							Data:        []byte("<svg></svg>"),
							Filename:    "other/mysvg.svg", // Full path with directory
						},
					},
				},
			},
		},
		OutputFormat: common.OutputFmtEpub2,
	}

	sheet, err := publication.AssembleStylesheet(c.Book.Stylesheets, c.OutputFormat)
	if err != nil {
		t.Fatal(err)
	}
	err = writeStylesheet(zw, sheet)
	if err != nil {
		t.Fatalf("writeStylesheet() error = %v", err)
	}

	if err := zw.Close(); err != nil {
		t.Fatalf("close zip error = %v", err)
	}

	// Verify the resource goes to "other" directory
	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatalf("zip reader error = %v", err)
	}

	foundResource := false
	for _, f := range zr.File {
		if f.Name == "OEBPS/other/mysvg.svg" {
			foundResource = true
		}
		if f.Name == "OEBPS/fonts/mysvg.svg" {
			t.Error("non-font resource should not be in fonts directory")
		}
	}

	if !foundResource {
		t.Error("SVG resource not found in other/ directory")
	}
}

func TestWriteImages(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	images := fb2.BookImages{
		"img1": &fb2.BookImage{
			Filename: "test.jpg",
			Data:     []byte("fake image data"),
			MimeType: "image/jpeg",
		},
	}

	err := writeImages(zw, images)
	if err != nil {
		t.Fatalf("writeImages() error = %v", err)
	}

	if err := zw.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}

	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatalf("open zip: %v", err)
	}

	var foundImage bool
	for _, f := range zr.File {
		if strings.Contains(f.Name, "test.jpg") {
			foundImage = true
			rc, err := f.Open()
			if err != nil {
				t.Fatalf("open image: %v", err)
			}
			data, _ := io.ReadAll(rc)
			rc.Close()

			if string(data) != "fake image data" {
				t.Error("Image data doesn't match")
			}
		}
	}

	if !foundImage {
		t.Error("Image file not found in zip")
	}
}

func TestWriteXHTMLChapter(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	doc := etree.NewDocument()
	root := doc.CreateElement("html").CreateElement("body")
	p := root.CreateElement("p")
	p.SetText("Chapter content")

	chapter := publication.Chapter{
		ID:       "ch01",
		Filename: "ch01.xhtml",
		Title:    "Test Chapter",
		Doc:      doc,
	}

	err := writeXHTMLChapter(zw, &chapter)
	if err != nil {
		t.Fatalf("writeXHTMLChapter() error = %v", err)
	}

	if err := zw.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}

	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatalf("open zip: %v", err)
	}

	var foundChapter bool
	for _, f := range zr.File {
		if strings.Contains(f.Name, "ch01.xhtml") {
			foundChapter = true
			rc, err := f.Open()
			if err != nil {
				t.Fatalf("open chapter: %v", err)
			}
			content, _ := io.ReadAll(rc)
			rc.Close()

			if !strings.Contains(string(content), "Chapter content") {
				t.Error("Chapter content not found")
			}
		}
	}

	if !foundChapter {
		t.Error("Chapter file not found in zip")
	}
}

func TestWriteOPF(t *testing.T) {
	_, env, log := setupTestContext(t)
	cfg := &env.Cfg.Document

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	c := &content.Content{
		Book: &fb2.FictionBook{
			Description: fb2.Description{
				TitleInfo: fb2.TitleInfo{
					BookTitle: fb2.TextField{Value: "Test Book"},
					Lang:      language.Make("en"),
					Authors: []fb2.Author{
						{
							FirstName:  "John",
							MiddleName: "Q",
							LastName:   "Public",
						},
					},
				},
				DocumentInfo: fb2.DocumentInfo{
					ID: "test-book-123",
				},
			},
		},
		OutputFormat: common.OutputFmtEpub2,
		ImagesIndex:  make(fb2.BookImages),
		CoverID:      "cover1",
	}

	chapters := []publication.Chapter{
		{
			ID:       "ch01",
			Filename: "ch01.xhtml",
			Title:    "Chapter 1",
		},
	}

	err := writeOPF(zw, c, cfg, chapters, log)
	if err != nil {
		t.Fatalf("writeOPF() error = %v", err)
	}

	if err := zw.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}

	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatalf("open zip: %v", err)
	}

	var foundOPF bool
	for _, f := range zr.File {
		if strings.HasSuffix(f.Name, "content.opf") {
			foundOPF = true
			rc, err := f.Open()
			if err != nil {
				t.Fatalf("open opf: %v", err)
			}
			content, _ := io.ReadAll(rc)
			rc.Close()

			if !strings.Contains(string(content), "Test Book") {
				t.Error("OPF should contain book title")
			}
			if !strings.Contains(string(content), "Public John Q") {
				t.Error("OPF should contain author name")
			}
			if !strings.Contains(string(content), "test-book-123") {
				t.Error("OPF should contain book ID")
			}

			// Parse to verify structure
			doc := etree.NewDocument()
			if err := doc.ReadFromBytes(content); err != nil {
				t.Fatalf("parse OPF: %v", err)
			}

			pkg := doc.SelectElement("package")
			if pkg == nil {
				t.Error("Missing package element")
			}

			metadata := pkg.SelectElement("metadata")
			if metadata == nil {
				t.Error("Missing metadata element")
			}

			manifest := pkg.SelectElement("manifest")
			if manifest == nil {
				t.Error("Missing manifest element")
			}

			spine := pkg.SelectElement("spine")
			if spine == nil {
				t.Error("Missing spine element")
			}
		}
	}

	if !foundOPF {
		t.Error("OPF file not found in zip")
	}
}

func TestWriteOPF_Epub3(t *testing.T) {
	_, env, log := setupTestContext(t)
	cfg := &env.Cfg.Document

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	c := &content.Content{
		Book: &fb2.FictionBook{
			Description: fb2.Description{
				TitleInfo: fb2.TitleInfo{
					BookTitle: fb2.TextField{Value: "Test Book EPUB3"},
					Lang:      language.Make("en"),
					Authors: []fb2.Author{
						{FirstName: "Jane", LastName: "Doe"},
					},
				},
				DocumentInfo: fb2.DocumentInfo{
					ID: "test-book-epub3",
				},
			},
		},
		OutputFormat: common.OutputFmtEpub3,
		ImagesIndex: fb2.BookImages{
			"cover": &fb2.BookImage{
				Filename: "images/cover.jpg",
				MimeType: "image/jpeg",
			},
		},
		CoverID: "cover",
	}

	chapters := []publication.Chapter{
		{ID: "ch01", Filename: "ch01.xhtml", Title: "Chapter 1"},
	}

	err := writeOPF(zw, c, cfg, chapters, log)
	if err != nil {
		t.Fatalf("writeOPF() error = %v", err)
	}

	if err := zw.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}

	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatalf("open zip: %v", err)
	}

	for _, f := range zr.File {
		if strings.HasSuffix(f.Name, "content.opf") {
			rc, err := f.Open()
			if err != nil {
				t.Fatalf("open opf: %v", err)
			}
			content, _ := io.ReadAll(rc)
			rc.Close()

			doc := etree.NewDocument()
			if err := doc.ReadFromBytes(content); err != nil {
				t.Fatalf("parse OPF: %v", err)
			}

			pkg := doc.SelectElement("package")
			version := pkg.SelectAttrValue("version", "")
			if version != "3.0" {
				t.Errorf("EPUB3 version = %v, want 3.0", version)
			}

			metadata := pkg.SelectElement("metadata")
			if metadata == nil {
				t.Fatal("missing metadata")
			}
			coverMeta := metadata.FindElement("meta[@name='cover']")
			if coverMeta == nil {
				t.Fatal("missing legacy cover meta for EPUB3 Apple compatibility")
			}
			if got := coverMeta.SelectAttrValue("content", ""); got != "book-cover-image" {
				t.Errorf("cover meta content = %q, want book-cover-image", got)
			}

			manifest := pkg.SelectElement("manifest")
			if manifest == nil {
				t.Fatal("missing manifest")
			}
			coverImage := manifest.FindElement("item[@id='book-cover-image']")
			if coverImage == nil {
				t.Fatal("missing cover image manifest item")
			}
			if got := coverImage.SelectAttrValue("properties", ""); got != "cover-image" {
				t.Errorf("cover image properties = %q, want cover-image", got)
			}
		}
	}
}

func TestWriteOPF_Kepub3(t *testing.T) {
	_, env, log := setupTestContext(t)
	cfg := &env.Cfg.Document

	c := &content.Content{
		Book: &fb2.FictionBook{
			Description: fb2.Description{
				TitleInfo: fb2.TitleInfo{
					BookTitle: fb2.TextField{Value: "Test Book KEPUB3"},
					Lang:      language.Make("en"),
				},
				DocumentInfo: fb2.DocumentInfo{ID: "test-book-kepub3"},
			},
		},
		OutputFormat: common.OutputFmtKepub3,
		ImagesIndex: fb2.BookImages{
			"cover": &fb2.BookImage{
				Filename: "images/cover.jpg",
				MimeType: "image/jpeg",
			},
		},
		CoverID: "cover",
	}

	opfContent := writeOPFStringForTest(t, c, cfg, log)
	for _, want := range []string{
		`<package xmlns="http://www.idpf.org/2007/opf" unique-identifier="BookId" version="3.0">`,
		`<item id="nav" href="nav.xhtml" media-type="application/xhtml+xml" properties="nav"/>`,
		`<item id="book-cover-image" href="images/cover.jpg" media-type="image/jpeg" properties="cover-image"/>`,
		`<meta property="dcterms:modified">`,
	} {
		if !strings.Contains(opfContent, want) {
			t.Fatalf("KEPUB3 OPF missing %s\n%s", want, opfContent)
		}
	}
	for _, unwanted := range []string{`id="ncx"`, `toc="ncx"`, `<guide>`} {
		if strings.Contains(opfContent, unwanted) {
			t.Fatalf("KEPUB3 OPF should not contain %s\n%s", unwanted, opfContent)
		}
	}
}

func TestWriteOPF_DublinCoreExpansion(t *testing.T) {
	_, env, log := setupTestContext(t)
	cfg := &env.Cfg.Document
	cfg.Metainformation.CreatorNameTemplate = `{{ .LastName }}, {{ .FirstName }}`

	tests := []struct {
		name   string
		format common.OutputFmt
		date   *fb2.Date
		want   []string
	}{
		{
			name:   "epub2",
			format: common.OutputFmtEpub2,
			want: []string{
				`<dc:date opf:event="publication">1999</dc:date>`,
				`<dc:publisher>Test Publisher</dc:publisher>`,
				`<dc:contributor opf:role="trl">Translator, Tina</dc:contributor>`,
				`<dc:subject>sf</dc:subject>`,
				`<meta name="fb2:genre" content="sf"/>`,
				`<dc:subject>space</dc:subject>`,
				`<dc:subject>adventure</dc:subject>`,
				`<dc:identifier opf:scheme="ISBN">9780306406157</dc:identifier>`,
			},
		},
		{
			name:   "epub3",
			format: common.OutputFmtEpub3,
			date:   &fb2.Date{Display: "26.10.2015"},
			want: []string{
				`<dc:date>2015-10-26</dc:date>`,
				`<dc:publisher>Test Publisher</dc:publisher>`,
				`<dc:contributor id="translator0">Translator, Tina</dc:contributor>`,
				`<meta refines="#translator0" property="role" scheme="marc:relators">trl</meta>`,
				`<dc:subject id="subject-genre-0">sf</dc:subject>`,
				`<meta refines="#subject-genre-0" property="authority">FB2</meta>`,
				`<meta refines="#subject-genre-0" property="term">sf</meta>`,
				`<dc:subject>space</dc:subject>`,
				`<dc:identifier id="isbn">9780306406157</dc:identifier>`,
				`<meta refines="#isbn" property="identifier-type" scheme="onix:codelist5">15</meta>`,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &content.Content{
				Book: &fb2.FictionBook{
					Description: fb2.Description{
						TitleInfo: fb2.TitleInfo{
							BookTitle:   fb2.TextField{Value: "Test Book"},
							Lang:        language.Make("en"),
							Authors:     []fb2.Author{{FirstName: "Ann", LastName: "Author"}},
							Translators: []fb2.Author{{FirstName: "Tina", LastName: "Translator"}},
							Genres:      []fb2.GenreRef{{Value: "sf"}},
							Keywords:    &fb2.TextField{Value: "space; adventure, sf"},
							Date:        tt.date,
						},
						DocumentInfo: fb2.DocumentInfo{ID: "book-id"},
						PublishInfo: &fb2.PublishInfo{
							Publisher: &fb2.TextField{Value: "Test Publisher"},
							Year:      "1999",
							ISBN:      &fb2.TextField{Value: "978-0-306-40615-7"},
						},
					},
				},
				OutputFormat: tt.format,
				ImagesIndex:  make(fb2.BookImages),
			}

			opfContent := writeOPFStringForTest(t, c, cfg, log)
			for _, want := range tt.want {
				if !strings.Contains(opfContent, want) {
					t.Fatalf("OPF missing %s\n%s", want, opfContent)
				}
			}
		})
	}
}

func writeOPFStringForTest(t *testing.T, c *content.Content, cfg *config.DocumentConfig, log *zap.Logger) string {
	t.Helper()

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	chapters := []publication.Chapter{{ID: "ch01", Filename: "ch01.xhtml", Title: "Chapter 1"}}
	if err := writeOPF(zw, c, cfg, chapters, log); err != nil {
		t.Fatalf("writeOPF() error = %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}

	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatalf("open zip: %v", err)
	}
	for _, f := range zr.File {
		if strings.HasSuffix(f.Name, "content.opf") {
			rc, err := f.Open()
			if err != nil {
				t.Fatalf("open opf: %v", err)
			}
			content, err := io.ReadAll(rc)
			if closeErr := rc.Close(); closeErr != nil && err == nil {
				err = closeErr
			}
			if err != nil {
				t.Fatalf("read opf: %v", err)
			}
			return string(content)
		}
	}
	t.Fatal("OPF file not found in zip")
	return ""
}

func TestWriteNCX(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	c := &content.Content{
		Book: &fb2.FictionBook{
			Description: fb2.Description{
				TitleInfo: fb2.TitleInfo{
					BookTitle: fb2.TextField{Value: "Test Book NCX"},
					Lang:      language.Make("en"),
				},
				DocumentInfo: fb2.DocumentInfo{
					ID: "test-ncx-123",
				},
			},
		},
		OutputFormat: common.OutputFmtEpub2,
		ImagesIndex:  make(fb2.BookImages),
	}

	chapters := []publication.Chapter{
		{
			ID:           "ch01",
			Filename:     "ch01.xhtml",
			Title:        "Chapter 1",
			IncludeInTOC: true,
			Section: &fb2.Section{
				Title: &fb2.Title{
					Items: []fb2.TitleItem{
						{Paragraph: &fb2.Paragraph{
							Text: []fb2.InlineSegment{{Text: "Chapter 1"}},
						}},
					},
				},
			},
		},
		{
			ID:           "ch02",
			Filename:     "ch02.xhtml",
			Title:        "Chapter 2",
			IncludeInTOC: true,
		},
	}

	defaultCfg := &config.DocumentConfig{}
	err := writeNCX(zw, c, defaultCfg, publication.CollectNavigation(chapters, nil))
	if err != nil {
		t.Fatalf("writeNCX() error = %v", err)
	}

	if err := zw.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}

	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatalf("open zip: %v", err)
	}

	var foundNCX bool
	for _, f := range zr.File {
		if strings.HasSuffix(f.Name, ".ncx") {
			foundNCX = true
			rc, err := f.Open()
			if err != nil {
				t.Fatalf("open ncx: %v", err)
			}
			content, _ := io.ReadAll(rc)
			rc.Close()

			if !strings.Contains(string(content), "Test Book NCX") {
				t.Error("NCX should contain book title")
			}
			if !strings.Contains(string(content), "test-ncx-123") {
				t.Error("NCX should contain book ID")
			}

			// Parse to verify structure
			doc := etree.NewDocument()
			if err := doc.ReadFromBytes(content); err != nil {
				t.Fatalf("parse NCX: %v", err)
			}

			ncx := doc.SelectElement("ncx")
			if ncx == nil {
				t.Error("Missing ncx element")
			}

			navMap := ncx.SelectElement("navMap")
			if navMap == nil {
				t.Error("Missing navMap element")
			}

			navPoints := navMap.SelectElements("navPoint")
			if len(navPoints) == 0 {
				t.Error("Should have navPoint elements")
			}
		}
	}

	if !foundNCX {
		t.Error("NCX file not found in zip")
	}
}

func TestWriteNCX_TOCTypeNesting(t *testing.T) {
	title := func(text string) *fb2.Title {
		return &fb2.Title{Items: []fb2.TitleItem{{Paragraph: &fb2.Paragraph{Text: []fb2.InlineSegment{{Text: text}}}}}}
	}
	section := func(id, titleText string, children ...fb2.FlowItem) *fb2.Section {
		return &fb2.Section{ID: id, Title: title(titleText), Content: children}
	}

	chapters := []publication.Chapter{{
		ID:           "ch01",
		Filename:     "ch01.xhtml",
		Title:        "A",
		IncludeInTOC: true,
		Section: section("a", "A",
			fb2.FlowItem{Kind: fb2.FlowSection, Section: section("b", "B",
				fb2.FlowItem{Kind: fb2.FlowSection, Section: section("c", "C",
					fb2.FlowItem{Kind: fb2.FlowSection, Section: section("d", "D")},
				)},
			)},
		),
	}}

	tests := []struct {
		name    string
		tocType common.TOCType
		want    []tocSnapshot
	}{
		{
			name:    "normal",
			tocType: common.TOCTypeNormal,
			want: []tocSnapshot{
				{Title: "A", Children: []tocSnapshot{{Title: "B", Children: []tocSnapshot{{Title: "C", Children: []tocSnapshot{{Title: "D"}}}}}}},
			},
		},
		{
			name:    "old kindle",
			tocType: common.TOCTypeOldKindle,
			want:    []tocSnapshot{{Title: "A", Children: []tocSnapshot{{Title: "B"}, {Title: "C"}, {Title: "D"}}}},
		},
		{
			name:    "flat",
			tocType: common.TOCTypeFlat,
			want:    []tocSnapshot{{Title: "A"}, {Title: "B"}, {Title: "C"}, {Title: "D"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			zw := zip.NewWriter(&buf)
			c := &content.Content{
				Book: &fb2.FictionBook{Description: fb2.Description{
					TitleInfo:    fb2.TitleInfo{BookTitle: fb2.TextField{Value: "Book"}},
					DocumentInfo: fb2.DocumentInfo{ID: "book-id"},
				}},
				OutputFormat: common.OutputFmtEpub2,
			}
			cfg := &config.DocumentConfig{TOCType: tt.tocType}

			if err := writeNCX(zw, c, cfg, publication.CollectNavigation(chapters, nil)); err != nil {
				t.Fatalf("writeNCX() error = %v", err)
			}
			if err := zw.Close(); err != nil {
				t.Fatalf("close zip: %v", err)
			}

			doc := readSingleNCXFromZip(t, buf.Bytes())
			navMap := doc.SelectElement("ncx").SelectElement("navMap")
			got := snapshotNCXNavPoints(navMap)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("NCX tree = %#v, want %#v", got, tt.want)
			}
		})
	}
}

type tocSnapshot struct {
	Title    string
	Children []tocSnapshot
}

func readSingleNCXFromZip(t *testing.T, data []byte) *etree.Document {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("open zip: %v", err)
	}
	for _, f := range zr.File {
		if strings.HasSuffix(f.Name, ".ncx") {
			rc, err := f.Open()
			if err != nil {
				t.Fatalf("open ncx: %v", err)
			}
			content, _ := io.ReadAll(rc)
			rc.Close()
			doc := etree.NewDocument()
			if err := doc.ReadFromBytes(content); err != nil {
				t.Fatalf("parse NCX: %v", err)
			}
			return doc
		}
	}
	t.Fatal("NCX file not found in zip")
	return nil
}

func snapshotNCXNavPoints(parent *etree.Element) []tocSnapshot {
	var out []tocSnapshot
	for _, navPoint := range parent.SelectElements("navPoint") {
		navLabel := navPoint.SelectElement("navLabel")
		var title string
		if navLabel != nil {
			if text := navLabel.SelectElement("text"); text != nil {
				title = text.Text()
			}
		}
		var children []tocSnapshot
		if child := snapshotNCXNavPoints(navPoint); len(child) > 0 {
			children = child
		}
		out = append(out, tocSnapshot{Title: title, Children: children})
	}
	return out
}

func TestWriteNav(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	c := &content.Content{
		Book: &fb2.FictionBook{
			Description: fb2.Description{
				TitleInfo: fb2.TitleInfo{
					BookTitle: fb2.TextField{Value: "Test Book NAV"},
					Lang:      language.Make("en"),
				},
			},
		},
		OutputFormat: common.OutputFmtEpub3,
		ImagesIndex:  make(fb2.BookImages),
	}

	chapters := []publication.Chapter{
		{
			ID:           "ch01",
			Filename:     "ch01.xhtml",
			Title:        "Chapter 1",
			IncludeInTOC: true,
			Section: &fb2.Section{
				Title: &fb2.Title{
					Items: []fb2.TitleItem{
						{Paragraph: &fb2.Paragraph{
							Text: []fb2.InlineSegment{{Text: "Chapter 1"}},
						}},
					},
				},
			},
		},
	}

	cfg := &config.DocumentConfig{
		TOCPage: config.TOCPageConfig{
			Placement: common.TOCPagePlacementNone,
		},
	}

	err := writeNav(zw, c, cfg, chapters, publication.CollectNavigation(chapters, nil))
	if err != nil {
		t.Fatalf("writeNav() error = %v", err)
	}

	if err := zw.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}

	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatalf("open zip: %v", err)
	}

	var foundNav bool
	for _, f := range zr.File {
		if strings.Contains(f.Name, "nav.xhtml") {
			foundNav = true
			rc, err := f.Open()
			if err != nil {
				t.Fatalf("open nav: %v", err)
			}
			content, _ := io.ReadAll(rc)
			rc.Close()

			if !strings.Contains(string(content), "Chapter 1") {
				t.Error("NAV should contain chapter title")
			}

			// Parse to verify structure
			doc := etree.NewDocument()
			if err := doc.ReadFromBytes(content); err != nil {
				t.Fatalf("parse NAV: %v", err)
			}

			html := doc.SelectElement("html")
			if html == nil {
				t.Error("Missing html element")
			}

			nav := html.FindElement("//nav[@epub:type='toc']")
			if nav == nil {
				// Try alternative
				body := html.SelectElement("body")
				if body != nil {
					nav = body.SelectElement("nav")
				}
			}
			if nav == nil {
				t.Error("Missing nav element with toc type")
			}
		}
	}

	if !foundNav {
		t.Error("NAV file not found in zip")
	}
}

func TestWriteNav_TOCTypeNesting(t *testing.T) {
	title := func(text string) *fb2.Title {
		return &fb2.Title{Items: []fb2.TitleItem{{Paragraph: &fb2.Paragraph{Text: []fb2.InlineSegment{{Text: text}}}}}}
	}
	section := func(id, titleText string, children ...fb2.FlowItem) *fb2.Section {
		return &fb2.Section{ID: id, Title: title(titleText), Content: children}
	}

	chapters := []publication.Chapter{{
		ID:           "ch01",
		Filename:     "ch01.xhtml",
		Title:        "A",
		IncludeInTOC: true,
		Section: section("a", "A",
			fb2.FlowItem{Kind: fb2.FlowSection, Section: section("b", "B",
				fb2.FlowItem{Kind: fb2.FlowSection, Section: section("c", "C",
					fb2.FlowItem{Kind: fb2.FlowSection, Section: section("d", "D")},
				)},
			)},
		),
	}}

	tests := []struct {
		name    string
		tocType common.TOCType
		want    []tocSnapshot
	}{
		{
			name:    "normal",
			tocType: common.TOCTypeNormal,
			want: []tocSnapshot{
				{Title: "A", Children: []tocSnapshot{{Title: "B", Children: []tocSnapshot{{Title: "C", Children: []tocSnapshot{{Title: "D"}}}}}}},
			},
		},
		{
			name:    "old kindle",
			tocType: common.TOCTypeOldKindle,
			want:    []tocSnapshot{{Title: "A", Children: []tocSnapshot{{Title: "B"}, {Title: "C"}, {Title: "D"}}}},
		},
		{
			name:    "flat",
			tocType: common.TOCTypeFlat,
			want:    []tocSnapshot{{Title: "A"}, {Title: "B"}, {Title: "C"}, {Title: "D"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			zw := zip.NewWriter(&buf)
			c := &content.Content{
				Book: &fb2.FictionBook{Description: fb2.Description{
					TitleInfo: fb2.TitleInfo{BookTitle: fb2.TextField{Value: "Book"}},
				}},
				OutputFormat: common.OutputFmtEpub3,
			}
			cfg := &config.DocumentConfig{TOCType: tt.tocType}

			if err := writeNav(zw, c, cfg, chapters, publication.CollectNavigation(chapters, nil)); err != nil {
				t.Fatalf("writeNav() error = %v", err)
			}
			if err := zw.Close(); err != nil {
				t.Fatalf("close zip: %v", err)
			}

			doc := readSingleNavFromZip(t, buf.Bytes())
			nav := doc.FindElement("//nav[@epub:type='toc']")
			if nav == nil {
				t.Fatal("missing toc nav")
			}
			got := snapshotNavOL(nav.SelectElement("ol"))
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("NAV tree = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func readSingleNavFromZip(t *testing.T, data []byte) *etree.Document {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("open zip: %v", err)
	}
	for _, f := range zr.File {
		if strings.Contains(f.Name, "nav.xhtml") {
			rc, err := f.Open()
			if err != nil {
				t.Fatalf("open nav: %v", err)
			}
			content, _ := io.ReadAll(rc)
			rc.Close()
			doc := etree.NewDocument()
			if err := doc.ReadFromBytes(content); err != nil {
				t.Fatalf("parse NAV: %v", err)
			}
			return doc
		}
	}
	t.Fatal("NAV file not found in zip")
	return nil
}

func snapshotNavOL(ol *etree.Element) []tocSnapshot {
	if ol == nil {
		return nil
	}
	var out []tocSnapshot
	for _, li := range ol.SelectElements("li") {
		a := li.SelectElement("a")
		var title string
		if a != nil {
			title = a.Text()
		}
		var children []tocSnapshot
		if child := snapshotNavOL(li.SelectElement("ol")); len(child) > 0 {
			children = child
		}
		out = append(out, tocSnapshot{Title: title, Children: children})
	}
	return out
}

func TestWriteCoverPage(t *testing.T) {
	_, _, log := setupTestContext(t)

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	c := &content.Content{
		Book: &fb2.FictionBook{
			Description: fb2.Description{
				TitleInfo: fb2.TitleInfo{
					BookTitle: fb2.TextField{Value: "Test Book"},
				},
			},
		},
		OutputFormat: common.OutputFmtEpub2,
		ImagesIndex: fb2.BookImages{
			"cover": &fb2.BookImage{
				Filename: "cover.jpg",
				MimeType: "image/jpeg",
			},
		},
		CoverID: "cover",
	}

	cfg := &config.DocumentConfig{}

	err := writeCoverPage(zw, c, cfg, log)
	if err != nil {
		t.Fatalf("writeCoverPage() error = %v", err)
	}

	if err := zw.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}

	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatalf("open zip: %v", err)
	}

	var foundCover bool
	for _, f := range zr.File {
		if strings.Contains(f.Name, "cover") && strings.HasSuffix(f.Name, ".xhtml") {
			foundCover = true
			rc, err := f.Open()
			if err != nil {
				t.Fatalf("open cover: %v", err)
			}
			content, _ := io.ReadAll(rc)
			rc.Close()

			if !strings.Contains(string(content), "cover.jpg") {
				t.Error("Cover page should reference cover image")
			}
		}
	}

	if !foundCover {
		t.Error("Cover page not found in zip")
	}
}

func TestGenerateCoverPageDoc_Epub3CoverSemantics(t *testing.T) {
	_, _, log := setupTestContext(t)

	c := &content.Content{
		Book: &fb2.FictionBook{
			Description: fb2.Description{
				TitleInfo: fb2.TitleInfo{
					BookTitle: fb2.TextField{Value: "Test Book"},
				},
			},
		},
		OutputFormat: common.OutputFmtEpub3,
		ImagesIndex: fb2.BookImages{
			"cover": &fb2.BookImage{
				Filename: "cover.jpg",
				MimeType: "image/jpeg",
			},
		},
		CoverID: "cover",
	}

	doc, err := generateCoverPageDoc(c, &config.DocumentConfig{}, log)
	if err != nil {
		t.Fatalf("generateCoverPageDoc() error = %v", err)
	}

	html := doc.SelectElement("html")
	if html == nil {
		t.Fatal("missing html element")
	}
	if got := html.SelectAttrValue("xmlns:epub", ""); got != "http://www.idpf.org/2007/ops" {
		t.Errorf("xmlns:epub = %q, want EPUB namespace", got)
	}

	body := html.SelectElement("body")
	if body == nil {
		t.Fatal("missing body element")
	}
	if got := body.SelectAttrValue("epub:type", ""); got != "cover" {
		t.Errorf("body epub:type = %q, want cover", got)
	}
}

func TestGenerate_WithTOCPageBefore(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	testFB2Path := "../../testdata/_Test.fb2"
	fb2File, err := os.Open(testFB2Path)
	if err != nil {
		t.Fatalf("Failed to open test FB2: %v", err)
	}
	defer fb2File.Close()

	ctx, env, log := setupTestContext(t)
	env.Overwrite = true
	tmpDir := t.TempDir()

	c, err := content.Prepare(ctx, fb2File, testFB2Path, common.OutputFmtEpub2, log)
	if err != nil {
		t.Fatalf("Failed to prepare content: %v", err)
	}

	outputPath := filepath.Join(tmpDir, "test-toc-before.epub")
	cfg := env.Cfg.Document
	cfg.TOCPage.Placement = common.TOCPagePlacementBefore

	err = Generate(ctx, c, outputPath, "", &cfg, log)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	zr, err := zip.OpenReader(outputPath)
	if err != nil {
		t.Fatalf("Failed to open output as zip: %v", err)
	}
	defer zr.Close()

	var foundTOCPage bool
	for _, f := range zr.File {
		if strings.Contains(f.Name, "toc-page.xhtml") {
			foundTOCPage = true
			rc, err := f.Open()
			if err != nil {
				t.Fatalf("open toc page: %v", err)
			}
			content, _ := io.ReadAll(rc)
			rc.Close()

			if !strings.Contains(string(content), "Contents") {
				t.Error("TOC page should contain title 'Contents'")
			}
		}
	}

	if !foundTOCPage {
		t.Error("TOC page not found in zip")
	}

	// Check OPF to verify TOC page is in spine
	var foundInOPF bool
	for _, f := range zr.File {
		if strings.HasSuffix(f.Name, "content.opf") {
			rc, err := f.Open()
			if err != nil {
				t.Fatalf("open opf: %v", err)
			}
			content, _ := io.ReadAll(rc)
			rc.Close()

			if strings.Contains(string(content), "toc-page") {
				foundInOPF = true
			}
		}
	}

	if !foundInOPF {
		t.Error("TOC page not found in OPF manifest")
	}
}

func TestGenerate_WithTOCPageAfter(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	testFB2Path := "../../testdata/_Test.fb2"
	fb2File, err := os.Open(testFB2Path)
	if err != nil {
		t.Fatalf("Failed to open test FB2: %v", err)
	}
	defer fb2File.Close()

	ctx, env, log := setupTestContext(t)
	env.Overwrite = true
	tmpDir := t.TempDir()

	c, err := content.Prepare(ctx, fb2File, testFB2Path, common.OutputFmtEpub3, log)
	if err != nil {
		t.Fatalf("Failed to prepare content: %v", err)
	}

	outputPath := filepath.Join(tmpDir, "test-toc-after.epub")
	cfg := env.Cfg.Document
	cfg.TOCPage.Placement = common.TOCPagePlacementAfter

	err = Generate(ctx, c, outputPath, "", &cfg, log)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	zr, err := zip.OpenReader(outputPath)
	if err != nil {
		t.Fatalf("Failed to open output as zip: %v", err)
	}
	defer zr.Close()

	// EPUB3 keeps nav.xhtml as the machine navigation document and uses a
	// separate generated toc-page.xhtml as the visible TOC page, matching EPUB2.
	var foundTOCPage bool
	var foundNav bool
	var opfContent string
	for _, f := range zr.File {
		switch {
		case strings.Contains(f.Name, "toc-page.xhtml"):
			foundTOCPage = true
			rc, err := f.Open()
			if err != nil {
				t.Fatalf("open toc page: %v", err)
			}
			content, _ := io.ReadAll(rc)
			rc.Close()

			if !strings.Contains(string(content), "Тестовая книга") {
				t.Error("TOC page should contain book title")
			}
		case strings.Contains(f.Name, "nav.xhtml"):
			foundNav = true
		case strings.HasSuffix(f.Name, "content.opf"):
			rc, err := f.Open()
			if err != nil {
				t.Fatalf("open opf: %v", err)
			}
			content, _ := io.ReadAll(rc)
			rc.Close()
			opfContent = string(content)
		}
	}

	if !foundTOCPage {
		t.Error("TOC page not found in zip")
	}
	if !foundNav {
		t.Error("nav.xhtml not found in zip")
	}
	if !strings.Contains(opfContent, "toc-page") {
		t.Error("TOC page not found in OPF manifest/spine")
	}
	if strings.Contains(opfContent, `idref="nav"`) {
		t.Error("nav.xhtml should not be used as the visible TOC spine item")
	}
}

func TestWriteOPF_EPUB3Collections(t *testing.T) {
	_, env, log := setupTestContext(t)
	cfg := &env.Cfg.Document

	num1 := 1
	num5 := 5
	num10 := 10

	tests := []struct {
		name      string
		sequences []fb2.Sequence
		format    common.OutputFmt
		validate  func(t *testing.T, opfContent string)
	}{
		{
			name: "EPUB3 single sequence with number",
			sequences: []fb2.Sequence{
				{Name: "Test Series", Number: &num5},
			},
			format: common.OutputFmtEpub3,
			validate: func(t *testing.T, opfContent string) {
				if !strings.Contains(opfContent, `<meta property="belongs-to-collection" id="collection1">Test Series</meta>`) {
					t.Error("Expected belongs-to-collection metadata")
				}
				if !strings.Contains(opfContent, `<meta refines="#collection1" property="collection-type">series</meta>`) {
					t.Error("Expected collection-type='series'")
				}
				if !strings.Contains(opfContent, `<meta refines="#collection1" property="group-position">5</meta>`) {
					t.Error("Expected group-position='5'")
				}
				if strings.Contains(opfContent, `calibre:series`) {
					t.Error("EPUB3 should not have calibre metadata")
				}
			},
		},
		{
			name: "EPUB2 single sequence with number",
			sequences: []fb2.Sequence{
				{Name: "Test Series", Number: &num5},
			},
			format: common.OutputFmtEpub2,
			validate: func(t *testing.T, opfContent string) {
				if !strings.Contains(opfContent, `<meta name="calibre:series" content="Test Series"/>`) {
					t.Error("Expected calibre:series metadata")
				}
				if !strings.Contains(opfContent, `<meta name="calibre:series_index" content="5"/>`) {
					t.Error("Expected calibre:series_index='5'")
				}
				if strings.Contains(opfContent, `belongs-to-collection`) {
					t.Error("EPUB2 should not have belongs-to-collection metadata")
				}
			},
		},
		{
			name: "KEPUB single sequence",
			sequences: []fb2.Sequence{
				{Name: "Test Series", Number: &num5},
			},
			format: common.OutputFmtKepub,
			validate: func(t *testing.T, opfContent string) {
				if !strings.Contains(opfContent, `<meta name="calibre:series" content="Test Series"/>`) {
					t.Error("Expected calibre:series metadata for KEPUB")
				}
				if strings.Contains(opfContent, `belongs-to-collection`) {
					t.Error("KEPUB should not have belongs-to-collection metadata")
				}
			},
		},
		{
			name: "KEPUB3 single sequence with number",
			sequences: []fb2.Sequence{
				{Name: "Test Series", Number: &num5},
			},
			format: common.OutputFmtKepub3,
			validate: func(t *testing.T, opfContent string) {
				if !strings.Contains(opfContent, `<meta property="belongs-to-collection" id="collection1">Test Series</meta>`) {
					t.Error("Expected belongs-to-collection metadata for KEPUB3")
				}
				if strings.Contains(opfContent, `calibre:series`) {
					t.Error("KEPUB3 should not have calibre metadata")
				}
			},
		},
		{
			name: "EPUB3 multiple sequences",
			sequences: []fb2.Sequence{
				{Name: "Series One", Number: &num1},
				{Name: "Series Two", Number: &num5},
				{Name: "Series Three", Number: nil},
			},
			format: common.OutputFmtEpub3,
			validate: func(t *testing.T, opfContent string) {
				if !strings.Contains(opfContent, `<meta property="belongs-to-collection" id="collection1">Series One</meta>`) {
					t.Error("Expected first collection")
				}
				if !strings.Contains(opfContent, `<meta property="belongs-to-collection" id="collection2">Series Two</meta>`) {
					t.Error("Expected second collection")
				}
				if !strings.Contains(opfContent, `<meta property="belongs-to-collection" id="collection3">Series Three</meta>`) {
					t.Error("Expected third collection")
				}
				if !strings.Contains(opfContent, `<meta refines="#collection1" property="group-position">1</meta>`) {
					t.Error("Expected group-position for first collection")
				}
				if !strings.Contains(opfContent, `<meta refines="#collection2" property="group-position">5</meta>`) {
					t.Error("Expected group-position for second collection")
				}
				// Third sequence has no number, should not have group-position
				if strings.Contains(opfContent, `<meta refines="#collection3" property="group-position">`) {
					t.Error("Third collection should not have group-position")
				}
			},
		},
		{
			name: "EPUB3 nested sequences",
			sequences: []fb2.Sequence{
				{
					Name:   "Parent Series",
					Number: &num1,
					Children: []fb2.Sequence{
						{Name: "Child Series One", Number: &num5},
						{Name: "Child Series Two", Number: &num10},
					},
				},
			},
			format: common.OutputFmtEpub3,
			validate: func(t *testing.T, opfContent string) {
				if !strings.Contains(opfContent, `<meta property="belongs-to-collection" id="collection1">Parent Series</meta>`) {
					t.Error("Expected parent collection")
				}
				if !strings.Contains(opfContent, `<meta refines="#collection1" property="collection-type">series</meta>`) {
					t.Error("Expected parent to be type 'series'")
				}
				if !strings.Contains(opfContent, `<meta property="belongs-to-collection" id="collection2">Child Series One</meta>`) {
					t.Error("Expected first child collection")
				}
				if !strings.Contains(opfContent, `<meta refines="#collection2" property="collection-type">set</meta>`) {
					t.Error("Expected child to be type 'set'")
				}
				if !strings.Contains(opfContent, `<meta property="belongs-to-collection" id="collection3">Child Series Two</meta>`) {
					t.Error("Expected second child collection")
				}
				if !strings.Contains(opfContent, `<meta refines="#collection3" property="collection-type">set</meta>`) {
					t.Error("Expected second child to be type 'set'")
				}
			},
		},
		{
			name:      "No sequences",
			sequences: []fb2.Sequence{},
			format:    common.OutputFmtEpub3,
			validate: func(t *testing.T, opfContent string) {
				if strings.Contains(opfContent, `belongs-to-collection`) {
					t.Error("Should not have collection metadata when no sequences")
				}
				if strings.Contains(opfContent, `calibre:series`) {
					t.Error("Should not have calibre metadata when no sequences")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			zw := zip.NewWriter(&buf)

			c := &content.Content{
				Book: &fb2.FictionBook{
					Description: fb2.Description{
						TitleInfo: fb2.TitleInfo{
							BookTitle: fb2.TextField{Value: "Test Book"},
							Lang:      language.Make("en"),
							Authors: []fb2.Author{
								{FirstName: "John", LastName: "Doe"},
							},
							Sequences: tt.sequences,
						},
						DocumentInfo: fb2.DocumentInfo{
							ID: "test-book-id",
						},
					},
				},
				OutputFormat: tt.format,
				ImagesIndex:  make(fb2.BookImages),
			}

			chapters := []publication.Chapter{
				{ID: "ch01", Filename: "ch01.xhtml", Title: "Chapter 1"},
			}

			err := writeOPF(zw, c, cfg, chapters, log)
			if err != nil {
				t.Fatalf("writeOPF() error = %v", err)
			}

			if err := zw.Close(); err != nil {
				t.Fatalf("close zip: %v", err)
			}

			zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
			if err != nil {
				t.Fatalf("open zip: %v", err)
			}

			var opfContent string
			for _, f := range zr.File {
				if strings.HasSuffix(f.Name, "content.opf") {
					rc, err := f.Open()
					if err != nil {
						t.Fatalf("open opf: %v", err)
					}
					contentBytes, _ := io.ReadAll(rc)
					rc.Close()
					opfContent = string(contentBytes)
					break
				}
			}

			if opfContent == "" {
				t.Fatal("content.opf not found")
			}

			tt.validate(t, opfContent)
		})
	}
}

// makeSection creates an fb2.Section with a title and ID for test purposes.
func makeSection(id, title string) *fb2.Section {
	s := &fb2.Section{ID: id}
	if title != "" {
		s.Title = &fb2.Title{Items: []fb2.TitleItem{{
			Paragraph: &fb2.Paragraph{Text: []fb2.InlineSegment{{Text: title}}},
		}}}
	}
	return s
}

// makeWrapperChapterData creates a publication.Chapter for an untitled wrapper section
// whose children are titled subsections. This simulates the FB2 pattern:
//
//	<section>           ← untitled wrapper
//	  <section><title>Chapter 1</title>...</section>
//	  <section><title>Chapter 2</title>...</section>
//	</section>
func makeWrapperChapterData(filename string, children ...*fb2.Section) publication.Chapter {
	wrapper := &fb2.Section{ID: "wrapper"}
	for _, child := range children {
		wrapper.Content = append(wrapper.Content, fb2.FlowItem{
			Kind:    fb2.FlowSection,
			Section: child,
		})
	}
	return publication.Chapter{
		ID:           "wrapper-ch",
		Filename:     filename,
		Title:        "",
		IncludeInTOC: false,
		Section:      wrapper,
	}
}

func TestWriteNCX_UntitledWrapperSection(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	c := &content.Content{
		Book: &fb2.FictionBook{
			Description: fb2.Description{
				TitleInfo: fb2.TitleInfo{
					BookTitle: fb2.TextField{Value: "Test Book"},
					Lang:      language.Make("en"),
				},
				DocumentInfo: fb2.DocumentInfo{ID: "test-ncx-wrapper"},
			},
		},
		OutputFormat: common.OutputFmtEpub2,
		ImagesIndex:  make(fb2.BookImages),
	}

	// One untitled wrapper chapter containing two titled subsections
	chapters := []publication.Chapter{
		makeWrapperChapterData("ch01.xhtml",
			makeSection("sec1", "Chapter 1"),
			makeSection("sec2", "Chapter 2"),
		),
	}

	cfg := &config.DocumentConfig{}
	err := writeNCX(zw, c, cfg, publication.CollectNavigation(chapters, nil))
	if err != nil {
		t.Fatalf("writeNCX() error = %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}

	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatalf("open zip: %v", err)
	}

	for _, f := range zr.File {
		if !strings.HasSuffix(f.Name, ".ncx") {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			t.Fatalf("open ncx: %v", err)
		}
		data, _ := io.ReadAll(rc)
		rc.Close()

		doc := etree.NewDocument()
		if err := doc.ReadFromBytes(data); err != nil {
			t.Fatalf("parse NCX: %v", err)
		}

		navMap := doc.FindElement("//navMap")
		if navMap == nil {
			t.Fatal("Missing navMap element")
		}

		navPoints := navMap.SelectElements("navPoint")
		if len(navPoints) != 2 {
			t.Fatalf("expected 2 navPoints (promoted from wrapper), got %d", len(navPoints))
		}

		for i, expected := range []string{"Chapter 1", "Chapter 2"} {
			label := navPoints[i].FindElement("navLabel/text")
			if label == nil || label.Text() != expected {
				got := ""
				if label != nil {
					got = label.Text()
				}
				t.Errorf("navPoint[%d] label = %q, want %q", i, got, expected)
			}
		}
		return
	}
	t.Error("NCX file not found in zip")
}

func TestWriteNCX_MixedTitledAndWrapper(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	c := &content.Content{
		Book: &fb2.FictionBook{
			Description: fb2.Description{
				TitleInfo: fb2.TitleInfo{
					BookTitle: fb2.TextField{Value: "Test Book"},
					Lang:      language.Make("en"),
				},
				DocumentInfo: fb2.DocumentInfo{ID: "test-ncx-mixed"},
			},
		},
		OutputFormat: common.OutputFmtEpub2,
		ImagesIndex:  make(fb2.BookImages),
	}

	// Mix of titled chapters and untitled wrapper chapters
	chapters := []publication.Chapter{
		{
			ID:           "intro",
			Filename:     "intro.xhtml",
			Title:        "Introduction",
			IncludeInTOC: true,
		},
		makeWrapperChapterData("ch01.xhtml",
			makeSection("sec1", "Part 1"),
			makeSection("sec2", "Part 2"),
		),
		{
			ID:           "epilog",
			Filename:     "epilog.xhtml",
			Title:        "Epilogue",
			IncludeInTOC: true,
		},
	}

	cfg := &config.DocumentConfig{}
	err := writeNCX(zw, c, cfg, publication.CollectNavigation(chapters, nil))
	if err != nil {
		t.Fatalf("writeNCX() error = %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}

	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatalf("open zip: %v", err)
	}

	for _, f := range zr.File {
		if !strings.HasSuffix(f.Name, ".ncx") {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			t.Fatalf("open ncx: %v", err)
		}
		data, _ := io.ReadAll(rc)
		rc.Close()

		doc := etree.NewDocument()
		if err := doc.ReadFromBytes(data); err != nil {
			t.Fatalf("parse NCX: %v", err)
		}

		navMap := doc.FindElement("//navMap")
		if navMap == nil {
			t.Fatal("Missing navMap element")
		}

		navPoints := navMap.SelectElements("navPoint")
		// Introduction + Part 1 + Part 2 (promoted) + Epilogue = 4
		if len(navPoints) != 4 {
			t.Fatalf("expected 4 navPoints, got %d", len(navPoints))
		}

		expected := []string{"Introduction", "Part 1", "Part 2", "Epilogue"}
		for i, want := range expected {
			label := navPoints[i].FindElement("navLabel/text")
			if label == nil || label.Text() != want {
				got := ""
				if label != nil {
					got = label.Text()
				}
				t.Errorf("navPoint[%d] label = %q, want %q", i, got, want)
			}
		}
		return
	}
	t.Error("NCX file not found in zip")
}

func TestWriteNav_UntitledWrapperSection(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	c := &content.Content{
		Book: &fb2.FictionBook{
			Description: fb2.Description{
				TitleInfo: fb2.TitleInfo{
					BookTitle: fb2.TextField{Value: "Test Book NAV Wrapper"},
					Lang:      language.Make("en"),
				},
			},
		},
		OutputFormat: common.OutputFmtEpub3,
		ImagesIndex:  make(fb2.BookImages),
	}

	chapters := []publication.Chapter{
		makeWrapperChapterData("ch01.xhtml",
			makeSection("sec1", "Chapter 1"),
			makeSection("sec2", "Chapter 2"),
			makeSection("sec3", "Chapter 3"),
		),
	}

	cfg := &config.DocumentConfig{
		TOCPage: config.TOCPageConfig{
			Placement: common.TOCPagePlacementNone,
		},
	}

	err := writeNav(zw, c, cfg, chapters, publication.CollectNavigation(chapters, nil))
	if err != nil {
		t.Fatalf("writeNav() error = %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}

	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatalf("open zip: %v", err)
	}

	for _, f := range zr.File {
		if !strings.Contains(f.Name, "nav.xhtml") {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			t.Fatalf("open nav: %v", err)
		}
		data, _ := io.ReadAll(rc)
		rc.Close()

		navContent := string(data)
		for _, want := range []string{"Chapter 1", "Chapter 2", "Chapter 3"} {
			if !strings.Contains(navContent, want) {
				t.Errorf("NAV should contain %q (promoted from wrapper section)", want)
			}
		}
		return
	}
	t.Error("NAV file not found in zip")
}
