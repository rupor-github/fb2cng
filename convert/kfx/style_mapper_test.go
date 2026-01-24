package kfx

import "testing"

func TestStyleMapperDelegatesToConverter(t *testing.T) {
	mapper := NewStyleMapper(nil, nil)

	props, warnings := mapper.MapRule(Selector{Raw: ".cls", Class: "cls"}, map[string]CSSValue{
		"font-weight": {Keyword: "bold"},
		"clear":       {Keyword: "both"},
	})

	if len(warnings) != 0 {
		t.Fatalf("unexpected warnings: %v", warnings)
	}

	if props[SymFontWeight] != SymBold {
		t.Fatalf("expected font-weight to map to bold, got %v", props[SymFontWeight])
	}
	if props[SymFloatClear] != SymBoth {
		t.Fatalf("expected clear to map to both, got %v", props[SymFloatClear])
	}
}

func TestStyleMapperStylesheet(t *testing.T) {
	parser := NewParser(nil)
	mapper := NewStyleMapper(nil, nil)

	css := []byte(`h1 { font-weight: bold; }`)
	sheet := parser.Parse(css)

	styles, warnings := mapper.MapStylesheet(sheet)
	if len(warnings) != 0 {
		t.Fatalf("unexpected warnings: %v", warnings)
	}
	if len(styles) != 1 {
		t.Fatalf("expected 1 style, got %d", len(styles))
	}
	if styles[0].Name != "h1" {
		t.Fatalf("expected style name h1, got %s", styles[0].Name)
	}
	weight, ok := styles[0].Properties[SymFontWeight]
	if !ok {
		t.Fatalf("font-weight not set")
	}
	if weight != SymBold && weight != SymbolValue(SymBold) {
		t.Fatalf("expected bold weight, got %v", weight)
	}
}

func TestStyleMapperWidowsOrphansTransformer(t *testing.T) {
	mapper := NewStyleMapper(nil, nil)

	props, warnings := mapper.MapRule(Selector{Raw: "p", Element: "p"}, map[string]CSSValue{
		"widows":  {Value: 2},
		"orphans": {Value: 3},
	})
	if len(warnings) != 0 {
		t.Fatalf("unexpected warnings: %v", warnings)
	}

	value, ok := props[SymKeepLinesTogether]
	if !ok {
		t.Fatalf("missing keep_lines_together property")
	}
	keepMap, ok := value.(map[KFXSymbol]any)
	if !ok {
		t.Fatalf("keep_lines_together should be a symbol map, got %T", value)
	}
	if keepMap[SymKeepLinesTogether] != true {
		t.Fatalf("keep_lines_together flag missing")
	}
	if first, ok := keepMap[SymKeepFirst]; !ok || first != 2 {
		t.Fatalf("expected widows->first=2, got %v", keepMap[SymKeepFirst])
	}
	if last, ok := keepMap[SymKeepLast]; !ok || last != 3 {
		t.Fatalf("expected orphans->last=3, got %v", keepMap[SymKeepLast])
	}
}

func TestStyleMapperSnapBlockOnlyForImages(t *testing.T) {
	mapper := NewStyleMapper(nil, nil)

	props, _ := mapper.MapRule(Selector{Raw: "div", Element: "div"}, map[string]CSSValue{
		"float": {Keyword: "snap-block"},
	})
	if _, ok := props[SymFloat]; ok {
		t.Fatalf("snap-block float should be ignored for non-img elements")
	}

	propsImg, _ := mapper.MapRule(Selector{Raw: "img", Element: "img"}, map[string]CSSValue{
		"float": {Keyword: "snap-block"},
	})
	if val, ok := propsImg[SymFloat]; !ok {
		t.Fatalf("snap-block float should be preserved for images")
	} else if sym, ok := symbolIDFromAny(val); !ok || sym != snapBlockSymbol(t) {
		t.Fatalf("expected snap_block symbol, got %v", val)
	}
}

