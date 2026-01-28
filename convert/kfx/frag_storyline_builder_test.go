package kfx

import (
	"testing"
)

func TestSegmentNestedStyleEvents_NoEvents(t *testing.T) {
	result := SegmentNestedStyleEvents(nil)
	if len(result) != 0 {
		t.Errorf("expected empty result, got %v", result)
	}
}

func TestSegmentNestedStyleEvents_SingleEvent(t *testing.T) {
	events := []StyleEventRef{
		{Offset: 0, Length: 10, Style: "code"},
	}
	result := SegmentNestedStyleEvents(events)
	if len(result) != 1 {
		t.Errorf("expected 1 event, got %d", len(result))
	}
	if result[0].Style != "code" {
		t.Errorf("expected style 'code', got %q", result[0].Style)
	}
}

func TestSegmentNestedStyleEvents_NonOverlapping(t *testing.T) {
	events := []StyleEventRef{
		{Offset: 0, Length: 5, Style: "code"},
		{Offset: 10, Length: 5, Style: "emphasis"},
	}
	result := SegmentNestedStyleEvents(events)
	if len(result) != 2 {
		t.Errorf("expected 2 events, got %d", len(result))
	}
}

func TestSegmentNestedStyleEvents_NestedLinkInCode(t *testing.T) {
	// Simulates: <code>text <a>link</a> more</code>
	// code covers [0-14], link covers [5-9]
	// With overlapping approach (like KP3): both events preserved
	// Output sorted by offset, then length desc: code[0-14], link[5-9]
	events := []StyleEventRef{
		{Offset: 5, Length: 4, Style: "link", LinkTo: "target"},
		{Offset: 0, Length: 14, Style: "code"},
	}
	result := SegmentNestedStyleEvents(events)

	if len(result) != 2 {
		t.Fatalf("expected 2 events (overlapping preserved), got %d: %+v", len(result), result)
	}

	// First event: code [0-14] (outer, longer, comes first)
	if result[0].Offset != 0 || result[0].Length != 14 || result[0].Style != "code" {
		t.Errorf("event 0: expected code[0-14], got %+v", result[0])
	}

	// Second event: link [5-9] (inner, shorter, comes second)
	if result[1].Offset != 5 || result[1].Length != 4 || result[1].Style != "link" {
		t.Errorf("event 1: expected link[5-9], got %+v", result[1])
	}
	if result[1].LinkTo != "target" {
		t.Errorf("event 1: expected LinkTo='target', got %q", result[1].LinkTo)
	}
}

func TestSegmentNestedStyleEvents_MultipleNested(t *testing.T) {
	// Simulates: <code>a <strong>b</strong> c <em>d</em> e</code>
	// code [0-16], strong [2-3], em [8-9]
	// With overlapping approach: all events preserved, sorted by offset then length desc
	events := []StyleEventRef{
		{Offset: 2, Length: 1, Style: "strong"},
		{Offset: 8, Length: 1, Style: "emphasis"},
		{Offset: 0, Length: 16, Style: "code"},
	}
	result := SegmentNestedStyleEvents(events)

	// Expected: code[0-16], strong[2-3], emphasis[8-9]
	if len(result) != 3 {
		t.Fatalf("expected 3 events, got %d: %+v", len(result), result)
	}

	expected := []struct {
		offset, length int
		style          string
	}{
		{0, 16, "code"},
		{2, 1, "strong"},
		{8, 1, "emphasis"},
	}

	for i, exp := range expected {
		if result[i].Offset != exp.offset || result[i].Length != exp.length || result[i].Style != exp.style {
			t.Errorf("event %d: expected %s[%d-%d], got %s[%d-%d]",
				i, exp.style, exp.offset, exp.offset+exp.length,
				result[i].Style, result[i].Offset, result[i].Offset+result[i].Length)
		}
	}
}

