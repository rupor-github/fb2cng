package builders

import "fbc/convert/kfx"

// Builder produces one or more fragments.
//
// Builders are intended to be small and composable: one builder per fragment
// type ($538, $490, ...), and optional builders for auxiliary types ($164/$417,
// $259/$260/$145, etc.).
type Builder interface {
	Build() ([]kfx.Fragment, error)
}
