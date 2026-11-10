package fb2

import (
	"fbc/common"
	"fbc/config"
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
func ImagePolicyFor(format common.OutputFmt, cfg *config.ImagesConfig) ImagePolicy {
	requireJPEG := format.IsKindle() ||
		(format == common.OutputFmtMd && cfg.Markdown == common.MarkdownImageModeEmbedded)
	return ImagePolicy{
		RequireJPEG:       requireJPEG,
		NativeSVG:         format.IsKindle(),
		KindleSVGStrokes:  requireJPEG,
		PreservePDFSizing: format == common.OutputFmtPdf,
		ConvertWebP:       format.IsKobo(),
	}
}
