package kfx

// ptrFloat64 creates a pointer to a float64 value.
// Returns nil if the value is effectively zero (within epsilon).
func ptrFloat64(v float64) *float64 {
	const epsilon = 1e-9
	if v >= -epsilon && v <= epsilon {
		return nil // Treat near-zero as no margin
	}
	return &v
}

// marginValue returns the float64 value from a margin pointer, or 0 if nil.
func marginValue(m *float64) float64 {
	if m == nil {
		return 0
	}
	return *m
}
