package convert

import (
	"maps"
	"slices"
	"sort"

	"github.com/maruel/natural"

	"fbc/fb2"
	"fbc/utils/debug"
)

type treeWriter struct {
	*debug.TreeWriter
}

// String returns a readable tree of the whole Content starting with parsed FictionBook.
// It exists solely for manual inspection during debugging.
func (c *Content) String() string {
	if c == nil {
		return "<nil Content>"
	}

	out := c.book.String()

	if len(c.footnotesIndex) > 0 {
		tw := treeWriter{debug.NewTreeWriter()}

		tw.Line(0, "Footnotes index: %d", len(c.footnotesIndex))
		keys := slices.Collect(maps.Keys(c.footnotesIndex))
		sort.Sort(natural.StringSlice(keys))
		for _, k := range keys {
			ref := c.footnotesIndex[k]
			tw.Line(1, "Reference[%q] body[%d] section[%d]", k, ref.BodyIdx, ref.SectionIdx)
		}
		out += "\n" + tw.String()
	}

	if len(c.imagesIndex) > 0 {
		tw := treeWriter{debug.NewTreeWriter()}

		tw.Line(0, "Book cover ID: %q", c.coverID)
		tw.Line(0, "Images index: %d", len(c.imagesIndex))
		keys := slices.Collect(maps.Keys(c.imagesIndex))
		sort.Sort(natural.StringSlice(keys))
		for _, k := range keys {
			img := c.imagesIndex[k]
			tw.Line(1, "Image[%q] mime[%q] size[%d]", k, img.MimeType, len(img.Data))
		}
		out += "\n" + tw.String()
	}

	if len(c.idsIndex) > 0 {
		tw := debug.NewTreeWriter()
		tw.Line(0, "IDIndex (%d entries)", len(c.idsIndex))
		keys := slices.Collect(maps.Keys(c.idsIndex))
		sort.Sort(natural.StringSlice(keys))
		for _, k := range keys {
			ref := c.idsIndex[k]
			tw.Line(1, "ID=%q type=%q", k, ref.Type)
			tw.Line(2, "Location Path: %s", fb2.FormatRefPath(ref.Path))
		}
		out += "\n" + tw.String()
	}

	if len(c.linksRevIndex) > 0 {
		tw := debug.NewTreeWriter()
		totalLinks := 0
		for _, refs := range c.linksRevIndex {
			totalLinks += len(refs)
		}
		tw.Line(0, "ReverseLinkIndex (%d targets, %d total links)", len(c.linksRevIndex), totalLinks)

		keys := slices.Collect(maps.Keys(c.linksRevIndex))
		sort.Sort(natural.StringSlice(keys))
		for _, k := range keys {
			refs := c.linksRevIndex[k]
			tw.Line(1, "Target=%q (%d links)", k, len(refs))
			for i, ref := range refs {
				tw.Line(2, "Link[%d] type=%q", i, ref.Type)
				tw.Line(3, "Location Path: %s", fb2.FormatRefPath(ref.Path))
			}
		}
		out += "\n" + tw.String()
	}

	return out
}
