package azw3

import (
	"strings"
	"testing"

	"github.com/beevik/etree"
	"go.uber.org/zap"

	"fbc/common"
	"fbc/config"
	"fbc/convert/internal/publicationtest"
	"fbc/convert/publication"
	"fbc/fb2"
)

func assertRenderedPublicationLinks(t *testing.T, pub *publication.Publication) {
	t.Helper()
	documents := make(map[string]*etree.Document)
	for chapter := range pub.Documents() {
		documents[chapter.Filename] = chapter.Doc
	}
	publicationtest.AssertLinks(t, documents, pub.IDToFile)
}

func TestAZW3FootnoteModes(t *testing.T) {
	for _, mode := range []common.FootnotesMode{
		common.FootnotesModeDefault, common.FootnotesModeFloat, common.FootnotesModeFloatRenumbered,
	} {
		t.Run(mode.String(), func(t *testing.T) {
			c := publicationtest.Content(common.OutputFmtAzw3, mode, 20)
			c.PageTrackingEnabled = true
			c.MoreParaStr = ""
			c.BacklinkTemplate = `{{.Format}}/{{.RefNumber}} <&> "'🙂`
			c.FootnotesIndex["note2"] = fb2.FootnoteRef{BodyIdx: 2, SectionIdx: 0}
			main := &c.Book.Bodies[0].Sections[0]
			main.Content = append(main.Content, main.Content[1])
			first := &c.Book.Bodies[1].Sections[0]
			first.Title = azw3TestTitle("Title\u00a0<&> \"'🙂")
			first.Lang = "ru"
			first.Content = []fb2.FlowItem{
				azw3TestParagraph("First <&> \"'🙂 ", fb2.InlineSegment{
					Kind: fb2.InlineStrong, Text: "bold", Children: []fb2.InlineSegment{{Kind: fb2.InlineEmphasis, Text: " nested"}},
				}),
				{Kind: fb2.FlowImage, Image: &fb2.Image{Href: "#unused", Alt: "diagram <&> \"'🙂"}},
				{Kind: fb2.FlowImage, Image: &fb2.Image{Href: "#unused"}},
				azw3TestParagraph("Caption: ", fb2.InlineSegment{Kind: fb2.InlineLink, Href: "https://example.org/?a=1&b=2",
					Children: []fb2.InlineSegment{{Kind: fb2.InlineEmphasis, Text: "see <&> \"'🙂"}}}),
				azw3TestParagraph("Fallback: ", fb2.InlineSegment{Kind: fb2.InlineLink, Href: "https://example.org/?a=<x>&b=\"'🙂"}),
				azw3TestParagraph("Nested note: ", fb2.InlineSegment{Kind: fb2.InlineLink, Href: "#note2", Text: "second"}),
			}
			if mode == common.FootnotesModeDefault {
				// Only internal links are relevant to assertPublicationLinks in this branch.
				first.Content = append(first.Content[:3], azw3TestParagraph("Authored link: ", fb2.InlineSegment{
					Kind: fb2.InlineLink, Href: "#main", Text: "caption",
				}))
			}
			wantTitle := "Title\u00a0<&> \"'🙂"
			if mode == common.FootnotesModeFloat {
				wantTitle = "[" + wantTitle + "]"
			}
			if mode == common.FootnotesModeFloatRenumbered {
				c.Book, c.FootnotesIndex = c.Book.NormalizeFootnoteLabels(
					c.FootnotesIndex, `{{.Format}}/{{.NoteNumber}} <&> "'🙂`, c.OutputFormat, zap.NewNop(),
				)
				wantTitle = "azw3/1 <&> \"'🙂"
			}
			pub, err := publication.Build(t.Context(), c, &config.DocumentConfig{}, publicationOptions(), zap.NewNop())
			if err != nil {
				t.Fatal(err)
			}
			assertRenderedPublicationLinks(t, pub)
			for chapter := range pub.Documents() {
				data, err := chapter.Doc.WriteToString()
				if err != nil {
					t.Fatal(err)
				}
				roundtrip := etree.NewDocument()
				if err := roundtrip.ReadFromString(data); err != nil {
					t.Fatal("special characters broke XML serialization", err)
				}
				if mode == common.FootnotesModeDefault {
					if len(pub.Backlinks) != 2 {
						t.Fatal("a nonempty template must enable default AZW3 return links")
					}
					note := chapter.Doc.FindElement("//div[@id='note1']")
					if note != nil && (note.FindElement(".//strong") == nil || note.FindElement(".//img") == nil) {
						t.Fatal("default mode lost authored note formatting or images")
					}
					if note != nil {
						link := note.FindElement(".//a[@class='link-internal']")
						if link == nil || azw3FootnoteInlineTextFromElement(link) != "caption" {
							t.Fatal("default mode must keep the original clickable link caption")
						}
					}
					continue
				}
				aside := roundtrip.FindElement("//aside[@id='note1']")
				if aside == nil {
					continue
				}
				if aside.SelectAttrValue("xml:lang", "") != "ru" || len(aside.ChildElements()) != 1 {
					t.Fatal("note must retain its language and contain one paragraph")
				}
				p := aside.SelectElement("p")
				if p == nil || len(p.ChildElements()) != 3 {
					t.Fatal("expected only leading title return and two trailing backlinks")
				}
				leading := p.ChildElements()[0]
				if leading.Text() != wantTitle || leading.SelectAttrValue("role", "") != "doc-backlink" ||
					!strings.HasSuffix(leading.SelectAttrValue("href", ""), "#ref-note1-1") {
					t.Fatal("leading return did not use the title/normalized label and first reference", leading)
				}
				text := azw3FootnoteInlineTextFromElement(p)
				for _, want := range []string{
					wantTitle + "\u00a0First <&> \"'🙂 bold nested\n[img:diagram <&> \"'🙂]\n[img:]\nCaption: [link:see <&> \"'🙂]\n",
					"Fallback: [link:https://example.org/?a=<x>&b=\"'🙂]\nNested note: ",
					"\nazw3/1 <&> \"'🙂\u00a0azw3/2 <&> \"'🙂",
				} {
					if !strings.Contains(text, want) {
						t.Errorf("note text %q missing %q", text, want)
					}
				}
				if roundtrip.FindElement("//div[@class='footnote-title']") != nil {
					t.Fatal("float notes must not have a separate title")
				}
				if len(c.BackLinkIndex["note2"]) != 0 || len(c.BackLinkIndex["note1"]) != 2 || len(pub.Backlinks) != 2 {
					t.Fatal("flattened authored links must not register phantom return destinations", c.BackLinkIndex)
				}
			}
		})
	}
}

