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
	"fbc/convert/internal/publicationtest"
	"fbc/fb2"
)

func TestBuild(t *testing.T) {
	formats := []common.OutputFmt{
		common.OutputFmtEpub2, common.OutputFmtEpub3, common.OutputFmtKepub, common.OutputFmtKepub3,
	}
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
						c := publicationtest.Content(format, mode, pageSize)
						cfg := config.DocumentConfig{
							Annotation: config.AnnotationConfig{Enable: true, Title: "Annotation", InTOC: true},
							TOCPage: config.TOCPageConfig{
								Placement: placement, AuthorsTemplate: "{{.Format}}",
							},
						}
						pub, err := Build(context.Background(), c, &cfg, testOptions(format), zap.NewNop())
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
						wantLandmarks := []Landmark{{Kind: "bodymatter", Title: "Start", Href: "index00001.xhtml"}}
						if placement != common.TOCPagePlacementNone {
							wantLandmarks = append(wantLandmarks, Landmark{Kind: "toc", Title: "Table of Contents", Href: "toc-page-1.xhtml"})
						}
						if !reflect.DeepEqual(pub.Landmarks, wantLandmarks) {
							t.Errorf("semantic destinations = %+v, want %+v", pub.Landmarks, wantLandmarks)
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
						wantBacklinks := 1
						if backlinks != wantBacklinks {
							t.Errorf("backlinks = %d, want %d", backlinks, wantBacklinks)
						}
						wantAsides := 0
						if mode.IsFloat() && testMarkup(format) == HTML5 {
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

func assertPublicationLinks(t *testing.T, pub *Publication) {
	t.Helper()
	documents := make(map[string]*etree.Document)
	for chapter := range pub.Documents() {
		documents[chapter.Filename] = chapter.Doc
	}
	publicationtest.AssertLinks(t, documents, pub.IDToFile)
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
			pub, err := Build(tt.ctx, tt.c, tt.cfg, Options{Markup: XHTML11}, zap.NewNop())
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
