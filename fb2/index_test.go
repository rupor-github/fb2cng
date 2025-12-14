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