func TestAZW3FootnoteBacklinkSuppression(t *testing.T) {
	for _, mode := range []common.FootnotesMode{
		common.FootnotesModeDefault, common.FootnotesModeFloat, common.FootnotesModeFloatRenumbered,
	} {
		for _, tt := range []struct {
			name, template string
			count          int
		}{
			{name: "suppressed", template: `{{if ne .Format "azw3"}}return{{end}}`},
			{name: "enabled", template: `[{{.RefNumber}}]`, count: 2},
			{name: "selective", template: `{{if eq .RefNumber 2}}[{{.RefNumber}}]{{end}}`, count: 1},
		} {
			t.Run(mode.String()+"/"+tt.name, func(t *testing.T) {
				c := publicationtest.Content(common.OutputFmtAzw3, mode, 0)
				c.BacklinkTemplate = tt.template
				c.Book.Bodies[1].Sections[0].Title = azw3TestTitle("17")
				main := &c.Book.Bodies[0].Sections[0]
				main.Content = append(main.Content, main.Content[1])
				pub, err := publication.Build(t.Context(), c, &config.DocumentConfig{}, publicationOptions(), zap.NewNop())
				if err != nil {
					t.Fatal(err)
				}
				assertRenderedPublicationLinks(t, pub)
				if len(pub.Backlinks) != tt.count {
					t.Fatalf("generated returns = %d, want %d", len(pub.Backlinks), tt.count)
				}
				for chapter := range pub.Documents() {
					if mode.IsFloat() {
						p := chapter.Doc.FindElement("//aside[@id='note1']/p")
						if p == nil {
							continue
						}
						links := p.ChildElements()
						wantTitle := "17"
						if mode == common.FootnotesModeFloat {
							wantTitle = "[17]"
						}
						if len(links) != tt.count+1 || links[0].Text() != wantTitle ||
							!strings.HasSuffix(links[0].SelectAttrValue("href", ""), "#ref-note1-1") {
							t.Fatal("template suppression changed the leading title return")
						}
						if tt.count == 0 && azw3FootnoteInlineTextFromElement(p) != wantTitle+"\u00a0First note" {
							t.Fatal("suppression left a trailing separator or fallback caption")
						}
						if tt.name == "selective" && azw3FootnoteInlineTextFromElement(p) != wantTitle+"\u00a0First note\n[2]" {
							t.Fatal("selective suppression left unwanted separators or wrong occurrence labels")
						}
					} else if note := chapter.Doc.FindElement("//div[@id='note1']"); note != nil {
						wantParagraphs := 1
						if tt.count > 0 {
							wantParagraphs++
						}
						if len(note.SelectElements("p")) != wantParagraphs {
							t.Fatal("suppression left an empty backlink paragraph")
						}
					}
				}
			})
		}
	}
}

