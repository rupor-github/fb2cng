package publication

import (
	"strings"
	"testing"

	"github.com/beevik/etree"
	"go.uber.org/zap"

	"fbc/common"
	"fbc/config"
	"fbc/content"
	"fbc/fb2"
)

func TestEPUBBacklinkSeparators(t *testing.T) {
	for _, format := range []common.OutputFmt{
		common.OutputFmtEpub2, common.OutputFmtEpub3, common.OutputFmtKepub, common.OutputFmtKepub3,
	} {
		for _, tt := range []struct {
			name, template, want string
			wantRefs             []string
		}{
			{name: "empty", template: `{{- "" -}}`},
			{name: "whitespace", template: "{{ ` \n\t ` }}"},
			{
				name: "middle only", template: `{{- if eq .RefNumber 2 -}}↩<&>🙂{{- end -}}`, want: "prefix↩<&>🙂",
				wantRefs: []string{"#ref-n1-2"},
			},
			{
				name: "first and last", template: `{{- if ne .RefNumber 2 -}}↩<&>🙂{{- end -}}`, want: "prefix↩<&>🙂\u00a0↩<&>🙂",
				wantRefs: []string{"#ref-n1-1", "#ref-n1-3"},
			},
		} {
			t.Run(format.String()+"/"+tt.name, func(t *testing.T) {
				c := &renderState{Content: &content.Content{OutputFormat: format, BacklinkTemplate: tt.template}}
				for range 3 {
					c.AddFootnoteBackLinkRef("n1")
				}
				parent := etree.NewElement("p")
				wrote := appendEPUBFootnoteBacklinks(parent, c, "n1", epubFootnoteBacklinkOptions{LeadingSeparator: "prefix"})
				if wrote != (len(tt.wantRefs) > 0) {
					t.Fatal("empty/nonempty return status does not match generated links")
				}
				var text strings.Builder
				var hrefs []string
				for _, token := range parent.Child {
					switch child := token.(type) {
					case *etree.CharData:
						text.WriteString(child.Data)
					case *etree.Element:
						text.WriteString(backlinkText(child))
						hrefs = append(hrefs, child.SelectAttrValue("href", ""))
					}
				}
				if text.String() != tt.want || len(hrefs) != len(tt.wantRefs) {
					t.Fatalf("text/targets = %q/%v, want %q/%v", text.String(), hrefs, tt.want, tt.wantRefs)
				}
				for i, href := range hrefs {
					if href != tt.wantRefs[i] {
						t.Fatalf("target %d = %q, want %q", i, href, tt.wantRefs[i])
					}
				}
			})
		}
	}
}

