package kfx

import "strings"

// ResolveImage creates the final style for an image element within this context.
// Unlike Resolve(), this does NOT inherit from kfx-unknown (images don't need line-height).
// Position-based margin filtering is applied from the container stack.
//
// classes: space-separated CSS classes (e.g., "image-vignette")
// Returns the registered style name.
func (sc StyleContext) ResolveImage(classes string) string {
	merged := make(map[KFXSymbol]any)

	if sc.registry == nil {
		return ""
	}

	// Resolve classes directly - no kfx-unknown base, no tag defaults
	// Images only get properties from their specific classes
	for class := range strings.FieldsSeq(classes) {
		if def, ok := sc.registry.Get(class); ok {
			resolved := sc.registry.resolveInheritance(def)
			for k, v := range resolved.Properties {
				// Skip text-specific properties that don't apply to images
				switch k {
				case SymLineHeight, SymTextIndent, SymTextAlignment:
					continue
				}
				merged[k] = v
			}
		}
	}

	// Apply vertical margin distribution from container stack
	sc.applyContainerMargins(merged)

	// Use RegisterResolvedRaw to avoid adding kfx-unknown base (no line-height for images)
	// Standard filtering (height: auto, table props) is still applied
	return sc.registry.RegisterResolvedRaw(merged)
}

// ResolveImageWithDimensions resolves style for an image with calculated dimensions.
// This unifies all dimension-based image styling, applying position filtering consistently.
//
// Parameters:
//   - kind: ImageBlock or ImageInline
//   - imageWidth, imageHeight: pixel dimensions (height only used for ImageInline)
//   - blockStyle: for block images, optional style spec to inherit from (e.g., "image", "subtitle")
//     If blockStyle contains "image", this is a standalone block image.
//     Otherwise, it's an image inside another block element.
//     Ignored for ImageInline.
//
// Block image behavior:
//   - Standalone block images (blockStyle contains "image"):
//   - Full-width (≥512px): Fixed 2.6lh margins (KP3 behavior), NO position filtering
//   - Smaller: Position filtering applies from containerStack
//   - Always centered (box-align: center)
//   - Images inside other blocks (paragraph, subtitle, etc.):
//   - Inherit text-indent as margin-left (KP3 aligns such images with paragraph text)
//   - Inherit margin-left from container context
//   - Position filtering applies
//   - Centered only if block has text-align: center
//
// Inline image behavior:
//   - Uses em dimensions (width/height converted from pixels using 16px base)
//   - baseline-style: center for vertical alignment within text
//   - Applies properties from "image-inline" CSS style
//   - No position filtering (inline images don't participate in margin collapsing)
//
// Returns the registered style name.
func (sc StyleContext) ResolveImageWithDimensions(kind ImageKind, imageWidth, imageHeight int, blockStyle string) string {
	if sc.registry == nil {
		return ""
	}

	props := make(map[KFXSymbol]any)

	if kind == ImageInline {
		return sc.resolveInlineImage(props, imageWidth, imageHeight)
	}

	return sc.resolveBlockImage(props, imageWidth, blockStyle)
}

// resolveInlineImage handles ImageInline styling.
// Inline images use em dimensions and baseline-style for text alignment.
func (sc StyleContext) resolveInlineImage(props map[KFXSymbol]any, imageWidth, imageHeight int) string {
	const baseFontSizePx = 16.0 // Standard em base size

	// Convert pixel dimensions to em (using 16px base)
	widthEm := float64(imageWidth) / baseFontSizePx
	heightEm := float64(imageHeight) / baseFontSizePx

	// Apply properties from "image-inline" CSS style
	sc.registry.EnsureBaseStyle("image-inline")
	if def, ok := sc.registry.Get("image-inline"); ok {
		resolved := sc.registry.resolveInheritance(def)
		for k, v := range resolved.Properties {
			// Filter out properties that don't apply to inline images
			switch k {
			case SymTextIndent, SymTextAlignment, SymLineHeight,
				SymWidth, SymHeight, SymMaxWidth, SymMaxHeight, SymMinWidth:
				// Skip text-specific and dimension properties
				// Dimensions are calculated from actual image size, not CSS
				continue
			}
			props[k] = v
		}
	}

	// Add KFX-specific inline image properties
	props[SymBaselineStyle] = SymbolValue(SymCenter)       // baseline-style: center
	props[SymWidth] = DimensionValue(widthEm, SymUnitEm)   // width in em
	props[SymHeight] = DimensionValue(heightEm, SymUnitEm) // height in em

	return sc.registry.RegisterResolvedRaw(props)
}

