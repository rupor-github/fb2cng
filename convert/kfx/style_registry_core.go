package kfx

// StyleRegistry manages style definitions and generates style fragments.
type StyleRegistry struct {
	styles map[string]StyleDef
	order  []string        // Preserve insertion order
	used   map[string]bool // Track which styles are actually used (for BuildFragments)
	usage  map[string]styleUsage

	resolved        map[string]string // signature -> resolved style name
	resolvedCounter int

	tracer *StyleTracer // Optional tracer for debugging style resolution

	externalLinks *ExternalLinkRegistry // Tracks external link URLs -> anchor IDs

	bodyFontFamily string // Body font family name (e.g., "paragraph") for "default" substitution
}

// NewStyleRegistry creates a new style registry.
func NewStyleRegistry() *StyleRegistry {
	return &StyleRegistry{
		styles:        make(map[string]StyleDef),
		used:          make(map[string]bool),
		usage:         make(map[string]styleUsage),
		resolved:      make(map[string]string),
		externalLinks: NewExternalLinkRegistry(),
		// Start from 55 so the first style is "s1J" (base36(55) == 1J), like KP3 samples.
		resolvedCounter: 54,
	}
}

// SetTracer sets the style tracer for debugging.
func (sr *StyleRegistry) SetTracer(t *StyleTracer) {
	sr.tracer = t
}

// Tracer returns the style tracer, or nil if none is set.
func (sr *StyleRegistry) Tracer() *StyleTracer {
	return sr.tracer
}

// SetBodyFontFamily sets the body font family name for "default" substitution.
// When BuildFragments converts styles, font-family values matching the body font
// will be replaced with "default" (as KP3 does).
func (sr *StyleRegistry) SetBodyFontFamily(family string) {
	sr.bodyFontFamily = family
}

// RegisterExternalLink registers an external URL and returns its anchor ID.
// Multiple references to the same URL will share the same anchor ID.
func (sr *StyleRegistry) RegisterExternalLink(url string) string {
	return sr.externalLinks.Register(url)
}

// BuildExternalLinkFragments creates anchor fragments for all registered external URLs.
func (sr *StyleRegistry) BuildExternalLinkFragments() []*Fragment {
	return sr.externalLinks.BuildFragments()
}

// RecomputeUsedStyles scans content fragments and marks which styles are actually referenced.
// This must be called before BuildFragments to ensure only used styles are included in output.
//
// The method recursively scans all fragments looking for $157 (style) symbol references,
// and marks those styles as used. This handles styles in:
//   - Content entries ($146)
//   - Style events ($142)
//   - Nested containers and children
func (sr *StyleRegistry) RecomputeUsedStyles(fragments *FragmentList) {
	// Clear existing usage flags
	sr.used = make(map[string]bool)

	// Scan all fragments for style references
	for _, frag := range fragments.All() {
		sr.scanValueForStyles(frag.Value)
	}
}

// scanValueForStyles recursively scans a value for style symbol references.
func (sr *StyleRegistry) scanValueForStyles(v any) {
	switch val := v.(type) {
	case StructValue:
		sr.scanStructForStyles(val)
	case map[KFXSymbol]any:
		sr.scanStructForStyles(val)
	case []any:
		for _, item := range val {
			sr.scanValueForStyles(item)
		}
	}
}

// scanStructForStyles scans a struct value for style references.
func (sr *StyleRegistry) scanStructForStyles(s map[KFXSymbol]any) {
	// Check for direct style reference ($157)
	if styleVal, ok := s[SymStyle]; ok {
		switch v := styleVal.(type) {
		case SymbolByNameValue:
			// Style stored as string name (before serialization)
			styleName := string(v)
			if styleName != "" {
				sr.used[styleName] = true
			}
		case SymbolValue:
			// Style stored as resolved symbol ID (after serialization)
			// This shouldn't happen in our use case, but handle it for completeness
			styleName := traceSymbolName(KFXSymbol(v))
			if styleName != "" {
				sr.used[styleName] = true
			}
		}
	}

	// Recursively scan all values
	for _, val := range s {
		sr.scanValueForStyles(val)
	}
}

