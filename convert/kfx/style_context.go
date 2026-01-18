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
type StyleContext struct {
	// Inherited properties accumulated from ancestors.
	// Only CSS-inherited properties are stored here.
	inherited map[KFXSymbol]any

	// Full scope chain from root to current level (for debugging/future use)
	scopes []StyleScope
}

// NewStyleContext creates an empty context (root level).
func NewStyleContext() StyleContext {
	return StyleContext{
		inherited: make(map[KFXSymbol]any),
		scopes:    nil,
	}
}

// Push enters a new element scope and returns a new context with that element's
// inherited properties added. Non-inherited properties are ignored for inheritance.
//
// tag: HTML element type ("div", "p", "h1", etc.)
// classes: space-separated CSS classes ("section poem" or "" for none)
// registry: style registry to look up property definitions
func (sc StyleContext) Push(tag, classes string, registry *StyleRegistry) StyleContext {
	// Copy existing inherited properties
	newInherited := make(map[KFXSymbol]any, len(sc.inherited))
	maps.Copy(newInherited, sc.inherited)

	// Add inherited properties from tag defaults
	if tag != "" {
		if def, ok := registry.Get(tag); ok {
			resolved := registry.resolveInheritance(def)
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
			if registry != nil {
				registry.EnsureBaseStyle(class)
			}
			if def, ok := registry.Get(class); ok {
				resolved := registry.resolveInheritance(def)
				for sym, val := range resolved.Properties {
					if isInheritedProperty(sym) {
						newInherited[sym] = val
					}
				}
			}
		}
	}

	// Append to scope chain
	newScopes := append(sc.scopes, StyleScope{Tag: tag, Classes: classList})

	return StyleContext{
		inherited: newInherited,
		scopes:    newScopes,
	}
}

// PushBlock enters a block container scope that passes margin properties to children.
// Unlike Push() which only passes CSS-inherited properties, PushBlock() also passes
// margin-left and margin-right so children are properly indented within the container.
// This matches KP3 behavior where container margins apply to each child element directly
// rather than being applied to a wrapper block.
//
// Use PushBlock for: epigraph, poem, stanza, cite, annotation, footnote contexts.
// Use Push for: inline contexts or when standard CSS inheritance is desired.
func (sc StyleContext) PushBlock(tag, classes string, registry *StyleRegistry) StyleContext {
	// Copy existing inherited properties
	newInherited := make(map[KFXSymbol]any, len(sc.inherited))
	maps.Copy(newInherited, sc.inherited)

	// Add block-inherited properties from tag defaults
	if tag != "" {
		if def, ok := registry.Get(tag); ok {
			resolved := registry.resolveInheritance(def)
			for sym, val := range resolved.Properties {
				if isBlockInheritedProperty(sym) {
					newInherited[sym] = val
				}
			}
		}
	}

	// Parse and add block-inherited properties from each class
	var classList []string
	if classes != "" {
		classList = strings.Fields(classes)
		for _, class := range classList {
			if registry != nil {
				registry.EnsureBaseStyle(class)
			}
			if def, ok := registry.Get(class); ok {
				resolved := registry.resolveInheritance(def)
				for sym, val := range resolved.Properties {
					if isBlockInheritedProperty(sym) {
						newInherited[sym] = val
					}
				}
			}
		}
	}

	// Append to scope chain
	newScopes := append(sc.scopes, StyleScope{Tag: tag, Classes: classList})

	return StyleContext{
		inherited: newInherited,
		scopes:    newScopes,
	}
}

