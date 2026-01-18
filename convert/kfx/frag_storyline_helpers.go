package kfx

import (
	"strings"

	"fbc/fb2"
)

// resolveInlineStyle resolves an inline style spec to a generated style name.
// It does NOT mark the style as used - the caller must call MarkUsage for styles
// that actually survive processing (e.g., after segmentation deduplication).
func resolveInlineStyle(styles *StyleRegistry, ancestorTag string, styleSpec string) string {
	if styles == nil || styleSpec == "" {
		return styleSpec
	}
	parts := strings.Fields(styleSpec)
	if len(parts) == 1 {
		name := parts[0]
		if ancestorTag != "" {
			descendant := ancestorTag + "--" + name
			if _, ok := styles.Get(descendant); ok {
				return styles.EnsureStyleNoMark(descendant)
			}
		}
		return styles.EnsureStyleNoMark(name)
	}
	return styles.ResolveStyleNoMark(styleSpec)
}

// titleHasInlineImages checks if any title paragraph contains inline images.
// Used to decide whether to use combined heading or separate paragraph approach.
func titleHasInlineImages(title *fb2.Title) bool {
	for _, item := range title.Items {
		if item.Paragraph != nil {
			if paragraphHasInlineImages(item.Paragraph) {
				return true
			}
		}
	}
	return false
}

// paragraphHasInlineImages recursively checks if a paragraph has inline images.
func paragraphHasInlineImages(para *fb2.Paragraph) bool {
	for i := range para.Text {
		if segmentHasInlineImages(&para.Text[i]) {
			return true
		}
	}
	return false
}

// segmentHasInlineImages recursively checks if a segment or its children contain images.
func segmentHasInlineImages(seg *fb2.InlineSegment) bool {
	if seg.Kind == fb2.InlineImageSegment {
		return true
	}
	for i := range seg.Children {
		if segmentHasInlineImages(&seg.Children[i]) {
			return true
		}
	}
	return false
}

// paragraphHasTextContent checks if a paragraph contains any actual text (not just images).
// Returns true if there's non-whitespace text, false if the paragraph is image-only.
func paragraphHasTextContent(para *fb2.Paragraph) bool {
	for i := range para.Text {
		if segmentHasTextContent(&para.Text[i]) {
			return true
		}
	}
	return false
}

// segmentHasTextContent recursively checks if a segment contains text.
func segmentHasTextContent(seg *fb2.InlineSegment) bool {
	if seg.Kind == fb2.InlineImageSegment {
		return false
	}
	// Check if this segment has non-whitespace text
	if strings.TrimSpace(seg.Text) != "" {
		return true
	}
	// Check children recursively
	for i := range seg.Children {
		if segmentHasTextContent(&seg.Children[i]) {
			return true
		}
	}
	return false
}
