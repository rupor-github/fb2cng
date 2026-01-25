package kfx

import "fmt"

// BlockBuilder collects content entries for a wrapper/container element.
// It mirrors how EPUB generates <div class="..."> wrappers.
//
// Style resolution is deferred until Build() time to enable position-aware resolution.
// This allows KP3-compatible margin filtering (CSS margin collapsing):
//   - First element: KEEPS margin-top, loses margin-bottom
//   - Last element: loses margin-top, KEEPS margin-bottom
//   - Single element: KEEPS both margins
//   - Middle elements: loses both margins
type BlockBuilder struct {
	styleSpec string         // Raw style specification (e.g., "poem", "cite") - resolved at Build() time
	styles    *StyleRegistry // Style registry for deferred resolution
	ctx       StyleContext   // Style context for child resolution (includes wrapper scope)
	eid       int            // EID for the wrapper container
	children  []ContentRef   // Nested content entries (styles resolved at Build() time)
}

// StartBlock begins a new wrapper/container block.
// All content added until EndBlock is called will be nested inside this wrapper.
// The styleSpec is the raw style name (e.g., "chapter-title", "body-title") - resolution
// is deferred until Build() time when the actual position in the storyline is known.
// Children get position-based style filtering via StyleContext:
//   - First child: gets wrapper's margin-top, loses margin-bottom
//   - Last child: loses margin-top, gets wrapper's margin-bottom
//   - Single child: gets wrapper's margins
//   - Middle children: lose both vertical margins
//
// Returns the EID of the wrapper for reference.
func (sb *StorylineBuilder) StartBlock(styleSpec string, styles *StyleRegistry) int {
	eid := sb.eidCounter
	sb.eidCounter++

	// Create StyleContext for child resolution - children will be counted in EndBlock
	// The context will be finalized with proper item count when EndBlock is called
	ctx := NewStyleContext(styles).Push("div", styleSpec)

	sb.blockStack = append(sb.blockStack, &BlockBuilder{
		styleSpec: styleSpec,
		styles:    styles,
		ctx:       ctx,
		eid:       eid,
		children:  make([]ContentRef, 0),
	})

	return eid
}

// StartBlockWithChildPositions is an alias for StartBlock.
// Deprecated: Use StartBlock directly - all blocks now apply position-aware styling to children.
func (sb *StorylineBuilder) StartBlockWithChildPositions(styleSpec string, styles *StyleRegistry) int {
	return sb.StartBlock(styleSpec, styles)
}

// EndBlock closes the current wrapper block and adds it to the storyline.
// The wrapper becomes a container entry with nested children.
// Empty wrappers (with no children) are discarded to avoid position_map validation errors.
//
// Style resolution is deferred until Build() time when the actual position in the
// storyline is known. This enables KP3-compatible margin filtering based on position.
// Child styles are resolved via StyleContext at Build() time.
func (sb *StorylineBuilder) EndBlock() {
	if len(sb.blockStack) == 0 {
		return
	}

	block := sb.blockStack[len(sb.blockStack)-1]
	sb.blockStack = sb.blockStack[:len(sb.blockStack)-1]

	// Skip empty wrappers - they have no content and cause position_map validation errors
	if len(block.children) == 0 {
		return
	}

	// Convert children to content entries for the $146 list
	// Child style resolution is deferred to Build() time
	children := make([]any, 0, len(block.children))
	for _, child := range block.children {
		children = append(children, NewContentEntry(child))
	}

	// Finalize the StyleContext with proper item count and title-block margin mode
	// Title blocks use margin-top based spacing (first loses margin-top, non-last lose margin-bottom)
	childCount := len(block.children)
	ctx := block.ctx.PushBlock("", "", childCount, true) // titleBlockMargins=true

	// Create wrapper with StyleSpec for deferred resolution at Build() time
	wrapperRef := ContentRef{
		EID:       block.eid,
		Type:      SymText, // Container wrappers use $269 (text) type in KFX
		StyleSpec: block.styleSpec,
		Children:  children,
	}

	// Store children refs and context for deferred style resolution at Build() time.
	// Child styles are resolved via StyleContext.Resolve() with proper position filtering.
	wrapperRef.childRefs = block.children
	wrapperRef.styleCtx = &ctx

	if len(sb.blockStack) > 0 {
		sb.blockStack[len(sb.blockStack)-1].children = append(sb.blockStack[len(sb.blockStack)-1].children, wrapperRef)
	} else {
		sb.contentEntries = append(sb.contentEntries, wrapperRef)
	}
}

// addEntry is the internal method that routes content to the appropriate destination.
func (sb *StorylineBuilder) addEntry(ref ContentRef) int {
	if len(sb.blockStack) > 0 {
		// Add to current block's children
		sb.blockStack[len(sb.blockStack)-1].children = append(sb.blockStack[len(sb.blockStack)-1].children, ref)
	} else {
		// Add directly to storyline
		sb.contentEntries = append(sb.contentEntries, ref)
	}
	return ref.EID
}

