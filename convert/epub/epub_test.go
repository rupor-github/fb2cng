package epub

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/beevik/etree"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
	"golang.org/x/text/language"

	"fbc/config"
	"fbc/content"
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

func TestGenerateFootnoteBodyID(t *testing.T) {
	tests := []struct {
		name     string
		body     *fb2.Body
		index    int
		expected string
	}{
		{
			name:     "body with name",
			body:     &fb2.Body{Name: "notes"},
			index:    0,
			expected: "notes-0",
		},
		{
			name:     "body with name and higher index",
			body:     &fb2.Body{Name: "notes"},
			index:    5,
			expected: "notes-5",
		},
		{
			name:     "body without name",
			body:     &fb2.Body{},
			index:    0,
			expected: "footnote-body-0",
		},
		{
			name:     "body without name with index",
			body:     &fb2.Body{},
			index:    3,
			expected: "footnote-body-3",
		},
		{
			name:     "duplicate body names get unique IDs",
			body:     &fb2.Body{Name: "notes"},
			index:    1,
			expected: "notes-1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := generateFootnoteBodyID(tt.body, tt.index)
			if result != tt.expected {
				t.Errorf("generateFootnoteBodyID() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestGenerateFootnoteBodyID_UniquenessWithDuplicateNames(t *testing.T) {
	bodies := []*fb2.Body{
		{Name: "notes"},
		{Name: "notes"},
		{Name: "notes"},
	}

	ids := make(map[string]bool)
	for i, body := range bodies {
		id := generateFootnoteBodyID(body, i)
		if ids[id] {
			t.Errorf("Duplicate ID generated: %s", id)
		}
		ids[id] = true
	}

	if len(ids) != len(bodies) {
		t.Errorf("Expected %d unique IDs, got %d", len(bodies), len(ids))
	}
}

func TestCreateXHTMLDocument(t *testing.T) {
	c := &content.Content{}
	title := "Test Chapter"
	doc, root := createXHTMLDocument(c, title)

	if doc == nil {
		t.Fatal("createXHTMLDocument returned nil")
	}

	if root == nil {
		t.Fatal("createXHTMLDocument returned nil root element")
	}

	docRoot := doc.Root()
	if docRoot == nil || docRoot.Tag != "html" {
		t.Error("Root element should be <html>")
	}

	head := docRoot.SelectElement("head")
	if head == nil {
		t.Fatal("Missing <head> element")
	}

	titleElem := head.SelectElement("title")
	if titleElem == nil || titleElem.Text() != title {
		t.Errorf("Title element text = %v, want %v", titleElem.Text(), title)
	}

	body := docRoot.SelectElement("body")
	if body == nil {
		t.Error("Missing <body> element")
	}

	// For non-Kobo format, root should be body
	if root != body {
		t.Error("For non-Kobo format, root element should be body")
	}
}

func TestCreateXHTMLDocument_Kobo(t *testing.T) {
	c := &content.Content{OutputFormat: config.OutputFmtKepub}
	title := "Test Chapter"
	doc, root := createXHTMLDocument(c, title)

	if doc == nil {
		t.Fatal("createXHTMLDocument returned nil")
	}

	if root == nil {
		t.Fatal("createXHTMLDocument returned nil root element")
	}

	docRoot := doc.Root()
	if docRoot == nil || docRoot.Tag != "html" {
		t.Error("Root element should be <html>")
	}

	body := docRoot.SelectElement("body")
	if body == nil {
		t.Fatal("Missing <body> element")
	}

	// For Kobo format, body should contain book-columns div
	var bookColumnsDiv *etree.Element
	for _, child := range body.ChildElements() {
		if child.Tag == "div" && child.SelectAttrValue("id", "") == "book-columns" {
			bookColumnsDiv = child
			break
		}
	}
	if bookColumnsDiv == nil {
		t.Fatal("Missing book-columns div for Kobo format")
	}

	// book-columns should contain book-inner div
	var bookInnerDiv *etree.Element
	for _, child := range bookColumnsDiv.ChildElements() {
		if child.Tag == "div" && child.SelectAttrValue("id", "") == "book-inner" {
			bookInnerDiv = child
			break
		}
	}
	if bookInnerDiv == nil {
		t.Fatal("Missing book-inner div for Kobo format")
	}

	// For Kobo format, root should be the book-inner div
	if root != bookInnerDiv {
		t.Error("For Kobo format, root element should be book-inner div")
	}
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

func TestBuildStyleAttr(t *testing.T) {
	tests := []struct {
		name      string
		baseStyle string
		align     string
		vAlign    string
		contains  []string
	}{
		{
			name:      "empty styles",
			baseStyle: "",
			align:     "",
			vAlign:    "",
			contains:  []string{},
		},
		{
			name:      "base style only",
			baseStyle: "color: red;",
			align:     "",
			vAlign:    "",
			contains:  []string{"color: red;"},
		},
		{
			name:      "with alignment",
			baseStyle: "",
			align:     "center",
			vAlign:    "",
			contains:  []string{"text-align: center;"},
		},
		{
			name:      "with vertical alignment",
			baseStyle: "",
			align:     "",
			vAlign:    "middle",
			contains:  []string{"vertical-align: middle;"},
		},
		{
			name:      "all combined",
			baseStyle: "color: red;",
			align:     "center",
			vAlign:    "top",
			contains:  []string{"color: red;", "text-align: center;", "vertical-align: top;"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildStyleAttr(tt.baseStyle, tt.align, tt.vAlign)
			for _, expected := range tt.contains {
				if !strings.Contains(result, expected) {
					t.Errorf("buildStyleAttr() should contain %q, got %v", expected, result)
				}
			}
			if len(tt.contains) == 0 && result != "" {
				t.Errorf("buildStyleAttr() = %v, want empty", result)
			}
		})
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

func TestCollectIDsFromSection(t *testing.T) {
	section := &fb2.Section{
		ID: "section1",
		Content: []fb2.FlowItem{
			{Kind: fb2.FlowSection, Section: &fb2.Section{ID: "section1-1"}},
			{Kind: fb2.FlowSection, Section: &fb2.Section{ID: "section1-2"}},
		},
	}

	filename := "chapter01.xhtml"
	idToFile := make(idToFileMap)

	collectIDsFromSection(section, filename, idToFile)

	if idToFile["section1"] != filename {
		t.Errorf("ID 'section1' should map to %v, got %v", filename, idToFile["section1"])
	}
	if idToFile["section1-1"] != filename {
		t.Errorf("ID 'section1-1' should map to %v, got %v", filename, idToFile["section1-1"])
	}
	if idToFile["section1-2"] != filename {
		t.Errorf("ID 'section1-2' should map to %v, got %v", filename, idToFile["section1-2"])
	}
}

func TestCalculateSectionDepth(t *testing.T) {
	tests := []struct {
		name     string
		section  *fb2.Section
		expected int
	}{
		{
			name:     "no subsections",
			section:  &fb2.Section{},
			expected: 1,
		},
		{
			name: "one level",
			section: &fb2.Section{
				Content: []fb2.FlowItem{
					{
						Kind: fb2.FlowSection,
						Section: &fb2.Section{
							Title: &fb2.Title{},
						},
					},
				},
			},
			expected: 2,
		},
		{
			name: "two levels",
			section: &fb2.Section{
				Content: []fb2.FlowItem{
					{
						Kind: fb2.FlowSection,
						Section: &fb2.Section{
							Title: &fb2.Title{},
							Content: []fb2.FlowItem{
								{
									Kind: fb2.FlowSection,
									Section: &fb2.Section{
										Title: &fb2.Title{},
									},
								},
							},
						},
					},
				},
			},
			expected: 3,
		},
		{
			name: "multiple branches",
			section: &fb2.Section{
				Content: []fb2.FlowItem{
					{
						Kind: fb2.FlowSection,
						Section: &fb2.Section{
							Title: &fb2.Title{},
							Content: []fb2.FlowItem{
								{
									Kind: fb2.FlowSection,
									Section: &fb2.Section{
										Title: &fb2.Title{},
										Content: []fb2.FlowItem{
											{
												Kind: fb2.FlowSection,
												Section: &fb2.Section{
													Title: &fb2.Title{},
												},
											},
										},
									},
								},
							},
						},
					},
					{
						Kind:    fb2.FlowSection,
						Section: &fb2.Section{Title: &fb2.Title{}},
					},
				},
			},
			expected: 4,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := calculateSectionDepth(tt.section, 1)
			if result != tt.expected {
				t.Errorf("calculateSectionDepth() = %v, want %v", result, tt.expected)
			}
		})
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

func TestProcessFootnoteBodies(t *testing.T) {
	_, _, log := setupTestContext(t)

	c := &content.Content{
		Book: &fb2.FictionBook{},
	}

	tests := []struct {
		name             string
		bodies           []*fb2.Body
		expectError      bool
		expectedChapters int
	}{
		{
			name: "single footnote body",
			bodies: []*fb2.Body{
				{
					Name: "notes",
					Kind: fb2.BodyFootnotes,
					Title: &fb2.Title{
						Items: []fb2.TitleItem{
							{Paragraph: &fb2.Paragraph{
								Text: []fb2.InlineSegment{{Text: "Footnotes"}},
							}},
						},
					},
					Sections: []fb2.Section{
						{ID: "note1"},
					},
				},
			},
			expectError:      false,
			expectedChapters: 1,
		},
		{
			name: "multiple footnote bodies with same name",
			bodies: []*fb2.Body{
				{
					Name: "notes",
					Kind: fb2.BodyFootnotes,
					Sections: []fb2.Section{
						{ID: "note1"},
					},
				},
				{
					Name: "notes",
					Kind: fb2.BodyFootnotes,
					Sections: []fb2.Section{
						{ID: "note2"},
					},
				},
			},
			expectError:      false,
			expectedChapters: 2,
		},
		{
			name: "footnote body without name",
			bodies: []*fb2.Body{
				{
					Sections: []fb2.Section{
						{ID: "note1"},
					},
				},
			},
			expectError:      false,
			expectedChapters: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			idToFile := make(idToFileMap)
			existingChapters := []chapterData{}
			chapters, err := processFootnoteBodies(c, tt.bodies, existingChapters, idToFile, log)

			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if len(chapters) != tt.expectedChapters {
				t.Errorf("Expected %d chapters, got %d", tt.expectedChapters, len(chapters))
			}

			// Verify unique body IDs in generated XHTML
			if len(chapters) > 0 && chapters[0].Doc != nil {
				bodyDivs := chapters[0].Doc.FindElements("//div[@class='footnote-body']")
				ids := make(map[string]bool)
				for _, div := range bodyDivs {
					id := div.SelectAttrValue("id", "")
					if id == "" {
						t.Error("footnote-body div missing id attribute")
						continue
					}
					if ids[id] {
						t.Errorf("Duplicate body ID found: %s", id)
					}
					ids[id] = true
				}
			}
		})
	}
}

func TestFixInternalLinks(t *testing.T) {
	_, _, log := setupTestContext(t)

	c := &content.Content{}

	// Create test chapters with links
	doc1, root1 := createXHTMLDocument(c, "Chapter 1")
	p1 := root1.CreateElement("p")
	p1.CreateAttr("id", "para1")
	a1 := p1.CreateElement("a")
	a1.CreateAttr("href", "#para2")
	a1.SetText("Link to para2")

	doc2, root2 := createXHTMLDocument(c, "Chapter 2")
	p2 := root2.CreateElement("p")
	p2.CreateAttr("id", "para2")

	chapters := []chapterData{
		{ID: "ch1", Filename: "ch1.xhtml", Doc: doc1},
		{ID: "ch2", Filename: "ch2.xhtml", Doc: doc2},
	}

	idToFile := idToFileMap{
		"para1": "ch1.xhtml",
		"para2": "ch2.xhtml",
	}

	fixInternalLinks(chapters, idToFile, log)

	// Verify link was updated
	link := doc1.FindElement("//a[@href]")
	if link == nil {
		t.Fatal("Link not found")
	}

	href := link.SelectAttrValue("href", "")
	expected := "ch2.xhtml#para2"
	if href != expected {
		t.Errorf("Link href = %v, want %v", href, expected)
	}
}

func TestFixInternalLinks_SameFile(t *testing.T) {
	_, _, log := setupTestContext(t)

	c := &content.Content{}
	doc, root := createXHTMLDocument(c, "Chapter 1")
	p1 := root.CreateElement("p")
	p1.CreateAttr("id", "para1")
	a := p1.CreateElement("a")
	a.CreateAttr("href", "#para2")
	p2 := root.CreateElement("p")
	p2.CreateAttr("id", "para2")

	chapters := []chapterData{
		{ID: "ch1", Filename: "ch1.xhtml", Doc: doc},
	}

	idToFile := idToFileMap{
		"para1": "ch1.xhtml",
		"para2": "ch1.xhtml",
	}

	fixInternalLinks(chapters, idToFile, log)

	// Verify link remains unchanged (same file)
	link := doc.FindElement("//a[@href]")
	href := link.SelectAttrValue("href", "")
	expected := "#para2"
	if href != expected {
		t.Errorf("Link href = %v, want %v (should remain unchanged)", href, expected)
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
	c, err := content.Prepare(ctx, fb2File, testFB2Path, config.OutputFmtEpub2, log)
	if err != nil {
		t.Fatalf("Failed to prepare content: %v", err)
	}
	// WorkDir is already set by Prepare
	t.Logf("WorkDir from Prepare: %s", c.WorkDir)
	t.Logf("Test tmpDir: %s", tmpDir)

	outputPath := filepath.Join(tmpDir, "test.epub")
	t.Logf("Output path: %s", outputPath)
	cfg := env.Cfg.Document

	err = Generate(ctx, c, outputPath, &cfg, log)
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
			workdirPath := filepath.Join(c.WorkDir, filepath.Base(outputPath))
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
	c, err := content.Prepare(ctx, fb2File, testFB2Path, config.OutputFmtEpub2, log)
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

	err = Generate(ctx, c, outputPath, &cfg, log)
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
		OutputFormat: config.OutputFmtEpub2,
		ImagesIndex:  make(fb2.BookImages),
		WorkDir:      tmpDir,
	}

	outputPath := filepath.Join(tmpDir, "existing.epub")

	// Create existing file
	if err := os.WriteFile(outputPath, []byte("existing"), 0644); err != nil {
		t.Fatalf("Create existing file: %v", err)
	}

	cfg := &config.DocumentConfig{}
	err := Generate(ctx, c, outputPath, cfg, log)

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
		OutputFormat: config.OutputFmtEpub2,
		ImagesIndex:  make(fb2.BookImages),
		WorkDir:      tmpDir,
	}

	outputPath := filepath.Join(tmpDir, "test.epub")
	cfg := &config.DocumentConfig{}

	err := Generate(ctx, c, outputPath, cfg, log)
	if err == nil {
		t.Error("Generate() should fail with cancelled context")
	}
}

func TestAppendImageElement(t *testing.T) {
	_, _, _ = setupTestContext(t)

	c := &content.Content{
		ImagesIndex: fb2.BookImages{
			"img1": &fb2.BookImage{
				MimeType: "image/jpeg",
				Filename: "img1.jpg",
			},
		},
	}

	parent := etree.NewElement("div")
	img := &fb2.Image{Href: "#img1", Alt: "Test Image"}

	appendImageElement(parent, c, img)

	// Image is wrapped in a div - use FindElement for attribute search
	div := parent.FindElement("//div[@class='image']")
	if div == nil {
		// Try without XPath
		div = parent.SelectElement("div")
		if div == nil {
			t.Fatal("Image div not created")
		}
		if div.SelectAttrValue("class", "") != "image" {
			t.Fatalf("Div class = %v, want 'image'", div.SelectAttrValue("class", ""))
		}
	}

	imgElem := div.SelectElement("img")
	if imgElem == nil {
		t.Fatal("Image element not created")
	}

	src := imgElem.SelectAttrValue("src", "")
	if !strings.Contains(src, "img1.jpg") {
		t.Errorf("Image src = %v, should contain img1.jpg", src)
	}

	alt := imgElem.SelectAttrValue("alt", "")
	if alt != "Test Image" {
		t.Errorf("Image alt = %v, want 'Test Image'", alt)
	}
}

func TestAppendImageElement_MissingImage(t *testing.T) {
	_ = setupTestLogger(t)

	c := &content.Content{
		ImagesIndex: make(fb2.BookImages),
	}

	parent := etree.NewElement("div")
	img := &fb2.Image{Href: "#missing"}

	appendImageElement(parent, c, img)

	// Should still create element but with placeholder
	div := parent.SelectElement("div")
	if div == nil {
		t.Error("Image div should still be created for missing image")
		return
	}

	imgElem := div.SelectElement("img")
	if imgElem == nil {
		t.Error("Image element should still be created for missing image")
	}
}

func BenchmarkGenerateFootnoteBodyID(b *testing.B) {
	body := &fb2.Body{Name: "notes"}
	for i := 0; i < b.N; i++ {
		_ = generateFootnoteBodyID(body, i%100)
	}
}

func BenchmarkCreateXHTMLDocument(b *testing.B) {
	c := &content.Content{}
	for i := 0; i < b.N; i++ {
		_, _ = createXHTMLDocument(c, fmt.Sprintf("Chapter %d", i))
	}
}

// Additional tests for improving coverage

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

func TestAppendTitleAsDiv(t *testing.T) {
	_, _, _ = setupTestContext(t)

	c := &content.Content{
		ImagesIndex: make(fb2.BookImages),
	}

	parent := etree.NewElement("div")

	title := &fb2.Title{
		Items: []fb2.TitleItem{
			{Paragraph: &fb2.Paragraph{
				ID:   "title1",
				Text: []fb2.InlineSegment{{Text: "First Title"}},
			}},
			{EmptyLine: true},
			{Paragraph: &fb2.Paragraph{
				Text:  []fb2.InlineSegment{{Text: "Second Title"}},
				Style: "custom-style",
			}},
		},
	}

	appendTitleAsDiv(parent, c, title, "test-title")

	titleDiv := parent.SelectElement("div")
	if titleDiv == nil {
		t.Fatal("Title div not created")
	}

	if titleDiv.SelectAttrValue("class", "") != "test-title" {
		t.Errorf("Title div class = %v, want 'test-title'", titleDiv.SelectAttrValue("class", ""))
	}

	// Check paragraphs
	paras := titleDiv.SelectElements("p")
	if len(paras) != 2 {
		t.Fatalf("Expected 2 paragraphs, got %d", len(paras))
	}

	// First paragraph should have test-title-first class
	if !strings.Contains(paras[0].SelectAttrValue("class", ""), "test-title-first") {
		t.Errorf("First paragraph class should contain 'test-title-first'")
	}

	// Check for br element
	br := titleDiv.SelectElement("br")
	if br == nil {
		t.Error("Empty line should create br element")
	}

	// Second paragraph with custom style
	if !strings.Contains(paras[1].SelectAttrValue("class", ""), "custom-style") {
		t.Error("Second paragraph should have custom-style class")
	}
}

func TestAppendEpigraphs(t *testing.T) {
	_, _, log := setupTestContext(t)

	c := &content.Content{
		ImagesIndex: make(fb2.BookImages),
	}

	parent := etree.NewElement("div")

	epigraphs := []fb2.Epigraph{
		{
			Flow: fb2.Flow{
				Items: []fb2.FlowItem{
					{
						Kind: fb2.FlowParagraph,
						Paragraph: &fb2.Paragraph{
							Text: []fb2.InlineSegment{{Text: "Epigraph text"}},
						},
					},
				},
			},
			TextAuthors: []fb2.Paragraph{
				{Text: []fb2.InlineSegment{{Text: "Author Name"}}},
			},
		},
	}

	err := appendEpigraphs(parent, c, epigraphs, 1, log)
	if err != nil {
		t.Fatalf("appendEpigraphs() error = %v", err)
	}

	epigraphDiv := parent.SelectElement("div[@class='epigraph']")
	if epigraphDiv == nil {
		// Try without xpath
		epigraphDiv = parent.SelectElement("div")
		if epigraphDiv == nil || epigraphDiv.SelectAttrValue("class", "") != "epigraph" {
			t.Fatal("Epigraph div not created")
		}
	}

	// Check for text-author
	authorP := epigraphDiv.FindElement("//p[@class='text-author']")
	if authorP == nil {
		authorP = epigraphDiv.SelectElement("p")
	}
	if authorP == nil {
		t.Error("Text author paragraph not created")
	}
}

func TestAppendPoemElement(t *testing.T) {
	_, _, log := setupTestContext(t)

	c := &content.Content{
		ImagesIndex: make(fb2.BookImages),
	}

	parent := etree.NewElement("div")

	poem := &fb2.Poem{
		ID:   "poem1",
		Lang: "en",
		Title: &fb2.Title{
			Items: []fb2.TitleItem{
				{Paragraph: &fb2.Paragraph{
					Text: []fb2.InlineSegment{{Text: "Poem Title"}},
				}},
			},
		},
		Stanzas: []fb2.Stanza{
			{
				Verses: []fb2.Paragraph{
					{Text: []fb2.InlineSegment{{Text: "First line"}}},
					{Text: []fb2.InlineSegment{{Text: "Second line"}}},
				},
			},
		},
		TextAuthors: []fb2.Paragraph{
			{Text: []fb2.InlineSegment{{Text: "Poet Name"}}},
		},
	}

	err := appendPoemElement(parent, c, poem, 1, log)
	if err != nil {
		t.Fatalf("appendPoemElement() error = %v", err)
	}

	poemDiv := parent.SelectElement("div")
	if poemDiv == nil {
		t.Fatal("Poem div not created")
	}

	if poemDiv.SelectAttrValue("id", "") != "poem1" {
		t.Error("Poem div should have id attribute")
	}

	if poemDiv.SelectAttrValue("xml:lang", "") != "en" {
		t.Error("Poem div should have xml:lang attribute")
	}

	// Check for stanza
	stanzaDiv := poemDiv.SelectElement("div")
	if stanzaDiv == nil {
		t.Error("Stanza div not created")
		return
	}

	// Check verses - count all p elements in stanza
	verses := stanzaDiv.SelectElements("p")
	if len(verses) < 1 {
		t.Errorf("Expected at least 1 verse paragraph, got %d", len(verses))
	}
}

func TestAppendPoemElement_WithDate(t *testing.T) {
	_, _, log := setupTestContext(t)

	c := &content.Content{
		ImagesIndex: make(fb2.BookImages),
	}

	parent := etree.NewElement("div")

	poem := &fb2.Poem{
		Stanzas: []fb2.Stanza{
			{
				Verses: []fb2.Paragraph{
					{Text: []fb2.InlineSegment{{Text: "Line"}}},
				},
			},
		},
		Date: &fb2.Date{
			Display: "December 2025",
		},
	}

	err := appendPoemElement(parent, c, poem, 1, log)
	if err != nil {
		t.Fatalf("appendPoemElement() error = %v", err)
	}

	poemDiv := parent.SelectElement("div")
	dateP := poemDiv.FindElement("//p[@class='date']")
	if dateP == nil {
		// Try alternative
		for _, p := range poemDiv.SelectElements("p") {
			if p.SelectAttrValue("class", "") == "date" {
				dateP = p
				break
			}
		}
	}

	if dateP == nil {
		t.Error("Date paragraph not created")
	} else if dateP.Text() != "December 2025" {
		t.Errorf("Date text = %v, want 'December 2025'", dateP.Text())
	}
}

func TestAppendCiteElement(t *testing.T) {
	_, _, log := setupTestContext(t)

	c := &content.Content{
		ImagesIndex: make(fb2.BookImages),
	}

	parent := etree.NewElement("div")

	cite := &fb2.Cite{
		ID:   "cite1",
		Lang: "en",
		Items: []fb2.FlowItem{
			{
				Kind: fb2.FlowParagraph,
				Paragraph: &fb2.Paragraph{
					Text: []fb2.InlineSegment{{Text: "Citation text"}},
				},
			},
		},
		TextAuthors: []fb2.Paragraph{
			{Text: []fb2.InlineSegment{{Text: "Cited Author"}}},
		},
	}

	err := appendCiteElement(parent, c, cite, 1, log)
	if err != nil {
		t.Fatalf("appendCiteElement() error = %v", err)
	}

	blockquote := parent.SelectElement("blockquote")
	if blockquote == nil {
		t.Fatal("Blockquote not created")
	}

	if blockquote.SelectAttrValue("id", "") != "cite1" {
		t.Error("Blockquote should have id attribute")
	}

	if blockquote.SelectAttrValue("xml:lang", "") != "en" {
		t.Error("Blockquote should have xml:lang attribute")
	}
}

func TestAppendTableElement(t *testing.T) {
	_, _, _ = setupTestContext(t)

	c := &content.Content{
		ImagesIndex: make(fb2.BookImages),
	}

	parent := etree.NewElement("div")

	table := &fb2.Table{
		ID: "table1",
		Rows: []fb2.TableRow{
			{
				Cells: []fb2.TableCell{
					{
						Content: []fb2.InlineSegment{{Text: "Cell 1"}},
					},
					{
						Content: []fb2.InlineSegment{{Text: "Cell 2"}},
						Align:   "center",
						VAlign:  "middle",
					},
				},
			},
		},
	}

	appendTableElement(parent, c, table)

	tableElem := parent.SelectElement("table")
	if tableElem == nil {
		t.Fatal("Table element not created")
	}

	if tableElem.SelectAttrValue("id", "") != "table1" {
		t.Error("Table should have id attribute")
	}

	// Check for row
	tr := tableElem.SelectElement("tr")
	if tr == nil {
		t.Error("Table row not created")
	}

	// Check for cells
	cells := tr.SelectElements("td")
	if len(cells) != 2 {
		t.Errorf("Expected 2 cells, got %d", len(cells))
	}

	// Check alignment attributes on second cell
	if len(cells) > 1 {
		style := cells[1].SelectAttrValue("style", "")
		if !strings.Contains(style, "text-align") || !strings.Contains(style, "vertical-align") {
			t.Error("Second cell should have alignment styles")
		}
	}
}

func TestWriteStylesheet(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	c := &content.Content{
		Book: &fb2.FictionBook{
			Stylesheets: []fb2.Stylesheet{
				{Type: "text/css", Data: "/* custom */"}},
		},
		OutputFormat: config.OutputFmtEpub2,
	}

	defaultStyle := []byte("body { font-family: serif; }")

	err := writeStylesheet(zw, c, defaultStyle)
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

func TestWriteImages(t *testing.T) {
	_, _, log := setupTestContext(t)

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	images := fb2.BookImages{
		"img1": &fb2.BookImage{
			Filename: "test.jpg",
			Data:     []byte("fake image data"),
			MimeType: "image/jpeg",
		},
	}

	err := writeImages(zw, images, log)
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
	_, _, log := setupTestContext(t)

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	c := &content.Content{}
	doc, root := createXHTMLDocument(c, "Test Chapter")
	p := root.CreateElement("p")
	p.SetText("Chapter content")

	chapter := chapterData{
		ID:       "ch01",
		Filename: "ch01.xhtml",
		Title:    "Test Chapter",
		Doc:      doc,
	}

	err := writeXHTMLChapter(zw, &chapter, log)
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

func TestCollectIDsFromBody(t *testing.T) {
	body := &fb2.Body{
		Sections: []fb2.Section{
			{
				ID: "s1",
				Content: []fb2.FlowItem{
					{
						Kind: fb2.FlowParagraph,
						Paragraph: &fb2.Paragraph{
							ID:   "p1",
							Text: []fb2.InlineSegment{{Text: "test"}},
						},
					},
					{
						Kind: fb2.FlowSection,
						Section: &fb2.Section{
							ID: "s1-1",
						},
					},
				},
			},
		},
	}

	filename := "test.xhtml"
	idToFile := make(idToFileMap)

	collectIDsFromBody(body, filename, idToFile)

	if idToFile["s1"] != filename {
		t.Errorf("Section s1 should map to %v", filename)
	}

	if idToFile["p1"] != filename {
		t.Errorf("Paragraph p1 should map to %v", filename)
	}

	if idToFile["s1-1"] != filename {
		t.Errorf("Subsection s1-1 should map to %v", filename)
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
		OutputFormat: config.OutputFmtEpub2,
		ImagesIndex:  make(fb2.BookImages),
		CoverID:      "cover1",
	}

	chapters := []chapterData{
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
		OutputFormat: config.OutputFmtEpub3,
		ImagesIndex:  make(fb2.BookImages),
	}

	chapters := []chapterData{
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
		}
	}
}

func TestWriteNCX(t *testing.T) {
	_, _, log := setupTestContext(t)

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
		OutputFormat: config.OutputFmtEpub2,
		ImagesIndex:  make(fb2.BookImages),
	}

	chapters := []chapterData{
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

	err := writeNCX(zw, c, chapters, log)
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

func TestWriteNav(t *testing.T) {
	_, _, log := setupTestContext(t)

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
		OutputFormat: config.OutputFmtEpub3,
		ImagesIndex:  make(fb2.BookImages),
	}

	chapters := []chapterData{
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

	err := writeNav(zw, c, chapters, log)
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
		OutputFormat: config.OutputFmtEpub2,
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

func TestGenerateTOCPage(t *testing.T) {
	log := setupTestLogger(t)

	c := &content.Content{
		Book: &fb2.FictionBook{
			Description: fb2.Description{
				TitleInfo: fb2.TitleInfo{
					BookTitle: fb2.TextField{Value: "Test Book"},
				},
			},
		},
	}

	chapters := []chapterData{
		{
			ID:           "ch1",
			Filename:     "ch1.xhtml",
			Title:        "Chapter 1",
			IncludeInTOC: true,
		},
		{
			ID:           "ch2",
			Filename:     "ch2.xhtml",
			Title:        "Chapter 2",
			IncludeInTOC: true,
		},
	}

	cfg := &config.TOCPageConfig{
		Title: "Table of Contents",
	}

	tocChapter := generateTOCPage(c, chapters, cfg, log)

	if tocChapter.ID != "toc-page" {
		t.Errorf("Expected ID 'toc-page', got '%s'", tocChapter.ID)
	}

	if tocChapter.Filename != "toc-page.xhtml" {
		t.Errorf("Expected filename 'toc-page.xhtml', got '%s'", tocChapter.Filename)
	}

	if tocChapter.Title != "Table of Contents" {
		t.Errorf("Expected title 'Table of Contents', got '%s'", tocChapter.Title)
	}

	if tocChapter.Doc == nil {
		t.Fatal("TOC document should not be nil")
	}

	// Check document structure
	body := tocChapter.Doc.Root().SelectElement("body")
	if body == nil {
		t.Fatal("Body element not found")
	}

	// Check body has CSS class
	if body.SelectAttrValue("class", "") != "toc-page" {
		t.Errorf("Expected body class 'toc-page', got '%s'", body.SelectAttrValue("class", ""))
	}

	h1 := body.SelectElement("h1")
	if h1 == nil {
		t.Fatal("H1 element not found")
	}

	// Check h1 has CSS class
	if h1.SelectAttrValue("class", "") != "toc-title" {
		t.Errorf("Expected h1 class 'toc-title', got '%s'", h1.SelectAttrValue("class", ""))
	}

	// H1 now contains the book title, not the TOC title
	if h1.Text() != "Test Book" {
		t.Errorf("Expected H1 text 'Test Book', got '%s'", h1.Text())
	}

	ol := body.SelectElement("ol")
	if ol == nil {
		t.Fatal("OL element not found")
	}

	// Check ol has CSS class
	if ol.SelectAttrValue("class", "") != "toc-list" {
		t.Errorf("Expected ol class 'toc-list', got '%s'", ol.SelectAttrValue("class", ""))
	}

	items := ol.SelectElements("li")
	if len(items) != 2 {
		t.Errorf("Expected 2 list items, got %d", len(items))
	}

	// Check first item
	if len(items) > 0 {
		// Check li has CSS class
		if items[0].SelectAttrValue("class", "") != "toc-item" {
			t.Errorf("Expected li class 'toc-item', got '%s'", items[0].SelectAttrValue("class", ""))
		}

		a := items[0].SelectElement("a")
		if a == nil {
			t.Fatal("First item should have anchor element")
		}

		// Check anchor has CSS class
		if a.SelectAttrValue("class", "") != "toc-link" {
			t.Errorf("Expected a class 'toc-link', got '%s'", a.SelectAttrValue("class", ""))
		}

		href := a.SelectAttrValue("href", "")
		if href != "ch1.xhtml" {
			t.Errorf("Expected href 'ch1.xhtml', got '%s'", href)
		}
		if a.Text() != "Chapter 1" {
			t.Errorf("Expected link text 'Chapter 1', got '%s'", a.Text())
		}
	}
}

func TestGenerateTOCPage_IDCollision(t *testing.T) {
	log := setupTestLogger(t)

	c := &content.Content{
		Book: &fb2.FictionBook{
			Description: fb2.Description{
				TitleInfo: fb2.TitleInfo{
					BookTitle: fb2.TextField{Value: "Test Book"},
				},
			},
		},
	}

	// Create chapters with IDs that will collide with TOC page IDs
	chapters := []chapterData{
		{
			ID:           "toc-page",
			Filename:     "toc-page.xhtml",
			Title:        "Chapter with TOC ID",
			IncludeInTOC: true,
		},
		{
			ID:           "toc-page-1",
			Filename:     "toc-page-1.xhtml",
			Title:        "Another collision",
			IncludeInTOC: true,
		},
		{
			ID:           "ch1",
			Filename:     "ch1.xhtml",
			Title:        "Normal Chapter",
			IncludeInTOC: true,
		},
	}

	cfg := &config.TOCPageConfig{
		Title: "Table of Contents",
	}

	tocChapter := generateTOCPage(c, chapters, cfg, log)

	// Should get toc-page-2 since toc-page and toc-page-1 are taken
	if tocChapter.ID != "toc-page-2" {
		t.Errorf("Expected ID 'toc-page-2' (avoiding collisions), got '%s'", tocChapter.ID)
	}

	if tocChapter.Filename != "toc-page-2.xhtml" {
		t.Errorf("Expected filename 'toc-page-2.xhtml', got '%s'", tocChapter.Filename)
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

	c, err := content.Prepare(ctx, fb2File, testFB2Path, config.OutputFmtEpub2, log)
	if err != nil {
		t.Fatalf("Failed to prepare content: %v", err)
	}

	outputPath := filepath.Join(tmpDir, "test-toc-before.epub")
	cfg := env.Cfg.Document
	cfg.TOCPage.Placement = config.TOCPagePlacementBefore
	cfg.TOCPage.Title = "Contents"

	err = Generate(ctx, c, outputPath, &cfg, log)
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

	c, err := content.Prepare(ctx, fb2File, testFB2Path, config.OutputFmtEpub3, log)
	if err != nil {
		t.Fatalf("Failed to prepare content: %v", err)
	}

	outputPath := filepath.Join(tmpDir, "test-toc-after.epub")
	cfg := env.Cfg.Document
	cfg.TOCPage.Placement = config.TOCPagePlacementAfter
	cfg.TOCPage.Title = "Table of Contents"

	err = Generate(ctx, c, outputPath, &cfg, log)
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

			if !strings.Contains(string(content), "Table of Contents") {
				t.Error("TOC page should contain title 'Table of Contents'")
			}
		}
	}

	if !foundTOCPage {
		t.Error("TOC page not found in zip")
	}
}

// TestFloatModeFootnotes verifies Amazon KDP-compliant footnote markup
func TestFloatModeFootnotes(t *testing.T) {
	tests := []struct {
		name           string
		format         config.OutputFmt
		mode           config.FootnotesMode
		expectNoteref  bool // EPUB3: expect epub:type="noteref"
		expectAside    bool // EPUB3: expect <aside epub:type="footnote">
		expectRefID    bool // EPUB2/3: expect id on <a> reference
		expectBacklink bool // expect back-reference link
	}{
		{
			name:           "EPUB3 float mode",
			format:         config.OutputFmtEpub3,
			mode:           config.FootnotesModeFloat,
			expectNoteref:  true,
			expectAside:    true,
			expectRefID:    true,
			expectBacklink: true,
		},
		{
			name:           "EPUB2 float mode",
			format:         config.OutputFmtEpub2,
			mode:           config.FootnotesModeFloat,
			expectNoteref:  false,
			expectAside:    false,
			expectRefID:    true,
			expectBacklink: true,
		},
		{
			name:           "EPUB3 default mode",
			format:         config.OutputFmtEpub3,
			mode:           config.FootnotesModeDefault,
			expectNoteref:  false,
			expectAside:    false,
			expectRefID:    false,
			expectBacklink: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, _, log := setupTestContext(t)

			// Create test content with footnote
			c := &content.Content{
				OutputFormat:  tt.format,
				FootnotesMode: tt.mode,
				BackLinkIndex: make(map[string][]content.BackLinkRef),
				Book: &fb2.FictionBook{
					Bodies: []fb2.Body{
						{
							Sections: []fb2.Section{
								{
									ID: "chapter1",
									Content: []fb2.FlowItem{
										{
											Kind: fb2.FlowParagraph,
											Paragraph: &fb2.Paragraph{
												Text: []fb2.InlineSegment{
													{Kind: fb2.InlineText, Text: "Text with footnote"},
													{
														Kind: fb2.InlineLink,
														Href: "#note1",
														Children: []fb2.InlineSegment{
															{Kind: fb2.InlineText, Text: "1"},
														},
													},
												},
											},
										},
									},
								},
							},
						},
						{
							Name: "notes",
							Kind: fb2.BodyFootnotes,
							Sections: []fb2.Section{
								{
									ID: "note1",
									Content: []fb2.FlowItem{
										{
											Kind: fb2.FlowParagraph,
											Paragraph: &fb2.Paragraph{
												Text: []fb2.InlineSegment{
													{Kind: fb2.InlineText, Text: "Footnote text"},
												},
											},
										},
									},
								},
							},
						},
					},
				},
				FootnotesIndex: fb2.FootnoteRefs{
					"note1": fb2.FootnoteRef{BodyIdx: 1, SectionIdx: 0},
				},
			}

			chapters, _, err := convertToXHTML(ctx, c, log)
			if err != nil {
				t.Fatalf("convertToXHTML() error = %v", err)
			}

			// Find chapter with main content
			var mainChapter *chapterData
			for i := range chapters {
				if chapters[i].Doc != nil && strings.Contains(chapters[i].ID, "index") {
					mainChapter = &chapters[i]
					break
				}
			}
			if mainChapter == nil {
				t.Fatal("Main chapter not found")
			}

			// Check for footnote reference attributes
			linkElems := mainChapter.Doc.FindElements("//a[@href='#note1']")
			if len(linkElems) == 0 {
				t.Fatal("Footnote link not found")
			}
			link := linkElems[0]

			if tt.expectNoteref {
				epubType := link.SelectAttrValue("epub:type", "")
				if epubType != "noteref" {
					t.Errorf("Expected epub:type='noteref', got '%s'", epubType)
				}
			}

			if tt.expectRefID {
				refID := link.SelectAttrValue("id", "")
				if refID == "" {
					t.Error("Expected id attribute on footnote reference")
				} else if !strings.HasPrefix(refID, "ref-note1-") {
					t.Errorf("Expected ref ID to start with 'ref-note1-', got '%s'", refID)
				}
			}

			// Find footnote chapter
			var fnChapter *chapterData
			for i := range chapters {
				if chapters[i].Doc != nil && strings.Contains(chapters[i].ID, "footnote") {
					fnChapter = &chapters[i]
					break
				}
			}
			if fnChapter == nil {
				t.Fatal("Footnote chapter not found")
			}

			if tt.expectAside {
				// Check for <aside epub:type="footnote">
				asides := fnChapter.Doc.FindElements("//aside[@epub:type='footnote']")
				if len(asides) == 0 {
					t.Error("Expected <aside epub:type='footnote'> not found")
				}
			}

			if tt.expectBacklink {
				// Check for back-reference link
				var backlinks []*etree.Element
				if tt.format == config.OutputFmtEpub2 || tt.format == config.OutputFmtKepub {
					// EPUB2: backlink is inside <p class="footnote">
					backlinks = fnChapter.Doc.FindElements("//p[@class='footnote']/a")
				} else {
					// EPUB3: backlink is in separate <p class="footnote-backlink">
					backlinks = fnChapter.Doc.FindElements("//p[@class='footnote-backlink']/a")
				}
				if len(backlinks) == 0 {
					t.Error("Expected back-reference link not found")
				} else {
					backlink := backlinks[0]
					href := backlink.SelectAttrValue("href", "")
					// Backlink should include filename and anchor
					if !strings.Contains(href, ".xhtml#ref-note1-") {
						t.Errorf("Expected backlink href to contain '.xhtml#ref-note1-', got '%s'", href)
					}
					if backlink.Text() != backlinkSym {
						t.Errorf("Expected backlink text '%s', got '%s'", backlinkSym, backlink.Text())
					}
				}
			}
		})
	}
}
