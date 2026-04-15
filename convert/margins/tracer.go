package margins

// Tracer is an optional interface for tracing margin collapse operations.
// Implementations can log or record each collapse step for debugging.
// When nil or disabled, no tracing overhead is incurred.
type Tracer interface {
	// IsEnabled returns true if tracing is active.
	IsEnabled() bool

	// TraceMarginCollapse logs a single margin collapse operation.
	//
	// Parameters:
	//   collapseType  — phase name ("empty", "first-child", "last-child", "sibling", etc.)
	//   nodeID        — human-readable identifier for the affected node
	//   beforeMT/MB   — margin-top/bottom BEFORE the operation (nil = unset)
	//   afterMT/MB    — margin-top/bottom AFTER the operation (nil = unset)
	//   containerInfo — description of the enclosing container
	TraceMarginCollapse(collapseType, nodeID string, beforeMT, beforeMB, afterMT, afterMB *float64, containerInfo string)
}
