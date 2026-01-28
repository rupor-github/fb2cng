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
