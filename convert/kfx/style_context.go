package kfx

import (
	"maps"
	"strings"
)

// StyleScope represents a single level in the element hierarchy.
// It captures both the element tag and its classes at that level.
type StyleScope struct {
	Tag     string   // HTML element tag: "div", "p", "h1", "span", etc.
	Classes []string // CSS classes applied to this element
}

// marginOrigin tracks which styles contributed to an inherited margin value.
// This is used to implement YJCumulativeInSameContainerRuleMerger correctly:
// - Same style contributing twice → override (don't double-count)
// - Different styles contributing → accumulate
type marginOrigin struct {
	value        any             // The margin value
	contributors map[string]bool // Style names that contributed to this margin
}

// emptyLineState holds mutable state for empty line margin handling.
// This is shared across all context copies to allow margin propagation
// across container boundaries (e.g., empty line before a poem affects
// the poem's first verse).
type emptyLineState struct {
	// pendingMargin is the margin from the last empty line that should
	// be applied to the next content element's margin-top.
	pendingMargin float64
}

// ImageKind specifies how an image should be styled.
type ImageKind int

const (
	ImageBlock  ImageKind = iota // Standalone block image (centered, width%)
	ImageInline                  // Inline within text (em dimensions, baseline-style)
)

// StyleContext accumulates inherited CSS properties as we descend the element hierarchy.
// This mimics how browsers propagate inherited properties from parent to child.
//
// In CSS, some properties (font-*, color, text-align, line-height, etc.) automatically
// inherit from parent to child elements. Other properties (margin, padding, border, etc.)
// do NOT inherit - they apply only to the element where they're defined.
//
// When resolving a style for an element:
// 1. Inherited properties come from the accumulated context (ancestors)
// 2. Non-inherited properties come only from the element's own tag/classes
//
// Vertical margin collapsing is handled by post-processing in CollapseMargins(),
// which implements CSS-compliant margin collapsing rules after all content is generated.
//
// Empty-line handling: Instead of creating content entries for empty-lines, we store
// their margin in emptyLineState.pendingMargin. The next element's margin-top is
// set to the empty-line's margin, matching KP3 behavior. The emptyLineState is shared
// across all context copies to allow propagation across container boundaries.
//
// Container identity tracking: For margin-left/right, we track which style names
// contributed to the inherited value. This implements YJCumulativeInSameContainerRuleMerger:
// when resolving an element's style, if the same style name already contributed to the
// inherited margin, we don't accumulate again (override instead). Different style names
// DO accumulate (e.g., poem margin + verse margin).
type StyleContext struct {
	// registry is the style registry used for style lookups and registration.
	registry *StyleRegistry

	// Inherited properties accumulated from ancestors.
	// Only CSS-inherited properties are stored here.
	inherited map[KFXSymbol]any

	// marginOrigins tracks which styles contributed to inherited margin-left/right values.
	// Key is SymMarginLeft or SymMarginRight, value is marginOrigin with contributor tracking.
	// This enables YJCumulativeInSameContainerRuleMerger to correctly decide whether to
	// accumulate (different contributors) or override (same contributor).
	marginOrigins map[KFXSymbol]*marginOrigin

	// Full scope chain from root to current level (for debugging/future use)
	scopes []StyleScope

	// emptyLine holds shared state for empty line margin handling.
	// This is a pointer to allow mutation across value copies of StyleContext.
	// All contexts within a processing scope share the same emptyLineState.
	emptyLine *emptyLineState
}

// NewStyleContext creates an empty context (root level) with the given style registry.
// Empty-line tracking is enabled by default for all contexts.
func NewStyleContext(registry *StyleRegistry) StyleContext {
	return StyleContext{
		registry:      registry,
		inherited:     make(map[KFXSymbol]any),
		marginOrigins: make(map[KFXSymbol]*marginOrigin),
		scopes:        nil,
		emptyLine:     &emptyLineState{}, // Shared empty line state
	}
}

// ScopePath returns a CSS-like path string showing the element hierarchy.
// Example: "div.poem > div.stanza" or "body > p.verse"
func (sc StyleContext) ScopePath() string {
	if len(sc.scopes) == 0 {
		return "(root)"
	}
	parts := make([]string, 0, len(sc.scopes))
	for _, scope := range sc.scopes {
		part := scope.Tag
		if part == "" {
			part = "*"
		}
		for _, class := range scope.Classes {
			part += "." + class
		}
		parts = append(parts, part)
	}
	return strings.Join(parts, " > ")
}

// Push enters a new element scope and returns a new context with that element's
// inherited properties added. Non-inherited properties are ignored for inheritance.
//
// tag: HTML element type ("div", "p", "h1", etc.)
// classes: space-separated CSS classes ("section poem" or "" for none)
func (sc StyleContext) Push(tag, classes string) StyleContext {
	// Copy existing inherited properties
	newInherited := make(map[KFXSymbol]any, len(sc.inherited))
	maps.Copy(newInherited, sc.inherited)

	// Add inherited properties from tag defaults
	if tag != "" && sc.registry != nil {
		if def, ok := sc.registry.Get(tag); ok {
			resolved := sc.registry.resolveInheritance(def)
			for sym, val := range resolved.Properties {
				if isInheritedProperty(sym) {
					newInherited[sym] = val
				}
			}
		}
	}

	// Parse and add inherited properties from each class
	var classList []string
	if classes != "" {
		classList = strings.Fields(classes)
		for _, class := range classList {
			if sc.registry != nil {
				sc.registry.EnsureBaseStyle(class)
				if def, ok := sc.registry.Get(class); ok {
					resolved := sc.registry.resolveInheritance(def)
					for sym, val := range resolved.Properties {
						if isInheritedProperty(sym) {
							newInherited[sym] = val
						}
					}
				}
			}
		}
	}

	// Append to scope chain
	newScopes := append(sc.scopes, StyleScope{Tag: tag, Classes: classList})

	return StyleContext{
		registry:  sc.registry,
		inherited: newInherited,
		scopes:    newScopes,
		emptyLine: sc.emptyLine, // Preserve empty-line tracking
	}
}