func TestStyleMapperLineBreakMapsStringEnum(t *testing.T) {
	mapper := NewStyleMapper(nil, nil)

	props, _ := mapper.MapRule(Selector{Raw: "p", Element: "p"}, map[string]CSSValue{
		"line-break": {Keyword: "loose"},
	})

	lineBreakSym := mustSymbol(t, "line_break")
	val, ok := props[lineBreakSym]
	if !ok {
		t.Fatalf("line_break property missing")
	}
	if sym, ok := symbolIDFromAny(val); !ok || sym != mustSymbol(t, "loose") {
		t.Fatalf("expected line_break=loose symbol, got %v", val)
	}
}

func TestStyleMapperImageBorderAttribute(t *testing.T) {
	mapper := NewStyleMapper(nil, nil)

	props, warnings := mapper.MapRule(Selector{Raw: "img", Element: "img"}, map[string]CSSValue{
		"border": {Value: 2, Unit: "px"},
	})
	if len(warnings) != 0 {
		t.Fatalf("unexpected warnings: %v", warnings)
	}

	weightTopSym := mustSymbol(t, "border_weight_top")
	if weight, ok := props[weightTopSym]; !ok {
		t.Fatalf("missing border_weight_top")
	} else if val, unit, ok := measureParts(weight); !ok || val != 2 || unit != SymUnitPx {
		t.Fatalf("expected 2px border weight, got %v (ok=%v unit=%v)", weight, ok, unit)
	}

	styleTopSym := mustSymbol(t, "border_style_top")
	if style, ok := props[styleTopSym]; !ok {
		t.Fatalf("missing border_style_top")
	} else if sym, ok := symbolIDFromAny(style); !ok || sym != mustSymbol(t, "solid") {
		t.Fatalf("expected solid border style, got %v", style)
	}
}

func TestStyleMapperMarkColors(t *testing.T) {
	mapper := NewStyleMapper(nil, nil)

	props, warnings := mapper.MapRule(Selector{Raw: "mark", Element: "mark"}, map[string]CSSValue{})
	if len(warnings) != 0 {
		t.Fatalf("unexpected warnings: %v", warnings)
	}

	if bg, ok := props[SymTextBackgroundColor]; !ok {
		t.Fatalf("missing text_background_color")
	} else if bg != MakeColorValue(255, 255, 0) {
		t.Fatalf("expected yellow background, got %v", bg)
	}
	if fg, ok := props[SymTextColor]; !ok {
		t.Fatalf("missing text_color")
	} else if fg != MakeColorValue(0, 0, 0) {
		t.Fatalf("expected black text, got %v", fg)
	}
}

func TestStyleMapperBorderColorAttributes(t *testing.T) {
	mapper := NewStyleMapper(nil, nil)

	props, warnings := mapper.MapRule(Selector{Raw: "div", Element: "div"}, map[string]CSSValue{
		"border-top-color":  {Raw: "#010203"},
		"outline-color":     {Raw: "rgb(4,5,6)"},
		"column-rule-color": {Raw: "rgba(7,8,9,0.5)"},
	})
	if len(warnings) != 0 {
		t.Fatalf("unexpected warnings: %v", warnings)
	}

	colorSym := mustSymbol(t, "border_color_top")
	if val, ok := props[colorSym]; !ok {
		t.Fatalf("missing border_color_top")
	} else if val != MakeColorValue(1, 2, 3) {
		t.Fatalf("expected #010203 color, got %v", val)
	}
	if val, ok := props[mustSymbol(t, "outline_color")]; !ok {
		t.Fatalf("missing outline_color")
	} else if val != MakeColorValue(4, 5, 6) {
		t.Fatalf("expected outline rgb(4,5,6), got %v", val)
	}
	if val, ok := props[mustSymbol(t, "column_rule_color")]; !ok {
		t.Fatalf("missing column_rule_color")
	} else if val != MakeColorValue(7, 8, 9) {
		t.Fatalf("expected column rule rgba(7,8,9,*), got %v", val)
	}
}

func TestStyleMapperBGColorTransformer(t *testing.T) {
	mapper := NewStyleMapper(nil, nil)

	props, warnings := mapper.MapRule(Selector{Raw: "body", Element: "body"}, map[string]CSSValue{
		"bgcolor": {Raw: "#112233"},
	})
	if len(warnings) != 0 {
		t.Fatalf("unexpected warnings: %v", warnings)
	}

	if val, ok := props[SymFillColor]; !ok {
		t.Fatalf("missing fill_color")
	} else if val != MakeColorValue(0x11, 0x22, 0x33) {
		t.Fatalf("expected fill_color #112233, got %v", val)
	}
}

