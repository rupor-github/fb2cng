package kfx

// AddEmptyLineMargin adds margin from an empty-line element to the pending margin.
// This margin will be set as the next element's margin-top when Resolve() is called.
// The margin value should be in line-height units (lh).
//
// Additionally, this sets keepMarginBottom so the next element preserves its
// margin-bottom even if position filtering would normally remove it.
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
	sc.emptyLine.keepMarginBottom = true
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

// ConsumeKeepMarginBottom returns the keepMarginBottom flag without clearing it.
// Returns true if the current element should keep its margin-bottom.
//
// Once an empty-line is encountered, ALL subsequent elements in the block
// keep their margin-bottom (not just the immediate next element). The flag
// stays true until the block processing completes.
func (sc StyleContext) ConsumeKeepMarginBottom() bool {
	if sc.emptyLine == nil {
		return false // Safety check - should never happen
	}
	// Don't clear the flag - once set by empty-line, all subsequent
	// elements in the block should keep their margin-bottom
	return sc.emptyLine.keepMarginBottom
}

// HasPendingMargin returns true if there's accumulated empty-line margin to apply.
func (sc StyleContext) HasPendingMargin() bool {
	if sc.emptyLine == nil {
		return false // Safety check - should never happen
	}
	return sc.emptyLine.pendingMargin > 0
}

// GetEmptyLineMargin extracts the margin-top value from the emptyline style.
// This returns only the emptyline's own margin, without any inherited context.
// Returns the margin value in lh units, or 0 if not found.
func (sc StyleContext) GetEmptyLineMargin(styleSpec string, registry *StyleRegistry) float64 {
	// Look up the emptyline style directly without context inheritance
	if def, ok := registry.Get(styleSpec); ok {
		resolved := registry.resolveInheritance(def)
		if mt, ok := resolved.Properties[SymMarginTop]; ok {
			if val, unit, ok := measureParts(mt); ok && unit == SymUnitLh {
				return val
			}
		}
	}
	return 0
}
