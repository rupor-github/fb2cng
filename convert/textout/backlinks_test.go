package textout

import (
	"path/filepath"
	"strings"
	"testing"

	"fbc/common"
	"fbc/config"
	"fbc/fb2"
)

func TestMarkdownBacklinkTemplateModes(t *testing.T) {
	cfg, err := config.LoadConfiguration("")
	if err != nil {
		t.Fatal(err)
	}
	for _, mode := range []common.FootnotesMode{
		common.FootnotesModeDefault, common.FootnotesModeFloat, common.FootnotesModeFloatRenumbered,
	} {
		for _, tt := range []struct {
			name, template string
			wantReturns    int
			wantErr        bool
		}{
			{name: "default template", template: cfg.Document.Footnotes.BacklinkTemplate, wantReturns: 4},
			{name: "nonempty", template: " RETURN ", wantReturns: 4},
			{name: "empty", template: `{{- "" -}}`},
			{name: "whitespace", template: "{{ ` \n\t ` }}"},
			{
				name: "selective", template: `{{- if eq .RefNumber 2 -}}RETURN{{- end -}}`, wantReturns: 1,
			},
			{
				name:     "location-dependent main references",
				template: `{{- if and .LocationNumber (eq .SectionTitle "Chapter") -}}RETURN{{- end -}}`, wantReturns: 3,
			},
			{name: "missing template", wantErr: true},
			{name: "invalid syntax", template: "{{", wantErr: true},
			{name: "execution failure", template: "{{.MissingField}}", wantErr: true},
			{name: "later reference failure", template: "{{if eq .RefNumber 2}}{{.MissingField}}{{else}}return{{end}}", wantErr: true},
		} {
			t.Run(mode.String()+"/"+tt.name, func(t *testing.T) {
				c := footnoteContent(common.OutputFmtMd, mode)
				c.BacklinkTemplate = tt.template
				firstNote := &c.Book.Bodies[1].Sections[0]
				firstNote.Content = append(firstNote.Content, paragraphItem(
					fb2.InlineSegment{Kind: fb2.InlineLink, Href: "#chapter-1", Text: "chapter"},
					fb2.InlineSegment{Kind: fb2.InlineText, Text: " and "},
					fb2.InlineSegment{Kind: fb2.InlineLink, Href: "https://example.org", Text: "external"},
				))
				main := &c.Book.Bodies[0].Sections[0]
				main.Content = append(main.Content, paragraphItem(
					fb2.InlineSegment{Kind: fb2.InlineText, Text: "Repeated "},
					fb2.InlineSegment{Kind: fb2.InlineLink, Href: "#note-1", Text: "1"},
					fb2.InlineSegment{Kind: fb2.InlineText, Text: " and another note "},
					fb2.InlineSegment{Kind: fb2.InlineLink, Href: "#note-2", Text: "2"},
				))
				c.FootnotesIndex["note-2"] = fb2.FootnoteRef{BodyIdx: 1, SectionIdx: 1, DisplayText: "Note 2"}
				c.Book.Bodies[1].Sections = append(c.Book.Bodies[1].Sections, fb2.Section{
					ID: "note-2", Title: title("Note 2"), Content: []fb2.FlowItem{paragraphItem(
						fb2.InlineSegment{Kind: fb2.InlineText, Text: "Nested reference "},
						fb2.InlineSegment{Kind: fb2.InlineLink, Href: "#note-1", Text: "1"},
					)},
				})
				output := filepath.Join(t.TempDir(), "book.md")
				data, err := Render(c, testConfig(), output)
				if tt.wantErr {
					if err == nil || !strings.Contains(err.Error(), "backlink_template") || data != nil {
						t.Fatalf("Render() = %q, %v; want no output and a backlink template error", data, err)
					}
					return
				}
				if err != nil {
					t.Fatal(err)
				}
				got := string(data)
				for _, text := range []string{
					"Text", "Footnote text.", "Nested reference", "## Notes", "[chapter](#chapter-1)", "[external](https://example.org)",
				} {
					assertContains(t, got, text)
				}
				prefix := ""
				if mode.IsFloat() {
					prefix = "note-"
				}
				for _, note := range []struct {
					id   string
					refs int
				}{{id: "note-1", refs: 3}, {id: "note-2", refs: 1}} {
					assertContains(t, got, `<a id="`+prefix+note.id+`"></a>`)
					if count := strings.Count(got, "](#"+prefix+note.id+")"); count != note.refs {
						t.Fatalf("forward links to %s = %d, want %d", note.id, count, note.refs)
					}
					if count := len(c.BackLinkIndex[note.id]); count != note.refs {
						t.Fatalf("registered references to %s = %d, want %d", note.id, count, note.refs)
					}
					for _, ref := range c.BackLinkIndex[note.id] {
						assertContains(t, got, `<a id="`+ref.RefID+`"></a>`)
					}
				}
				if count := strings.Count(got, "](book.md#ref-"); count != tt.wantReturns {
					t.Fatalf("generated returns = %d, want %d:\n%s", count, tt.wantReturns, got)
				}
				if tt.name == "selective" {
					assertContains(t, got, "[RETURN](book.md#ref-note-1-2)")
					if strings.Contains(got, "\u00a0") {
						t.Fatalf("suppressed links left separators:\n%s", got)
					}
				}
				if tt.wantReturns == 0 || tt.name == "selective" {
					if strings.Contains(got, `\[\<\]`) || strings.Contains(got, "\n\n\n") {
						t.Fatalf("suppressed links left fallback text or empty paragraphs:\n%s", got)
					}
				}
			})
		}
	}
}

func TestMarkdownEmptyNoteBacklinkSuppression(t *testing.T) {
	for _, mode := range []common.FootnotesMode{
		common.FootnotesModeDefault, common.FootnotesModeFloat, common.FootnotesModeFloatRenumbered,
	} {
		t.Run(mode.String(), func(t *testing.T) {
			c := footnoteContent(common.OutputFmtMd, mode)
			c.BacklinkTemplate = `{{- "" -}}`
			c.Book.Bodies[1].Sections[0].Content = nil
			got, err := renderForTest(c, testConfig())
			if err != nil {
				t.Fatal(err)
			}
			target := "note-1"
			if mode.IsFloat() {
				target = "note-note-1"
			}
			assertContains(t, got, `<a id="`+target+`"></a>`)
			assertContains(t, got, "](#"+target+")")
			assertContains(t, got, `<a id="ref-note-1-1"></a>`)
			if strings.Contains(got, "](#ref-") || strings.Contains(got, `\[\<\]`) {
				t.Fatalf("empty note retained return links or fallback labels:\n%s", got)
			}
		})
	}
}

func TestTXTUnaffectedByBacklinkTemplate(t *testing.T) {
	for _, mode := range []common.FootnotesMode{
		common.FootnotesModeDefault, common.FootnotesModeFloat, common.FootnotesModeFloatRenumbered,
	} {
		t.Run(mode.String(), func(t *testing.T) {
			var want string
			for _, template := range []string{"RETURN", `{{- "" -}}`, "{{ ` \n\t ` }}", "{{", "{{.MissingField}}"} {
				c := footnoteContent(common.OutputFmtTxt, mode)
				c.BacklinkTemplate = template
				got, err := renderForTest(c, testConfig())
				if err != nil {
					t.Fatal(err)
				}
				if want == "" {
					want = got
				} else if got != want {
					t.Fatal("backlink template changed TXT output")
				}
			}
		})
	}
}
