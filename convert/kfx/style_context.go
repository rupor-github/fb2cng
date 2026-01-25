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
// This is shared across all container frames to allow margin propagation
// across container boundaries (e.g., empty line before a poem affects
// the poem's first verse).
type emptyLineState struct {
	// pendingMargin is the margin from the last empty line that should
	// be applied to the next content element's margin-top.
	pendingMargin float64

	// keepMarginBottom, when true, signals that subsequent elements should
	// keep their margin-bottom even if position filtering would normally
	// remove it (for middle elements). Once set by an empty line, this
	// stays true until container processing completes.
	keepMarginBottom bool
}

// containerFrame holds margin info for a single container level in the stack.
// When entering a container (poem, stanza, cite, etc.), a frame is pushed.
// When processing completes, the frame is effectively "popped" via Go's value semantics.
type containerFrame struct {
	marginTop         float64 // Container's margin-top (for first child)
	marginBottom      float64 // Container's margin-bottom (for last child)
	itemCount         int     // Total items in this container
	currentItem       int     // Current item index (0-based)
	isLastInParent    bool    // Whether this container is the last item in its parent
	titleBlockMargins bool    // If true, use title-block margin style (spacing via margin-top)
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
// StyleContext also supports position tracking for block contexts. When processing
// items within a block (annotation, epigraph, etc.), the containerStack tracks position
// and applies KP3-compatible position filtering (margin collapsing simulation).
//
// Empty-line handling: Instead of creating content entries for empty-lines, we store
// their margin in emptyLineState.pendingMargin. The next element's margin-top is
// set to the empty-line's margin, and its margin-bottom is preserved (not removed
// by position filtering), matching KP3 behavior. The emptyLineState is shared across
// all context copies to allow propagation across container boundaries.
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

	// containerStack is a stack of container frames for nested margin handling.
	// When entering a container (poem, stanza, cite, etc.), push a frame.
	// When exiting, it's effectively popped via Go's value semantics.
	// Top of stack is used for vertical margin distribution.
	containerStack []containerFrame

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

// ContainerPath returns a summary of the container stack with positions.
// Example: "poem[2/3] > stanza[5/14]" showing current item / total items.
func (sc StyleContext) ContainerPath() string {
	if len(sc.containerStack) == 0 {
		return "(no containers)"
	}
	parts := make([]string, 0, len(sc.containerStack))
	for i, frame := range sc.containerStack {
		// Try to get a name from the corresponding scope
		name := "container"
		if i < len(sc.scopes) {
			scope := sc.scopes[i]
			if len(scope.Classes) > 0 {
				name = scope.Classes[len(scope.Classes)-1] // Use last class as name
			} else if scope.Tag != "" {
				name = scope.Tag
			}
		}
		parts = append(parts, formatContainerFrame(name, frame))
	}
	return strings.Join(parts, " > ")
}

// formatContainerFrame formats a single container frame for display.
func formatContainerFrame(name string, frame containerFrame) string {
	pos := frame.currentItem + 1 // 1-based for display
	total := frame.itemCount
	result := name + "[" + itoa(pos) + "/" + itoa(total) + "]"
	if frame.titleBlockMargins {
		result += "*" // Mark title-block mode
	}
	return result
}

// itoa is a simple int-to-string without importing strconv.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	if n < 0 {
		return "-" + itoa(-n)
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
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

// Advance moves to the next item in the current container.
// Call this after processing each item in a container.
// Returns a new StyleContext with updated position in the top container frame.
//
// If there's no container stack, this is a no-op.
func (sc StyleContext) Advance() StyleContext {
	if len(sc.containerStack) == 0 {
		return sc // No container stack - nothing to advance
	}

	// Copy stack and increment current item in top frame
	newStack := make([]containerFrame, len(sc.containerStack))
	copy(newStack, sc.containerStack)
	newStack[len(newStack)-1].currentItem++

	return StyleContext{
		registry:       sc.registry,
		inherited:      sc.inherited,
		marginOrigins:  sc.marginOrigins,
		scopes:         sc.scopes,
		containerStack: newStack,
		emptyLine:      sc.emptyLine,
	}
}

// HasPosition returns true if this context has position information set.
// This is true when a container stack with tracked positions is present.
func (sc StyleContext) HasPosition() bool {
	return len(sc.containerStack) > 0
}
