package kfx

import "slices"

// SegmentNestedStyleEvents processes a list of possibly overlapping style events
// (as produced by recursive inline walks) and returns deduplicated, sorted events.
//
// Unlike a segmentation approach that splits overlapping events into non-overlapping
// pieces, this function preserves overlapping events as KP3 (Kindle Previewer 3) does.
// The Kindle renderer handles overlapping style_events correctly by applying them
// in order, with later events taking precedence for overlapping regions.
//
// The algorithm:
// 1. Deduplicate events at same offset+length (keep most specific)
// 2. Sort by offset ascending, then by length descending (outer events first)
//
// For events with identical offset and length (e.g., from <a><sup>text</sup></a>),
// only the event with the longest style name (most merged styles) or LinkTo is kept.
//
// Example: For <sup><a>text</a></sup> where sup covers [0-6] and link covers [1-5]:
//
//	Input:  [{0,6,sup}, {1,5,link}]
//	Output: [{0,6,sup}, {1,5,link}]  (overlapping events preserved)
//
// The renderer applies sup style to [0-6], then link style to [1-5], so [1-5]
// gets both styles with link taking precedence for conflicting properties.
func SegmentNestedStyleEvents(events []StyleEventRef) []StyleEventRef {
	if len(events) == 0 {
		return nil
	}
	if len(events) == 1 {
		return events
	}

	// Deduplicate events with identical offset+length by keeping the one
	// with the most specific (longest) style name or with LinkTo.
	type posKey struct {
		offset, length int
	}
	bestByPos := make(map[posKey]StyleEventRef)
	for _, ev := range events {
		key := posKey{ev.Offset, ev.Length}
		if existing, ok := bestByPos[key]; ok {
			// Keep the one with LinkTo or longer style name (more merged styles)
			keepNew := false
			if ev.LinkTo != "" && existing.LinkTo == "" {
				keepNew = true
			} else if ev.LinkTo == existing.LinkTo && len(ev.Style) > len(existing.Style) {
				keepNew = true
			}
			if keepNew {
				bestByPos[key] = ev
			}
		} else {
			bestByPos[key] = ev
		}
	}

	// Convert back to slice
	result := make([]StyleEventRef, 0, len(bestByPos))
	for _, ev := range bestByPos {
		result = append(result, ev)
	}

	// Sort by offset ascending, then by length descending (outer/longer events first)
	// This matches KP3 output ordering where outer spans come before inner spans
	slices.SortFunc(result, func(a, b StyleEventRef) int {
		if a.Offset != b.Offset {
			return a.Offset - b.Offset
		}
		// Same offset: longer (outer) events come first
		return b.Length - a.Length
	})

	return result
}

// FillStyleEventGaps fills gaps and empty-styled regions in style events with a base style.
// KP3 ensures every character position in headings has at least a base line-height style.
// This function:
// 1. Removes events with empty Style (no link info)
// 2. Fills gaps between styled regions with baseStyle
// 3. Returns sorted events covering the full content range
//
// Parameters:
//   - events: Deduplicated, sorted style events (from SegmentNestedStyleEvents)
//   - totalLen: Total length of the content string in runes
//   - baseStyle: Style name to use for gap fills (e.g., line-height-only style)
//
// The function preserves link-only events (empty Style but has LinkTo) since
// these are needed for navigation even without visual styling.
func FillStyleEventGaps(events []StyleEventRef, totalLen int, baseStyle string) []StyleEventRef {
	if totalLen <= 0 {
		return nil
	}

	// Filter out empty-styled events (no style AND no link)
	var filtered []StyleEventRef
	for _, ev := range events {
		if ev.Style != "" || ev.LinkTo != "" {
			filtered = append(filtered, ev)
		}
	}

	// If no styled events, return single base style covering everything
	if len(filtered) == 0 {
		if baseStyle == "" {
			return nil
		}
		return []StyleEventRef{{Offset: 0, Length: totalLen, Style: baseStyle}}
	}

	// If no base style to fill with, just return filtered events
	if baseStyle == "" {
		return filtered
	}

	// Track which positions are covered by styled events.
	// We use a simple approach: find gaps between event boundaries.
	// Since events can overlap (KP3 behavior), we track the furthest end position seen.
	var result []StyleEventRef
	covered := 0 // Furthest position covered so far

	for _, ev := range filtered {
		// If there's a gap before this event, fill it
		if ev.Offset > covered {
			result = append(result, StyleEventRef{
				Offset: covered,
				Length: ev.Offset - covered,
				Style:  baseStyle,
			})
		}
		result = append(result, ev)
		// Update covered position (events may overlap, so take max)
		if end := ev.Offset + ev.Length; end > covered {
			covered = end
		}
	}

	// Fill any gap at the end
	if covered < totalLen {
		result = append(result, StyleEventRef{
			Offset: covered,
			Length: totalLen - covered,
			Style:  baseStyle,
		})
	}

	return result
}