func TestAZW3FloatTitleBrackets(t *testing.T) {
	for _, mode := range []common.FootnotesMode{common.FootnotesModeFloat, common.FootnotesModeFloatRenumbered} {
		for _, tt := range []struct {
			name, title, floatTitle string
		}{
			{name: "original title", title: "Примечание 1", floatTitle: "[Примечание 1]"},
			{name: "already bracketed", title: "[Комментарий I]", floatTitle: "[Комментарий I]"},
			{name: "trimmed", title: "  [17]  ", floatTitle: "[17]"},
			{name: "opening bracket only", title: "[17", floatTitle: "[[17]"},
			{name: "closing bracket only", title: "17]", floatTitle: "[17]]"},
			{name: "special characters", title: "<&> \"'🙂", floatTitle: "[<&> \"'🙂]"},
			{name: "empty"},
		} {
			t.Run(mode.String()+"/"+tt.name, func(t *testing.T) {
				c := publicationtest.Content(common.OutputFmtAzw3, mode, 0)
				c.BacklinkTemplate = `{{ "" }}`
				c.Book.Bodies[1].Sections[0].Title = azw3TestTitle(tt.title)
				c.Book.Bodies[2].Sections[0].Title = azw3TestTitle(tt.title)
				pub, err := publication.Build(t.Context(), c, &config.DocumentConfig{}, publicationOptions(), zap.NewNop())
				if err != nil {
					t.Fatal(err)
				}
				assertRenderedPublicationLinks(t, pub)
				want := strings.TrimSpace(tt.title)
				if mode == common.FootnotesModeFloat {
					want = tt.floatTitle
				}
				found := false
				foundUnlinked := false
				for chapter := range pub.Documents() {
					serialized, err := chapter.Doc.WriteToBytes()
					if err != nil {
						t.Fatal(err)
					}
					doc := etree.NewDocument()
					if err := doc.ReadFromBytes(serialized); err != nil {
						t.Fatal(err)
					}
					if p := doc.FindElement("//aside[@id='note1']/p"); p != nil {
						found = true
						body := "First note"
						if want != "" {
							leading := p.SelectElement("a")
							if leading == nil || leading.Text() != want || leading.SelectAttrValue("role", "") != "doc-backlink" {
								t.Fatal("unexpected leading title return", leading)
							}
							body = want + "\u00a0" + body
						} else if p.SelectElement("a") != nil {
							t.Fatal("empty titles must not create empty or bracket-only returns")
						}
						if got := azw3FootnoteInlineTextFromElement(p); got != body {
							t.Fatalf("note text = %q, want %q", got, body)
						}
					}
					if p := doc.FindElement("//aside[@id='note2']/p"); p != nil {
						foundUnlinked = true
						body := "Second note"
						if want != "" {
							body = want + "\u00a0" + body
						}
						if got := azw3FootnoteInlineTextFromElement(p); got != body || len(p.ChildElements()) != 0 {
							t.Fatalf("unlinked note = %q, want %q without generated markup", got, body)
						}
					}
					for _, link := range doc.FindElements("//a[@class='link-footnote']") {
						if link.Text() != "1" {
							t.Fatal("title brackets changed the authored forward-reference caption")
						}
					}
				}
				if !found || !foundUnlinked || c.Book.Bodies[1].Sections[0].Title.Items[0].Paragraph.Text[0].Text != tt.title {
					t.Fatal("missing note or mutated source title")
				}
			})
		}
	}
}

