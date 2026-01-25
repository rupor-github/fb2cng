package kfx

import "slices"

// SegmentNestedStyleEvents takes a list of possibly overlapping style events
// (as produced by recursive inline walks) and returns non-overlapping events.
//
// The algorithm:
// 1. Deduplicate events at same offset+length (keep most specific)
// 2. For each position, determine which event should be active (shortest/most-specific wins)
// 3. Generate non-overlapping segments based on these active events
//
// This handles complex cases like <code>text <a>link</a> more</code> where:
//   - code event covers [0-14]
//   - link event covers [5-9]
//
// Result (non-overlapping):
//   - [0-5] code style
//   - [5-9] link style (already has code properties merged)
//   - [9-14] code style
//
// For events with identical offset and length (e.g., from <a><sup>text</sup></a>),
// only the event with the longest style name (most merged styles) or LinkTo is kept.
//
// Events are returned sorted by offset ascending.
func SegmentNestedStyleEvents(events []StyleEventRef) []StyleEventRef {
	if len(events) == 0 {
		return nil
	}
	if len(events) == 1 {
		return events
	}

	// First, deduplicate events with identical offset+length by keeping the one
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
	deduped := make([]StyleEventRef, 0, len(bestByPos))
	for _, ev := range bestByPos {
		deduped = append(deduped, ev)
	}

	if len(deduped) == 1 {
		return deduped
	}

	// Collect all unique boundary points (starts and ends of events)
	pointSet := make(map[int]struct{})
	for _, ev := range deduped {
		pointSet[ev.Offset] = struct{}{}
		pointSet[ev.Offset+ev.Length] = struct{}{}
	}
	points := make([]int, 0, len(pointSet))
	for p := range pointSet {
		points = append(points, p)
	}
	slices.Sort(points)

	// For each segment between consecutive points, find the most specific
	// (shortest) event that covers it. Shorter events are more specific because
	// they represent inner/nested elements with merged styles.
	var result []StyleEventRef
	for i := 0; i < len(points)-1; i++ {
		segStart := points[i]
		segEnd := points[i+1]
		if segEnd <= segStart {
			continue
		}

		// Find the shortest event that fully covers this segment
		var bestEvent *StyleEventRef
		bestLength := int(^uint(0) >> 1) // max int

		for j := range deduped {
			ev := &deduped[j]
			evEnd := ev.Offset + ev.Length
			// Event covers segment if ev.Offset <= segStart && evEnd >= segEnd
			if ev.Offset <= segStart && evEnd >= segEnd {
				if ev.Length < bestLength {
					bestLength = ev.Length
					bestEvent = ev
				} else if ev.Length == bestLength {
					// Tie-breaker: prefer event with LinkTo or longer style name
					if ev.LinkTo != "" && bestEvent.LinkTo == "" {
						bestEvent = ev
					} else if len(ev.Style) > len(bestEvent.Style) {
						bestEvent = ev
					}
				}
			}
		}

		if bestEvent != nil {
			// Create or extend a segment with this style
			seg := StyleEventRef{
				Offset:         segStart,
				Length:         segEnd - segStart,
				Style:          bestEvent.Style,
				LinkTo:         bestEvent.LinkTo,
				IsFootnoteLink: bestEvent.IsFootnoteLink,
			}

			// Try to merge with previous segment if same style and adjacent
			if len(result) > 0 {
				prev := &result[len(result)-1]
				if prev.Style == seg.Style &&
					prev.LinkTo == seg.LinkTo &&
					prev.IsFootnoteLink == seg.IsFootnoteLink &&
					prev.Offset+prev.Length == seg.Offset {
					// Extend previous segment
					prev.Length += seg.Length
					continue
				}
			}
			result = append(result, seg)
		}
	}

	return result
}
