package tocnav

import (
	"reflect"
	"testing"

	"fbc/common"
)

func TestShape(t *testing.T) {
	items := []Item{
		{Title: "A", Href: "a", Level: 1},
		{Title: "B", Href: "b", Level: 2},
		{Title: "C", Href: "c", Level: 3},
		{Title: "D", Href: "d", Level: 4},
		{Title: "E", Href: "e", Level: 5},
		{Title: "F", Href: "f", Level: 3},
		{Title: "G", Href: "g", Level: 2},
		{Title: "H", Href: "h", Level: 1},
	}

	tests := []struct {
		name    string
		tocType common.TOCType
		want    []nodeSnapshot
	}{
		{
			name:    "normal",
			tocType: common.TOCTypeNormal,
			want: []nodeSnapshot{
				{Title: "A", Children: []nodeSnapshot{
					{Title: "B", Children: []nodeSnapshot{
						{Title: "C", Children: []nodeSnapshot{
							{Title: "D", Children: []nodeSnapshot{{Title: "E"}}},
						}},
						{Title: "F"},
					}},
					{Title: "G"},
				}},
				{Title: "H"},
			},
		},
		{
			name:    "old kindle",
			tocType: common.TOCTypeOldKindle,
			want: []nodeSnapshot{
				{Title: "A", Children: []nodeSnapshot{
					{Title: "B"},
					{Title: "C"},
					{Title: "D"},
					{Title: "E"},
					{Title: "F"},
					{Title: "G"},
				}},
				{Title: "H"},
			},
		},
		{
			name:    "flat",
			tocType: common.TOCTypeFlat,
			want: []nodeSnapshot{
				{Title: "A"},
				{Title: "B"},
				{Title: "C"},
				{Title: "D"},
				{Title: "E"},
				{Title: "F"},
				{Title: "G"},
				{Title: "H"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := snapshotNodes(Shape(items, tt.tocType))
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("Shape() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestShape_NormalKeepsBookTitleAtTopLevel(t *testing.T) {
	items := []Item{
		{Title: "Book", Href: "book", Level: 0},
		{Title: "Chapter", Href: "chapter", Level: 1},
		{Title: "Section", Href: "section", Level: 2},
	}

	got := snapshotNodes(Shape(items, common.TOCTypeNormal))
	want := []nodeSnapshot{
		{Title: "Book"},
		{Title: "Chapter", Children: []nodeSnapshot{{Title: "Section"}}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Shape() = %#v, want %#v", got, want)
	}
}

func TestShape_Empty(t *testing.T) {
	if got := Shape(nil, common.TOCTypeNormal); got != nil {
		t.Fatalf("Shape(nil) = %#v, want nil", got)
	}
}

type nodeSnapshot struct {
	Title    string
	Children []nodeSnapshot
}

func snapshotNodes(nodes []*Node) []nodeSnapshot {
	out := make([]nodeSnapshot, 0, len(nodes))
	for _, node := range nodes {
		var children []nodeSnapshot
		if len(node.Children) > 0 {
			children = snapshotNodes(node.Children)
		}
		out = append(out, nodeSnapshot{
			Title:    node.Item.Title,
			Children: children,
		})
	}
	return out
}
