package kfx

import (
	"testing"

	"go.uber.org/zap"

	"fbc/fb2"
)

func TestSegmentStyle_BasicTypes(t *testing.T) {
	tests := []struct {
		name         string
		segment      *fb2.InlineSegment
		wantStyle    string
		wantIsLink   bool
		wantLinkTo   string
		wantFootnote bool
	}{
		{
			name:      "strong",
			segment:   &fb2.InlineSegment{Kind: fb2.InlineStrong},
			wantStyle: "strong",
		},
		{
			name:      "emphasis",
			segment:   &fb2.InlineSegment{Kind: fb2.InlineEmphasis},
			wantStyle: "emphasis",
		},
		{
			name:      "strikethrough",
			segment:   &fb2.InlineSegment{Kind: fb2.InlineStrikethrough},
			wantStyle: "strikethrough",
		},
		{
			name:      "sub",
			segment:   &fb2.InlineSegment{Kind: fb2.InlineSub},
			wantStyle: "sub",
		},
		{
			name:      "sup",
			segment:   &fb2.InlineSegment{Kind: fb2.InlineSup},
			wantStyle: "sup",
		},
		{
			name:      "code",
			segment:   &fb2.InlineSegment{Kind: fb2.InlineCode},
			wantStyle: "code",
		},
		{
			name:      "named style",
			segment:   &fb2.InlineSegment{Kind: fb2.InlineNamedStyle, Style: "custom-class"},
			wantStyle: "custom-class",
		},
		{
			name:       "internal link without content",
			segment:    &fb2.InlineSegment{Kind: fb2.InlineLink, Href: "#section1"},
			wantStyle:  "link-internal",
			wantIsLink: true,
			wantLinkTo: "section1",
		},
		{
			name:       "external link",
			segment:    &fb2.InlineSegment{Kind: fb2.InlineLink, Href: "https://example.com"},
			wantStyle:  "link-external",
			wantIsLink: true,
			wantLinkTo: "", // external links don't set linkTo without registry
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			segStyle, isLink, linkTo, isFootnote, _ := SegmentStyle(tt.segment, nil, nil)

			if segStyle != tt.wantStyle {
				t.Errorf("segStyle = %q, want %q", segStyle, tt.wantStyle)
			}
			if isLink != tt.wantIsLink {
				t.Errorf("isLink = %v, want %v", isLink, tt.wantIsLink)
			}
			if linkTo != tt.wantLinkTo {
				t.Errorf("linkTo = %q, want %q", linkTo, tt.wantLinkTo)
			}
			if isFootnote != tt.wantFootnote {
				t.Errorf("isFootnote = %v, want %v", isFootnote, tt.wantFootnote)
			}
		})
	}
}

func TestSegmentStyle_FootnoteLink(t *testing.T) {
	// Footnote link detection requires a content object with FootnotesIndex.
	// Since content.Content is complex, we test this indirectly through other paths.
	// The key logic is: if c.FootnotesIndex[linkTo] exists, segStyle becomes "link-footnote".
	// This test verifies the internal link path (without Content).
	seg := &fb2.InlineSegment{Kind: fb2.InlineLink, Href: "#note1"}

	segStyle, isLink, linkTo, isFootnote, _ := SegmentStyle(seg, nil, nil)

	// Without Content, internal links default to "link-internal"
	if segStyle != "link-internal" {
		t.Errorf("segStyle = %q, want %q", segStyle, "link-internal")
	}
	if !isLink {
		t.Error("expected isLink = true")
	}
	if linkTo != "note1" {
		t.Errorf("linkTo = %q, want %q", linkTo, "note1")
	}
	if isFootnote {
		t.Error("expected isFootnote = false (no Content provided)")
	}
}