func TestEPUBBacklinkTemplateModes(t *testing.T) {
	cfg, err := config.LoadConfiguration("")
	if err != nil {
		t.Fatal(err)
	}
	for _, format := range []common.OutputFmt{
		common.OutputFmtEpub2, common.OutputFmtEpub3, common.OutputFmtKepub, common.OutputFmtKepub3,
	} {
		for _, mode := range []common.FootnotesMode{
			common.FootnotesModeDefault, common.FootnotesModeFloat, common.FootnotesModeFloatRenumbered,
		} {
			for _, pageSize := range []int{0, 30} {
				for _, tt := range []struct {
					name, template string
					wantReturns    int
				}{
					{name: "default template", template: cfg.Document.Footnotes.BacklinkTemplate, wantReturns: 4},
					{name: "nonempty", template: " RETURN ", wantReturns: 4},
					{name: "empty", template: `{{- "" -}}`},
					{name: "whitespace", template: "{{ ` \n\t ` }}"},
					{name: "selective", template: `{{- if eq .RefNumber 2 -}}RETURN{{- end -}}`, wantReturns: 1},
					{name: "page-dependent", template: `{{- if .PageNumber -}}RETURN{{- end -}}`},
					{name: "missing template", wantReturns: 4},
					{name: "invalid syntax", template: "{{", wantReturns: 4},
					{name: "execution failure", template: "{{.MissingField}}", wantReturns: 4},
				} {
					name := format.String() + "/" + mode.String() + "/" + tt.name
					if pageSize > 0 {
						name += "/pages"
					}
					t.Run(name, func(t *testing.T) {
						c := buildTestContent(format, mode, pageSize)
						c.BacklinkTemplate = tt.template
						c.MoreParaStr = "(~) "
						link := func(target string) fb2.FlowItem {
							return fb2.FlowItem{Kind: fb2.FlowParagraph, Paragraph: &fb2.Paragraph{Text: []fb2.InlineSegment{
								{Kind: fb2.InlineLink, Href: "#" + target, Text: "reference"},
							}}}
						}
						main := &c.Book.Bodies[0].Sections[0]
						main.Content = append(main.Content, link("note1"), link("note2"))
						c.FootnotesIndex["note2"] = fb2.FootnoteRef{BodyIdx: 2, SectionIdx: 0}
						first, second := &c.Book.Bodies[1].Sections[0], &c.Book.Bodies[2].Sections[0]
						first.Content = append(first.Content, link("main"))
						second.Content = append(second.Content, link("note1"))
						pub, err := Build(t.Context(), c, &config.DocumentConfig{}, testMarkup(format), zap.NewNop())
						if err != nil {
							t.Fatal(err)
						}
						assertPublicationLinks(t, pub)
						returns, forwards, asides, more, authored := 0, 0, 0, 0, 0
						returnHrefs := make(map[string]bool)
						for chapter := range pub.Documents() {
							koboIDs := make(map[string]bool)
							for _, span := range chapter.Doc.FindElements("//span[@class='koboSpan']") {
								id := span.SelectAttrValue("id", "")
								if id == "" || koboIDs[id] {
									t.Fatalf("missing or duplicate Kobo span ID %q", id)
								}
								koboIDs[id] = true
							}
							asides += len(chapter.Doc.FindElements("//aside[@epub:type='footnote']"))
							more += len(chapter.Doc.FindElements("//span[@class='footnote-more']"))
							for _, anchor := range chapter.Doc.FindElements("//a") {
								switch anchor.SelectAttrValue("class", "") {
								case "link-backlink":
									returns++
									returnHrefs[anchor.SelectAttrValue("href", "")] = true
									if strings.TrimSpace(backlinkText(anchor)) == "" {
										t.Fatal("generated empty backlink element")
									}
									if testMarkup(format) == HTML5 && (anchor.SelectAttrValue("epub:type", "") != "backlink" ||
										anchor.SelectAttrValue("role", "") != "doc-backlink") {
										t.Fatal("generated return lost EPUB3 backlink semantics")
									}
									if format.IsKobo() && anchor.FindElement("./span[@class='koboSpan']") == nil {
										t.Fatal("generated return lost Kobo span")
									}
								case "link-footnote":
									forwards++
									wantType, wantRole := "", ""
									if mode.IsFloat() && testMarkup(format) == HTML5 {
										wantType, wantRole = "noteref", "doc-noteref"
									}
									if anchor.SelectAttrValue("epub:type", "") != wantType || anchor.SelectAttrValue("role", "") != wantRole {
										t.Fatal("suppression changed forward note semantics")
									}
								case "link-internal":
									if strings.HasSuffix(anchor.SelectAttrValue("href", ""), "#main") {
										authored++
									}
								}
							}
							for _, paragraph := range chapter.Doc.FindElements("//p") {
								if len(paragraph.Child) == 0 {
									t.Fatal("suppressed returns left an empty paragraph")
								}
							}
						}
						wantReturns := tt.wantReturns
						if tt.name == "page-dependent" && pageSize > 0 {
							wantReturns = 4
						}
						if returns != wantReturns || forwards != 4 || authored != 1 {
							t.Fatalf("returns/forwards/authored = %d/%d/%d, want %d/4/1", returns, forwards, authored, wantReturns)
						}
						if tt.name == "selective" && !returnHrefs[c.BackLinkIndex["note1"][1].Href] {
							t.Fatalf("selective return points to the wrong occurrence: %v", returnHrefs)
						}
						for _, note := range []struct {
							id   string
							refs int
						}{{id: "note1", refs: 3}, {id: "note2", refs: 1}} {
							if len(c.BackLinkIndex[note.id]) != note.refs || pub.IDToFile[note.id] == "" {
								t.Fatalf("lost note target or reference registration: %s", note.id)
							}
							for _, ref := range c.BackLinkIndex[note.id] {
								if pub.IDToFile[ref.RefID] == "" {
									t.Fatalf("lost reference anchor %s", ref.RefID)
								}
							}
						}
						wantAsides, wantMore := 0, 0
						if mode.IsFloat() {
							wantMore = 2
							if testMarkup(format) == HTML5 {
								wantAsides = 2
							}
						}
						if asides != wantAsides || more != wantMore {
							t.Fatalf("asides/more indicators = %d/%d, want %d/%d", asides, more, wantAsides, wantMore)
						}
					})
				}
			}
		}
	}
}

func TestEPUBEmptyNoteBacklinkSuppression(t *testing.T) {
	for _, format := range []common.OutputFmt{
		common.OutputFmtEpub2, common.OutputFmtEpub3, common.OutputFmtKepub, common.OutputFmtKepub3,
	} {
		for _, mode := range []common.FootnotesMode{
			common.FootnotesModeDefault, common.FootnotesModeFloat, common.FootnotesModeFloatRenumbered,
		} {
			t.Run(format.String()+"/"+mode.String(), func(t *testing.T) {
				c := buildTestContent(format, mode, 0)
				c.BacklinkTemplate = `{{- "" -}}`
				c.Book.Bodies[1].Sections[0].Content = nil
				pub, err := Build(t.Context(), c, &config.DocumentConfig{}, testMarkup(format), zap.NewNop())
				if err != nil {
					t.Fatal(err)
				}
				assertPublicationLinks(t, pub)
				if pub.IDToFile["note1"] == "" || pub.IDToFile["ref-note1-1"] == "" {
					t.Fatal("lost empty note destination or reference anchor")
				}
				var note *etree.Element
				for chapter := range pub.Documents() {
					if len(chapter.Doc.FindElements("//a[@class='link-backlink']")) != 0 {
						t.Fatal("empty note retained generated returns")
					}
					if found := chapter.Doc.FindElement("//*[@id='note1']"); found != nil {
						note = found
					}
				}
				if note == nil || len(note.Child) != 0 {
					t.Fatal("suppressed empty note gained stray separators or return markup")
				}
			})
		}
	}
}
