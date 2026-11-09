package publication

import (
	"context"
	"errors"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/beevik/etree"
	"go.uber.org/zap"

	"fbc/common"
	"fbc/config"
	"fbc/content"
	"fbc/fb2"
)

func TestBuild(t *testing.T) {
	formats := []common.OutputFmt{common.OutputFmtEpub2, common.OutputFmtEpub3, common.OutputFmtKepub, common.OutputFmtKepub3}
	modes := []common.FootnotesMode{common.FootnotesModeDefault, common.FootnotesModeFloat, common.FootnotesModeFloatRenumbered}
	placements := []common.TOCPagePlacement{common.TOCPagePlacementNone, common.TOCPagePlacementBefore, common.TOCPagePlacementAfter}
	for _, format := range formats {
		for _, mode := range modes {
			for _, placement := range placements {
				for _, pageSize := range []int{0, 30} {
					name := format.String() + "/" + mode.String() + "/" + placement.String()
					if pageSize > 0 {
						name += "/pages"
					}
					t.Run(name, func(t *testing.T) {
						c := buildTestContent(format, mode, pageSize)
						cfg := config.DocumentConfig{
							Annotation: config.AnnotationConfig{Enable: true, Title: "Annotation", InTOC: true},
							TOCPage: config.TOCPageConfig{
								Placement: placement, AuthorsTemplate: "{{.Format}}",
							},
						}
						pub, err := Build(context.Background(), c, &cfg, testMarkup(format), zap.NewNop())
						if err != nil {
							t.Fatal(err)
						}
						if c.OutputFormat != format {
							t.Fatal("builder changed the actual output format")
						}
						documents := slices.Collect(pub.Documents())
						wantFiles := []string{"annotation-page-1.xhtml", "index00001.xhtml", "footnotes.xhtml"}
						if placement == common.TOCPagePlacementBefore {
							wantFiles = append([]string{"toc-page-1.xhtml"}, wantFiles...)
						} else if placement == common.TOCPagePlacementAfter {
							wantFiles = append(wantFiles, "toc-page-1.xhtml")
						}
						var gotFiles []string
						for _, chapter := range documents {
							gotFiles = append(gotFiles, chapter.Filename)
						}
						if !slices.Equal(gotFiles, wantFiles) {
							t.Fatalf("document order = %v, want %v", gotFiles, wantFiles)
						}
						if len(pub.Chapters) != len(documents)+1 {
							t.Error("second note body should add navigation, not a second document")
						}
						for _, id := range []string{"main", "note1", "note2", "annotation-paragraph"} {
							if pub.IDToFile[id] == "" {
								t.Errorf("rendered ID %q missing", id)
							}
						}
						if !reflect.DeepEqual(pub.Navigation, CollectNavigation(pub.Chapters, pub.IDToFile)) {
							t.Error("publication navigation differs from rendered chapter navigation")
						}
						if pub.CoverID != "cover" || len(pub.Images) != 2 || pub.Images["unused"] != nil {
							t.Error("image selection must retain cover and used art only")
						}
						if len(pub.Stylesheet.Resources) != 1 || !strings.Contains(string(pub.Stylesheet.Data), "fonts/probe.ttf") {
							t.Error("CSS-only font missing from the publication")
						}
						assertPublicationLinks(t, pub)
						asides, backlinks := 0, 0
						for _, chapter := range documents {
							asides += len(chapter.Doc.FindElements("//aside"))
							for _, link := range chapter.Doc.FindElements("//a[@class='link-backlink']") {
								backlinks++
								if backlinkText(link) != format.String() {
									t.Errorf("backlink template lost actual format: %q", backlinkText(link))
								}
							}
							if strings.HasPrefix(chapter.ID, "toc-page") {
								authors := chapter.Doc.FindElement("//span[@class='toc-title-next']")
								if backlinkText(authors) != format.String() {
									t.Error("visible TOC template lost actual format")
								}
							}
						}
						if backlinks != 1 {
							t.Errorf("backlinks = %d, want 1", backlinks)
						}
						wantAsides := 0
						if mode.IsFloat() && format.IsEPUB3() {
							wantAsides = 2
						}
						if asides != wantAsides {
							t.Errorf("asides = %d, want %d", asides, wantAsides)
						}
						if pageSize == 0 {
							if len(pub.Pages) != 0 || c.PageTrackingEnabled {
								t.Error("pagination was enabled unexpectedly")
							}
							return
						}
						if !c.PageTrackingEnabled || len(pub.Pages) != c.TotalPages {
							t.Error("pagination state was not retained")
						}
						pageFiles := []string{"cover.xhtml"}
						if placement == common.TOCPagePlacementBefore {
							pageFiles = append(pageFiles, "toc-page-1.xhtml")
						}
						pageFiles = append(pageFiles, "annotation-page-1.xhtml")
						for i, filename := range pageFiles {
							if pub.Pages[i].Filename != filename || pub.Pages[i].SpanID != "" {
								t.Errorf("generated page %d = %+v, want %s without marker", i, pub.Pages[i], filename)
							}
						}
						if placement == common.TOCPagePlacementAfter && pub.Pages[len(pub.Pages)-1].Filename != "toc-page-1.xhtml" {
							t.Error("trailing visible TOC missing from page map")
						}
						for i, page := range pub.Pages {
							if page.PageNum != i+1 {
								t.Errorf("page %d has number %d", i, page.PageNum)
							}
							if page.SpanID != "" && pub.IDToFile[page.SpanID] != page.Filename {
								t.Errorf("page marker %s does not resolve to %s", page.SpanID, page.Filename)
							}
						}
						c.ResetPageMap()
						if len(pub.Pages) == 0 {
							t.Error("publication pagination must survive changes to Content's tracker")
						}
					})
				}
			}
		}
	}
}