func TestSegmentNestedStyleEvents_SameOffset(t *testing.T) {
	// Edge case: events starting at same offset with different lengths
	// This happens with <code><a>link</a></code> where code and link start at same position
	// With overlapping approach: both preserved, sorted by length desc (outer first)
	events := []StyleEventRef{
		{Offset: 0, Length: 4, Style: "link", LinkTo: "target"},
		{Offset: 0, Length: 10, Style: "code"},
	}
	result := SegmentNestedStyleEvents(events)

	// Expected: code[0-10] (longer, outer), link[0-4] (shorter, inner)
	if len(result) != 2 {
		t.Fatalf("expected 2 events, got %d: %+v", len(result), result)
	}

	if result[0].Offset != 0 || result[0].Length != 10 || result[0].Style != "code" {
		t.Errorf("event 0: expected code[0-10], got %+v", result[0])
	}
	if result[1].Offset != 0 || result[1].Length != 4 || result[1].Style != "link" {
		t.Errorf("event 1: expected link[0-4], got %+v", result[1])
	}
}

func TestSegmentStyleEvents_Basic(t *testing.T) {
	// Test with a base style and inline event - both preserved as overlapping
	events := []StyleEventRef{
		{Offset: 0, Length: 15, Style: "code"}, // base style
		{Offset: 5, Length: 3, Style: "link"},  // inline event
	}
	result := SegmentNestedStyleEvents(events)

	if len(result) != 2 {
		t.Fatalf("expected 2 events (overlapping preserved), got %d: %+v", len(result), result)
	}

	// code[0-15], link[5-8]
	if result[0].Offset != 0 || result[0].Length != 15 || result[0].Style != "code" {
		t.Errorf("event 0: expected code[0-15], got %+v", result[0])
	}
	if result[1].Offset != 5 || result[1].Length != 3 || result[1].Style != "link" {
		t.Errorf("event 1: expected link[5-8], got %+v", result[1])
	}
}

func TestSegmentNestedStyleEvents_Deduplication(t *testing.T) {
	// Test deduplication: same offset+length, keep one with LinkTo or longer style
	events := []StyleEventRef{
		{Offset: 0, Length: 5, Style: "short"},
		{Offset: 0, Length: 5, Style: "longer-style"},
	}
	result := SegmentNestedStyleEvents(events)

	if len(result) != 1 {
		t.Fatalf("expected 1 event (deduplicated), got %d: %+v", len(result), result)
	}

	// Should keep the one with longer style name
	if result[0].Style != "longer-style" {
		t.Errorf("expected style 'longer-style' (longer name), got %q", result[0].Style)
	}
}

func TestSegmentNestedStyleEvents_DeduplicationPreferLinkTo(t *testing.T) {
	// Test deduplication: same offset+length, prefer event with LinkTo
	events := []StyleEventRef{
		{Offset: 0, Length: 5, Style: "longer-style-name"},
		{Offset: 0, Length: 5, Style: "link", LinkTo: "target"},
	}
	result := SegmentNestedStyleEvents(events)

	if len(result) != 1 {
		t.Fatalf("expected 1 event (deduplicated), got %d: %+v", len(result), result)
	}

	// Should keep the one with LinkTo
	if result[0].LinkTo != "target" {
		t.Errorf("expected LinkTo='target' (preferred), got %q", result[0].LinkTo)
	}
}

func TestSegmentNestedStyleEvents_SupAndLink(t *testing.T) {
	// Simulates KP3 behavior for <sup><a>text</a></sup>
	// sup covers [62-68] (space + text + space), link covers [63-67] (just text)
	// Both events should be preserved as overlapping
	events := []StyleEventRef{
		{Offset: 62, Length: 6, Style: "sup-style"},
		{Offset: 63, Length: 4, Style: "link-style", LinkTo: "note_1", IsFootnoteLink: true},
	}
	result := SegmentNestedStyleEvents(events)

	if len(result) != 2 {
		t.Fatalf("expected 2 events (overlapping like KP3), got %d: %+v", len(result), result)
	}

	// First: sup[62-68] (starts earlier)
	if result[0].Offset != 62 || result[0].Length != 6 || result[0].Style != "sup-style" {
		t.Errorf("event 0: expected sup-style[62-68], got %+v", result[0])
	}

	// Second: link[63-67] (starts later, inside sup)
	if result[1].Offset != 63 || result[1].Length != 4 || result[1].Style != "link-style" {
		t.Errorf("event 1: expected link-style[63-67], got %+v", result[1])
	}
	if result[1].LinkTo != "note_1" {
		t.Errorf("event 1: expected LinkTo='note_1', got %q", result[1].LinkTo)
	}
	if !result[1].IsFootnoteLink {
		t.Error("event 1: expected IsFootnoteLink=true")
	}
}
