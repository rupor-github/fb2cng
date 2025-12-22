package builders

// StyleValue represents a KFX style value with unit and numeric value.
// Corresponds to structure like {'$306': "$310", '$307': 1.0}
type StyleValue struct {
	Unit  string  `ion:"$306,symbol"`
	Value float64 `ion:"$307"`
}

// Style represents a KFX $157 style fragment.
type Style struct {
	StyleName     string      `ion:"$173,symbol"`
	FontStyle     string      `ion:"$12,symbol,omitempty"`  // $350=normal, $382=italic
	FontWeight    string      `ion:"$13,symbol,omitempty"`  // $361=bold
	FontSize      *StyleValue `ion:"$16,omitempty"`         // font_size
	TextColor     int64       `ion:"$19,omitempty"`         // text_color (ARGB)
	TextAlign     string      `ion:"$34,symbol,omitempty"`  // $59=left, $320=center, $321=justify
	TextIndent    *StyleValue `ion:"$36,omitempty"`         // text_indent
	LineHeight    *StyleValue `ion:"$42,omitempty"`         // line_height
	BaselineStyle string      `ion:"$44,symbol,omitempty"`  // $370=superscript
	MarginTop     *StyleValue `ion:"$47,omitempty"`         // margin_top
	MarginLeft    *StyleValue `ion:"$48,omitempty"`         // margin_left
	MarginBottom  *StyleValue `ion:"$49,omitempty"`         // margin_bottom
	MarginRight   *StyleValue `ion:"$50,omitempty"`         // margin_right
	Width         *StyleValue `ion:"$56,omitempty"`         // width
	MinHeight     *StyleValue `ion:"$62,omitempty"`         // min_height
	FillColor     int64       `ion:"$70,omitempty"`         // fill_color (ARGB)
	BorderColor   int64       `ion:"$83,omitempty"`         // border_color (ARGB)
	BorderStyle   string      `ion:"$88,symbol,omitempty"`  // border_style ($328=solid, $336=dashed)
	BorderWeight  *StyleValue `ion:"$93,omitempty"`         // border_weight
	SizingBounds  string      `ion:"$546,symbol,omitempty"` // $377=content_bounds
	BoxAlign      string      `ion:"$580,symbol,omitempty"` // $320=center
	LayoutHints   []string    `ion:"$761,symbol,omitempty"` // [$760=treat_as_title]
}

// StyleDef defines a named style.
type StyleDef struct {
	ID    string
	Style Style
}