// resolveBlockImage handles ImageBlock styling with position-based margin filtering.
func (sc StyleContext) resolveBlockImage(props map[KFXSymbol]any, imageWidth int, blockStyle string) string {
	widthPercent := ImageWidthPercent(imageWidth)
	isStandaloneBlock := strings.Contains(blockStyle, "image")
	isFullWidth := imageWidth >= int(KP3ContentWidthPx)

	// Track if block style has text-align: center - this should become box-align: center for images
	hasTextAlignCenter := false

	// Resolve the block style to get its properties
	if blockStyle != "" {
		for part := range strings.FieldsSeq(blockStyle) {
			sc.registry.EnsureBaseStyle(part)
			if def, ok := sc.registry.Get(part); ok {
				resolved := sc.registry.resolveInheritance(def)
				for k, v := range resolved.Properties {
					// Filter out properties that don't apply to block images
					switch k {
					case SymTextIndent, SymLineHeight,
						SymWidth, SymHeight, SymMaxWidth, SymMaxHeight, SymMinWidth:
						// Skip text-specific and dimension properties
						// text-indent is handled separately below
						// Dimensions are calculated from actual image size, not CSS
						continue
					case SymTextAlignment:
						// Track text-align: center - we'll convert this to box-align: center for images
						if v == SymCenter || v == SymbolValue(SymCenter) {
							hasTextAlignCenter = true
						}
						continue
					}
					props[k] = v
				}
			}
		}
	}

	// For standalone images, also include properties from "image-block" style
	// which corresponds to CSS "img.image-block" selector.
	if isStandaloneBlock {
		sc.registry.EnsureBaseStyle("image-block")
		if def, ok := sc.registry.Get("image-block"); ok {
			resolved := sc.registry.resolveInheritance(def)
			for k, v := range resolved.Properties {
				switch k {
				case SymTextIndent, SymTextAlignment, SymLineHeight,
					SymWidth, SymHeight, SymMaxWidth, SymMaxHeight, SymMinWidth:
					continue
				}
				props[k] = v
			}
		}
	}

	// Add image-specific properties
	props[SymWidth] = DimensionValue(widthPercent, SymUnitPercent)

	// Determine centering and margin-left behavior
	if isStandaloneBlock || hasTextAlignCenter {
		props[SymBoxAlign] = SymbolValue(SymCenter)
	} else {
		// For non-standalone images, apply margin-left from context
		// First try inherited margin-left from container (e.g., cite block)
		if ml, ok := sc.inherited[SymMarginLeft]; ok && ml != nil {
			props[SymMarginLeft] = ml
		}
		// If no margin-left, use text-indent for alignment (KP3 behavior for footnote images)
		if _, hasML := props[SymMarginLeft]; !hasML {
			if ti := sc.ResolveProperty("", blockStyle, SymTextIndent); ti != nil {
				props[SymMarginLeft] = ti
			}
		}
	}

	// Ensure line-height is present (KP3 requires it for block images)
	props[SymLineHeight] = DimensionValue(1, SymUnitLh)

	// Handle margins based on image type
	if isStandaloneBlock && isFullWidth {
		// Full-width standalone images: fixed 2.6lh margins (KP3 behavior)
		// NO position filtering - these always get the same margins
		props[SymMarginTop] = DimensionValue(2.6, SymUnitLh)
		props[SymMarginBottom] = DimensionValue(2.6, SymUnitLh)
	} else {
		// All other block images: apply position-based margins from container stack
		sc.applyContainerMargins(props)
	}

	return sc.registry.RegisterResolvedRaw(props)
}
