package azw3

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/beevik/etree"
	"go.uber.org/zap"

	"fbc/config"
	"fbc/fb2"
)

func TestGenerateAZW3DropcapMarkup(t *testing.T) {
	defaults, err := os.ReadFile(filepath.Join("..", "default.css"))
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		name string
		text []fb2.InlineSegment
		want string
	}{
		{name: "Latin", text: []fb2.InlineSegment{{Kind: fb2.InlineText, Text: "A complete paragraph."}}, want: "A complete paragraph."},
		{name: "Cyrillic and formatting", text: []fb2.InlineSegment{
			{Kind: fb2.InlineText, Text: "Буквица "},
			{Kind: fb2.InlineStrong, Text: "рядом"},
			{Kind: fb2.InlineText, Text: " с текстом."},
		}, want: "Буквица рядом с текстом."},
		{name: "non-BMP initial", text: []fb2.InlineSegment{{Kind: fb2.InlineText, Text: "𐐀 Unicode text."}}, want: "𐐀 Unicode text."},
		{name: "short opening", text: []fb2.InlineSegment{{Kind: fb2.InlineText, Text: "Убили, значит, Фердинанда."}},
			want: "Убили, значит, Фердинанда."},
	} {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := config.LoadConfiguration("")
			if err != nil {
				t.Fatal(err)
			}
			cfg.Document.Dropcaps.Enable = true
			ctx, c := prepareGenerationTest(t, cfg, defaults)
			paragraph := c.Book.Bodies[0].Sections[0].Content[0].Paragraph
			if !strings.Contains(paragraph.Style, "has-dropcap") {
				t.Fatal("content preparation did not mark the first paragraph for a drop cap")
			}
			paragraph.Text = tt.text
			const following = "The next paragraph must remain separate from the drop-cap paragraph."
			section := &c.Book.Bodies[0].Sections[0]
			section.Content = slices.Insert(section.Content, 1, fb2.FlowItem{
				Kind: fb2.FlowParagraph,
				Paragraph: &fb2.Paragraph{Text: []fb2.InlineSegment{
					{Kind: fb2.InlineText, Text: following},
				}},
			})
			c.Debug = true
			if _, err := Generate(ctx, c, filepath.Join(t.TempDir(), "dropcap.azw3"), false, &cfg.Document, zap.NewNop()); err != nil {
				t.Fatal(err)
			}
			for _, stage := range []string{"azw3-input", "azw3-compiled"} {
				t.Run(stage, func(t *testing.T) {
					files, err := filepath.Glob(filepath.Join(c.WorkDir, stage, "*.xhtml"))
					if err != nil {
						t.Fatal(err)
					}
					count := 0
					for _, name := range files {
						doc := etree.NewDocument()
						if err := doc.ReadFromFile(name); err != nil {
							t.Fatal(err)
						}
						for _, p := range doc.FindElements("//p[@class='has-dropcap']") {
							count++
							initials := p.FindElements("./span[@class='dropcap']")
							if len(initials) != 1 || elementText(initials[0]) != string([]rune(tt.want)[0]) {
								t.Fatal("drop cap is not a single, directly contained Unicode initial")
							}
							if elementText(p) != tt.want {
								t.Fatalf("drop-cap generation changed paragraph text: %q, want %q", elementText(p), tt.want)
							}
							if tt.name == "Cyrillic and formatting" && p.FindElement("./strong") == nil {
								t.Fatal("drop-cap generation lost following inline formatting")
							}
							siblings := p.Parent().ChildElements()
							next := slices.Index(siblings, p) + 1
							if next >= len(siblings) || siblings[next].Tag != "p" || elementText(siblings[next]) != following {
								t.Fatal("drop-cap generation changed the following paragraph structure")
							}
						}
					}
					if count != 1 {
						t.Fatalf("generated drop-cap paragraphs = %d, want 1", count)
					}
				})
			}
		})
	}
}
