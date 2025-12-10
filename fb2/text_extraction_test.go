package fb2

import (
	"testing"
)

func TestParagraph_AsPlainText(t *testing.T) {
	tests := []struct {
		name     string
		para     Paragraph
		expected string
	}{
		{
			name: "simple text",
			para: Paragraph{
				Text: []InlineSegment{
					{Kind: InlineText, Text: "Hello world"},
				},
			},
			expected: "Hello world",
		},
		{
			name: "multiple segments",
			para: Paragraph{
				Text: []InlineSegment{
					{Kind: InlineText, Text: "First "},
					{Kind: InlineStrong, Text: "bold"},
					{Kind: InlineText, Text: " last"},
				},
			},
			expected: "First bold last",
		},
		{
			name: "empty paragraph",
			para: Paragraph{
				Text: []InlineSegment{},
			},
			expected: "",
		},
		{
			name: "with whitespace",
			para: Paragraph{
				Text: []InlineSegment{
					{Kind: InlineText, Text: "  text with spaces  "},
				},
			},
			expected: "text with spaces",
		},
		{
			name: "nested children",
			para: Paragraph{
				Text: []InlineSegment{
					{
						Kind: InlineStrong,
						Text: "Bold ",
						Children: []InlineSegment{
							{Kind: InlineEmphasis, Text: "and italic"},
						},
					},
				},
			},
			expected: "Bold and italic",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.para.AsPlainText()
			if got != tt.expected {
				t.Errorf("AsPlainText() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestParagraph_AsImageAlt(t *testing.T) {
	tests := []struct {
		name     string
		para     Paragraph
		expected string
	}{
		{
			name: "single image with alt",
			para: Paragraph{
				Text: []InlineSegment{
					{
						Kind: InlineImageSegment,
						Image: &InlineImage{
							Href: "#img1",
							Alt:  "Test image",
						},
					},
				},
			},
			expected: "Test image",
		},
		{
			name: "multiple images",
			para: Paragraph{
				Text: []InlineSegment{
					{
						Kind: InlineImageSegment,
						Image: &InlineImage{
							Href: "#img1",
							Alt:  "First",
						},
					},
					{
						Kind: InlineImageSegment,
						Image: &InlineImage{
							Href: "#img2",
							Alt:  "Second",
						},
					},
				},
			},
			expected: "First Second",
		},
		{
			name: "image without alt",
			para: Paragraph{
				Text: []InlineSegment{
					{
						Kind: InlineImageSegment,
						Image: &InlineImage{
							Href: "#img1",
							Alt:  "",
						},
					},
				},
			},
			expected: "",
		},
		{
			name: "no images",
			para: Paragraph{
				Text: []InlineSegment{
					{Kind: InlineText, Text: "Just text"},
				},
			},
			expected: "",
		},
		{
			name: "nested image in segment",
			para: Paragraph{
				Text: []InlineSegment{
					{
						Kind: InlineStrong,
						Children: []InlineSegment{
							{
								Kind: InlineImageSegment,
								Image: &InlineImage{
									Href: "#nested",
									Alt:  "Nested image",
								},
							},
						},
					},
				},
			},
			expected: "Nested image",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.para.AsImageAlt()
			if got != tt.expected {
				t.Errorf("AsImageAlt() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestParagraph_AsTOCText(t *testing.T) {
	tests := []struct {
		name     string
		para     Paragraph
		fallback string
		expected string
	}{
		{
			name: "with plain text",
			para: Paragraph{
				Text: []InlineSegment{
					{Kind: InlineText, Text: "Chapter Title"},
				},
			},
			fallback: "Fallback",
			expected: "Chapter Title",
		},
		{
			name: "empty with image alt",
			para: Paragraph{
				Text: []InlineSegment{
					{
						Kind: InlineImageSegment,
						Image: &InlineImage{
							Href: "#img",
							Alt:  "Image Title",
						},
					},
				},
			},
			fallback: "Fallback",
			expected: "Image Title",
		},
		{
			name: "empty paragraph uses fallback",
			para: Paragraph{
				Text: []InlineSegment{},
			},
			fallback: "Default Title",
			expected: "Default Title",
		},
		{
			name: "text takes precedence over image",
			para: Paragraph{
				Text: []InlineSegment{
					{Kind: InlineText, Text: "Text Title"},
					{
						Kind: InlineImageSegment,
						Image: &InlineImage{
							Href: "#img",
							Alt:  "Image Alt",
						},
					},
				},
			},
			fallback: "Fallback",
			expected: "Text Title",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.para.AsTOCText(tt.fallback)
			if got != tt.expected {
				t.Errorf("AsTOCText() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestInlineSegment_AsText(t *testing.T) {
	tests := []struct {
		name     string
		seg      InlineSegment
		expected string
	}{
		{
			name:     "simple text",
			seg:      InlineSegment{Kind: InlineText, Text: "Hello"},
			expected: "Hello",
		},
		{
			name: "with children",
			seg: InlineSegment{
				Kind: InlineStrong,
				Text: "Bold ",
				Children: []InlineSegment{
					{Kind: InlineText, Text: "text"},
				},
			},
			expected: "Bold text",
		},
		{
			name: "link is skipped",
			seg: InlineSegment{
				Kind: InlineLink,
				Text: "Link text",
			},
			expected: "",
		},
		{
			name: "nested links are skipped",
			seg: InlineSegment{
				Kind: InlineStrong,
				Text: "Start ",
				Children: []InlineSegment{
					{Kind: InlineLink, Text: "skip this"},
					{Kind: InlineText, Text: " end"},
				},
			},
			expected: "Start  end",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.seg.AsText()
			if got != tt.expected {
				t.Errorf("AsText() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestInlineSegment_AsImageAlt(t *testing.T) {
	tests := []struct {
		name     string
		seg      InlineSegment
		expected string
	}{
		{
			name: "inline image with alt",
			seg: InlineSegment{
				Kind: InlineImageSegment,
				Image: &InlineImage{
					Href: "#img",
					Alt:  "Alt text",
				},
			},
			expected: "Alt text",
		},
		{
			name: "inline image without alt",
			seg: InlineSegment{
				Kind: InlineImageSegment,
				Image: &InlineImage{
					Href: "#img",
					Alt:  "",
				},
			},
			expected: "",
		},
		{
			name: "non-image segment",
			seg: InlineSegment{
				Kind: InlineText,
				Text: "Regular text",
			},
			expected: "",
		},
		{
			name: "nested image in children",
			seg: InlineSegment{
				Kind: InlineStrong,
				Children: []InlineSegment{
					{
						Kind: InlineImageSegment,
						Image: &InlineImage{
							Href: "#nested",
							Alt:  "Nested alt",
						},
					},
				},
			},
			expected: "Nested alt",
		},
		{
			name: "multiple images in children",
			seg: InlineSegment{
				Kind: InlineStrong,
				Children: []InlineSegment{
					{
						Kind: InlineImageSegment,
						Image: &InlineImage{
							Href: "#img1",
							Alt:  "First",
						},
					},
					{
						Kind: InlineImageSegment,
						Image: &InlineImage{
							Href: "#img2",
							Alt:  "Second",
						},
					},
				},
			},
			expected: "First Second",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.seg.AsImageAlt()
			if got != tt.expected {
				t.Errorf("AsImageAlt() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestFlow_AsPlainText(t *testing.T) {
	tests := []struct {
		name     string
		flow     Flow
		expected string
	}{
		{
			name: "single paragraph",
			flow: Flow{
				Items: []FlowItem{
					{
						Kind: FlowParagraph,
						Paragraph: &Paragraph{
							Text: []InlineSegment{
								{Kind: InlineText, Text: "First paragraph."},
							},
						},
					},
				},
			},
			expected: "First paragraph.",
		},
		{
			name: "multiple items",
			flow: Flow{
				Items: []FlowItem{
					{
						Kind: FlowParagraph,
						Paragraph: &Paragraph{
							Text: []InlineSegment{
								{Kind: InlineText, Text: "First."},
							},
						},
					},
					{
						Kind: FlowParagraph,
						Paragraph: &Paragraph{
							Text: []InlineSegment{
								{Kind: InlineText, Text: "Second."},
							},
						},
					},
				},
			},
			expected: "First. Second.",
		},
		{
			name:     "empty flow",
			flow:     Flow{},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.flow.AsPlainText()
			if got != tt.expected {
				t.Errorf("AsPlainText() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestSection_AsTitleText(t *testing.T) {
	tests := []struct {
		name     string
		section  Section
		fallback string
		expected string
	}{
		{
			name: "with title",
			section: Section{
				Title: &Title{
					Items: []TitleItem{
						{
							Paragraph: &Paragraph{
								Text: []InlineSegment{
									{Kind: InlineText, Text: "Section Title"},
								},
							},
						},
					},
				},
			},
			fallback: "Fallback",
			expected: "Section Title",
		},
		{
			name: "without title uses fallback",
			section: Section{
				Title: nil,
			},
			fallback: "Default",
			expected: "Default",
		},
		{
			name: "with ID but no title",
			section: Section{
				ID:    "sect_123",
				Title: nil,
			},
			fallback: "Fallback",
			expected: "Fallback",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.section.AsTitleText(tt.fallback)
			if got != tt.expected {
				t.Errorf("AsTitleText() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestBody_AsTitleText(t *testing.T) {
	tests := []struct {
		name     string
		body     Body
		fallback string
		expected string
	}{
		{
			name: "with title",
			body: Body{
				Title: &Title{
					Items: []TitleItem{
						{
							Paragraph: &Paragraph{
								Text: []InlineSegment{
									{Kind: InlineText, Text: "Body Title"},
								},
							},
						},
					},
				},
			},
			fallback: "Fallback",
			expected: "Body Title",
		},
		{
			name: "without title uses fallback",
			body: Body{
				Title: nil,
			},
			fallback: "Default Body",
			expected: "Default Body",
		},
		{
			name: "empty title uses fallback",
			body: Body{
				Title: &Title{
					Items: []TitleItem{
						{
							Paragraph: &Paragraph{
								Text: []InlineSegment{},
							},
						},
					},
				},
			},
			fallback: "Default",
			expected: "Default",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.body.AsTitleText(tt.fallback)
			if got != tt.expected {
				t.Errorf("AsTitleText() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestEpigraph_AsPlainText(t *testing.T) {
	tests := []struct {
		name     string
		epigraph Epigraph
		expected string
	}{
		{
			name: "with content and author",
			epigraph: Epigraph{
				Flow: Flow{
					Items: []FlowItem{
						{
							Kind: FlowParagraph,
							Paragraph: &Paragraph{
								Text: []InlineSegment{
									{Kind: InlineText, Text: "Quote text"},
								},
							},
						},
					},
				},
				TextAuthors: []Paragraph{
					{
						Text: []InlineSegment{
							{Kind: InlineText, Text: "Author Name"},
						},
					},
				},
			},
			expected: "Quote text Author Name",
		},
		{
			name: "without author",
			epigraph: Epigraph{
				Flow: Flow{
					Items: []FlowItem{
						{
							Kind: FlowParagraph,
							Paragraph: &Paragraph{
								Text: []InlineSegment{
									{Kind: InlineText, Text: "Just quote"},
								},
							},
						},
					},
				},
				TextAuthors: []Paragraph{},
			},
			expected: "Just quote",
		},
		{
			name: "empty epigraph",
			epigraph: Epigraph{
				Flow:        Flow{},
				TextAuthors: []Paragraph{},
			},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.epigraph.AsPlainText()
			if got != tt.expected {
				t.Errorf("AsPlainText() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestPoem_AsPlainText(t *testing.T) {
	poem := Poem{
		Title: &Title{
			Items: []TitleItem{
				{
					Paragraph: &Paragraph{
						Text: []InlineSegment{
							{Kind: InlineText, Text: "Poem Title"},
						},
					},
				},
			},
		},
		Stanzas: []Stanza{
			{
				Verses: []Paragraph{
					{
						Text: []InlineSegment{
							{Kind: InlineText, Text: "First line"},
						},
					},
				},
			},
		},
	}

	got := poem.AsPlainText()
	if got == "" {
		t.Error("AsPlainText() should not be empty")
	}
	// Should contain both title and verse content
}

func TestCite_AsPlainText(t *testing.T) {
	cite := Cite{
		Items: []FlowItem{
			{
				Kind: FlowParagraph,
				Paragraph: &Paragraph{
					Text: []InlineSegment{
						{Kind: InlineText, Text: "Citation text"},
					},
				},
			},
		},
		TextAuthors: []Paragraph{
			{
				Text: []InlineSegment{
					{Kind: InlineText, Text: "Source"},
				},
			},
		},
	}

	got := cite.AsPlainText()
	if got != "Citation text Source" {
		t.Errorf("AsPlainText() = %q, want %q", got, "Citation text Source")
	}
}

func TestTable_AsPlainText(t *testing.T) {
	table := Table{
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
							{Kind: InlineText, Text: "Cell 2"},
						},
					},
				},
			},
		},
	}

	got := table.AsPlainText()
	if got == "" {
		t.Error("AsPlainText() should not be empty")
	}
}