func TestStyleMapperBGRepeatTransformer(t *testing.T) {
	mapper := NewStyleMapper(nil, nil)

	props, warnings := mapper.MapRule(Selector{Raw: "div", Element: "div"}, map[string]CSSValue{
		"background-repeat-x": {Keyword: "repeat"},
		"background-repeat-y": {Keyword: "no-repeat"},
	})
	if len(warnings) != 0 {
		t.Fatalf("unexpected warnings: %v", warnings)
	}

	if val, ok := props[mustSymbol(t, "background_repeat")]; !ok {
		t.Fatalf("missing background_repeat")
	} else if sym, ok := symbolIDFromAny(val); !ok || sym != mustSymbol(t, "background_repeat") {
		t.Fatalf("unexpected repeat symbol %v", val)
	}
}

func TestStyleMapperXYStyleTransformer(t *testing.T) {
	mapper := NewStyleMapper(nil, nil)

	props, warnings := mapper.MapRule(Selector{Raw: "div", Element: "div"}, map[string]CSSValue{
		"background-sizex": {Value: 10, Unit: "px"},
		"background-sizey": {Value: 20, Unit: "px"},
	})
	if len(warnings) != 0 {
		t.Fatalf("unexpected warnings: %v", warnings)
	}

	check := func(name string, want float64) {
		sym := mustSymbol(t, name)
		val, ok := props[sym]
		if !ok {
			t.Fatalf("missing %s", name)
		}
		if v, unit, ok := measureParts(val); !ok || unit != SymUnitPx || v != want {
			t.Fatalf("expected %s=%gpx, got %v (unit=%v ok=%v)", name, want, val, unit, ok)
		}
	}

	check("background_sizex", 10)
	check("background_sizey", 20)
}

func TestStyleMapperYJExtensions(t *testing.T) {
	mapper := NewStyleMapper(nil, nil)

	shape := "M0,0 L1,1"
	props, warnings := mapper.MapRule(Selector{Raw: "div", Element: "div"}, map[string]CSSValue{
		"-amzn-shape-outside":       {Raw: shape},
		"-amzn-max-crop-percentage": {Raw: "5,5,5,5"},
		"-webkit-box-shadow":        {Raw: "0 0 1px #000"},
		"text-shadow":               {Raw: "1px 1px 2px rgba(0,0,0,0.5)"},
	})
	if len(warnings) != 0 {
		t.Fatalf("unexpected warnings: %v", warnings)
	}

	if val, ok := props[mustSymbol(t, "yj.border_path")]; !ok || val != shape {
		t.Fatalf("expected yj.border_path=%s, got %v", shape, val)
	}
	if val, ok := props[mustSymbol(t, "yj.max_crop")]; !ok {
		t.Fatalf("expected yj.max_crop")
	} else if sv, ok := toStructValue(val); !ok {
		t.Fatalf("max_crop should be struct, got %T", val)
	} else {
		checkCrop := func(sym KFXSymbol, expected float64) {
			dim, ok := sv[sym]
			if !ok {
				t.Fatalf("missing crop side %v", sym)
			}
			if v, unit, ok := measureParts(dim); !ok || unit != SymUnitPercent || v != expected {
				t.Fatalf("expected %v=%.0f%%, got %v (unit=%v ok=%v)", sym, expected, dim, unit, ok)
			}
		}
		checkCrop(SymTop, 5)
		checkCrop(SymRight, 5)
		checkCrop(SymBottom, 5)
		checkCrop(SymLeft, 5)
	}
	checkShadow := func(val any, expectSpread bool, expectColor int64, offsets ...float64) {
		list, ok := val.([]StructValue)
		if !ok {
			t.Fatalf("shadow should be []StructValue, got %T", val)
		}
		if len(list) != 1 {
			t.Fatalf("expected 1 shadow entry, got %d", len(list))
		}
		shadow := list[0]
		expectDim := func(sym KFXSymbol, want float64) {
			dim, ok := shadow[sym]
			if !ok {
				t.Fatalf("missing shadow component %v", sym)
			}
			if v, unit, ok := measureParts(dim); !ok || unit != SymUnitPx || v != want {
				t.Fatalf("expected %v=%gpx, got %v (unit=%v ok=%v)", sym, want, dim, unit, ok)
			}
		}
		expectDim(mustSymbol(t, "horizontal_offset"), offsets[0])
		expectDim(mustSymbol(t, "vertical_offset"), offsets[1])
		if len(offsets) > 2 {
			expectDim(mustSymbol(t, "blur"), offsets[2])
		}
		if expectSpread {
			expectDim(mustSymbol(t, "spread"), offsets[3])
		} else if _, ok := shadow[mustSymbol(t, "spread")]; ok {
			t.Fatalf("unexpected spread for text shadow")
		}
		if expectColor != 0 {
			colorSym := mustSymbol(t, "color")
			if c, ok := shadow[colorSym]; !ok || c != expectColor {
				t.Fatalf("expected color %d, got %v (shadow=%v)", expectColor, shadow[colorSym], shadow)
			}
		}
	}

	if val, ok := props[mustSymbol(t, "shadows")]; !ok {
		t.Fatalf("expected shadows")
	} else {
		checkShadow(val, true, MakeColorValue(0, 0, 0), 0, 0, 1, 0)
	}
	if val, ok := props[mustSymbol(t, "text_shadows")]; !ok {
		t.Fatalf("expected text_shadows")
	} else {
		checkShadow(val, false, MakeColorValue(0, 0, 0), 1, 1, 2)
	}
}

