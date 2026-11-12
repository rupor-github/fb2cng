package fb2

import (
	"fbc/common"
)

// ImagePolicy separates raster compatibility from backend vector capability.
// In particular, Kindle JPEG preparation does not imply support for native SVG.
type ImagePolicy struct {
	RequireJPEG       bool
	NativeSVG         bool
	KindleSVGStrokes  bool
	PreservePDFSizing bool
	ConvertWebP       bool
}

// ImagePolicyFor selects the capabilities of the output adapters.
// markdownImages affects only Markdown; other formats use their own capabilities.
func ImagePolicyFor(format common.OutputFmt, markdownImages common.MarkdownImageMode) ImagePolicy {
	requireJPEG := format.IsKindle() ||
		(format == common.OutputFmtMd && markdownImages == common.MarkdownImageModeEmbedded)
	return ImagePolicy{
		RequireJPEG:       requireJPEG,
		NativeSVG:         format.IsKindle(),
		KindleSVGStrokes:  requireJPEG,
		PreservePDFSizing: format == common.OutputFmtPdf,
		ConvertWebP:       format.IsKobo(),
	}
}
