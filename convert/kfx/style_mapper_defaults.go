package kfx

import "maps"

func mapDefaultWrapperStyles(mapper *StyleMapper) ([]StyleDef, []string) {
	return mapper.MapWrappers(defaultWrapperCSS())
}

func defaultTitlePieceProps() map[string]map[string]CSSValue {
	return map[string]map[string]CSSValue{
		"body-title-header": {
			"text-indent": {Value: 0},
			"text-align":  {Keyword: "center"},
			"font-weight": {Keyword: "bold"},
		},
		"chapter-title-header": {
			"text-indent": {Value: 0},
			"text-align":  {Keyword: "center"},
			"font-weight": {Keyword: "bold"},
		},
		"section-title-header": {
			"text-indent": {Value: 0},
			"text-align":  {Keyword: "center"},
			"font-weight": {Keyword: "bold"},
		},
		"body-title-header-emptyline": {
			"display":       {Keyword: "block"},
			"margin-top":    {Value: 0.8, Unit: "em"},
			"margin-right":  {Value: 0, Unit: "em"},
			"margin-bottom": {Value: 0.8, Unit: "em"},
			"margin-left":   {Value: 0, Unit: "em"},
		},
		"chapter-title-header-emptyline": {
			"display":       {Keyword: "block"},
			"margin-top":    {Value: 0.8, Unit: "em"},
			"margin-right":  {Value: 0, Unit: "em"},
			"margin-bottom": {Value: 0.8, Unit: "em"},
			"margin-left":   {Value: 0, Unit: "em"},
		},
		"section-title-header-emptyline": {
			"display":       {Keyword: "block"},
			"margin-top":    {Value: 0.8, Unit: "em"},
			"margin-right":  {Value: 0, Unit: "em"},
			"margin-bottom": {Value: 0.8, Unit: "em"},
			"margin-left":   {Value: 0, Unit: "em"},
		},
		"body-title-header-break": {
			"display": {Keyword: "block"},
		},
		"chapter-title-header-break": {
			"display": {Keyword: "block"},
		},
		"section-title-header-break": {
			"display": {Keyword: "block"},
		},
		"body-title-header-first": {
			"display":     {Keyword: "inline"},
			"text-indent": {Value: 0},
		},
		"chapter-title-header-first": {
			"display":     {Keyword: "inline"},
			"text-indent": {Value: 0},
		},
		"section-title-header-first": {
			"display":     {Keyword: "inline"},
			"text-indent": {Value: 0},
		},
		"body-title-header-next": {
			"display":     {Keyword: "inline"},
			"text-indent": {Value: 0},
		},
		"chapter-title-header-next": {
			"display":     {Keyword: "inline"},
			"text-indent": {Value: 0},
		},
		"section-title-header-next": {
			"display":     {Keyword: "inline"},
			"text-indent": {Value: 0},
		},
		"toc-title": {
			"text-indent": {Value: 0},
			"text-align":  {Keyword: "center"},
			"font-weight": {Keyword: "bold"},
		},
		"toc-title-emptyline": {
			"display":       {Keyword: "block"},
			"margin-top":    {Value: 0.8, Unit: "em"},
			"margin-right":  {Value: 0, Unit: "em"},
			"margin-bottom": {Value: 0.8, Unit: "em"},
			"margin-left":   {Value: 0, Unit: "em"},
		},
		"toc-title-break": {
			"display": {Keyword: "block"},
		},
		"toc-title-first": {
			"display":     {Keyword: "inline"},
			"text-indent": {Value: 0},
		},
		"toc-title-next": {
			"display":     {Keyword: "inline"},
			"text-indent": {Value: 0},
		},
		"section-subtitle": {
			"page-break-before": {Keyword: "auto"},
			"text-align":        {Keyword: "center"},
			"font-weight":       {Keyword: "bold"},
			"text-indent":       {Value: 0},
			"margin-top":        {Value: 1, Unit: "em"},
			"margin-right":      {Value: 0, Unit: "em"},
			"margin-bottom":     {Value: 1, Unit: "em"},
			"margin-left":       {Value: 0, Unit: "em"},
			"page-break-after":  {Keyword: "avoid"},
		},
	}
}

func cloneCSSProps(props map[string]CSSValue) map[string]CSSValue {
	if len(props) == 0 {
		return nil
	}
	out := make(map[string]CSSValue, len(props))
	maps.Copy(out, props)
	return out
}

