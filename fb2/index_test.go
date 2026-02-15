package fb2

import (
	"testing"

	"github.com/beevik/etree"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
)

func TestParseBookXMLBodyKinds(t *testing.T) {
	log := zaptest.NewLogger(t, zaptest.WrapOptions(zap.AddCaller(), zap.AddCallerSkip(1)))
	t.Run("classifies main and footnotes and other", func(t *testing.T) {
		xml := `<FictionBook>
			<body><section id="s1"/></body>
			<body name="notes"><section id="n1"/></body>
			<body name="appendix"><section id="a1"/></body>
		</FictionBook>`
		doc := etree.NewDocument()
		if err := doc.ReadFromString(xml); err != nil {
			t.Fatalf("read xml: %v", err)
		}
		book, err := ParseBookXML(doc, []string{"notes", "footnotes"}, log)
		if err != nil {
			t.Fatalf("ParseBookXML: %v", err)
		}
		if len(book.Bodies) != 3 {
			t.Fatalf("expected 3 bodies, got %d", len(book.Bodies))
		}
		if book.Bodies[0].Kind != BodyMain {
			t.Fatalf("first body should be main, got %v", book.Bodies[0].Kind)
		}
		if book.Bodies[1].Kind != BodyFootnotes {
			t.Fatalf("second body should be footnotes, got %v", book.Bodies[1].Kind)
		}
		if book.Bodies[2].Kind != BodyOther {
			t.Fatalf("third body should be other, got %v", book.Bodies[2].Kind)
		}
	})

	t.Run("footnotes recognized via footnotes name", func(t *testing.T) {
		xml := `<FictionBook>
			<body name="main"><section id="s1"/></body>
			<body name="footnotes"><section id="f1"/></body>
		</FictionBook>`
		doc := etree.NewDocument()
		if err := doc.ReadFromString(xml); err != nil {
			t.Fatalf("read xml: %v", err)
		}
		book, err := ParseBookXML(doc, []string{"footnotes"}, log)
		if err != nil {
			t.Fatalf("ParseBookXML: %v", err)
		}
		if len(book.Bodies) != 2 {
			t.Fatalf("expected 2 bodies, got %d", len(book.Bodies))
		}
		if book.Bodies[0].Kind != BodyMain {
			t.Fatalf("first body should be main")
		}
		if book.Bodies[1].Kind != BodyFootnotes {
			t.Fatalf("second body should be footnotes")
		}
	})

	t.Run("custom list maps appendix to footnotes", func(t *testing.T) {
		xml := `<FictionBook>
			<body name="main"><section id="m"/></body>
			<body name="appendix"><section id="app"/></body>
		</FictionBook>`
		doc := etree.NewDocument()
		if err := doc.ReadFromString(xml); err != nil {
			t.Fatalf("read xml: %v", err)
		}
		book, err := ParseBookXML(doc, []string{"appendix"}, log)
		if err != nil {
			t.Fatalf("ParseBookXML: %v", err)
		}
		if len(book.Bodies) != 2 {
			t.Fatalf("expected 2 bodies, got %d", len(book.Bodies))
		}
		if book.Bodies[0].Kind != BodyMain {
			t.Fatalf("first body should be main")
		}
		if book.Bodies[1].Kind != BodyFootnotes {
			t.Fatalf("appendix should be classified as footnotes via custom list")
		}
	})
}

func TestBuildFootnotesIndex(t *testing.T) {
	log := zaptest.NewLogger(t, zaptest.WrapOptions(zap.AddCaller(), zap.AddCallerSkip(1)))

	t.Run("basic_index_building", func(t *testing.T) {
		// Build a book with one main body and one footnotes body
		book := &FictionBook{
			Bodies: []Body{
				{Kind: BodyMain, Sections: []Section{{ID: "main1"}}},
				{Kind: BodyFootnotes, Sections: []Section{{ID: "fn1"}, {ID: "fn2"}}},
			},
		}

		// Normalize first (mirrors convert.parse flow) and get index
		_, index := book.NormalizeFootnoteBodies(log)

		if len(index) != 2 {
			// Should include fn1 and fn2 only
			t.Fatalf("expected 2 footnotes in index, got %d", len(index))
		}
		ref1, ok1 := index["fn1"]
		ref2, ok2 := index["fn2"]
		if !ok1 || !ok2 {
			t.Fatalf("expected fn1 and fn2 keys present, got: %v %v", ok1, ok2)
		}
		if ref1.BodyIdx != 1 || ref2.BodyIdx != 1 {
			t.Fatalf("expected references to point to footnotes body index 1, got %d and %d", ref1.BodyIdx, ref2.BodyIdx)
		}
		if ref1.SectionIdx != 0 || ref2.SectionIdx != 1 {
			t.Fatalf("section indices mismatch: %+v %+v", ref1, ref2)
		}
	})

	t.Run("skips_empty_and_duplicate_ids", func(t *testing.T) {
		// Build a footnotes body containing an empty ID and duplicate IDs.
		// We intentionally DO NOT normalize to ensure BuildFootnotesIndex internal
		// skipping logic works even if normalization was skipped.
		book := &FictionBook{
			Bodies: []Body{
				{Kind: BodyFootnotes, Sections: []Section{{ID: "fnA"}, {ID: ""}, {ID: "fnA"}, {ID: "fnB"}}},
			},
		}

		index := book.buildFootnotesIndex(log)
		if len(index) != 2 {
			t.Fatalf("expected 2 entries (fnA, fnB) skipping empty and duplicate, got %d", len(index))
		}
		refA, okA := index["fnA"]
		refB, okB := index["fnB"]
		if !okA || !okB {
			t.Fatalf("expected fnA and fnB keys present")
		}
		// fnA should refer to first instance (section index 0), duplicate (index 2) skipped
		if refA.SectionIdx != 0 {
			t.Fatalf("expected fnA SectionIdx=0, got %d", refA.SectionIdx)
		}
		// fnB is at original section index 3
		if refB.SectionIdx != 3 {
			t.Fatalf("expected fnB SectionIdx=3, got %d", refB.SectionIdx)
		}
	})

	t.Run("ignores_non_footnote_bodies", func(t *testing.T) {
		book := &FictionBook{Bodies: []Body{
			{Kind: BodyMain, Sections: []Section{{ID: "m1"}}},
			{Kind: BodyOther, Sections: []Section{{ID: "x1"}}},
		}}
		index := book.buildFootnotesIndex(log)
		if len(index) != 0 {
			t.Fatalf("expected empty index when no footnotes bodies present, got %d", len(index))
		}
	})
}