// resolveChildStyles resolves StyleSpec fields in child refs to Style.
// Uses StyleContext.Resolve() for text elements and ResolveStyle() for images.
//
// The wrapper's styleCtx (set in EndBlock) provides for text elements:
// - Proper CSS inheritance from wrapper to children
// - Container margin distribution (first/last/middle)
// - Title-block margin mode (spacing via margin-top)
//
// Images use ResolveStyle() directly because they need different handling:
// - No line-height inheritance (images use height: auto)
// - Direct style lookup without CSS text inheritance
func (sb *StorylineBuilder) resolveChildStyles(wrapper *ContentRef) {
	count := len(wrapper.childRefs)
	if count == 0 {
		return
	}

	// Get the StyleContext set by EndBlock
	ctx := wrapper.styleCtx
	if ctx == nil {
		// This should never happen - EndBlock always sets styleCtx
		panic("resolveChildStyles: wrapper has no StyleContext")
	}

	// Rebuild Children with resolved styles
	wrapper.Children = make([]any, 0, count)

	for i := range wrapper.childRefs {
		child := &wrapper.childRefs[i]

		// Only resolve if StyleSpec is set and Style is not already resolved
		if child.StyleSpec != "" && child.Style == "" {
			if child.Type == SymImage {
				// Images: use StyleContext.ResolveImage() for position filtering
				// without text-specific inheritance (no line-height from kfx-unknown)
				_, classes := parseStyleSpec(child.StyleSpec)
				child.Style = ctx.ResolveImage(classes)
				sb.styles.MarkUsage(child.Style, styleUsageImage)
			} else {
				// Text elements: use StyleContext.Resolve() for proper inheritance
				tag, classes := parseStyleSpec(child.StyleSpec)
				child.Style = ctx.Resolve(tag, classes)
				sb.styles.MarkUsage(child.Style, styleUsageText)
			}

			// For RawEntry children (e.g., mixed content with images), update the style in the entry
			if child.RawEntry != nil {
				child.RawEntry = child.RawEntry.Set(SymStyle, SymbolByName(child.Style))
			}
		}

		// Advance context position for next child (affects text elements)
		*ctx = ctx.Advance()

		// Convert to content entry
		wrapper.Children = append(wrapper.Children, NewContentEntry(*child))
	}
}

// applyStorylinePositionFiltering resolves deferred styles for top-level content entries.
//
// Based on KP3 Java code analysis (com/amazon/yj/i/b/d.java and b.java), position-based
// margin filtering is ONLY applied when content is FRAGMENTED (split due to 64K limits
// or page breaks). Regular consecutive elements that are not fragmented should KEEP
// their margins.
//
// Since fb2cng doesn't fragment content, top-level entries are resolved WITHOUT
// position filtering (using PositionFirstAndLast to keep all margins).
//
// Position filtering IS applied to children within wrapper blocks, as those represent
// content within a container where margin collapsing makes sense.
//
// For wrapper entries (with Children), the wrapper style is resolved and child styles
// are also resolved with position filtering applied to children.
// For RawEntry entries, the style field in the pre-built structure is updated.
func (sb *StorylineBuilder) applyStorylinePositionFiltering() {
	if len(sb.contentEntries) == 0 || sb.styles == nil {
		return
	}

	// Resolve all entries with StyleSpec WITHOUT position filtering
	// Top-level entries keep all their margins since they are not fragmented
	for i := range sb.contentEntries {
		entry := &sb.contentEntries[i]

		if entry.StyleSpec == "" {
			continue
		}

		// Resolve style WITHOUT position filtering - keep all margins
		// Use fresh StyleContext (no container stack) so margins are preserved
		tag, classes := parseStyleSpec(entry.StyleSpec)
		resolvedStyle := NewStyleContext(sb.styles).Resolve(tag, classes)

		if entry.RawEntry != nil {
			// For RawEntry, update the style field in the pre-built structure
			entry.RawEntry = entry.RawEntry.Set(SymStyle, SymbolByName(resolvedStyle))
			entry.Style = resolvedStyle
		} else {
			entry.Style = resolvedStyle
		}

		// For wrapper entries with childRefs, resolve child styles
		if len(entry.childRefs) > 0 {
			sb.resolveChildStyles(entry)
		}

		// Trace assignment for wrappers
		if len(entry.Children) > 0 {
			sb.styles.tracer.TraceAssign("wrapper", fmt.Sprintf("%d", entry.EID), resolvedStyle, sb.sectionName+"/"+sb.name, entry.StyleSpec)
			sb.styles.MarkUsage(resolvedStyle, styleUsageWrapper)
		}
	}
}
