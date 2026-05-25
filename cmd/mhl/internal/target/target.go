package target

import "fbc/common"

const (
	EPUB = "fb2epub"
	MOBI = "fb2mobi"
	PDF  = "fb2pdf"
)

func Supported(name string) bool {
	switch name {
	case EPUB, MOBI, PDF:
		return true
	default:
		return false
	}
}

func DefaultOutputFormat(name string) common.OutputFmt {
	switch name {
	case MOBI:
		return common.OutputFmtKfx
	case PDF:
		return common.OutputFmtPdf
	default:
		return common.OutputFmtEpub2
	}
}

func SupportsOutputFormat(name string, format common.OutputFmt) bool {
	switch name {
	case MOBI:
		return format.ForKindle()
	case EPUB:
		return format == common.OutputFmtEpub2 || format == common.OutputFmtEpub3 || format == common.OutputFmtKepub
	case PDF:
		return format == common.OutputFmtPdf
	default:
		return false
	}
}