func TestAZW3FootnotePlaceholders(t *testing.T) {
	link := func(caption, href string) fb2.InlineSegment {
		return fb2.InlineSegment{Kind: fb2.InlineLink, Text: caption, Href: href}
	}
	image := func(alt string) fb2.InlineSegment {
		return fb2.InlineSegment{Kind: fb2.InlineImageSegment, Image: &fb2.InlineImage{Href: "#image", Alt: alt}}
	}
	for _, tt := range []struct {
		name     string
		segments []fb2.InlineSegment
		want     string
	}{
		{name: "image", segments: []fb2.InlineSegment{image("diagram")}, want: "[img:diagram]"},
		{name: "trim image alt", segments: []fb2.InlineSegment{image("  diagram  ")}, want: "[img:diagram]"},
		{name: "image without alt", segments: []fb2.InlineSegment{image("")}, want: "[img:]"},
		{name: "image with whitespace alt", segments: []fb2.InlineSegment{image(" \t ")}, want: "[img:]"},
		{name: "link caption", segments: []fb2.InlineSegment{link("see note", "#note")}, want: "[link:see note]"},
		{name: "link href", segments: []fb2.InlineSegment{link("", "https://example.org")}, want: "[link:https://example.org]"},
		{name: "whitespace caption", segments: []fb2.InlineSegment{link(" \t ", "#note")}, want: "[link:#note]"},
		{name: "already bracketed", segments: []fb2.InlineSegment{link("[2]", "#note")}, want: "[2]"},
		{name: "bracketed spaces", segments: []fb2.InlineSegment{link(" [2]\u00a0", "#note")}, want: " [2]\u00a0"},
		{name: "empty link", segments: []fb2.InlineSegment{link("", "")}},
		{name: "opening bracket only", segments: []fb2.InlineSegment{link("[2", "#note")}, want: "[link:[2]"},
		{name: "surrounding whitespace", segments: []fb2.InlineSegment{
			{Kind: fb2.InlineText, Text: "before"}, link(" see note ", "#note"), {Kind: fb2.InlineText, Text: "after"},
		}, want: "before [link:see note] after"},
		{name: "source brackets outside link", segments: []fb2.InlineSegment{
			{Kind: fb2.InlineText, Text: "["}, link("перекрёстная ссылка", "#note"), {Kind: fb2.InlineText, Text: "]"},
		}, want: "[link:перекрёстная ссылка]"},
		{name: "source brackets inside formatting", segments: []fb2.InlineSegment{{Kind: fb2.InlineEmphasis,
			Children: []fb2.InlineSegment{
				{Kind: fb2.InlineText, Text: "["}, link("перекрёстная ссылка", "#note"), {Kind: fb2.InlineText, Text: "]"},
			},
		}}, want: "[link:перекрёстная ссылка]"},
		{name: "source brackets across formatting", segments: []fb2.InlineSegment{
			{Kind: fb2.InlineStrong, Text: "before ["},
			{Kind: fb2.InlineEmphasis, Children: []fb2.InlineSegment{link("note", "#note")}},
			{Kind: fb2.InlineCode, Text: "] after"},
		}, want: "before [link:note] after"},
		{name: "source brackets with spaces", segments: []fb2.InlineSegment{
			{Kind: fb2.InlineText, Text: "[ "}, link("note", "#note"), {Kind: fb2.InlineText, Text: " ]"},
		}, want: "[ link:note ]"},
		{name: "source brackets around href fallback", segments: []fb2.InlineSegment{
			{Kind: fb2.InlineText, Text: "["}, link("", "#note"), {Kind: fb2.InlineText, Text: "]"},
		}, want: "[link:#note]"},
		{name: "source brackets around special caption", segments: []fb2.InlineSegment{
			{Kind: fb2.InlineText, Text: "["}, link("<&> \"'🙂", "#note"), {Kind: fb2.InlineText, Text: "]"},
		}, want: "[link:<&> \"'🙂]"},
		{name: "literal placeholder unchanged", segments: []fb2.InlineSegment{
			{Kind: fb2.InlineText, Text: "[[link:literal]]"},
		}, want: "[[link:literal]]"},
		{name: "authored nested brackets unchanged", segments: []fb2.InlineSegment{
			{Kind: fb2.InlineText, Text: "["}, link("[2]", "#note"), {Kind: fb2.InlineText, Text: "]"},
		}, want: "[[2]]"},
		{name: "unpaired source bracket", segments: []fb2.InlineSegment{
			{Kind: fb2.InlineText, Text: "["}, link("note", "#note"),
		}, want: "[[link:note]"},
		{name: "formatted caption", segments: []fb2.InlineSegment{{Kind: fb2.InlineLink, Href: "#note",
			Children: []fb2.InlineSegment{{Kind: fb2.InlineStrong, Text: "see ",
				Children: []fb2.InlineSegment{{Kind: fb2.InlineEmphasis, Text: "note"}}}},
		}}, want: "[link:see note]"},
		{name: "linked image", segments: []fb2.InlineSegment{{Kind: fb2.InlineLink, Href: "#note",
			Children: []fb2.InlineSegment{image("diagram")},
		}}, want: "[img:diagram]"},
		{name: "linked image without alt", segments: []fb2.InlineSegment{{Kind: fb2.InlineLink, Href: "#note",
			Children: []fb2.InlineSegment{image("")},
		}}, want: "[img:]"},
		{name: "special image alt", segments: []fb2.InlineSegment{image("<&> \"'🙂")}, want: "[img:<&> \"'🙂]"},
		{name: "special caption", segments: []fb2.InlineSegment{link("<&> \"'🙂", "#note")}, want: "[link:<&> \"'🙂]"},
		{name: "special href", segments: []fb2.InlineSegment{link("", "https://example.org/?q=<x>&a=\"'🙂")},
			want: "[link:https://example.org/?q=<x>&a=\"'🙂]"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got := azw3FootnoteInlineText(tt.segments)
			if got != tt.want {
				t.Fatalf("placeholder text = %q, want %q", got, tt.want)
			}
			doc := etree.NewDocument()
			doc.CreateElement("p").CreateText(got)
			serialized, err := doc.WriteToBytes()
			if err != nil {
				t.Fatal(err)
			}
			roundtrip := etree.NewDocument()
			if err := roundtrip.ReadFromBytes(serialized); err != nil {
				t.Fatal(err)
			}
			if roundtrip.Root().Text() != tt.want || len(roundtrip.Root().ChildElements()) != 0 {
				t.Fatal("placeholder text was double-escaped or interpreted as markup")
			}
		})
	}
}

