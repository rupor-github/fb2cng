package publication

import (
	"context"
	"strings"
	"testing"

	"github.com/beevik/etree"
	"go.uber.org/zap"

	"fbc/common"
	"fbc/config"
	"fbc/convert/internal/publicationtest"
	"fbc/fb2"
)

func TestBuildMarkupIndependentOfOutputFormat(t *testing.T) {
	for _, tt := range []struct {
		name   string
		format common.OutputFmt
		markup Markup
	}{
		{name: "html5 with epub2 identity", format: common.OutputFmtEpub2, markup: HTML5},
		{name: "xhtml11 with epub3 identity", format: common.OutputFmtEpub3, markup: XHTML11},
		{name: "html5 with non-epub identity", format: common.OutputFmtAzw8, markup: HTML5},
		{name: "html5 with azw3 identity", format: common.OutputFmtAzw3, markup: HTML5},
	} {
		for _, mode := range []common.FootnotesMode{
			common.FootnotesModeDefault, common.FootnotesModeFloat, common.FootnotesModeFloatRenumbered,
		} {
			t.Run(tt.name+"/"+mode.String(), func(t *testing.T) {
				c := publicationtest.Content(tt.format, mode, 0)
				cfg := config.DocumentConfig{
					TOCPage: config.TOCPageConfig{Placement: common.TOCPagePlacementBefore, AuthorsTemplate: "{{.Format}}"},
				}
				pub, err := Build(context.Background(), c, &cfg, Options{Markup: tt.markup}, zap.NewNop())
				if err != nil {
					t.Fatal(err)
				}
				if c.OutputFormat != tt.format {
					t.Fatal("markup selection changed the actual output format")
				}
				asides, noterefs, backlinkRoles := 0, 0, 0
				for chapter := range pub.Documents() {
					data, err := chapter.Doc.WriteToString()
					if err != nil {
						t.Fatal(err)
					}
					if strings.Contains(data, "DOCTYPE html PUBLIC") != (tt.markup == XHTML11) {
						t.Error("document type did not follow the explicit markup policy")
					}
					asides += len(chapter.Doc.FindElements("//aside"))
					for _, link := range chapter.Doc.FindElements("//a") {
						switch link.SelectAttrValue("epub:type", "") {
						case "noteref":
							noterefs++
						case "backlink":
							backlinkRoles++
						}
						if link.SelectAttrValue("class", "") == "link-backlink" && backlinkText(link) != tt.format.String() {
							t.Error("backlink template used the markup dialect instead of the actual format")
						}
					}
					if strings.HasPrefix(chapter.ID, "toc-page") {
						authors := chapter.Doc.FindElement("//span[@class='toc-title-next']")
						if backlinkText(authors) != tt.format.String() {
							t.Error("visible TOC template used the markup dialect instead of the actual format")
						}
					}
				}
				wantAsides, wantNoterefs, wantBacklinkRoles := 0, 0, 0
				if tt.markup == HTML5 {
					wantBacklinkRoles = 1
					if mode.IsFloat() {
						wantAsides, wantNoterefs = 2, 1
					}
				}
				if asides != wantAsides || noterefs != wantNoterefs || backlinkRoles != wantBacklinkRoles {
					t.Errorf("asides/noterefs/backlink roles = %d/%d/%d, want %d/%d/%d",
						asides, noterefs, backlinkRoles, wantAsides, wantNoterefs, wantBacklinkRoles)
				}
				assertPublicationLinks(t, pub)
			})
		}
	}
}

func TestBuildRejectsUnknownMarkup(t *testing.T) {
	c := publicationtest.Content(common.OutputFmtEpub3, common.FootnotesModeDefault, 0)
	pub, err := Build(context.Background(), c, &config.DocumentConfig{}, Options{Markup: Markup(255)}, zap.NewNop())
	if pub != nil || err == nil || !strings.Contains(err.Error(), "unsupported publication markup") {
		t.Fatalf("publication/error = %v/%v, want an unsupported-markup error", pub, err)
	}
	if len(c.BackLinkIndex) != 0 || len(c.UsedImageIDs) != 0 {
		t.Error("invalid markup should be rejected before rendering")
	}
}

func TestBuildFloatingFootnoteBacklinksOutsideAside(t *testing.T) {
	for _, format := range []common.OutputFmt{common.OutputFmtEpub3, common.OutputFmtKepub3} {
		for _, mode := range []common.FootnotesMode{common.FootnotesModeFloat, common.FootnotesModeFloatRenumbered} {
			t.Run(format.String()+"/"+mode.String(), func(t *testing.T) {
				c := publicationtest.Content(format, mode, 0)
				c.MoreParaStr = "(~) "
				c.FootnotesIndex["note2"] = fb2.FootnoteRef{BodyIdx: 2, SectionIdx: 0}
				link := func(target string) fb2.FlowItem {
					return fb2.FlowItem{Kind: fb2.FlowParagraph, Paragraph: &fb2.Paragraph{Text: []fb2.InlineSegment{
						{Kind: fb2.InlineLink, Href: "#" + target, Children: []fb2.InlineSegment{{Kind: fb2.InlineText, Text: "note"}}},
					}}}
				}
				main := &c.Book.Bodies[0].Sections[0]
				main.Content = append(main.Content, link("note1"))
				first, second := &c.Book.Bodies[1].Sections[0], &c.Book.Bodies[2].Sections[0]
				first.Content = append(first.Content, link("note2"))
				second.Content = append(second.Content, link("note1"))
				pub, err := Build(t.Context(), c, &config.DocumentConfig{}, testOptions(format), zap.NewNop())
				if err != nil {
					t.Fatal(err)
				}
				for chapter := range pub.Documents() {
					for _, aside := range chapter.Doc.FindElements("//aside") {
						id := aside.SelectAttrValue("id", "")
						if len(aside.FindElements(".//a[@class='link-backlink']")) != 0 {
							t.Fatal("generated location labels must remain outside the note's aside", id)
						}
						children := aside.Parent().ChildElements()
						var backs []*etree.Element
						for i, child := range children {
							if child == aside && i+1 < len(children) && children[i+1].Tag == "p" {
								backs = children[i+1].FindElements(".//a[@class='link-backlink']")
								break
							}
						}
						if len(backs) != len(c.BackLinkIndex[id]) || len(backs) == 0 {
							t.Fatal("every return link must follow its note", id)
						}
						for _, back := range backs {
							if back.SelectAttrValue("epub:type", "") != "backlink" || back.SelectAttrValue("role", "") != "doc-backlink" {
								t.Fatal("return links must not be marked as forward note references")
							}
							if pub.Backlinks[back] == "" {
								t.Fatal("generated links must retain their source reference identity")
							}
						}
						indicator := aside.FindElement(".//span[@class='footnote-more']")
						if indicator == nil {
							t.Fatal("generated backlinks stole the note's more-paragraph indicator")
						}
					}
				}
				assertPublicationLinks(t, pub)
			})
		}
	}
}