func buildTestContent(format common.OutputFmt, mode common.FootnotesMode, pageSize int) *content.Content {
	paragraph := func(id, text string) fb2.FlowItem {
		return fb2.FlowItem{Kind: fb2.FlowParagraph, Paragraph: &fb2.Paragraph{
			ID: id, Text: []fb2.InlineSegment{{Kind: fb2.InlineText, Text: text}},
		}}
	}
	main := makeSection("main", "Main chapter")
	main.Content = []fb2.FlowItem{
		paragraph("text", "A paragraph with enough text to create several approximate pages. Кириллица тоже считается."),
		{Kind: fb2.FlowParagraph, Paragraph: &fb2.Paragraph{Text: []fb2.InlineSegment{
			{Kind: fb2.InlineLink, Href: "#note1", Children: []fb2.InlineSegment{{Kind: fb2.InlineText, Text: "1"}}},
		}}},
		{Kind: fb2.FlowImage, Image: &fb2.Image{Href: "#art"}},
	}
	return &content.Content{
		OutputFormat: format, FootnotesMode: mode, PageSize: pageSize,
		BacklinkTemplate: "{{.Format}}", CoverID: "cover",
		IDsIndex:       fb2.IDIndex{"toc-page": {}, "annotation-page": {}},
		FootnotesIndex: fb2.FootnoteRefs{"note1": {BodyIdx: 1, SectionIdx: 0}},
		ImagesIndex: fb2.BookImages{
			"cover": {Filename: "images/cover.jpg"}, "art": {Filename: "images/art.png"}, "unused": {Filename: "images/unused.png"},
		},
		Book: &fb2.FictionBook{
			Description: fb2.Description{TitleInfo: fb2.TitleInfo{
				BookTitle: fb2.TextField{Value: "Publication"},
				Annotation: &fb2.Flow{Items: []fb2.FlowItem{
					paragraph("annotation-paragraph", "Annotation text that must not add in-document page markers."),
				}},
			}},
			Bodies: []fb2.Body{
				{Sections: []fb2.Section{*main}},
				{Name: "notes", Kind: fb2.BodyFootnotes, Sections: []fb2.Section{
					{ID: "note1", Content: []fb2.FlowItem{paragraph("", "First note")}},
				}},
				{Name: "comments", Kind: fb2.BodyFootnotes, Sections: []fb2.Section{
					{ID: "note2", Content: []fb2.FlowItem{paragraph("", "Second note")}},
				}},
			},
			Stylesheets: []fb2.Stylesheet{{Type: "text/css", Data: "@font-face { src: url('#font'); }", Resources: []fb2.StylesheetResource{
				{OriginalURL: "#font", Filename: "fonts/probe.ttf", MimeType: "font/ttf", Data: []byte("font bytes")},
			}}},
		},
	}
}

func assertPublicationLinks(t *testing.T, pub *Publication) {
	t.Helper()
	files := make(map[string]bool)
	for chapter := range pub.Documents() {
		files[chapter.Filename] = true
	}
	for chapter := range pub.Documents() {
		for _, link := range chapter.Doc.FindElements("//a[@href]") {
			filename, id, _ := strings.Cut(link.SelectAttrValue("href", ""), "#")
			if filename == "" {
				filename = chapter.Filename
			}
			if !files[filename] || (id != "" && pub.IDToFile[id] != filename) {
				t.Errorf("unresolved link in %s: %s", chapter.Filename, link.SelectAttrValue("href", ""))
			}
		}
	}
}

func TestBuildInvalidInput(t *testing.T) {
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	for _, tt := range []struct {
		name string
		ctx  context.Context
		c    *content.Content
		cfg  *config.DocumentConfig
	}{
		{name: "canceled", ctx: canceled},
		{name: "nil content", ctx: context.Background()},
		{name: "nil book", ctx: context.Background(), c: &content.Content{}},
		{name: "nil config", ctx: context.Background(), c: &content.Content{Book: &fb2.FictionBook{}}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			pub, err := Build(tt.ctx, tt.c, tt.cfg, XHTML11, zap.NewNop())
			if err == nil || pub != nil {
				t.Fatal("invalid input should return an error and no publication")
			}
			if tt.name == "canceled" && !errors.Is(err, context.Canceled) {
				t.Errorf("error = %v, want context.Canceled", err)
			}
		})
	}
}

func TestDocuments(t *testing.T) {
	pub := &Publication{Chapters: []Chapter{
		{Filename: "notes.xhtml#second"},
		{ID: "first", Filename: "notes.xhtml", Doc: etree.NewDocument()},
		{ID: "duplicate", Filename: "notes.xhtml#other", Doc: etree.NewDocument()},
		{ID: "next", Filename: "next.xhtml", Doc: etree.NewDocument()},
	}}
	var ids []string
	for chapter := range pub.Documents() {
		ids = append(ids, chapter.ID)
	}
	if !slices.Equal(ids, []string{"first", "next"}) {
		t.Errorf("document IDs = %v", ids)
	}
	visits := 0
	for range pub.Documents() {
		visits++
		break
	}
	if visits != 1 {
		t.Errorf("early termination visited %d documents", visits)
	}
}