// resolveProperties builds the merged property map for an element within this context.
// This performs CSS-style property merging with proper inheritance and descendant selectors,
// but does NOT register the style. Use this when you need to apply additional processing
// (like position filtering) before registration.
//
// Order of application (later overrides earlier):
// 1. Inherited properties accumulated through Push calls
// 2. Element tag defaults (all properties)
// 3. Element's classes (all properties, in order)
// 4. Descendant selectors matching any scope ancestor with current tag/classes
func (sc StyleContext) resolveProperties(tag, classes string, registry *StyleRegistry) map[KFXSymbol]any {
	merged := make(map[KFXSymbol]any)

	// 1. Start with inherited properties from context
	registry.mergePropertiesWithContext(merged, sc.inherited, mergeContextInline)

	// 2. Apply element tag defaults (all properties)
	if tag != "" {
		if def, ok := registry.Get(tag); ok {
			resolved := registry.resolveInheritance(def)
			registry.mergePropertiesWithContext(merged, resolved.Properties, mergeContextInline)
		}
	}

	// 3. Apply element's classes (all properties, in order)
	classList := strings.Fields(classes)
	if classes != "" {
		for class := range strings.FieldsSeq(classes) {
			if def, ok := registry.Get(class); ok {
				resolved := registry.resolveInheritance(def)
				registry.mergePropertiesWithContext(merged, resolved.Properties, mergeContextInline)
			}
		}
	}

	// 4. Apply descendant selectors matching any scope ancestor with current tag/classes.
	// This mirrors CSS descendant rules like ".section-title h2.section-title-header".
	if len(sc.scopes) > 0 {
		descCandidates := make([]string, 0, len(classList)+1)
		if tag != "" {
			descCandidates = append(descCandidates, tag)
		}
		descCandidates = append(descCandidates, classList...)

		for _, scope := range sc.scopes {
			ancestors := make([]string, 0, len(scope.Classes)+1)
			ancestors = append(ancestors, scope.Classes...)
			if scope.Tag != "" {
				ancestors = append(ancestors, scope.Tag)
			}

			for _, anc := range ancestors {
				for _, desc := range descCandidates {
					styleName := anc + "--" + desc
					if def, ok := registry.Get(styleName); ok {
						resolved := registry.resolveInheritance(def)
						registry.mergePropertiesWithContext(merged, resolved.Properties, mergeContextInline)
					}
				}
			}
		}
	}

	return merged
}

// Resolve creates the final style for an element within this context.
// Wrapper properties only influence children via standard CSS inheritance
// and descendant selectors; wrapper margins stay on the wrapper containers.
//
// tag: HTML element type ("p", "h1", "span", etc.)
// classes: space-separated CSS classes (or "" for none)
// registry: style registry for lookups and registration
// Returns the registered style name.
func (sc StyleContext) Resolve(tag, classes string, registry *StyleRegistry) string {
	merged := sc.resolveProperties(tag, classes, registry)
	return registry.RegisterResolved(merged)
}

// ResolveWithPosition creates the final style for an element within this context,
// applying position-based property filtering before registration.
//
// This is the preferred method when the element's position within its container is known.
// Position filtering removes properties like margin-top for first elements and margin-bottom
// for last elements, matching KP3's position-aware CSS processing.
//
// tag: HTML element type ("p", "h1", "span", etc.)
// classes: space-separated CSS classes (or "" for none)
// registry: style registry for lookups and registration
// pos: element's position within its container
// Returns the registered style name.
func (sc StyleContext) ResolveWithPosition(tag, classes string, registry *StyleRegistry, pos ElementPosition) string {
	merged := sc.resolveProperties(tag, classes, registry)
	filtered := FilterPropertiesByPosition(merged, pos)
	return registry.RegisterResolved(filtered)
}

// isInheritedProperty returns true for CSS properties that inherit by default.
// Reference: https://developer.mozilla.org/en-US/docs/Web/CSS/inheritance
func isInheritedProperty(sym KFXSymbol) bool {
	switch sym {
	// Font properties
	case SymFontFamily, SymFontSize, SymFontWeight, SymFontStyle:
		return true
	// Text properties
	case SymTextAlignment, SymTextIndent, SymLineHeight:
		return true
	// Color
	case SymTextColor:
		return true
	// Spacing that inherits
	case SymLetterspacing:
		return true
	default:
		return false
	}
}

// isBlockInheritedProperty returns true for properties that should be inherited
// in block container contexts. Unlike standard CSS inheritance, this includes
// margin-left and margin-right so children are properly indented within containers.
// This matches KP3 behavior where container margins apply to each child element.
func isBlockInheritedProperty(sym KFXSymbol) bool {
	if isInheritedProperty(sym) {
		return true
	}
	switch sym {
	case SymMarginLeft, SymMarginRight:
		return true
	default:
		return false
	}
}
