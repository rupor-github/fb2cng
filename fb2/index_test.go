package fb2

import (
	"os"
	"path/filepath"
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

func TestFictionBookClone(t *testing.T) {
	log := zaptest.NewLogger(t, zaptest.WrapOptions(zap.AddCaller(), zap.AddCallerSkip(1)))

	// Create a book with various fields populated
	original := &FictionBook{
		Description: Description{
			TitleInfo: TitleInfo{
				BookTitle: TextField{Value: "Original Title"},
				Authors: []Author{
					{FirstName: "John", LastName: "Doe"},
				},
			},
			DocumentInfo: DocumentInfo{
				ID:      "test-id-123",
				Version: "1.0",
			},
		},
		Bodies: []Body{
			{
				Name: "main",
				Kind: BodyMain,
				Sections: []Section{
					{
						ID: "section1",
						Content: []FlowItem{
							{
								Kind: FlowParagraph,
								Paragraph: &Paragraph{
									Text: []InlineSegment{
										{Kind: InlineText, Text: "Original text"},
									},
								},
							},
						},
					},
				},
			},
			{
				Name: "notes",
				Kind: BodyFootnotes,
				Sections: []Section{
					{
						ID: "note1",
						Content: []FlowItem{
							{
								Kind: FlowParagraph,
								Paragraph: &Paragraph{
									Text: []InlineSegment{
										{Kind: InlineText, Text: "Note text"},
									},
								},
							},
						},
					},
				},
			},
		},
		Binaries: []BinaryObject{
			{ID: "img1", ContentType: "image/png", Data: []byte{1, 2, 3}},
		},
	}

	// Clone the book
	cloned := original.clone()

	// Verify the clone has the same data
	if cloned.Description.TitleInfo.BookTitle.Value != "Original Title" {
		t.Errorf("clone has wrong book title: %s", cloned.Description.TitleInfo.BookTitle.Value)
	}
	if cloned.Description.DocumentInfo.ID != "test-id-123" {
		t.Errorf("clone has wrong document ID: %s", cloned.Description.DocumentInfo.ID)
	}
	if len(cloned.Bodies) != 2 {
		t.Fatalf("clone has wrong number of bodies: %d", len(cloned.Bodies))
	}
	if len(cloned.Binaries) != 1 {
		t.Fatalf("clone has wrong number of binaries: %d", len(cloned.Binaries))
	}

	// Now modify the clone and verify the original is unchanged
	cloned.Description.TitleInfo.BookTitle.Value = "Modified Title"
	cloned.Description.DocumentInfo.ID = "modified-id"
	cloned.Bodies[0].Name = "modified-main"
	cloned.Bodies[0].Sections[0].ID = "modified-section1"
	cloned.Bodies[0].Sections[0].Content[0].Paragraph.Text[0].Text = "Modified text"
	cloned.Bodies[1].Sections[0].ID = "modified-note1"
	cloned.Binaries[0].ID = "modified-img1"
	cloned.Binaries[0].Data[0] = 99

	// Verify original is unchanged
	if original.Description.TitleInfo.BookTitle.Value != "Original Title" {
		t.Errorf("original book title was modified: %s", original.Description.TitleInfo.BookTitle.Value)
	}
	if original.Description.DocumentInfo.ID != "test-id-123" {
		t.Errorf("original document ID was modified: %s", original.Description.DocumentInfo.ID)
	}
	if original.Bodies[0].Name != "main" {
		t.Errorf("original body name was modified: %s", original.Bodies[0].Name)
	}
	if original.Bodies[0].Sections[0].ID != "section1" {
		t.Errorf("original section ID was modified: %s", original.Bodies[0].Sections[0].ID)
	}
	if original.Bodies[0].Sections[0].Content[0].Paragraph.Text[0].Text != "Original text" {
		t.Errorf("original paragraph text was modified: %s", original.Bodies[0].Sections[0].Content[0].Paragraph.Text[0].Text)
	}
	if original.Bodies[1].Sections[0].ID != "note1" {
		t.Errorf("original note ID was modified: %s", original.Bodies[1].Sections[0].ID)
	}
	if original.Binaries[0].ID != "img1" {
		t.Errorf("original binary ID was modified: %s", original.Binaries[0].ID)
	}
	if original.Binaries[0].Data[0] != 1 {
		t.Errorf("original binary data was modified: %d", original.Binaries[0].Data[0])
	}

	// Test that NormalizeFootnoteBodies returns a proper clone
	normalized, _ := original.NormalizeFootnoteBodies(log)

	// Modify normalized version
	normalized.Bodies[1].Name = "modified-notes"

	// Verify original is still unchanged
	if original.Bodies[1].Name != "notes" {
		t.Errorf("original body name was modified by NormalizeFootnoteBodies: %s", original.Bodies[1].Name)
	}
}

func TestIndexTitleImages(t *testing.T) {
	log := zaptest.NewLogger(t, zaptest.WrapOptions(zap.AddCaller(), zap.AddCallerSkip(1)))

	// Create a book with an image in a section title
	book := &FictionBook{
		Bodies: []Body{
			{
				Kind: BodyMain,
				Sections: []Section{
					{
						ID: "section1",
						Title: &Title{
							Items: []TitleItem{
								{
									Paragraph: &Paragraph{
										Text: []InlineSegment{
											{
												Kind: InlineText,
												Text: "Text before image ",
											},
											{
												Kind:  InlineImageSegment,
												Image: &InlineImage{Href: "#title-image"},
											},
											{
												Kind: InlineText,
												Text: " text after image",
											},
										},
									},
								},
							},
						},
						Content: []FlowItem{
							{
								Kind: FlowParagraph,
								Paragraph: &Paragraph{
									Text: []InlineSegment{
										{Kind: InlineText, Text: "Content paragraph"},
									},
								},
							},
						},
					},
				},
			},
		},
		Binaries: []BinaryObject{
			{ID: "title-image", ContentType: "image/png", Data: []byte{1, 2, 3}},
		},
	}

	// Build link index
	links := book.buildReverseLinkIndex(log)

	// Verify title image is indexed
	refs, exists := links["title-image"]
	if !exists {
		t.Fatal("title-image not found in link index")
	}

	if len(refs) != 1 {
		t.Errorf("expected 1 reference to title-image, got %d", len(refs))
	}

	if refs[0].Type != "inline-image" {
		t.Errorf("expected type 'inline-image', got %q", refs[0].Type)
	}
}

func TestIndexMultipleTitleItems(t *testing.T) {
	log := zaptest.NewLogger(t, zaptest.WrapOptions(zap.AddCaller(), zap.AddCallerSkip(1)))

	book := &FictionBook{
		Bodies: []Body{
			{
				Kind: BodyMain,
				Sections: []Section{
					{
						ID: "section1",
						Title: &Title{
							Items: []TitleItem{
								{
									Paragraph: &Paragraph{
										Text: []InlineSegment{
											{Kind: InlineImageSegment, Image: &InlineImage{Href: "#img1"}},
										},
									},
								},
								{
									EmptyLine: true,
								},
								{
									Paragraph: &Paragraph{
										Text: []InlineSegment{
											{Kind: InlineImageSegment, Image: &InlineImage{Href: "#img2"}},
										},
									},
								},
							},
						},
					},
				},
			},
		},
		Binaries: []BinaryObject{
			{ID: "img1", ContentType: "image/png", Data: []byte{1}},
			{ID: "img2", ContentType: "image/png", Data: []byte{2}},
		},
	}

	links := book.buildReverseLinkIndex(log)

	// Verify both images are indexed
	if _, exists := links["img1"]; !exists {
		t.Error("img1 not found in link index")
	}
	if _, exists := links["img2"]; !exists {
		t.Error("img2 not found in link index")
	}
}

func TestTitleImageInTestFile(t *testing.T) {
	log := zaptest.NewLogger(t, zaptest.WrapOptions(zap.AddCaller(), zap.AddCallerSkip(1)))

	// Load the sample test file
	path := filepath.Clean("../testdata/_Test.fb2")
	if _, err := os.Stat(path); err != nil {
		t.Skip("test file not available")
	}

	doc := etree.NewDocument()
	if err := doc.ReadFromFile(path); err != nil {
		t.Fatalf("failed to load test file: %v", err)
	}

	book, err := ParseBookXML(doc, []string{"notes", "comments"}, log)
	if err != nil {
		t.Fatalf("failed to parse book: %v", err)
	}

	// Build link index
	_, _, links := book.NormalizeLinks(nil, log)

	// Verify title.png is in the index
	refs, exists := links["title.png"]
	if !exists {
		t.Fatal("title.png not found in link index")
	}

	if len(refs) == 0 {
		t.Fatal("title.png has no references")
	}

	// Should be an inline-image type
	foundInlineImage := false
	for _, ref := range refs {
		if ref.Type == "inline-image" {
			foundInlineImage = true
			break
		}
	}

	if !foundInlineImage {
		t.Errorf("expected at least one inline-image reference for title.png, got types: %v",
			func() []string {
				types := make([]string, len(refs))
				for i, r := range refs {
					types[i] = r.Type
				}
				return types
			}())
	}
}

func TestIndexTableCellImages(t *testing.T) {
	log := zaptest.NewLogger(t, zaptest.WrapOptions(zap.AddCaller(), zap.AddCallerSkip(1)))

	// Create a book with an image in a table cell
	book := &FictionBook{
		Bodies: []Body{
			{
				Kind: BodyMain,
				Sections: []Section{
					{
						ID: "section1",
						Content: []FlowItem{
							{
								Kind: FlowTable,
								Table: &Table{
									ID: "table1",
									Rows: []TableRow{
										{
											Cells: []TableCell{
												{
													Content: []InlineSegment{
														{Kind: InlineText, Text: "Cell 1"},
													},
												},
												{
													Content: []InlineSegment{
														{Kind: InlineImageSegment, Image: &InlineImage{Href: "#table-img"}},
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
		Binaries: []BinaryObject{
			{ID: "table-img", ContentType: "image/png", Data: []byte{1, 2, 3}},
		},
	}

	// Build link index
	links := book.buildReverseLinkIndex(log)

	// Verify table image is indexed
	refs, exists := links["table-img"]
	if !exists {
		t.Fatal("table-img not found in link index")
	}

	if len(refs) != 1 {
		t.Errorf("expected 1 reference to table-img, got %d", len(refs))
	}

	if refs[0].Type != "inline-image" {
		t.Errorf("expected type 'inline-image', got %q", refs[0].Type)
	}
}

func TestIndexTableCellIDs(t *testing.T) {
	log := zaptest.NewLogger(t, zaptest.WrapOptions(zap.AddCaller(), zap.AddCallerSkip(1)))

	book := &FictionBook{
		Bodies: []Body{
			{
				Kind: BodyMain,
				Sections: []Section{
					{
						ID: "section1",
						Content: []FlowItem{
							{
								Kind: FlowTable,
								Table: &Table{
									ID: "table1",
									Rows: []TableRow{
										{
											Cells: []TableCell{
												{
													ID: "cell1",
													Content: []InlineSegment{
														{Kind: InlineText, Text: "Cell 1"},
													},
												},
												{
													ID: "cell2",
													Content: []InlineSegment{
														{Kind: InlineText, Text: "Cell 2"},
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	// Build ID index
	ids := book.buildIDIndex(log)

	// Verify table ID is indexed
	if ref, exists := ids["table1"]; !exists {
		t.Error("table1 not found in ID index")
	} else if ref.Type != "table" {
		t.Errorf("table1 has wrong type: got %q, want 'table'", ref.Type)
	}

	// Verify cell IDs are indexed
	if ref, exists := ids["cell1"]; !exists {
		t.Error("cell1 not found in ID index")
	} else if ref.Type != "table-cell" {
		t.Errorf("cell1 has wrong type: got %q, want 'table-cell'", ref.Type)
	}

	if ref, exists := ids["cell2"]; !exists {
		t.Error("cell2 not found in ID index")
	} else if ref.Type != "table-cell" {
		t.Errorf("cell2 has wrong type: got %q, want 'table-cell'", ref.Type)
	}
}

func TestBBImageInTestFile(t *testing.T) {
	log := zaptest.NewLogger(t, zaptest.WrapOptions(zap.AddCaller(), zap.AddCallerSkip(1)))

	// Load the sample test file
	path := filepath.Clean("../testdata/_Test.fb2")
	if _, err := os.Stat(path); err != nil {
		t.Skip("test file not available")
	}

	doc := etree.NewDocument()
	if err := doc.ReadFromFile(path); err != nil {
		t.Fatalf("failed to load test file: %v", err)
	}

	book, err := ParseBookXML(doc, []string{"notes", "comments"}, log)
	if err != nil {
		t.Fatalf("failed to parse book: %v", err)
	}

	// Build link index
	_, _, links := book.NormalizeLinks(nil, log)

	// Verify bb.png is in the index
	refs, exists := links["bb.png"]
	if !exists {
		t.Fatal("bb.png not found in link index")
	}

	if len(refs) == 0 {
		t.Fatal("bb.png has no references")
	}

	// Should have multiple inline-image references (it's used in multiple table cells)
	inlineImageCount := 0
	for _, ref := range refs {
		if ref.Type == "inline-image" {
			inlineImageCount++
		}
	}

	if inlineImageCount == 0 {
		t.Error("expected at least one inline-image reference for bb.png")
	}

	t.Logf("bb.png has %d total references, %d are inline-image type", len(refs), inlineImageCount)
}
