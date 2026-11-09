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

// ImagePolicyFor selects the capabilities of the existing output adapters.
// A backend not yet exposed as an output format can supply its policy directly.
func ImagePolicyFor(format common.OutputFmt, cfg *config.ImagesConfig) ImagePolicy {
	requireJPEG := format.IsKindle() ||
		(format == common.OutputFmtMd && cfg.Markdown == common.MarkdownImageModeEmbedded)
	return ImagePolicy{
		RequireJPEG:       requireJPEG,
		NativeSVG:         format == common.OutputFmtKfx || format == common.OutputFmtAzw8,
		KindleSVGStrokes:  requireJPEG,
		PreservePDFSizing: format == common.OutputFmtPdf,
		ConvertWebP:       format.IsKobo(),
	}
}
