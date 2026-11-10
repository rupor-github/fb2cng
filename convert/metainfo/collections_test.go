package metainfo

import (
	"fmt"
	"testing"

	"github.com/beevik/etree"

	"fbc/fb2"
)

func TestAddSequences(t *testing.T) {
	number := 7
	for _, tt := range []struct {
		name                     string
		sequences                []fb2.Sequence
		titles, kinds, positions []string
	}{
		{name: "none"},
		{
			name: "flat numbered", sequences: []fb2.Sequence{{Name: "Series", Number: &number}},
			titles: []string{"Series"}, kinds: []string{"series"}, positions: []string{"7"},
		},
		{
			name: "nested and parallel", sequences: []fb2.Sequence{
				{Name: "Серия & One", Number: &number, Children: []fb2.Sequence{{Name: "Nested"}}}, {Name: "Parallel"},
			},
			titles: []string{"Серия & One", "Nested", "Parallel"}, kinds: []string{"series", "set", "series"},
			positions: []string{"7", "", ""},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			doc := etree.NewDocument()
			metadata := doc.CreateElement("metadata")
			AddSequences(metadata, tt.sequences)
			collections := metadata.FindElements("meta[@property='belongs-to-collection']")
			if len(collections) != len(tt.titles) {
				t.Fatalf("got %d collections, want %d", len(collections), len(tt.titles))
			}
			for i, collection := range collections {
				id := fmt.Sprintf("collection%d", i+1)
				if collection.Text() != tt.titles[i] || collection.SelectAttrValue("id", "") != id {
					t.Error("collection label, ordering or identity changed")
				}
				kind := metadata.FindElement("meta[@refines='#" + id + "'][@property='collection-type']")
				if kind == nil || kind.Text() != tt.kinds[i] {
					t.Error("collection nesting/type changed")
				}
				position := metadata.FindElement("meta[@refines='#" + id + "'][@property='group-position']")
				if tt.positions[i] == "" {
					if position != nil {
						t.Error("unnumbered collection acquired a position")
					}
				} else if position == nil || position.Text() != tt.positions[i] {
					t.Error("collection number changed")
				}
			}
		})
	}
}
