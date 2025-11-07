package fb2

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/beevik/etree"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
)

func TestBuildFictionBookFromSample(t *testing.T) {
	log := zaptest.NewLogger(t, zaptest.WrapOptions(zap.AddCaller(), zap.AddCallerSkip(1)))
	path := filepath.Clean(sampleFB2)
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("sample file missing: %v", err)
	}

	doc := loadSampleDocument(t)
	book, err := ParseBookXML(doc, []string{"notes", "footnotes"}, log)
	if err != nil {
		t.Fatalf("BuildFictionBook failed: %v", err)
	}

	if len(book.Description.TitleInfo.Authors) != 3 {
		t.Fatalf("expected 3 authors, got %d", len(book.Description.TitleInfo.Authors))
	}
	if book.Description.TitleInfo.Lang != "ru" {
		t.Fatalf("expected title-info lang ru, got %q", book.Description.TitleInfo.Lang)
	}
	if book.Description.TitleInfo.BookTitle.Value == "" {
		t.Fatalf("expected non-empty book title")
	}
	if len(book.Bodies) == 0 {
		t.Fatalf("expected at least one body")
	}
	if len(book.Binaries) == 0 {
		t.Fatalf("expected binary attachments")
	}
	if len(book.Binaries[0].Data) == 0 {
		t.Fatalf("expected decoded binary data")
	}
}

func mustElement(t *testing.T, xml string) *etree.Element {
	t.Helper()

	doc := etree.NewDocument()
	if err := doc.ReadFromString(xml); err != nil {
		t.Fatalf("read xml: %v", err)
	}
	if doc.Root() == nil {
		t.Fatalf("xml has no root element")
	}
	return doc.Root()
}

func TestParsePublishInfo(t *testing.T) {
	log := zaptest.NewLogger(t, zaptest.WrapOptions(zap.AddCaller(), zap.AddCallerSkip(1)))
	el := mustElement(t, `<publish-info>
		<book-name xml:lang="en">Test Title</book-name>
		<publisher>Pub</publisher>
		<city>City</city>
		<year>2023</year>
		<isbn>978-1234567890</isbn>
		<sequence name="Saga" number="2" xml:lang="ru" />
	</publish-info>`)

	info, err := parsePublishInfo(el, log)
	if err != nil {
		t.Fatalf("parsePublishInfo: %v", err)
	}
	if got := info.BookName.Value; got != "Test Title" {
		t.Fatalf("book name value mismatch: %q", got)
	}
	if got := info.Publisher.Value; got != "Pub" {
		t.Fatalf("publisher mismatch: %q", got)
	}
	if got := info.City.Value; got != "City" {
		t.Fatalf("city mismatch: %q", got)
	}
	if info.Year != "2023" {
		t.Fatalf("year mismatch: %q", info.Year)
	}
	if got := info.ISBN.Value; got != "978-1234567890" {
		t.Fatalf("isbn mismatch: %q", got)
	}
	if len(info.Sequences) != 1 {
		t.Fatalf("expected one sequence, got %d", len(info.Sequences))
	}
	seq := info.Sequences[0]
	if seq.Name != "Saga" || seq.Number == nil || *seq.Number != 2 {
		t.Fatalf("unexpected sequence: %+v", seq)
	}
}

func TestParseCustomInfo(t *testing.T) {
	log := zaptest.NewLogger(t, zaptest.WrapOptions(zap.AddCaller(), zap.AddCallerSkip(1)))
	el := mustElement(t, `<custom-info info-type="foo" xml:lang="es"> Valor </custom-info>`)

	custom := parseCustomInfo(el, log)
	if custom.Type != "foo" {
		t.Fatalf("expected type foo, got %q", custom.Type)
	}
	if custom.Value.Value != "Valor" {
		t.Fatalf("expected value 'Valor', got %q", custom.Value.Value)
	}
	if custom.Value.Lang != "es" {
		t.Fatalf("expected lang es, got %q", custom.Value.Lang)
	}
}