func defaultWrapperCSS() []WrapperCSS {
	wrappers := make([]WrapperCSS, 0, 32)
	titleProps := defaultTitlePieceProps()

	for _, wrapper := range []struct {
		class       string
		breakBefore bool
	}{
		{class: "body-title", breakBefore: true},
		{class: "chapter-title", breakBefore: true},
		{class: "section-title", breakBefore: false},
	} {
		props := map[string]CSSValue{
			"page-break-inside": {Keyword: "avoid"},
			"page-break-after":  {Keyword: "avoid"},
			"margin-top":        {Value: 2, Unit: "em"},
			"margin-right":      {Value: 0, Unit: "em"},
			"margin-bottom":     {Value: 1, Unit: "em"},
			"margin-left":       {Value: 0, Unit: "em"},
		}
		if wrapper.breakBefore {
			props["page-break-before"] = CSSValue{Keyword: "always"}
		}
		wrappers = append(wrappers, WrapperCSS{
			Tag:        "div",
			Classes:    []string{wrapper.class},
			Properties: props,
		})
	}

	// Note: page-break-before: always for h2.section-title-header is handled
	// in mapTitleDescendantWrappers() where it has proper ancestor context
	// to skip UserAgentStyleAddingTransformer (which injects font_size: 1.5em).
	// We do NOT add a standalone h2.section-title-header wrapper here because
	// it would get merged with the class-only wrapper below, polluting the
	// base section-title-header style with h2's font-size.

	for _, class := range []string{
		"body-title-header",
		"chapter-title-header",
		"section-title-header",
	} {
		wrappers = append(wrappers, WrapperCSS{
			Classes:    []string{class},
			Properties: cloneCSSProps(titleProps[class]),
		})
	}

	for _, class := range []string{
		"body-title-header-emptyline",
		"chapter-title-header-emptyline",
		"section-title-header-emptyline",
	} {
		wrappers = append(wrappers, WrapperCSS{
			Classes:    []string{class},
			Properties: cloneCSSProps(titleProps[class]),
		})
	}

	for _, class := range []string{
		"body-title-header-break",
		"chapter-title-header-break",
		"section-title-header-break",
	} {
		wrappers = append(wrappers, WrapperCSS{
			Classes:    []string{class},
			Properties: cloneCSSProps(titleProps[class]),
		})
	}

	for _, class := range []string{
		"body-title-header-first",
		"chapter-title-header-first",
		"section-title-header-first",
		"body-title-header-next",
		"chapter-title-header-next",
		"section-title-header-next",
	} {
		wrappers = append(wrappers, WrapperCSS{
			Classes:    []string{class},
			Properties: cloneCSSProps(titleProps[class]),
		})
	}

	for _, class := range []string{
		"toc-title",
		"toc-title-emptyline",
		"toc-title-break",
		"toc-title-first",
		"toc-title-next",
		"section-subtitle",
	} {
		if props, ok := titleProps[class]; ok {
			wrappers = append(wrappers, WrapperCSS{
				Classes:    []string{class},
				Properties: cloneCSSProps(props),
			})
		}
	}

	// Structural wrappers mirroring default.css
	wrappers = append(wrappers, []WrapperCSS{
		{Classes: []string{"image"}, Properties: map[string]CSSValue{
			"text-indent": {Value: 0},
			"text-align":  {Keyword: "center"},
		}},
		{Classes: []string{"image-block"}, Tag: "img", Properties: map[string]CSSValue{
			"max-width":  {Value: 100, Unit: "%"},
			"max-height": {Value: 100, Unit: "%"},
			"height":     {Keyword: "auto"},
		}},
		{Classes: []string{"image-inline"}, Tag: "img", Properties: map[string]CSSValue{
			"max-width":      {Value: 100, Unit: "%"},
			"max-height":     {Value: 100, Unit: "%"},
			"vertical-align": {Keyword: "middle"},
		}},
		{Classes: []string{"image-vignette"}, Tag: "img", Properties: map[string]CSSValue{
			"width":  {Value: 100, Unit: "%"},
			"height": {Keyword: "auto"},
		}},
		{Classes: []string{"vignette"}, Properties: map[string]CSSValue{
			"text-indent":   {Value: 0},
			"text-align":    {Keyword: "center"},
			"margin-top":    {Value: 0.5, Unit: "em"},
			"margin-bottom": {Value: 0.5, Unit: "em"},
		}},
		{Classes: []string{"vignette-book-title-top"}, Properties: map[string]CSSValue{
			"margin-top":    {Value: 1, Unit: "em"},
			"margin-bottom": {Value: 0.5, Unit: "em"},
		}},
		{Classes: []string{"vignette-book-title-bottom"}, Properties: map[string]CSSValue{
			"margin-top":    {Value: 0.5, Unit: "em"},
			"margin-bottom": {Value: 1, Unit: "em"},
		}},
		{Classes: []string{"vignette-chapter-title-top"}, Properties: map[string]CSSValue{
			"margin-top":    {Value: 1, Unit: "em"},
			"margin-bottom": {Value: 0.5, Unit: "em"},
		}},
		{Classes: []string{"vignette-chapter-title-bottom"}, Properties: map[string]CSSValue{
			"margin-top":    {Value: 0.5, Unit: "em"},
			"margin-bottom": {Value: 1, Unit: "em"},
		}},
		{Classes: []string{"vignette-chapter-end"}, Properties: map[string]CSSValue{
			"margin-top":        {Value: 1.5, Unit: "em"},
			"margin-bottom":     {Value: 1.5, Unit: "em"},
			"page-break-before": {Keyword: "avoid"},
		}},
		{Classes: []string{"vignette-section-title-top"}, Properties: map[string]CSSValue{
			"margin-top":    {Value: 0.8, Unit: "em"},
			"margin-bottom": {Value: 0.4, Unit: "em"},
		}},
		{Classes: []string{"vignette-section-title-bottom"}, Properties: map[string]CSSValue{
			"margin-top":    {Value: 0.4, Unit: "em"},
			"margin-bottom": {Value: 0.8, Unit: "em"},
		}},
		{Classes: []string{"vignette-section-end"}, Properties: map[string]CSSValue{
			"margin-top":        {Value: 1, Unit: "em"},
			"margin-bottom":     {Value: 1, Unit: "em"},
			"page-break-before": {Keyword: "avoid"},
		}},
		// epigraph: margins come from default.css; only font/text properties here
		// to avoid cumulative margin doubling when CSS is also loaded
		{Classes: []string{"epigraph"}, Properties: map[string]CSSValue{
			"text-align": {Keyword: "right"},
			"font-style": {Keyword: "italic"},
		}},
		{Classes: []string{"epigraph-subtitle"}, Properties: map[string]CSSValue{
			"text-align":    {Keyword: "right"},
			"font-style":    {Keyword: "italic"},
			"text-indent":   {Value: 0},
			"margin-top":    {Value: 0.3, Unit: "em"},
			"margin-bottom": {Value: 0.3, Unit: "em"},
		}},
		// annotation: margins come from default.css
		{Classes: []string{"annotation"}, Properties: map[string]CSSValue{}},
		{Classes: []string{"annotation-title"}, Properties: map[string]CSSValue{
			"margin-top":    {Value: 1, Unit: "em"},
			"margin-bottom": {Value: 1, Unit: "em"},
			"text-align":    {Keyword: "center"},
			"text-indent":   {Value: 0},
			"font-weight":   {Keyword: "bold"},
		}},
		{Classes: []string{"annotation-subtitle"}, Properties: map[string]CSSValue{
			"text-align":    {Keyword: "center"},
			"text-indent":   {Value: 0},
			"margin-top":    {Value: 0.5, Unit: "em"},
			"margin-bottom": {Value: 0.5, Unit: "em"},
			"font-weight":   {Keyword: "bold"},
		}},
		// poem: margins come from default.css; only font/text properties here
		{Classes: []string{"poem"}, Properties: map[string]CSSValue{
			"text-indent": {Value: 0},
			"font-style":  {Keyword: "italic"},
		}},
		{Classes: []string{"poem-title"}, Properties: map[string]CSSValue{
			"margin-top":    {Value: 1, Unit: "em"},
			"margin-bottom": {Value: 1, Unit: "em"},
		}},
		{Classes: []string{"poem-title-first"}, Properties: map[string]CSSValue{
			"text-align":    {Keyword: "center"},
			"text-indent":   {Value: 0},
			"margin-top":    {Value: 0, Unit: "em"},
			"margin-bottom": {Value: 0, Unit: "em"},
		}},
		{Classes: []string{"poem-title-next"}, Properties: map[string]CSSValue{
			"text-align":    {Keyword: "center"},
			"text-indent":   {Value: 0},
			"margin-top":    {Value: 0, Unit: "em"},
			"margin-bottom": {Value: 0, Unit: "em"},
		}},
		{Classes: []string{"poem-subtitle"}, Properties: map[string]CSSValue{
			"text-align":    {Keyword: "center"},
			"text-indent":   {Value: 0},
			"margin-top":    {Value: 0.5, Unit: "em"},
			"margin-bottom": {Value: 0.5, Unit: "em"},
		}},
		{Classes: []string{"stanza"}, Properties: map[string]CSSValue{
			"margin-top":    {Value: 0.5, Unit: "em"},
			"margin-bottom": {Value: 0.5, Unit: "em"},
		}},
		{Classes: []string{"stanza-title"}, Properties: map[string]CSSValue{
			"margin-top":    {Value: 0.5, Unit: "em"},
			"margin-bottom": {Value: 0.5, Unit: "em"},
		}},
		{Classes: []string{"stanza-title-first"}, Properties: map[string]CSSValue{
			"text-align":    {Keyword: "center"},
			"text-indent":   {Value: 0},
			"margin-top":    {Value: 0, Unit: "em"},
			"margin-bottom": {Value: 0, Unit: "em"},
		}},
		{Classes: []string{"stanza-title-next"}, Properties: map[string]CSSValue{
			"text-align":    {Keyword: "center"},
			"text-indent":   {Value: 0},
			"margin-top":    {Value: 0, Unit: "em"},
			"margin-bottom": {Value: 0, Unit: "em"},
		}},
		{Classes: []string{"stanza-subtitle"}, Properties: map[string]CSSValue{
			"text-align":    {Keyword: "center"},
			"text-indent":   {Value: 0},
			"margin-top":    {Value: 0.25, Unit: "em"},
			"margin-bottom": {Value: 0.25, Unit: "em"},
		}},
		// verse: margins come from default.css
		{Classes: []string{"verse"}, Properties: map[string]CSSValue{
			"text-indent": {Value: 0},
		}},
		// cite: margins come from default.css
		{Classes: []string{"cite"}, Properties: map[string]CSSValue{}},
		{Classes: []string{"cite-subtitle"}, Properties: map[string]CSSValue{
			"text-align":    {Keyword: "left"},
			"text-indent":   {Value: 0},
			"margin-top":    {Value: 0.5, Unit: "em"},
			"margin-bottom": {Value: 0.5, Unit: "em"},
		}},
		{Classes: []string{"text-author"}, Properties: map[string]CSSValue{
			"text-align":        {Keyword: "right"},
			"font-style":        {Keyword: "italic"},
			"text-indent":       {Value: 0},
			"page-break-before": {Keyword: "avoid"},
			"font-weight":       {Keyword: "bold"},
		}},
		{Classes: []string{"date"}, Properties: map[string]CSSValue{
			"text-align":        {Keyword: "right"},
			"text-indent":       {Value: 0},
			"margin-top":        {Value: 0.5, Unit: "em"},
			"margin-bottom":     {Value: 0.5, Unit: "em"},
			"page-break-before": {Keyword: "avoid"},
		}},
		// emptyline: margins come from default.css
		{Classes: []string{"emptyline"}, Properties: map[string]CSSValue{
			"display": {Keyword: "block"},
		}},
		{Classes: []string{"section"}, Properties: map[string]CSSValue{
			"margin-top":    {Value: 1, Unit: "em"},
			"margin-bottom": {Value: 1, Unit: "em"},
		}},
		{Classes: []string{"footnote"}, Properties: map[string]CSSValue{
			"text-indent": {Value: 0},
		}},
		{Classes: []string{"footnote-title"}, Properties: map[string]CSSValue{
			"margin-top":    {Value: 1, Unit: "em"},
			"margin-right":  {Value: 0, Unit: "em"},
			"margin-bottom": {Value: 0.5, Unit: "em"},
			"margin-left":   {Value: 0, Unit: "em"},
			"text-align":    {Keyword: "left"},
			"font-weight":   {Keyword: "bold"},
		}},
		{Classes: []string{"footnote-title-first"}, Properties: map[string]CSSValue{
			"text-indent":   {Value: 0},
			"margin-top":    {Value: 0.2, Unit: "em"},
			"margin-bottom": {Value: 0.2, Unit: "em"},
		}},
		{Classes: []string{"footnote-title-next"}, Properties: map[string]CSSValue{
			"text-indent":   {Value: 0},
			"margin-top":    {Value: 0.2, Unit: "em"},
			"margin-bottom": {Value: 0.2, Unit: "em"},
		}},
		{Classes: []string{"footnote-more"}, Properties: map[string]CSSValue{
			"text-decoration": {Keyword: "none"},
			"font-weight":     {Keyword: "bold"},
			"color":           {Raw: "gray"},
		}},
		{Classes: []string{"link-external"}, Properties: map[string]CSSValue{
			"text-decoration": {Keyword: "underline"},
		}},
		{Classes: []string{"link-internal"}, Properties: map[string]CSSValue{
			"text-decoration": {Keyword: "none"},
		}},
		{Classes: []string{"link-footnote"}, Properties: map[string]CSSValue{
			"text-decoration": {Keyword: "none"},
			"font-style":      {Keyword: "normal"},
			"vertical-align":  {Keyword: "super"},
			// Override default.css font-size (0.8em) with rem to prevent relative
			// multiplication when merged with sup (0.75rem). Using em would cause:
			// sup(0.75rem) × link-footnote(0.8em) = 0.6rem via YJRelativeRuleMerger.
			"font-size": {Value: 0.75, Unit: "rem"},
		}},
	}...)

	return wrappers
}

