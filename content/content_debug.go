package content

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

	out := c.Book.String()

	if len(c.FootnotesIndex) > 0 {
		tw := treeWriter{debug.NewTreeWriter()}

		tw.Line(0, "Footnotes index: %d", len(c.FootnotesIndex))
		keys := slices.Collect(maps.Keys(c.FootnotesIndex))
		sort.Sort(natural.StringSlice(keys))
		for _, k := range keys {
			ref := c.FootnotesIndex[k]
			tw.Line(1, "Reference[%q] body[%d] section[%d]", k, ref.BodyIdx, ref.SectionIdx)
		}
		out += "\n" + tw.String()
	}

	if len(c.ImagesIndex) > 0 {
		tw := treeWriter{debug.NewTreeWriter()}

		tw.Line(0, "Book cover ID: %q", c.CoverID)
		tw.Line(0, "Images index: %d", len(c.ImagesIndex))
		keys := slices.Collect(maps.Keys(c.ImagesIndex))
		sort.Sort(natural.StringSlice(keys))
		for _, k := range keys {
			img := c.ImagesIndex[k]
			tw.Line(1, "Image[%q] mime[%q] size[%d] dim[%dx%d]", k, img.MimeType, len(img.Data), img.Dim.Width, img.Dim.Height)
		}
		out += "\n" + tw.String()
	}

	if len(c.IDsIndex) > 0 {
		tw := debug.NewTreeWriter()
		tw.Line(0, "IDIndex (%d entries)", len(c.IDsIndex))
		keys := slices.Collect(maps.Keys(c.IDsIndex))
		sort.Sort(natural.StringSlice(keys))
		for _, k := range keys {
			ref := c.IDsIndex[k]
			tw.Line(1, "ID=%q type=%q", k, ref.Type)
			tw.Line(2, "Location Path: %s", fb2.FormatRefPath(ref.Path))
		}
		out += "\n" + tw.String()
	}

	if len(c.LinksRevIndex) > 0 {
		tw := debug.NewTreeWriter()
		totalLinks := 0
		for _, refs := range c.LinksRevIndex {
			totalLinks += len(refs)
		}
		tw.Line(0, "ReverseLinkIndex (%d targets, %d total links)", len(c.LinksRevIndex), totalLinks)

		keys := slices.Collect(maps.Keys(c.LinksRevIndex))
		sort.Sort(natural.StringSlice(keys))
		for _, k := range keys {
			refs := c.LinksRevIndex[k]
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
