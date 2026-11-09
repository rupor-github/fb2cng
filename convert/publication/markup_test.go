package publication

import (
	"context"
	"strings"
	"testing"

	"go.uber.org/zap"

	"fbc/common"
	"fbc/config"
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
	} {
		for _, mode := range []common.FootnotesMode{
			common.FootnotesModeDefault, common.FootnotesModeFloat, common.FootnotesModeFloatRenumbered,
		} {
			t.Run(tt.name+"/"+mode.String(), func(t *testing.T) {
				c := buildTestContent(tt.format, mode, 0)
				cfg := config.DocumentConfig{
					TOCPage: config.TOCPageConfig{Placement: common.TOCPagePlacementBefore, AuthorsTemplate: "{{.Format}}"},
				}
				pub, err := Build(context.Background(), c, &cfg, tt.markup, zap.NewNop())
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
	c := buildTestContent(common.OutputFmtEpub3, common.FootnotesModeDefault, 0)
	pub, err := Build(context.Background(), c, &config.DocumentConfig{}, Markup(255), zap.NewNop())
	if pub != nil || err == nil || !strings.Contains(err.Error(), "unsupported publication markup") {
		t.Fatalf("publication/error = %v/%v, want an unsupported-markup error", pub, err)
	}
	if len(c.BackLinkIndex) != 0 || len(c.UsedImageIDs) != 0 {
		t.Error("invalid markup should be rejected before rendering")
	}
}