// mapStructuralWrappers emits mapped StyleDefs for structural descendant rules.
func mapStructuralWrappers(mapper *StyleMapper) ([]StyleDef, []string) {
	warnings := make([]string, 0)
	defs := make([]StyleDef, 0)

	if titleDefs, titleWarnings := mapTitleDescendantWrappers(mapper); len(titleDefs) > 0 {
		defs = append(defs, titleDefs...)
		warnings = append(warnings, titleWarnings...)
	}

	return defs, warnings
}

func mapTitleDescendantWrappers(mapper *StyleMapper) ([]StyleDef, []string) {
	titleProps := defaultTitlePieceProps()
	defs := make([]StyleDef, 0)
	warnings := make([]string, 0)

	add := func(sel Selector, props map[string]CSSValue) {
		if mapped, ws := _mapStructuralSelector(sel, props, mapper); mapped.Name != "" {
			defs = append(defs, mapped)
			warnings = append(warnings, ws...)
		}
	}

	for _, cfg := range []struct {
		ancestor string
		tags     []string
		base     string
	}{
		{ancestor: "body-title", tags: []string{"h1"}, base: "body-title-header"},
		{ancestor: "chapter-title", tags: []string{"h1"}, base: "chapter-title-header"},
		{ancestor: "section-title", tags: []string{"h2", "h3", "h4", "h5", "h6"}, base: "section-title-header"},
	} {
		for _, tag := range cfg.tags {
			for _, suffix := range []string{"", "-first", "-next", "-break", "-emptyline"} {
				class := cfg.base + suffix
				props := cloneCSSProps(titleProps[class])
				if cfg.base == "section-title-header" && tag == "h2" {
					if props == nil {
						props = make(map[string]CSSValue)
					}
					props["page-break-before"] = CSSValue{Keyword: "always"}
				}
				sel := Selector{
					Raw:     "." + cfg.ancestor + " " + tag + "." + class,
					Element: tag,
					Class:   class,
					Ancestor: &Selector{
						Class: cfg.ancestor,
					},
				}
				add(sel, props)
			}
		}
	}

	for _, cfg := range []struct {
		ancestor string
		tag      string
		class    string
	}{
		{ancestor: "toc", tag: "h1", class: "toc-title"},
		{ancestor: "toc", tag: "h1", class: "toc-title-emptyline"},
		{ancestor: "toc", tag: "h1", class: "toc-title-break"},
		{ancestor: "toc", tag: "h1", class: "toc-title-first"},
		{ancestor: "toc", tag: "h1", class: "toc-title-next"},
		{ancestor: "section", tag: "p", class: "section-subtitle"},
	} {
		props := cloneCSSProps(titleProps[cfg.class])
		sel := Selector{
			Raw:     "." + cfg.ancestor + " " + cfg.tag + "." + cfg.class,
			Element: cfg.tag,
			Class:   cfg.class,
			Ancestor: &Selector{
				Class: cfg.ancestor,
			},
		}
		add(sel, props)
	}

	return defs, warnings
}

func _mapStructuralSelector(sel Selector, props map[string]CSSValue, mapper *StyleMapper) (StyleDef, []string) {
	if mapper == nil {
		return StyleDef{}, nil
	}
	mappedProps, warnings := mapper.MapCSS(sel, props)
	return StyleDef{
		Name:       sel.StyleName(),
		Properties: mappedProps,
	}, warnings
}
