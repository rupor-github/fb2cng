package kfx

import (
	"strings"

	"fbc/content"
	"fbc/fb2"
)

// SegmentStyle determines the style name for an inline segment.
// Returns the style name, link info, and whether a backlink ref was created.
func SegmentStyle(seg *fb2.InlineSegment, c *content.Content, styles *StyleRegistry) (segStyle string, isLink bool, linkTo string, isFootnoteLink bool, backlinkRefID string) {
	switch seg.Kind {
	case fb2.InlineStrong:
		segStyle = "strong"
	case fb2.InlineEmphasis:
		segStyle = "emphasis"
	case fb2.InlineStrikethrough:
		segStyle = "strikethrough"
	case fb2.InlineSub:
		segStyle = "sub"
	case fb2.InlineSup:
		segStyle = "sup"
	case fb2.InlineCode:
		segStyle = "code"
	case fb2.InlineNamedStyle:
		segStyle = seg.Style
	case fb2.InlineLink:
		isLink = true
		if after, found := strings.CutPrefix(seg.Href, "#"); found {
			linkTo = after
			if c != nil {
				if _, isNote := c.FootnotesIndex[linkTo]; isNote {
					segStyle = "link-footnote"
					isFootnoteLink = true
					ref := c.AddFootnoteBackLinkRef(linkTo)
					backlinkRefID = ref.RefID
				} else {
					segStyle = "link-internal"
				}
			} else {
				segStyle = "link-internal"
			}
		} else {
			segStyle = "link-external"
			if styles != nil {
				linkTo = styles.RegisterExternalLink(seg.Href)
			}
		}
	}
	return
}

// InjectPseudoBefore writes ::before pseudo-element content to the normalizing writer.
// The content inherits styling from the base element (no separate style event).
// Returns true if text was written.
func InjectPseudoBefore(segStyle string, styles *StyleRegistry, nw *normalizingWriter) bool {
	if segStyle == "" || styles == nil {
		return false
	}

	pc := styles.GetPseudoContentForClass(segStyle)
	if pc == nil || pc.Before == "" {
		return false
	}

	nw.WriteString(pc.Before)
	return true
}

// InjectPseudoAfter writes ::after pseudo-element content to the normalizing writer.
// The content inherits styling from the base element (no separate style event).
// Returns true if text was written.
func InjectPseudoAfter(segStyle string, styles *StyleRegistry, nw *normalizingWriter) bool {
	if segStyle == "" || styles == nil {
		return false
	}

	pc := styles.GetPseudoContentForClass(segStyle)
	if pc == nil || pc.After == "" {
		return false
	}

	nw.WriteString(pc.After)
	return true
}

// GetPseudoStartText returns the text that should be used for ContentStartOffset calculation.
// If the segment has ::before content, that's what starts first.
func GetPseudoStartText(seg *fb2.InlineSegment, segStyle string, styles *StyleRegistry) string {
	startText := seg.Text
	if startText == "" && len(seg.Children) > 0 {
		startText = findFirstText(seg)
	}

	if segStyle != "" && styles != nil {
		if pc := styles.GetPseudoContentForClass(segStyle); pc != nil && pc.Before != "" {
			startText = pc.Before
		}
	}

	return startText
}

// HasPseudoBefore returns true if the style has ::before pseudo-element content.
func HasPseudoBefore(segStyle string, styles *StyleRegistry) bool {
	if segStyle == "" || styles == nil {
		return false
	}
	if pc := styles.GetPseudoContentForClass(segStyle); pc != nil {
		return pc.Before != ""
	}
	return false
}

// HasPseudoAfter returns true if the style has ::after pseudo-element content.
func HasPseudoAfter(segStyle string, styles *StyleRegistry) bool {
	if segStyle == "" || styles == nil {
		return false
	}
	if pc := styles.GetPseudoContentForClass(segStyle); pc != nil {
		return pc.After != ""
	}
	return false
}
