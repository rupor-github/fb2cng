package kfx

import "fbc/convert/margins"

func ptrFloat64(v float64) *float64 { return margins.PtrFloat64(v) }

func marginsEqual(a, b *float64) bool { return margins.MarginsEqual(a, b) }