func TestAZW3FootnotePlainTextBlocks(t *testing.T) {
	paragraph := azw3TestParagraph("text")
	section := &fb2.Section{
		Title: azw3TestTitle("outer title"),
		Epigraphs: []fb2.Epigraph{{Flow: fb2.Flow{Items: []fb2.FlowItem{paragraph}},
			TextAuthors: []fb2.Paragraph{*azw3TestParagraph("epigraph author").Paragraph}}},
		Image:      &fb2.Image{Alt: "section image"},
		Annotation: &fb2.Flow{Items: []fb2.FlowItem{azw3TestParagraph("annotation")}},
		Content: []fb2.FlowItem{
			{Kind: fb2.FlowSubtitle, Subtitle: azw3TestParagraph("subtitle").Paragraph},
			{Kind: fb2.FlowEmptyLine},
			{Kind: fb2.FlowCite, Cite: &fb2.Cite{Items: []fb2.FlowItem{paragraph},
				TextAuthors: []fb2.Paragraph{*azw3TestParagraph("cite author").Paragraph}}},
			{Kind: fb2.FlowPoem, Poem: &fb2.Poem{Title: azw3TestTitle("poem"),
				Epigraphs: []fb2.Epigraph{{Flow: fb2.Flow{Items: []fb2.FlowItem{paragraph}}}},
				Subtitles: []fb2.Paragraph{*azw3TestParagraph("poem subtitle").Paragraph},
				Stanzas: []fb2.Stanza{{Title: azw3TestTitle("stanza"), Subtitle: azw3TestParagraph("stanza subtitle").Paragraph,
					Verses: []fb2.Paragraph{*azw3TestParagraph("verse one").Paragraph, *azw3TestParagraph("verse two").Paragraph}}},
				TextAuthors: []fb2.Paragraph{*azw3TestParagraph("poet").Paragraph}, Date: &fb2.Date{Display: "1900"}}},
			{Kind: fb2.FlowTable, Table: &fb2.Table{Rows: []fb2.TableRow{
				{Cells: []fb2.TableCell{{Content: azw3TestParagraph("a").Paragraph.Text}, {Content: azw3TestParagraph("b").Paragraph.Text}}},
				{Cells: []fb2.TableCell{{Content: azw3TestParagraph("c").Paragraph.Text}}},
			}}},
			{Kind: fb2.FlowSection, Section: &fb2.Section{Title: azw3TestTitle("nested"), Content: []fb2.FlowItem{paragraph}}},
			azw3TestParagraph("inline ", fb2.InlineSegment{Kind: fb2.InlineImageSegment, Image: &fb2.InlineImage{Alt: "alt"}},
				fb2.InlineSegment{Kind: fb2.InlineImageSegment, Image: &fb2.InlineImage{Href: "#omitted"}}),
		},
	}
	var out azw3FootnoteText
	out.section(section, false)
	want := "text\nepigraph author\n[img:section image]\nannotation\nsubtitle\n\ntext\ncite author\n" +
		"poem\ntext\npoem subtitle\nstanza\nstanza subtitle\nverse one\nverse two\npoet\n1900\na b\nc\nnested\ntext\ninline [img:alt][img:]"
	if got := strings.Join(out.lines, "\n"); got != want {
		t.Fatalf("plain text = %q, want %q", got, want)
	}
}