// BuildDefaultStyles creates a comprehensive set of styles for KFX output.
// Style names follow kfxlib conventions.
func BuildDefaultStyles() []StyleDef {
	// Unit constants
	unitLH := "$310"      // line-height units
	unitPercent := "$314" // percent
	unitPt := "$318"      // points
	unitRem := "$505"     // rem

	return []StyleDef{
		// sD7: Outer container style - chapter/section container
		{
			ID: "sD7",
			Style: Style{
				StyleName:    "sD7",
				FontStyle:    "$350", // normal
				FontWeight:   "$361", // bold
				FontSize:     &StyleValue{Unit: unitRem, Value: 1.25},
				LineHeight:   &StyleValue{Unit: unitLH, Value: 1.0101},
				MarginTop:    &StyleValue{Unit: unitLH, Value: 0.68475},
				MarginBottom: &StyleValue{Unit: unitLH, Value: 0.68475},
				FillColor:    4293848814,
				BorderColor:  4286611584,
				BorderStyle:  "$328", // solid
				BorderWeight: &StyleValue{Unit: unitPt, Value: 0.45},
				LayoutHints:  []string{"$760"}, // treat_as_title
			},
		},
		// sF: First paragraph in section (left aligned, no indent)
		{
			ID: "sF",
			Style: Style{
				StyleName: "sF",
				TextAlign: "$59", // left
			},
		},
		// sDC: Normal body paragraph with indent
		{
			ID: "sDC",
			Style: Style{
				StyleName:   "sDC",
				FontStyle:   "$382", // italic (but could be normal)
				TextAlign:   "$321", // justify
				TextIndent:  &StyleValue{Unit: unitPercent, Value: 6.25},
				LineHeight:  &StyleValue{Unit: unitLH, Value: 1.0},
				MarginTop:   &StyleValue{Unit: unitLH, Value: 0.833333},
				MarginLeft:  &StyleValue{Unit: unitPercent, Value: 12.5},
				MarginRight: &StyleValue{Unit: unitPercent, Value: 0.625},
			},
		},
		// sDE: Paragraph without top margin (continuation)
		{
			ID: "sDE",
			Style: Style{
				StyleName:   "sDE",
				FontStyle:   "$382", // italic
				TextAlign:   "$321", // justify
				TextIndent:  &StyleValue{Unit: unitPercent, Value: 6.25},
				LineHeight:  &StyleValue{Unit: unitLH, Value: 1.0},
				MarginLeft:  &StyleValue{Unit: unitPercent, Value: 12.5},
				MarginRight: &StyleValue{Unit: unitPercent, Value: 0.625},
			},
		},
		// s6P: Image container (centered, full width)
		{
			ID: "s6P",
			Style: Style{
				StyleName:  "s6P",
				LineHeight: &StyleValue{Unit: unitLH, Value: 1.0},
				Width:      &StyleValue{Unit: unitPercent, Value: 100.0},
				BoxAlign:   "$320", // center
			},
		},
		// sH: Chapter heading style
		{
			ID: "sH",
			Style: Style{
				StyleName:    "sH",
				FontStyle:    "$350", // normal
				FontWeight:   "$361", // bold
				FontSize:     &StyleValue{Unit: unitRem, Value: 1.375},
				LineHeight:   &StyleValue{Unit: unitLH, Value: 0.9495},
				MarginTop:    &StyleValue{Unit: unitLH, Value: 0.561168},
				MarginBottom: &StyleValue{Unit: unitLH, Value: 0.561168},
				FillColor:    4293388263,
				BorderStyle:  "$328", // solid
				BorderWeight: &StyleValue{Unit: unitPt, Value: 0.45},
				LayoutHints:  []string{"$760"}, // treat_as_title
			},
		},
		// s8AG: Inline span style (for styled text runs)
		{
			ID: "s8AG",
			Style: Style{
				StyleName:  "s8AG",
				LineHeight: &StyleValue{Unit: unitLH, Value: 1.0101},
			},
		},
		// s8AD: Italic span
		{
			ID: "s8AD",
			Style: Style{
				StyleName: "s8AD",
				FontStyle: "$382", // italic
			},
		},
		// sK: Basic justified text
		{
			ID: "sK",
			Style: Style{
				StyleName:  "sK",
				TextAlign:  "$321", // justify
				TextIndent: &StyleValue{Unit: unitPercent, Value: 6.25},
				LineHeight: &StyleValue{Unit: unitLH, Value: 1.0},
			},
		},
		// sN: Italic justified text
		{
			ID: "sN",
			Style: Style{
				StyleName:  "sN",
				FontStyle:  "$382", // italic
				TextAlign:  "$321", // justify
				TextIndent: &StyleValue{Unit: unitPercent, Value: 6.25},
				LineHeight: &StyleValue{Unit: unitLH, Value: 1.0},
			},
		},
		// s7B: Centered text
		{
			ID: "s7B",
			Style: Style{
				StyleName: "s7B",
				TextAlign: "$320", // center
			},
		},
		// sT: Justified text (simple)
		{
			ID: "sT",
			Style: Style{
				StyleName: "sT",
				TextAlign: "$321", // justify
			},
		},
		// sW: Same as sT (justified text)
		{
			ID: "sW",
			Style: Style{
				StyleName: "sW",
				TextAlign: "$321", // justify
			},
		},
		// s50: Line spacing only
		{
			ID: "s50",
			Style: Style{
				StyleName:  "s50",
				LineHeight: &StyleValue{Unit: unitLH, Value: 1.0},
				MarginTop:  &StyleValue{Unit: unitLH, Value: 0.833333},
			},
		},
		// s6J: Indented block
		{
			ID: "s6J",
			Style: Style{
				StyleName:    "s6J",
				TextAlign:    "$321", // justify
				TextIndent:   &StyleValue{Unit: unitPercent, Value: 6.25},
				LineHeight:   &StyleValue{Unit: unitLH, Value: 1.0},
				MarginBottom: &StyleValue{Unit: unitLH, Value: 0.59375},
			},
		},
		// s6S: Spacing with sizing bounds
		{
			ID: "s6S",
			Style: Style{
				StyleName:    "s6S",
				LineHeight:   &StyleValue{Unit: unitLH, Value: 1.0},
				MarginTop:    &StyleValue{Unit: unitLH, Value: 0.59375},
				SizingBounds: "$377", // content_bounds
			},
		},
		// sBS: Basic indented justified
		{
			ID: "sBS",
			Style: Style{
				StyleName:  "sBS",
				TextAlign:  "$321", // justify
				TextIndent: &StyleValue{Unit: unitPercent, Value: 6.25},
				LineHeight: &StyleValue{Unit: unitLH, Value: 1.0},
				MarginTop:  &StyleValue{Unit: unitLH, Value: 0.59375},
			},
		},
		// Legacy style names for compatibility
		{
			ID: "style_outer",
			Style: Style{
				StyleName:    "style_outer",
				LineHeight:   &StyleValue{Unit: unitLH, Value: 1.0},
				MarginTop:    &StyleValue{Unit: unitLH, Value: 0.5},
				MarginBottom: &StyleValue{Unit: unitLH, Value: 0.5},
			},
		},
		{
			ID: "style_first",
			Style: Style{
				StyleName: "style_first",
				TextAlign: "$59", // left (same as sF)
			},
		},
		{
			ID: "style_para",
			Style: Style{
				StyleName:  "style_para",
				TextAlign:  "$321", // justify
				TextIndent: &StyleValue{Unit: unitPercent, Value: 6.25},
				LineHeight: &StyleValue{Unit: unitLH, Value: 1.0},
				MarginTop:  &StyleValue{Unit: unitLH, Value: 0.5},
			},
		},
		{
			ID: "style_image",
			Style: Style{
				StyleName:  "style_image",
				LineHeight: &StyleValue{Unit: unitLH, Value: 1.0},
				Width:      &StyleValue{Unit: unitPercent, Value: 100.0},
				BoxAlign:   "$320", // center
			},
		},
		{
			ID: "style_heading",
			Style: Style{
				StyleName:  "style_heading",
				FontWeight: "$361", // bold
				LineHeight: &StyleValue{Unit: unitLH, Value: 1.0},
				MarginTop:  &StyleValue{Unit: unitLH, Value: 1.0},
				TextAlign:  "$321", // justify
			},
		},
	}
}