func TestParseOutputInstruction(t *testing.T) {
	log := zaptest.NewLogger(t, zaptest.WrapOptions(zap.AddCaller(), zap.AddCallerSkip(1)))
	el := mustElement(t, `<output mode="paid" include-all="allow" currency="USD" price="1.99" xmlns:xlink="http://www.w3.org/1999/xlink">
		<part xlink:href="#body" include="require" />
		<output-document-class name="web" create="allow" price="2.50">
			<part xlink:href="#ann" include="allow" />
		</output-document-class>
	</output>`)

	instruction, err := parseOutputInstruction(el, log)
	if err != nil {
		t.Fatalf("parseOutputInstruction: %v", err)
	}
	if instruction.Mode != ShareModePaid {
		t.Fatalf("mode mismatch: %v", instruction.Mode)
	}
	if instruction.IncludeAll != ShareAllow {
		t.Fatalf("include-all mismatch: %v", instruction.IncludeAll)
	}
	if instruction.Currency != "USD" {
		t.Fatalf("currency mismatch: %q", instruction.Currency)
	}
	if instruction.Price == nil || *instruction.Price != 1.99 {
		t.Fatalf("price mismatch: %v", instruction.Price)
	}
	if len(instruction.Parts) != 1 {
		t.Fatalf("expected one part, got %d", len(instruction.Parts))
	}
	part := instruction.Parts[0]
	if part.Href != "#body" || part.Include != ShareRequire {
		t.Fatalf("part mismatch: %+v", part)
	}
	if len(instruction.Documents) != 1 {
		t.Fatalf("expected one document, got %d", len(instruction.Documents))
	}
	doc := instruction.Documents[0]
	if doc.Name != "web" {
		t.Fatalf("document name mismatch: %q", doc.Name)
	}
	if doc.Create == nil || *doc.Create != ShareAllow {
		t.Fatalf("document create mismatch: %v", doc.Create)
	}
	if doc.Price == nil || *doc.Price != 2.50 {
		t.Fatalf("document price mismatch: %v", doc.Price)
	}
	if len(doc.Parts) != 1 || doc.Parts[0].Href != "#ann" {
		t.Fatalf("document parts mismatch: %+v", doc.Parts)
	}
}

func TestParseOutputInstructionInvalidMode(t *testing.T) {
	log := zaptest.NewLogger(t, zaptest.WrapOptions(zap.AddCaller(), zap.AddCallerSkip(1)))
	el := mustElement(t, `<output mode="other" include-all="allow" />`)
	if _, err := parseOutputInstruction(el, log); err == nil {
		t.Fatalf("expected error for invalid mode")
	}
}

func TestParsePartInstructionErrors(t *testing.T) {
	log := zaptest.NewLogger(t, zaptest.WrapOptions(zap.AddCaller(), zap.AddCallerSkip(1)))
	el := mustElement(t, `<part include="allow" />`)
	if _, err := parsePartInstruction(el, log); err == nil {
		t.Fatalf("expected missing href error")
	}
}

func TestParseOutputDocumentErrors(t *testing.T) {
	log := zaptest.NewLogger(t, zaptest.WrapOptions(zap.AddCaller(), zap.AddCallerSkip(1)))
	el := mustElement(t, `<output-document-class></output-document-class>`)
	if _, err := parseOutputDocument(el, log); err == nil {
		t.Fatalf("expected missing name error")
	}
}

func TestParseShareMode(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    ShareMode
		wantErr bool
	}{
		{"free", "free", ShareModeFree, false},
		{"paid", "paid", ShareModePaid, false},
		{"invalid", "gift", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseShareMode(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error for %q", tt.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("expected %v, got %v", tt.want, got)
			}
		})
	}
}

func TestParseShareDirective(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    ShareDirective
		wantErr bool
	}{
		{"require", "require", ShareRequire, false},
		{"allow", "allow", ShareAllow, false},
		{"deny", "deny", ShareDeny, false},
		{"invalid", "maybe", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseShareDirective(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error for %q", tt.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("expected %v, got %v", tt.want, got)
			}
		})
	}
}

func TestParseStylesheet(t *testing.T) {
	log := zaptest.NewLogger(t, zaptest.WrapOptions(zap.AddCaller(), zap.AddCallerSkip(1)))
	el := mustElement(t, `<stylesheet type="text/css">
		body { margin: 0; }
		p { color: black; }
	</stylesheet>`)

	sheet := parseStylesheet(el, log)
	if sheet.Type != "text/css" {
		t.Fatalf("expected type text/css, got %q", sheet.Type)
	}
	if sheet.Data == "" {
		t.Fatalf("expected non-empty stylesheet data")
	}
	if !containsSubstring(sheet.Data, "body") {
		t.Fatalf("expected stylesheet to contain 'body'")
	}
}