func TestHasExternalScheme(t *testing.T) {
	tests := []struct {
		href string
		want bool
	}{
		{"http://example.com", true},
		{"https://example.com/path", true},
		{"HTTP://EXAMPLE.COM", true},
		{"ftp://files.example.com/book.fb2", true},
		{"ftps://secure.example.com/book.fb2", true},
		{"mailto:user@example.com", true},
		{"MAILTO:user@example.com", true},
		// No scheme
		{"example.com", false},
		{"just some text", false},
		{"/path/to/file", false},
		{"relative/path", false},
		// Unsupported schemes
		{"file:///etc/passwd", false},
		{"javascript:alert(1)", false},
		{"data:text/html,<h1>hi</h1>", false},
		// Scheme-like but too short (no content after scheme)
		{"http://", false},
		{"https://", false},
		{"mailto:", false},
	}

	for _, tt := range tests {
		t.Run(tt.href, func(t *testing.T) {
			if got := hasExternalScheme(tt.href); got != tt.want {
				t.Errorf("hasExternalScheme(%q) = %v, want %v", tt.href, got, tt.want)
			}
		})
	}
}

func TestIndexHref(t *testing.T) {
	log := zap.NewNop()
	path := []any{"test"}

	t.Run("internal link", func(t *testing.T) {
		index := make(ReverseLinkIndex)
		indexHref(index, "#footnote1", "inline-link", path, log)

		refs, ok := index["footnote1"]
		if !ok || len(refs) != 1 {
			t.Fatalf("expected 1 ref under 'footnote1', got %v", index)
		}
		if refs[0].Type != "inline-link" {
			t.Errorf("expected type 'inline-link', got %q", refs[0].Type)
		}
	})

	t.Run("empty href", func(t *testing.T) {
		index := make(ReverseLinkIndex)
		indexHref(index, "", "inline-link", path, log)

		refs, ok := index["links/empty_href"]
		if !ok || len(refs) != 1 {
			t.Fatalf("expected 1 ref under 'links/empty_href', got %v", index)
		}
		if refs[0].Type != "empty-href-link" {
			t.Errorf("expected type 'empty-href-link', got %q", refs[0].Type)
		}
	})

	t.Run("valid external link", func(t *testing.T) {
		index := make(ReverseLinkIndex)
		indexHref(index, "https://example.com", "inline-link", path, log)

		refs, ok := index["https://example.com"]
		if !ok || len(refs) != 1 {
			t.Fatalf("expected 1 ref under URL key, got %v", index)
		}
		if refs[0].Type != "external-link" {
			t.Errorf("expected type 'external-link', got %q", refs[0].Type)
		}
	})

	t.Run("no scheme is broken link", func(t *testing.T) {
		index := make(ReverseLinkIndex)
		indexHref(index, "not-a-url", "inline-link", path, log)

		refs, ok := index["not-a-url"]
		if !ok || len(refs) != 1 {
			t.Fatalf("expected 1 ref under 'not-a-url', got %v", index)
		}
		if refs[0].Type != "broken-link" {
			t.Errorf("expected type 'broken-link', got %q", refs[0].Type)
		}
	})

	t.Run("javascript scheme is broken link", func(t *testing.T) {
		index := make(ReverseLinkIndex)
		indexHref(index, "javascript:alert(1)", "inline-link", path, log)

		refs, ok := index["javascript:alert(1)"]
		if !ok || len(refs) != 1 {
			t.Fatalf("expected 1 ref, got %v", index)
		}
		if refs[0].Type != "broken-link" {
			t.Errorf("expected type 'broken-link', got %q", refs[0].Type)
		}
	})

	t.Run("mailto is external link", func(t *testing.T) {
		index := make(ReverseLinkIndex)
		indexHref(index, "mailto:user@example.com", "inline-link", path, log)

		refs, ok := index["mailto:user@example.com"]
		if !ok || len(refs) != 1 {
			t.Fatalf("expected 1 ref, got %v", index)
		}
		if refs[0].Type != "external-link" {
			t.Errorf("expected type 'external-link', got %q", refs[0].Type)
		}
	})

	t.Run("broken link indexed under href not empty string", func(t *testing.T) {
		// Regression: old code used targetID (always "" in else branch)
		// instead of href as the index key.
		index := make(ReverseLinkIndex)
		indexHref(index, "some-random-text", "inline-link", path, log)

		if _, ok := index[""]; ok {
			t.Error("broken link should NOT be indexed under empty string")
		}
		refs, ok := index["some-random-text"]
		if !ok || len(refs) != 1 {
			t.Fatalf("expected 1 ref under 'some-random-text', got %v", index)
		}
		if refs[0].Type != "broken-link" {
			t.Errorf("expected type 'broken-link', got %q", refs[0].Type)
		}
	})
}