func TestAZW3FootnoteIncomingTargetsAndIntroductions(t *testing.T) {
	for _, mode := range []common.FootnotesMode{common.FootnotesModeFloat, common.FootnotesModeFloatRenumbered} {
		t.Run(mode.String(), func(t *testing.T) {
			c := publicationtest.Content(common.OutputFmtAzw3, mode, 0)
			first := &c.Book.Bodies[1].Sections[0]
			p := first.Content[0].Paragraph
			p.ID = "inside-note"
			c.IDsIndex[p.ID] = fb2.ElementRef{Path: []any{&c.Book.Bodies[1], first, p}}
			main := &c.Book.Bodies[0].Sections[0]
			main.Content = append(main.Content, azw3TestParagraph("", fb2.InlineSegment{
				Kind: fb2.InlineLink, Href: "#inside-note", Text: "paragraph target",
			}))
			c.Book.Bodies[2].Title = &fb2.Title{Items: []fb2.TitleItem{{Paragraph: azw3TestParagraph("Comments ", fb2.InlineSegment{
				Kind: fb2.InlineLink, Href: "#note1", Text: "intro reference",
			}).Paragraph}}}
			pub, err := publication.Build(t.Context(), c, &config.DocumentConfig{}, publicationOptions(), zap.NewNop())
			if err != nil {
				t.Fatal(err)
			}
			assertRenderedPublicationLinks(t, pub)
			if len(c.BackLinkIndex["note1"]) != 2 || len(pub.Backlinks) != 2 {
				t.Fatal("later introduction reference must be registered before rendering notes")
			}
			found := false
			for chapter := range pub.Documents() {
				for _, link := range chapter.Doc.FindElements("//a") {
					if link.Text() == "paragraph target" {
						found = true
						if !strings.HasSuffix(link.SelectAttrValue("href", ""), "#note1") {
							t.Fatal("incoming paragraph reference was not redirected to its note")
						}
					}
				}
			}
			if !found {
				t.Fatal("incoming link disappeared")
			}
		})
	}
}

func azw3TestParagraph(value string, rest ...fb2.InlineSegment) fb2.FlowItem {
	return fb2.FlowItem{Kind: fb2.FlowParagraph, Paragraph: &fb2.Paragraph{
		Text: append([]fb2.InlineSegment{{Kind: fb2.InlineText, Text: value}}, rest...),
	}}
}

func azw3TestTitle(value string) *fb2.Title {
	return &fb2.Title{Items: []fb2.TitleItem{{Paragraph: azw3TestParagraph(value).Paragraph}}}
}
