package kfx

// AddEmptyLineMargin adds margin from an empty-line element to the pending margin.
// This margin will be set as the next element's margin-top when Resolve() is called.
// The margin value should be in line-height units (lh).
//
// This method modifies shared state, so all copies of this context within
// the same processing scope will see the changes.
func (sc StyleContext) AddEmptyLineMargin(margin float64) {
	// emptyLine is always initialized by NewStyleContext() and preserved across copies
	if sc.emptyLine == nil {
		return // Safety check - should never happen
	}
	// Replace (not accumulate) - empty-line margin overwrites the following
	// element's margin-top, matching KP3 reference behavior.
	sc.emptyLine.pendingMargin = margin
}

// ConsumePendingMargin returns and clears any accumulated empty-line margin.
// Returns 0 if no margin is pending.
func (sc StyleContext) ConsumePendingMargin() float64 {
	if sc.emptyLine == nil {
		return 0 // Safety check - should never happen
	}
	margin := sc.emptyLine.pendingMargin
	sc.emptyLine.pendingMargin = 0
	return margin
}

// HasPendingMargin returns true if there's accumulated empty-line margin to apply.
func (sc StyleContext) HasPendingMargin() bool {
	if sc.emptyLine == nil {
		return false // Safety check - should never happen
	}
	return sc.emptyLine.pendingMargin > 0
}

// emptyLineDefaultMarginLh is the default margin for empty-line elements in lh units.
// KP3 uses 0.5lh regardless of the CSS margin value specified for .emptyline.
// This was determined by analyzing KP3-generated KFX files where CSS has
// `.emptyline { margin: 1em; }` (which converts to ~0.8333lh) but the actual
// margin applied to following elements is always 0.5lh.
const emptyLineDefaultMarginLh = 0.5

// GetEmptyLineMargin returns the margin value for empty-line elements.
// KP3 uses a fixed value of 0.5lh for empty-line spacing, regardless of
// the CSS margin specified. This matches the reference KFX output.
//
// Note: The CSS `.emptyline { margin: 1em; }` is still used for EPUB rendering,
// but KFX uses the hardcoded value that matches KP3's behavior.
func (sc StyleContext) GetEmptyLineMargin(styleSpec string, registry *StyleRegistry) float64 {
	// KP3 uses a fixed 0.5lh margin for empty-lines, not the CSS value.
	// This was verified by comparing CSS input (margin: 1em → 0.8333lh)
	// with KP3 output (margin-top: 0.5lh on following elements).
	//
	// We still check if the style exists to ensure empty-line handling
	// is only applied when the emptyline class is actually defined.
	if _, ok := registry.Get(styleSpec); ok {
		return emptyLineDefaultMarginLh
	}
	return 0
}
