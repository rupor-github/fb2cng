package pdf

import (
	"strings"

	"fbc/convert/pdf/docwriter"
	"fbc/convert/pdf/structure"
)

type pdfOutlines struct {
	RootID int
	Items  []*pdfOutlineItem
}

type pdfOutlineItem struct {
	ObjectID int
	Title    string
	PageID   int
	ParentID int
	PrevID   int
	NextID   int
	FirstID  int
	LastID   int
	Count    int
	Children []*pdfOutlineItem
}

func buildOutlines(entries []*structure.TOCEntry, pages []pdfPage, nextObjectID *int) pdfOutlines {
	anchorPages := make(map[string]int)
	for i := range pages {
		for _, id := range pages[i].Anchors {
			if _, exists := anchorPages[id]; !exists {
				anchorPages[id] = i
			}
		}
	}
	nodes := resolveOutlineItems(entries, pages, anchorPages)
	if len(nodes) == 0 {
		return pdfOutlines{}
	}
	outlines := pdfOutlines{
		RootID: *nextObjectID,
	}
	(*nextObjectID)++
	assignOutlineObjectIDs(nodes, nextObjectID)
	linkOutlineSiblings(outlines.RootID, nodes)
	outlines.Items = flattenOutlineItems(nodes)
	return outlines
}

func resolveOutlineItems(entries []*structure.TOCEntry, pages []pdfPage, anchorPages map[string]int) []*pdfOutlineItem {
	items := make([]*pdfOutlineItem, 0, len(entries))
	for _, entry := range entries {
		if entry == nil {
			continue
		}
		children := resolveOutlineItems(entry.Children, pages, anchorPages)
		pageIndex, ok := anchorPages[entry.ID]
		if !ok || pageIndex < 0 || pageIndex >= len(pages) || strings.TrimSpace(entry.Title) == "" {
			items = append(items, children...)
			continue
		}
		items = append(items, &pdfOutlineItem{
			Title:    entry.Title,
			PageID:   pages[pageIndex].ObjectID,
			Children: children,
		})
	}
	return items
}

func assignOutlineObjectIDs(items []*pdfOutlineItem, nextObjectID *int) {
	for _, item := range items {
		item.ObjectID = *nextObjectID
		(*nextObjectID)++
		assignOutlineObjectIDs(item.Children, nextObjectID)
	}
}

func linkOutlineSiblings(parentID int, items []*pdfOutlineItem) {
	for i, item := range items {
		item.ParentID = parentID
		if i > 0 {
			item.PrevID = items[i-1].ObjectID
		}
		if i+1 < len(items) {
			item.NextID = items[i+1].ObjectID
		}
		if len(item.Children) != 0 {
			item.FirstID = item.Children[0].ObjectID
			item.LastID = item.Children[len(item.Children)-1].ObjectID
			item.Count = countOutlineDescendants(item.Children)
			linkOutlineSiblings(item.ObjectID, item.Children)
		}
	}
}

func countOutlineDescendants(items []*pdfOutlineItem) int {
	count := len(items)
	for _, item := range items {
		count += countOutlineDescendants(item.Children)
	}
	return count
}

func flattenOutlineItems(items []*pdfOutlineItem) []*pdfOutlineItem {
	out := make([]*pdfOutlineItem, 0, countOutlineDescendants(items))
	var walk func([]*pdfOutlineItem)
	walk = func(items []*pdfOutlineItem) {
		for _, item := range items {
			out = append(out, item)
			walk(item.Children)
		}
	}
	walk(items)
	return out
}

func writeOutlineObjects(writer *docwriter.Writer, outlines pdfOutlines) error {
	if outlines.RootID == 0 {
		return nil
	}
	root := docwriter.Dict{
		"Count": docwriter.Integer(len(outlines.Items)),
		"Type":  docwriter.Name("Outlines"),
	}
	topLevel := topLevelOutlineItems(outlines)
	if len(topLevel) != 0 {
		root["First"] = docwriter.Ref{ObjectNumber: topLevel[0].ObjectID}
		root["Last"] = docwriter.Ref{ObjectNumber: topLevel[len(topLevel)-1].ObjectID}
	}
	if err := writer.Object(outlines.RootID, root); err != nil {
		return err
	}
	for _, item := range outlines.Items {
		dict := docwriter.Dict{
			"Dest": docwriter.Array{
				docwriter.Ref{ObjectNumber: item.PageID},
				docwriter.Name("Fit"),
			},
			"Parent": docwriter.Ref{ObjectNumber: item.ParentID},
			"Title":  docwriter.UTF16TextString(item.Title),
		}
		if item.PrevID != 0 {
			dict["Prev"] = docwriter.Ref{ObjectNumber: item.PrevID}
		}
		if item.NextID != 0 {
			dict["Next"] = docwriter.Ref{ObjectNumber: item.NextID}
		}
		if item.FirstID != 0 {
			dict["First"] = docwriter.Ref{ObjectNumber: item.FirstID}
			dict["Last"] = docwriter.Ref{ObjectNumber: item.LastID}
			dict["Count"] = docwriter.Integer(item.Count)
		}
		if err := writer.Object(item.ObjectID, dict); err != nil {
			return err
		}
	}
	return nil
}

func topLevelOutlineItems(outlines pdfOutlines) []*pdfOutlineItem {
	items := make([]*pdfOutlineItem, 0)
	for _, item := range outlines.Items {
		if item.ParentID == outlines.RootID {
			items = append(items, item)
		}
	}
	return items
}