func TestStyleMapperPageBleed(t *testing.T) {
	mapper := NewStyleMapper(nil, nil)

	props, warnings := mapper.MapRule(Selector{Raw: "div", Element: "div"}, map[string]CSSValue{
		"-amzn-page-align": {Raw: "left,right"},
	})
	if len(warnings) != 0 {
		t.Fatalf("unexpected warnings: %v", warnings)
	}

	expectSide := func(name string) {
		sym := mustSymbol(t, name)
		val, ok := props[sym]
		if !ok {
			t.Fatalf("missing %s", name)
		}
		if v, unit, ok := measureParts(val); !ok || unit != SymUnitPercent || v != -100 {
			t.Fatalf("expected %s=-100%%, got %v (unit=%v ok=%v)", name, val, unit, ok)
		}
	}

	expectSide("yj.user_margin_left_percentage")
	expectSide("yj.user_margin_right_percentage")
	if _, ok := props[mustSymbol(t, "yj.user_margin_top_percentage")]; ok {
		t.Fatalf("unexpected top margin when not requested")
	}
}

func TestStyleMapperBackgroundXYTransforms(t *testing.T) {
	mapper := NewStyleMapper(nil, nil)

	props, warnings := mapper.MapRule(Selector{Raw: "div", Element: "div"}, map[string]CSSValue{
		"background-position": {Raw: "10% 20%"},
		"background-size":     {Raw: "auto 50%"},
	})
	if len(warnings) != 0 {
		t.Fatalf("unexpected warnings: %v", warnings)
	}

	checkDim := func(name string, expected float64, unit KFXSymbol) {
		sym := mustSymbol(t, name)
		val, ok := props[sym]
		if !ok {
			t.Fatalf("missing %s", name)
		}
		if v, u, ok := measureParts(val); !ok || v != expected || u != unit {
			t.Fatalf("expected %s=%g %v, got %v (u=%v ok=%v)", name, expected, unit, val, u, ok)
		}
	}

	checkDim("background_positionx", 10, SymUnitPercent)
	checkDim("background_positiony", 20, SymUnitPercent)
	if _, ok := props[mustSymbol(t, "background_sizex")]; ok {
		t.Fatalf("background_sizex should be omitted when auto")
	}
	checkDim("background_sizey", 50, SymUnitPercent)
}

func snapBlockSymbol(t *testing.T) KFXSymbol {
	t.Helper()
	sym, ok := symbolIDFromString("snap_block")
	if !ok {
		t.Fatalf("snap_block symbol missing")
	}
	return sym
}

func mustSymbol(t *testing.T, name string) KFXSymbol {
	t.Helper()
	sym, ok := symbolIDFromString(name)
	if !ok {
		t.Fatalf("missing symbol %s", name)
	}
	return sym
}