func TestSegmentStyle_ExternalLinkWithRegistry(t *testing.T) {
	log := zap.NewNop()
	registry, _ := parseAndCreateRegistry(nil, nil, log)

	seg := &fb2.InlineSegment{Kind: fb2.InlineLink, Href: "https://example.com/page"}

	segStyle, isLink, linkTo, _, _ := SegmentStyle(seg, nil, registry)

	if segStyle != "link-external" {
		t.Errorf("segStyle = %q, want %q", segStyle, "link-external")
	}
	if !isLink {
		t.Error("expected isLink = true")
	}
	// With registry, external links get an anchor ID
	if linkTo == "" {
		t.Error("expected linkTo to be set for external link with registry")
	}
}

func TestInjectPseudoBefore(t *testing.T) {
	log := zap.NewNop()

	// Create registry with pseudo-element content
	css := []byte(`
		.link-footnote::before { content: "["; }
		.link-footnote::after { content: "]"; }
	`)
	registry, _ := parseAndCreateRegistry(css, nil, log)

	tests := []struct {
		name     string
		segStyle string
		styles   *StyleRegistry
		wantText bool
	}{
		{
			name:     "empty style",
			segStyle: "",
			styles:   registry,
			wantText: false,
		},
		{
			name:     "nil registry",
			segStyle: "link-footnote",
			styles:   nil,
			wantText: false,
		},
		{
			name:     "style with ::before",
			segStyle: "link-footnote",
			styles:   registry,
			wantText: true,
		},
		{
			name:     "style without pseudo content",
			segStyle: "emphasis",
			styles:   registry,
			wantText: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nw := newNormalizingWriter()
			result := InjectPseudoBefore(tt.segStyle, tt.styles, nw)

			if result != tt.wantText {
				t.Errorf("InjectPseudoBefore() = %v, want %v", result, tt.wantText)
			}

			if tt.wantText && nw.String() != "[" {
				t.Errorf("expected nw to contain '[', got %q", nw.String())
			}
		})
	}
}

func TestInjectPseudoAfter(t *testing.T) {
	log := zap.NewNop()

	// Create registry with pseudo-element content
	css := []byte(`
		.link-footnote::before { content: "["; }
		.link-footnote::after { content: "]"; }
	`)
	registry, _ := parseAndCreateRegistry(css, nil, log)

	tests := []struct {
		name     string
		segStyle string
		styles   *StyleRegistry
		wantText bool
	}{
		{
			name:     "empty style",
			segStyle: "",
			styles:   registry,
			wantText: false,
		},
		{
			name:     "nil registry",
			segStyle: "link-footnote",
			styles:   nil,
			wantText: false,
		},
		{
			name:     "style with ::after",
			segStyle: "link-footnote",
			styles:   registry,
			wantText: true,
		},
		{
			name:     "style without pseudo content",
			segStyle: "emphasis",
			styles:   registry,
			wantText: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nw := newNormalizingWriter()
			result := InjectPseudoAfter(tt.segStyle, tt.styles, nw)

			if result != tt.wantText {
				t.Errorf("InjectPseudoAfter() = %v, want %v", result, tt.wantText)
			}

			if tt.wantText && nw.String() != "]" {
				t.Errorf("expected nw to contain ']', got %q", nw.String())
			}
		})
	}
}