func (sr *StyleRegistry) mergeProperty(dst map[KFXSymbol]any, sym KFXSymbol, val any) {
	sr.mergePropertyWithContext(dst, sym, val, mergeContextInline)
}

func (sr *StyleRegistry) mergePropertyWithContext(dst map[KFXSymbol]any, sym KFXSymbol, val any, ctx mergeContext) {
	if existing, ok := dst[sym]; ok {
		merged, keep := mergeStyleProperty(sym, existing, val, ctx, sr.tracer)
		if keep {
			dst[sym] = merged
		}
		return
	}
	dst[sym] = val
}

func (sr *StyleRegistry) mergePropertiesWithContext(dst map[KFXSymbol]any, src map[KFXSymbol]any, ctx mergeContext) {
	for sym, val := range src {
		sr.mergePropertyWithContext(dst, sym, val, ctx)
	}
}

// Register adds a style to the registry.
// If a style with the same name already exists, the properties are merged,
// with new properties overriding existing ones (CSS cascade behavior).
func (sr *StyleRegistry) Register(def StyleDef) {
	existing, exists := sr.styles[def.Name]
	if !exists {
		sr.order = append(sr.order, def.Name)
		sr.styles[def.Name] = def
		sr.tracer.TraceRegister(def.Name, def.Properties)
		return
	}

	merged := make(map[KFXSymbol]any, len(existing.Properties)+len(def.Properties))
	mergeAllWithRules(merged, existing.Properties, mergeContextInline, sr.tracer)
	// Use mergeContextClassOverride for the new definition so CSS values properly
	// override defaults. With mergeContextInline (allowWritingModeConvert=true),
	// margin-top/bottom use override-maximum which keeps the larger value.
	// But CSS semantics require that later rules override earlier ones regardless
	// of which value is larger. mergeContextClassOverride (allowWritingModeConvert=false)
	// triggers the override rule for margins, matching CSS cascade behavior.
	mergeAllWithRules(merged, def.Properties, mergeContextClassOverride, sr.tracer)

	// Inherit parent from new def if specified
	parent := existing.Parent
	if def.Parent != "" {
		parent = def.Parent
	}

	// Preserve DescendantReplacement flag (true if either has it)
	descReplacement := existing.DescendantReplacement || def.DescendantReplacement

	// Later CSS rules override Hidden flag (CSS cascade)
	hidden := def.Hidden || existing.Hidden

	sr.styles[def.Name] = StyleDef{
		Name:                  def.Name,
		Parent:                parent,
		Properties:            merged,
		DescendantReplacement: descReplacement,
		Hidden:                hidden,
	}
	sr.tracer.TraceRegister(def.Name+" (merged)", merged)
}

// Get returns a style definition by name.
func (sr *StyleRegistry) Get(name string) (StyleDef, bool) {
	def, ok := sr.styles[name]
	return def, ok
}

// IsDescendantReplacement returns true if the named style uses replacement semantics
// for descendant selectors. When true, descendant selectors like "h1--sub" completely
// replace the base class rather than just overriding specific properties.
func (sr *StyleRegistry) IsDescendantReplacement(name string) bool {
	if def, ok := sr.styles[name]; ok {
		return def.DescendantReplacement
	}
	return false
}

// IsHidden returns true if the named style has CSS "display: none".
// This is used to skip generating content that would be hidden by CSS
// (e.g., footnote-more indicator when .footnote-more { display: none }).
func (sr *StyleRegistry) IsHidden(name string) bool {
	if def, ok := sr.styles[name]; ok {
		return def.Hidden
	}
	return false
}

// Names returns all registered style names in order.
func (sr *StyleRegistry) Names() []string {
	return sr.order
}
