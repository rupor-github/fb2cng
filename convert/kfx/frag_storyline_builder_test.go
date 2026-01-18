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
	// Expected result: code [0-5], link [5-9], code [9-14]
	events := []StyleEventRef{
		{Offset: 5, Length: 4, Style: "link", LinkTo: "target"},
		{Offset: 0, Length: 14, Style: "code"},
	}
	result := SegmentNestedStyleEvents(events)

	if len(result) != 3 {
		t.Fatalf("expected 3 events, got %d: %+v", len(result), result)
	}

	// First segment: code [0-5]
	if result[0].Offset != 0 || result[0].Length != 5 || result[0].Style != "code" {
		t.Errorf("event 0: expected code[0-5], got %+v", result[0])
	}

	// Second segment: link [5-9]
	if result[1].Offset != 5 || result[1].Length != 4 || result[1].Style != "link" {
		t.Errorf("event 1: expected link[5-9], got %+v", result[1])
	}
	if result[1].LinkTo != "target" {
		t.Errorf("event 1: expected LinkTo='target', got %q", result[1].LinkTo)
	}

	// Third segment: code [9-14]
	if result[2].Offset != 9 || result[2].Length != 5 || result[2].Style != "code" {
		t.Errorf("event 2: expected code[9-14], got %+v", result[2])
	}
}

func TestSegmentNestedStyleEvents_MultipleNested(t *testing.T) {
	// Simulates: <code>a <strong>b</strong> c <em>d</em> e</code>
	// code [0-16], strong [2-3], em [8-9]
	events := []StyleEventRef{
		{Offset: 2, Length: 1, Style: "strong"},
		{Offset: 8, Length: 1, Style: "emphasis"},
		{Offset: 0, Length: 16, Style: "code"},
	}
	result := SegmentNestedStyleEvents(events)

	// Expected: code[0-2], strong[2-3], code[3-8], emphasis[8-9], code[9-16]
	if len(result) != 5 {
		t.Fatalf("expected 5 events, got %d: %+v", len(result), result)
	}

	expected := []struct {
		offset, length int
		style          string
	}{
		{0, 2, "code"},
		{2, 1, "strong"},
		{3, 5, "code"},
		{8, 1, "emphasis"},
		{9, 7, "code"},
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
	// The shorter (inner) event should be preserved, outer segmented
	events := []StyleEventRef{
		{Offset: 0, Length: 4, Style: "link", LinkTo: "target"},
		{Offset: 0, Length: 10, Style: "code"},
	}
	result := SegmentNestedStyleEvents(events)

	// Expected: link[0-4], code[4-10]
	if len(result) != 2 {
		t.Fatalf("expected 2 events, got %d: %+v", len(result), result)
	}

	if result[0].Offset != 0 || result[0].Length != 4 || result[0].Style != "link" {
		t.Errorf("event 0: expected link[0-4], got %+v", result[0])
	}
	if result[1].Offset != 4 || result[1].Length != 6 || result[1].Style != "code" {
		t.Errorf("event 1: expected code[4-10], got %+v", result[1])
	}
}

func TestSegmentStyleEvents_Basic(t *testing.T) {
	// Test the original SegmentStyleEvents function
	events := []StyleEventRef{
		{Offset: 5, Length: 3, Style: "link"},
	}
	result := SegmentStyleEvents(events, "code", 15)

	if len(result) != 3 {
		t.Fatalf("expected 3 events, got %d: %+v", len(result), result)
	}

	// code[0-5], link[5-8], code[8-15]
	if result[0].Offset != 0 || result[0].Length != 5 || result[0].Style != "code" {
		t.Errorf("event 0: expected code[0-5], got %+v", result[0])
	}
	if result[1].Offset != 5 || result[1].Length != 3 || result[1].Style != "link" {
		t.Errorf("event 1: expected link[5-8], got %+v", result[1])
	}
	if result[2].Offset != 8 || result[2].Length != 7 || result[2].Style != "code" {
		t.Errorf("event 2: expected code[8-15], got %+v", result[2])
	}
}
