package publication

import (
	"errors"
	"slices"
	"testing"

	"github.com/beevik/etree"
	"go.uber.org/zap"

	"fbc/common"
	"fbc/config"
	"fbc/content"
	"fbc/convert/internal/publicationtest"
	"fbc/convert/kobo"
	"fbc/fb2"
)

func TestBuildDecorationIsExplicit(t *testing.T) {
	for _, format := range []common.OutputFmt{
		common.OutputFmtEpub2, common.OutputFmtKepub, common.OutputFmtKepub3, common.OutputFmtAzw3,
	} {
		for _, decorated := range []bool{false, true} {
			name := "plain"
			if decorated {
				name = "decorated"
			}
			t.Run(format.String()+"/"+name, func(t *testing.T) {
				c := publicationtest.Content(format, common.FootnotesModeFloat, 30)
				options := Options{Markup: HTML5}
				if decorated {
					options.Text = &kobo.Renderer{}
				}
				pub, err := Build(t.Context(), c, &config.DocumentConfig{}, options, zap.NewNop())
				if err != nil {
					t.Fatal(err)
				}
				for chapter := range pub.Documents() {
					if (chapter.Doc.FindElement("//div[@id='book-inner']") != nil) != decorated ||
						(len(chapter.Doc.FindElements("//span[@class='koboSpan']")) > 0) != decorated {
						t.Fatal("output identity selected implicit reader decoration")
					}
				}
				if c.OutputFormat != format || len(pub.Backlinks) != 1 {
					t.Fatal("explicit rendering policy changed template or generated-reference identity")
				}
				assertPublicationLinks(t, pub)
			})
		}
	}
}

type testFloatingFootnotes struct {
	calls []string
	err   error
}

func (r *testFloatingFootnotes) ReferenceBodies(bodies []*fb2.Body) []*fb2.Body {
	r.calls = append(r.calls, "references")
	return bodies
}

func (r *testFloatingFootnotes) Render(parent *etree.Element, section *fb2.Section, rendering FootnoteRendering) error {
	r.calls = append(r.calls, "render")
	if r.err != nil {
		return r.err
	}
	p := parent.CreateElement("p")
	p.CreateAttr("id", section.ID)
	p.CreateText("custom note")
	return rendering.AppendBacklinks(p, section.ID, " ")
}

func (r *testFloatingFootnotes) RewriteLinks(_ []Chapter, _ *content.Content, _ map[string]string) {
	r.calls = append(r.calls, "links")
}

func TestBuildFloatingFootnoteAdapter(t *testing.T) {
	failure := errors.New("custom note failed")
	for _, mode := range []common.FootnotesMode{common.FootnotesModeDefault, common.FootnotesModeFloat} {
		for _, fail := range []bool{false, true} {
			name := "success"
			if fail {
				name = "failure"
			}
			t.Run(mode.String()+"/"+name, func(t *testing.T) {
				renderer := &testFloatingFootnotes{}
				if fail {
					renderer.err = failure
				}
				c := publicationtest.Content(common.OutputFmtEpub3, mode, 0)
				pub, err := Build(t.Context(), c, &config.DocumentConfig{}, Options{
					Markup: HTML5, FloatingFootnotes: renderer,
				}, zap.NewNop())
				if mode.IsFloat() && fail {
					if !errors.Is(err, failure) || pub != nil {
						t.Fatalf("Build() = %v, %v; want the adapter error and no publication", pub, err)
					}
					return
				}
				if err != nil {
					t.Fatal(err)
				}
				if mode.IsFloat() {
					if !slices.Equal(renderer.calls, []string{"references", "render", "render", "links"}) {
						t.Fatalf("adapter lifecycle = %v", renderer.calls)
					}
				} else if len(renderer.calls) != 0 {
					t.Fatal("floating-note adapter was used in default mode")
				}
				assertPublicationLinks(t, pub)
			})
		}
	}
}

func TestBuildStylesheetAdapterError(t *testing.T) {
	failure := errors.New("stylesheet rewrite failed")
	c := publicationtest.Content(common.OutputFmtEpub3, common.FootnotesModeDefault, 0)
	pub, err := Build(t.Context(), c, &config.DocumentConfig{}, Options{
		Markup: HTML5,
		RewriteStylesheet: func([]byte, func(string) string) ([]byte, error) {
			return nil, failure
		},
	}, zap.NewNop())
	if !errors.Is(err, failure) || pub != nil {
		t.Fatalf("Build() = %v, %v; want the stylesheet error and no publication", pub, err)
	}
}
