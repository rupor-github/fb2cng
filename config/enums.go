package config

// Specification of requested footnotes processing mode.
// ENUM(default, inline, block, float)
type FootnotesMode int

// Specification of image resizing mode.
// ENUM(none, keepAR, stretch)
type ImageResizeMode int

// Specification of requested output type.
// ENUM(epub2, epub3, kepub, kfx)
type OutputFmt int

func (o OutputFmt) ForKindle() bool {
	return o == OutputFmtKfx
}

func (o OutputFmt) Ext() string {
	switch o {
	case OutputFmtKfx:
		return ".kfx"
	case OutputFmtEpub2, OutputFmtEpub3:
		return ".epub"
	case OutputFmtKepub:
		return ".kepub.epub"
	default:
		// this should never happen
		panic("unsupported format requested")
	}
}
