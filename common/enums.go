// The only reason this package exists is because we need some of the enums in
// MHL connector. And since connector is only relevant in Windows build and is
// not part of main program functionality I do not want its config to mix with
// main program configuration at all. So I have to separate enums into package.
package common

// Specification of requested footnotes processing mode.
// ENUM(default, float, floatRenumbered)
type FootnotesMode int

func (f FootnotesMode) IsFloat() bool {
	return f == FootnotesModeFloat || f == FootnotesModeFloatRenumbered
}

// Specification of image resizing mode.
// ENUM(none, keepAR, stretch)
type ImageResizeMode int

// Specification of TOC page placement.
// ENUM(none, before, after)
type TOCPagePlacement int

// type of vignette
// ENUM(book-title-top, book-title-bottom, chapter-title-top, chapter-title-bottom, chapter-end, section-title-top, section-title-bottom, section-end)
type VignettePos string

// Specification of requested output type.
// ENUM(epub2, epub3, kepub, kfx, azw8)
type OutputFmt int

func (o OutputFmt) ForKindle() bool {
	return o == OutputFmtKfx || o == OutputFmtAzw8
}

func (o OutputFmt) Ext() string {
	switch o {
	case OutputFmtKfx:
		return ".kfx"
	case OutputFmtAzw8:
		return ".azw8"
	case OutputFmtEpub2, OutputFmtEpub3:
		return ".epub"
	case OutputFmtKepub:
		return ".kepub.epub"
	default:
		// this should never happen
		panic("unsupported format requested")
	}
}
