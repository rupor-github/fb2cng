package model

// Fragment represents a single KFX fragment (aka entity payload) before it is
// packed into a CONT container.
//
// In KFXInput, fragment identity is determined by the entity table ID/type
// numbers, not by Ion annotations inside the payload.
type Fragment struct {
	FID   string
	FType string
	Value any
}
