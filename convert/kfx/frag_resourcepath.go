package kfx

// BuildResourcePath creates the $395 resource_path root fragment.
// KFXInput expects this fragment to exist (often with an empty entries list).
func BuildResourcePath() *Fragment {
	return NewRootFragment(SymResourcePath, NewStruct().SetList(SymEntries, []any{}))
}
