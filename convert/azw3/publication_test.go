package azw3

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"

	"go.uber.org/zap"

	"fbc/common"
	"fbc/config"
	"fbc/content"
	"fbc/convert/publication"
	"fbc/state"
)

func TestFB2PublicationText(t *testing.T) {
	for _, mode := range []common.FootnotesMode{
		common.FootnotesModeDefault, common.FootnotesModeFloat, common.FootnotesModeFloatRenumbered,
	} {
		for _, shape := range []common.TOCType{common.TOCTypeNormal, common.TOCTypeOldKindle, common.TOCTypeFlat} {
			for _, placement := range []common.TOCPagePlacement{
				common.TOCPagePlacementNone, common.TOCPagePlacementBefore, common.TOCPagePlacementAfter,
			} {
				name := fmt.Sprintf("publication-%s-%s-%s", mode, shape, placement)
				t.Run(name, func(t *testing.T) {
					pub := fb2TextPublication(t, mode, shape, placement)
					before := snapshotPublication(t, pub)
					compiled, err := compileText(context.Background(), pub)
					if err != nil {
						t.Fatal(err)
					}
					if !bytes.Equal(bytes.Join(before, nil), bytes.Join(snapshotPublication(t, pub), nil)) {
						t.Error("compiler changed shared source documents")
					}
					for _, item := range pub.Navigation {
						target, local, err := resolveTextTarget("nav.xhtml", item.Href)
						if err != nil || !local {
							t.Fatalf("invalid navigation target %s", item.Href)
						}
						if _, exists := compiled.Positions[target]; !exists {
							t.Errorf("navigation target %s has no native position", item.Href)
						}
					}
					for _, page := range pub.Pages {
						if _, exists := compiled.Positions[linkTarget{Filename: page.Filename, ID: page.SpanID}]; !exists {
							t.Errorf("page %d has no native position", page.PageNum)
						}
					}
					asides, backlinks := 0, 0
					for chapter := range pub.Documents() {
						asides += len(chapter.Doc.FindElements("//aside"))
						backlinks += len(chapter.Doc.FindElements("//a[@class='link-backlink']"))
					}
					if (mode.IsFloat() && asides != 3) || (!mode.IsFloat() && asides != 0) || backlinks != 5 {
						t.Errorf("unexpected source fixture: asides=%d, backlinks=%d", asides, backlinks)
					}
					data := assembleTextProbe(t, compiled)
					summary, err := json.MarshalIndent(struct {
						Chapters   int
						Navigation int
						Pages      int
						Asides     int
						Backlinks  int
					}{len(compiled.Documents), len(pub.Navigation), len(pub.Pages), asides, backlinks}, "", "  ")
					if err != nil {
						t.Fatal(err)
					}
					writeProbeArtifacts(t, map[string][]byte{name + ".azw3": data, name + ".json": summary})
					if os.Getenv("AZW3_KINDLEUNPACK_TEST") == "1" {
						work := unpackProbe(t, name, data)
						assertUnpackedText(t, work, pub, compiled)
					}
				})
			}
		}
	}
}

func fb2TextPublication(
	t *testing.T,
	mode common.FootnotesMode,
	shape common.TOCType,
	placement common.TOCPagePlacement,
) *publication.Publication {
	t.Helper()
	cfg, err := config.LoadConfiguration("")
	if err != nil {
		t.Fatal(err)
	}
	cfg.Document.Footnotes.Mode = mode
	cfg.Document.TOCType = shape
	cfg.Document.TOCPage.Placement = placement
	cfg.Document.Annotation.Enable, cfg.Document.Annotation.InTOC = true, true
	cfg.Document.PageMap.Enable, cfg.Document.PageMap.Size = true, 500
	cfg.Document.Vignettes = config.VignettesConfig{}
	ctx := state.ContextWithEnv(context.Background())
	env := state.EnvFromContext(ctx)
	env.Cfg, env.Log, env.NoCover = cfg, zap.NewNop(), true
	input := `<?xml version="1.0" encoding="UTF-8"?>
<FictionBook xmlns="http://www.gribuser.ru/xml/fictionbook/2.0" xmlns:l="http://www.w3.org/1999/xlink">
 <stylesheet type="text/css">p { color: navy; } aside { font-style: italic; } section { page-break-before: always; }</stylesheet>
 <description>
  <title-info><genre>prose</genre><author><first-name>Test</first-name><last-name>Author</last-name></author>
   <book-title>Native text publication</book-title><annotation><p>Generated annotation.</p></annotation><lang>ru</lang>
  </title-info>
  <document-info><author><nickname>Test</nickname></author><date value="2026-11-09">2026</date>
   <id>b352ad00-8e40-4a85-869f-c0c5e980abf1</id><version>1</version></document-info>
 </description>
 <body><title><p>Book introduction</p></title>
  <section id="chapter1"><title><p>Chapter one</p></title>
   <p id="reference1">First <a l:href="#note17" type="note">[17]</a> and repeated
    <a l:href="#note17" type="note">[17]</a> references.</p>
   <p><a l:href="https://example.org/?x=1&amp;y=2">External</a>.</p>
   <section id="nested"><title><p>Nested chapter</p></title><p>Nested text.</p></section>
  </section>
  <section id="chapter2"><title><p>Chapter two</p></title>` +
		strings.Repeat(`<p>Кириллица, café, 中文, 🙂 &amp; escaped text.</p>`, 120) + `
   <p>Another <a l:href="#comment23" type="note">[23]</a> reference.</p>
  </section>
 </body>
 <body name="notes"><title><p>Notes</p></title>
  <section id="note17"><title><p>17</p></title><epigraph><p>Epigraph.</p></epigraph>
   <p><strong>Rich note</strong>, <emphasis>emphasis</emphasis>, <code>code</code>.</p>
   <poem><stanza><v>First verse</v><v>Second verse</v></stanza></poem>
   <table><tr><td>Cell one</td><td>Cell two</td></tr></table>
   <p>Later note <a l:href="#note18" type="note">[18]</a>.</p>
  </section>
  <section id="note18"><title><p>18</p></title><p>Backward note
   <a l:href="#note17" type="note">[17]</a>.</p></section>
 </body>
 <body name="comments"><title><p>Comments</p></title>
  <section id="comment23"><title><p>23</p></title><p>Comment text.</p></section>
 </body>
</FictionBook>`
	// EPUB3 supplies the existing normalized-content control. Only its already-built
	// publication is compiled; this test does not invent an AZW3 enum or rerender it.
	c, err := content.Prepare(ctx, strings.NewReader(input), "native-text.fb2", common.OutputFmtEpub3, env.Log)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(c.WorkDir); err != nil {
			t.Errorf("remove fixture work directory: %v", err)
		}
	})
	pub, err := publication.Build(ctx, c, &cfg.Document, publication.HTML5, env.Log)
	if err != nil {
		t.Fatal(err)
	}
	if pub.CoverID != "" || len(pub.Images) != 0 || len(pub.Stylesheet.Resources) != 0 {
		t.Fatal("text-pass fixture unexpectedly requires resource compilation")
	}
	return pub
}
