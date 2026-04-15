package margins

// PtrFloat64 creates a pointer to a float64 value.
// Returns nil if the value is effectively zero (within epsilon).
func PtrFloat64(v float64) *float64 {
	const epsilon = 1e-9
	if v >= -epsilon && v <= epsilon {
		return nil // Treat near-zero as no margin
	}
	return &v
}

// MarginValue returns the float64 value from a margin pointer, or 0 if nil.
func MarginValue(m *float64) float64 {
	if m == nil {
		return 0
	}
	return *m
}

// MarginsEqual compares two margin pointers for equality.
// Two nil pointers are equal, and two non-nil pointers are equal if their values are equal.
func MarginsEqual(a, b *float64) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	// Use epsilon comparison for floating point
	const epsilon = 1e-9
	diff := *a - *b
	return diff >= -epsilon && diff <= epsilon
}