func TestParseDescriptionFull(t *testing.T) {
	log := zaptest.NewLogger(t, zaptest.WrapOptions(zap.AddCaller(), zap.AddCallerSkip(1)))
	el := mustElement(t, `<description xmlns:xlink="http://www.w3.org/1999/xlink">
		<title-info>
			<genre>sf</genre>
			<author>
				<first-name>John</first-name>
				<last-name>Doe</last-name>
			</author>
			<book-title>Test Book</book-title>
			<lang>en</lang>
		</title-info>
		<src-title-info>
			<genre>fantasy</genre>
			<author>
				<first-name>Jane</first-name>
				<last-name>Smith</last-name>
			</author>
			<book-title>Original Title</book-title>
			<lang>ru</lang>
		</src-title-info>
		<document-info>
			<author>
				<first-name>Editor</first-name>
				<last-name>Name</last-name>
			</author>
			<date>2023-01-01</date>
			<id>doc123</id>
		</document-info>
		<publish-info>
			<book-name>Published Name</book-name>
			<year>2023</year>
		</publish-info>
		<custom-info info-type="test">Custom Value</custom-info>
		<output mode="free" include-all="allow">
			<part xlink:href="#body" include="require" />
		</output>
	</description>`)

	desc, err := parseDescription(el, log)
	if err != nil {
		t.Fatalf("parseDescription: %v", err)
	}

	if desc.TitleInfo.BookTitle.Value != "Test Book" {
		t.Fatalf("title-info book title mismatch")
	}
	if desc.SrcTitleInfo == nil {
		t.Fatalf("expected src-title-info")
	}
	if desc.SrcTitleInfo.BookTitle.Value != "Original Title" {
		t.Fatalf("src-title-info book title mismatch")
	}
	if desc.DocumentInfo.ID != "doc123" {
		t.Fatalf("document-info id mismatch")
	}
	if desc.PublishInfo == nil || desc.PublishInfo.Year != "2023" {
		t.Fatalf("publish-info mismatch")
	}
	if len(desc.CustomInfo) != 1 || desc.CustomInfo[0].Type != "test" {
		t.Fatalf("custom-info mismatch")
	}
	if len(desc.Output) != 1 || desc.Output[0].Mode != ShareModeFree {
		t.Fatalf("output mismatch")
	}
}

func TestParseDescriptionWithOutputError(t *testing.T) {
	log := zaptest.NewLogger(t, zaptest.WrapOptions(zap.AddCaller(), zap.AddCallerSkip(1)))
	el := mustElement(t, `<description>
		<title-info>
			<genre>sf</genre>
			<author><first-name>T</first-name></author>
			<book-title>T</book-title>
			<lang>en</lang>
		</title-info>
		<document-info>
			<author><first-name>E</first-name></author>
			<date>2023</date>
			<id>1</id>
		</document-info>
		<output mode="invalid" include-all="allow" />
	</description>`)

	if _, err := parseDescription(el, log); err == nil {
		t.Fatalf("expected error for invalid output mode")
	}
}

func containsSubstring(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && (s == substr || len(s) > len(substr) && findInString(s, substr))
}

func findInString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestImageParsing(t *testing.T) {
	log := zaptest.NewLogger(t, zaptest.WrapOptions(zap.AddCaller(), zap.AddCallerSkip(1)))
	doc := loadSampleDocument(t)

	book, err := ParseBookXML(doc, []string{"notes"}, log)
	if err != nil {
		t.Fatalf("failed to parse book: %v", err)
	}

	// Test that images with IDs in section content are parsed correctly
	t.Run("images in section content have IDs", func(t *testing.T) {
		ids := book.buildIDIndex(log)

		// Check that i_3 (and other images) are indexed
		expectedImages := []string{"i_1", "i_2", "i_3", "i_4", "i_5"}
		for _, imgID := range expectedImages {
			ref, exists := ids[imgID]
			if !exists {
				t.Errorf("image %s not found in ID index", imgID)
				continue
			}
			if ref.Type != "image" {
				t.Errorf("image %s has wrong type: %s", imgID, ref.Type)
			}
		}
	})

	// Test that images are in section.Content, not section.Image
	t.Run("images are flow items not section metadata", func(t *testing.T) {
		foundI3 := false
		for _, body := range book.Bodies {
			for _, section := range body.Sections {
				for _, item := range section.Content {
					if item.Kind == FlowImage && item.Image != nil && item.Image.ID == "i_3" {
						foundI3 = true
						if item.Image.Href != "#skb01.png" {
							t.Errorf("i_3 has wrong href: %s", item.Image.Href)
						}
						if item.Image.Alt != "Прозрачность 1" {
							t.Errorf("i_3 has wrong alt: %s", item.Image.Alt)
						}
					}
				}
			}
		}
		if !foundI3 {
			t.Error("image i_3 not found in section content")
		}
	})
}