func TestGetPseudoStartText(t *testing.T) {
	log := zap.NewNop()

	// Create registry with pseudo-element content
	css := []byte(`
		.link-footnote::before { content: "["; }
	`)
	registry, _ := parseAndCreateRegistry(css, nil, log)

	tests := []struct {
		name      string
		seg       *fb2.InlineSegment
		segStyle  string
		styles    *StyleRegistry
		wantStart string
	}{
		{
			name:      "plain text segment",
			seg:       &fb2.InlineSegment{Text: "Hello"},
			segStyle:  "",
			styles:    nil,
			wantStart: "Hello",
		},
		{
			name:      "segment with ::before",
			seg:       &fb2.InlineSegment{Text: "1"},
			segStyle:  "link-footnote",
			styles:    registry,
			wantStart: "[", // ::before content comes first
		},
		{
			name: "segment with children",
			seg: &fb2.InlineSegment{
				Children: []fb2.InlineSegment{
					{Text: "Child text"},
				},
			},
			segStyle:  "",
			styles:    nil,
			wantStart: "Child text",
		},
		{
			name: "segment with children and ::before",
			seg: &fb2.InlineSegment{
				Children: []fb2.InlineSegment{
					{Text: "1"},
				},
			},
			segStyle:  "link-footnote",
			styles:    registry,
			wantStart: "[", // ::before overrides child text
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetPseudoStartText(tt.seg, tt.segStyle, tt.styles)

			if result != tt.wantStart {
				t.Errorf("GetPseudoStartText() = %q, want %q", result, tt.wantStart)
			}
		})
	}
}

func TestHasPseudoBefore(t *testing.T) {
	log := zap.NewNop()

	css := []byte(`
		.with-before::before { content: ">>>"; }
		.with-after::after { content: "<<<"; }
	`)
	registry, _ := parseAndCreateRegistry(css, nil, log)

	tests := []struct {
		name     string
		segStyle string
		styles   *StyleRegistry
		want     bool
	}{
		{"empty style", "", registry, false},
		{"nil registry", "with-before", nil, false},
		{"has ::before", "with-before", registry, true},
		{"only has ::after", "with-after", registry, false},
		{"no pseudo", "emphasis", registry, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := HasPseudoBefore(tt.segStyle, tt.styles)
			if result != tt.want {
				t.Errorf("HasPseudoBefore() = %v, want %v", result, tt.want)
			}
		})
	}
}

func TestHasPseudoAfter(t *testing.T) {
	log := zap.NewNop()

	css := []byte(`
		.with-before::before { content: ">>>"; }
		.with-after::after { content: "<<<"; }
	`)
	registry, _ := parseAndCreateRegistry(css, nil, log)

	tests := []struct {
		name     string
		segStyle string
		styles   *StyleRegistry
		want     bool
	}{
		{"empty style", "", registry, false},
		{"nil registry", "with-after", nil, false},
		{"has ::after", "with-after", registry, true},
		{"only has ::before", "with-before", registry, false},
		{"no pseudo", "emphasis", registry, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := HasPseudoAfter(tt.segStyle, tt.styles)
			if result != tt.want {
				t.Errorf("HasPseudoAfter() = %v, want %v", result, tt.want)
			}
		})
	}
}

func TestPseudoContentIntegration(t *testing.T) {
	// Test the full flow: CSS -> registry -> inject content
	log := zap.NewNop()

	css := []byte(`
		.link-footnote {
			font-size: 0.8em;
			vertical-align: super;
		}
		.link-footnote::before {
			content: "[";
		}
		.link-footnote::after {
			content: "]";
		}
	`)

	registry, warnings := parseAndCreateRegistry(css, nil, log)

	// Should have no warnings (pseudo content is valid)
	if len(warnings) > 0 {
		t.Logf("Warnings: %v", warnings)
	}

	// Verify pseudo content was extracted
	if !registry.HasPseudoContent() {
		t.Fatal("expected registry to have pseudo content")
	}

	pc := registry.GetPseudoContentForClass("link-footnote")
	if pc == nil {
		t.Fatal("expected pseudo content for link-footnote")
	}

	if pc.Before != "[" {
		t.Errorf("Before = %q, want %q", pc.Before, "[")
	}
	if pc.After != "]" {
		t.Errorf("After = %q, want %q", pc.After, "]")
	}

	// Test injection
	nw := newNormalizingWriter()
	InjectPseudoBefore("link-footnote", registry, nw)
	nw.WriteString("42")
	InjectPseudoAfter("link-footnote", registry, nw)

	expected := "[42]"
	if got := nw.String(); got != expected {
		t.Errorf("got %q, want %q", got, expected)
	}
}
