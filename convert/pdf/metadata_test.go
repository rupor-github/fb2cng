package pdf

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.uber.org/zap/zaptest"

	"fbc/common"
	"fbc/config"
	"fbc/content"
	"fbc/convert/pdf/docwriter"
	"fbc/fb2"
)

func TestInfoDictionaryIncludesSubjectAndKeywords(t *testing.T) {
	info := docwriter.Format(infoDictionary(skeletonDocument{
		Title:    "Title",
		Author:   "Author",
		Subject:  "Annotation excerpt",
		Keywords: "one, two",
	}))
	for _, want := range []string{
		"/Title " + docwriter.Format(docwriter.UTF16TextString("Title")),
		"/Author " + docwriter.Format(docwriter.UTF16TextString("Author")),
		"/Subject " + docwriter.Format(docwriter.UTF16TextString("Annotation excerpt")),
		"/Keywords " + docwriter.Format(docwriter.UTF16TextString("one, two")),
	} {
		if !strings.Contains(info, want) {
			t.Fatalf("info dictionary = %q, missing %q", info, want)
		}
	}
}

func TestBookMetadataSubjectAndKeywords(t *testing.T) {
	longAnnotation := strings.Repeat("слово ", 120)
	c := &content.Content{Book: &fb2.FictionBook{Description: fb2.Description{TitleInfo: fb2.TitleInfo{
		Annotation: &fb2.Flow{Items: []fb2.FlowItem{{
			Kind:      fb2.FlowParagraph,
			Paragraph: &fb2.Paragraph{Text: []fb2.InlineSegment{{Text: longAnnotation}}},
		}}},
		Keywords: &fb2.TextField{Value: "one\n two\tthree"},
	}}}}
	if got := bookSubject(c); len([]rune(got)) != metadataExcerptMaxRunes {
		t.Fatalf("bookSubject length = %d, want %d", len([]rune(got)), metadataExcerptMaxRunes)
	}
	if strings.Contains(bookSubject(c), "\n") || strings.Contains(bookSubject(c), "\t") {
		t.Fatalf("bookSubject did not normalize whitespace: %q", bookSubject(c))
	}
	if got := bookKeywords(c); got != "one two three" {
		t.Fatalf("bookKeywords() = %q, want normalized keywords", got)
	}
}

func TestBookMetadataUsesMetainformationTemplates(t *testing.T) {
	c := &content.Content{
		SrcName:      "source.fb2",
		OutputFormat: common.OutputFmtPdf,
		Book: &fb2.FictionBook{Description: fb2.Description{TitleInfo: fb2.TitleInfo{
			BookTitle: fb2.TextField{Value: "Книга"},
			Authors:   []fb2.Author{{FirstName: "Иван", LastName: "Иванов"}},
		}}},
	}
	cfg := &config.DocumentConfig{Metainformation: config.MetainformationConfig{
		TitleTemplate:       "{{.Title}} - копия",
		CreatorNameTemplate: "{{.LastName}}, {{.FirstName}}",
		Transliterate:       true,
	}}

	if got := bookTitle(c, cfg, zaptest.NewLogger(t)); got != "Kniga - kopiia" {
		t.Fatalf("bookTitle() = %q, want KFX-style template expansion then transliteration", got)
	}
	if got := bookAuthors(c, cfg, zaptest.NewLogger(t)); got != "Ivanov Ivan" {
		t.Fatalf("bookAuthors() = %q, want KFX-style author template expansion then transliteration", got)
	}
}

func TestGeneratePDFMetadataUsesMetainformationTemplates(t *testing.T) {
	tmpDir := t.TempDir()
	outputName := filepath.Join(tmpDir, "book.pdf")
	cfg := &config.DocumentConfig{
		Images: config.ImagesConfig{Screen: config.ScreenConfig{Width: 1264, Height: 1680, DPI: 300}},
		Metainformation: config.MetainformationConfig{
			TitleTemplate:       "{{.Title}} (PDF)",
			CreatorNameTemplate: "{{.LastName}}, {{.FirstName}}",
		},
	}
	c := &content.Content{
		SrcName:      "book.fb2",
		OutputFormat: common.OutputFmtPdf,
		Book: &fb2.FictionBook{Description: fb2.Description{TitleInfo: fb2.TitleInfo{
			BookTitle: fb2.TextField{Value: "Metadata Book"},
			Authors:   []fb2.Author{{FirstName: "Ada", LastName: "Lovelace"}},
		}}},
	}

	if err := Generate(context.Background(), c, outputName, cfg, zaptest.NewLogger(t)); err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	data, err := os.ReadFile(outputName)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	pdfText := string(data)
	for _, want := range []string{
		docwriter.Format(docwriter.UTF16TextString("Metadata Book (PDF)")),
		docwriter.Format(docwriter.UTF16TextString("Lovelace, Ada")),
	} {
		if !strings.Contains(pdfText, want) {
			t.Fatalf("generated PDF does not contain metadata %q", want)
		}
	}
}

func TestGeneratePDFMetadataSubjectAndKeywords(t *testing.T) {
	tmpDir := t.TempDir()
	outputName := filepath.Join(tmpDir, "book.pdf")
	cfg := &config.DocumentConfig{Images: config.ImagesConfig{Screen: config.ScreenConfig{Width: 1264, Height: 1680, DPI: 300}}}
	c := &content.Content{
		SrcName: "book.fb2",
		Book: &fb2.FictionBook{Description: fb2.Description{TitleInfo: fb2.TitleInfo{
			BookTitle:  fb2.TextField{Value: "Metadata Book"},
			Annotation: &fb2.Flow{Items: []fb2.FlowItem{{Kind: fb2.FlowParagraph, Paragraph: &fb2.Paragraph{Text: []fb2.InlineSegment{{Text: "Annotation text."}}}}}},
			Keywords:   &fb2.TextField{Value: "alpha, beta"},
		}}},
	}

	if err := Generate(context.Background(), c, outputName, cfg, zaptest.NewLogger(t)); err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	data, err := os.ReadFile(outputName)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	pdfText := string(data)
	for _, want := range []string{
		docwriter.Format(docwriter.UTF16TextString("Annotation text.")),
		docwriter.Format(docwriter.UTF16TextString("alpha, beta")),
	} {
		if !strings.Contains(pdfText, want) {
			t.Fatalf("generated PDF does not contain metadata %q", want)
		}
	}
}
