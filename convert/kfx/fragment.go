package kfx

import (
	"fmt"
	"sort"
)

// Fragment represents a single KFX fragment with type, id, and value.
//
// Fragment Naming Conventions:
//
// FIDName is used to identify fragments and gets resolved to local symbol IDs during
// serialization. Different fragment types use different naming patterns optimized for
// their purpose:
//
//   - Resources: Base36-encoded (e.g., "resource/rsrcA", "e1G") - matches Amazon KFX format
//   - Content: Descriptive format "content_{N}" or "content_{N}_{M}" - human-readable for debugging
//   - Storyline: Simple format "l{N}" (e.g., l1, l2) - readable sequential identifiers
//   - Section: Simple format "c{N}" (e.g., c0, c1) - readable sequential identifiers
//   - Styles: Semantic names from stylesheet (e.g., "body", "emphasis") - preserves meaning
//   - Anchors: Original document IDs (e.g., "note123") - maintains link integrity
//
// This mixed approach balances compatibility (base36 for resources), debuggability
// (human-readable names), and semantic preservation (original IDs/style names).
type Fragment struct {
	FType   KFXSymbol // Fragment type symbol ID (e.g., SymSection for $260)
	FID     KFXSymbol // Fragment ID symbol ID (equals FType for root fragments, or resolved from FIDName)
	FIDName string    // Fragment ID name for non-root fragments (see naming conventions above)
	Value   any       // Ion value (map[KFXSymbol]any for struct, []any for list, etc.)
}

// IsRoot returns true if this is a root fragment (fid == ftype).
func (f *Fragment) IsRoot() bool {
	return f.FIDName == "" && f.FID == f.FType
}

// IsSingleton returns true if this fragment type should only appear once.
func (f *Fragment) IsSingleton() bool {
	return ROOT_FRAGMENT_TYPES[f.FType]
}

// IsRaw returns true if this fragment's payload should be stored as raw bytes.
func (f *Fragment) IsRaw() bool {
	return RAW_FRAGMENT_TYPES[f.FType]
}

// Key returns the fragment key as (ftype, fid/fidname) pair.
func (f *Fragment) Key() FragmentKey {
	return FragmentKey{FType: f.FType, FID: f.FID, FIDName: f.FIDName}
}

// String returns a debug representation of the fragment.
func (f *Fragment) String() string {
	if f.IsRoot() {
		return fmt.Sprintf("Fragment(%s)", f.FType)
	}
	if f.FIDName != "" {
		return fmt.Sprintf("Fragment(%s, id=%s)", f.FType, f.FIDName)
	}
	return fmt.Sprintf("Fragment(%s, id=%s)", f.FType, f.FID)
}

// FragmentKey uniquely identifies a fragment by type and id.
type FragmentKey struct {
	FType   KFXSymbol
	FID     KFXSymbol
	FIDName string
}

// String returns a debug representation of the key.
func (k FragmentKey) String() string {
	if k.FIDName != "" {
		return fmt.Sprintf("%s:%s", k.FType, k.FIDName)
	}
	if k.FType == k.FID {
		return k.FType.String()
	}
	return fmt.Sprintf("%s:%s", k.FType, k.FID)
}

// FragmentList holds a collection of fragments with indexing.
type FragmentList struct {
	fragments []*Fragment
	byType    map[KFXSymbol][]*Fragment // ftype -> list of fragments
	byKey     map[FragmentKey]*Fragment // (ftype, fid) -> fragment
}

// NewFragmentList creates an empty fragment list.
func NewFragmentList() *FragmentList {
	return &FragmentList{
		byType: make(map[KFXSymbol][]*Fragment),
		byKey:  make(map[FragmentKey]*Fragment),
	}
}

// Add adds a fragment to the list.
func (fl *FragmentList) Add(f *Fragment) error {
	key := f.Key()
	if existing, ok := fl.byKey[key]; ok {
		return fmt.Errorf("duplicate fragment key %s (existing: %s)", key, existing)
	}
	fl.fragments = append(fl.fragments, f)
	fl.byType[f.FType] = append(fl.byType[f.FType], f)
	fl.byKey[key] = f
	return nil
}

// Get returns a fragment by type and id.
func (fl *FragmentList) Get(ftype, fid KFXSymbol) *Fragment {
	return fl.byKey[FragmentKey{FType: ftype, FID: fid}]
}

// GetRoot returns a root fragment by type (where fid == ftype).
func (fl *FragmentList) GetRoot(ftype KFXSymbol) *Fragment {
	return fl.Get(ftype, ftype)
}

// GetByType returns all fragments of a given type.
func (fl *FragmentList) GetByType(ftype KFXSymbol) []*Fragment {
	return fl.byType[ftype]
}

// All returns all fragments in insertion order.
func (fl *FragmentList) All() []*Fragment {
	return fl.fragments
}

// Len returns the number of fragments.
func (fl *FragmentList) Len() int {
	return len(fl.fragments)
}

// Types returns all unique fragment types present.
func (fl *FragmentList) Types() []KFXSymbol {
	types := make([]KFXSymbol, 0, len(fl.byType))
	for t := range fl.byType {
		types = append(types, t)
	}
	sort.Slice(types, func(i, j int) bool { return types[i] < types[j] })
	return types
}

// Remove removes a fragment by key.
func (fl *FragmentList) Remove(ftype, fid KFXSymbol) bool {
	key := FragmentKey{FType: ftype, FID: fid}
	f, ok := fl.byKey[key]
	if !ok {
		return false
	}
	delete(fl.byKey, key)

	// Remove from byType
	typeList := fl.byType[ftype]
	for i, frag := range typeList {
		if frag == f {
			fl.byType[ftype] = append(typeList[:i], typeList[i+1:]...)
			break
		}
	}

	// Remove from main list
	for i, frag := range fl.fragments {
		if frag == f {
			fl.fragments = append(fl.fragments[:i], fl.fragments[i+1:]...)
			break
		}
	}
	return true
}

// Clone creates a deep copy of the fragment list structure (not values).
func (fl *FragmentList) Clone() *FragmentList {
	newList := NewFragmentList()
	for _, f := range fl.fragments {
		newFrag := &Fragment{
			FType: f.FType,
			FID:   f.FID,
			Value: f.Value, // Shallow copy of value
		}
		_ = newList.Add(newFrag)
	}
	return newList
}

// SortedByType returns fragments sorted by type, then by id.
func (fl *FragmentList) SortedByType() []*Fragment {
	sorted := make([]*Fragment, len(fl.fragments))
	copy(sorted, fl.fragments)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].FType != sorted[j].FType {
			return sorted[i].FType < sorted[j].FType
		}
		return sorted[i].FID < sorted[j].FID
	})
	return sorted
}

// NewFragment creates a new fragment with the given type, id, and value.
func NewFragment(ftype, fid KFXSymbol, value any) *Fragment {
	return &Fragment{
		FType: ftype,
		FID:   fid,
		Value: value,
	}
}

// NewRootFragment creates a new root fragment (fid == ftype).
func NewRootFragment(ftype KFXSymbol, value any) *Fragment {
	return &Fragment{
		FType: ftype,
		FID:   ftype,
		Value: value,
	}
}

// StructValue is a helper type for building Ion struct values.
type StructValue map[KFXSymbol]any

// NewStruct creates a new empty struct value.
func NewStruct() StructValue {
	return make(StructValue)
}

// Set sets a field in the struct.
func (s StructValue) Set(field KFXSymbol, value any) StructValue {
	s[field] = value
	return s
}

// SetString sets a string field.
func (s StructValue) SetString(field KFXSymbol, value string) StructValue {
	return s.Set(field, value)
}

// SetInt sets an integer field.
func (s StructValue) SetInt(field KFXSymbol, value int64) StructValue {
	return s.Set(field, value)
}

// SetSymbol sets a symbol field.
func (s StructValue) SetSymbol(field KFXSymbol, symbolID KFXSymbol) StructValue {
	return s.Set(field, SymbolValue(symbolID))
}

// SetList sets a list field.
func (s StructValue) SetList(field KFXSymbol, items []any) StructValue {
	return s.Set(field, items)
}

// SetFloat sets a float64 field.
func (s StructValue) SetFloat(field KFXSymbol, value float64) StructValue {
	return s.Set(field, value)
}

// SetStruct sets a struct field.
func (s StructValue) SetStruct(field KFXSymbol, value StructValue) StructValue {
	return s.Set(field, value)
}

// Get gets a field value.
func (s StructValue) Get(field KFXSymbol) any {
	return s[field]
}

// GetInt gets an integer field value.
func (s StructValue) GetInt(field KFXSymbol) (int64, bool) {
	v, ok := s[field]
	if !ok {
		return 0, false
	}
	switch val := v.(type) {
	case int64:
		return val, true
	case int:
		return int64(val), true
	default:
		return 0, false
	}
}

// GetString gets a string field value.
func (s StructValue) GetString(field KFXSymbol) (string, bool) {
	v, ok := s[field]
	if !ok {
		return "", false
	}
	str, ok := v.(string)
	return str, ok
}

// ListValue is a helper type for building Ion list values.
type ListValue []any

// NewList creates a new empty list value.
func NewList() ListValue {
	return make(ListValue, 0)
}

// Add adds an item to the list.
func (l *ListValue) Add(item any) *ListValue {
	*l = append(*l, item)
	return l
}

// AddSymbol adds a symbol to the list.
func (l *ListValue) AddSymbol(symbolID KFXSymbol) *ListValue {
	return l.Add(SymbolValue(symbolID))
}

// AddString adds a string to the list.
func (l *ListValue) AddString(s string) *ListValue {
	return l.Add(s)
}

// SymbolValue represents a symbol value (as opposed to a string).
// When writing, this will be encoded as an Ion symbol, not a string.
type SymbolValue KFXSymbol

// SymbolByNameValue represents a symbol value by its string name.
// When writing, the name is resolved to a symbol ID using the local symbol table.
type SymbolByNameValue string

// SymbolByName creates a symbol value by name for later resolution.
func SymbolByName(name string) SymbolByNameValue {
	return SymbolByNameValue(name)
}

// RawValue represents raw bytes for $417/$418 fragments.
type RawValue []byte

// HasKey checks if a field exists in the struct.
func (s StructValue) HasKey(field KFXSymbol) bool {
	_, ok := s[field]
	return ok
}

// GetSymbol gets a symbol field value.
func (s StructValue) GetSymbol(field KFXSymbol) (SymbolValue, bool) {
	v, ok := s[field]
	if !ok {
		return 0, false
	}
	sym, ok := v.(SymbolValue)
	return sym, ok
}

// GetList gets a list field value.
func (s StructValue) GetList(field KFXSymbol) ([]any, bool) {
	v, ok := s[field]
	if !ok {
		return nil, false
	}
	switch val := v.(type) {
	case []any:
		return val, true
	case ListValue:
		return []any(val), true
	default:
		return nil, false
	}
}

// GetStruct gets a nested struct field value.
func (s StructValue) GetStruct(field KFXSymbol) (StructValue, bool) {
	v, ok := s[field]
	if !ok {
		return nil, false
	}
	switch val := v.(type) {
	case StructValue:
		return val, true
	case map[KFXSymbol]any:
		return StructValue(val), true
	default:
		return nil, false
	}
}

// Delete removes a field from the struct.
func (s StructValue) Delete(field KFXSymbol) StructValue {
	delete(s, field)
	return s
}

// Keys returns all field IDs in the struct.
func (s StructValue) Keys() []KFXSymbol {
	keys := make([]KFXSymbol, 0, len(s))
	for k := range s {
		keys = append(keys, k)
	}
	return keys
}

// Len returns the number of items in the list.
func (l ListValue) Len() int {
	return len(l)
}

// Get returns item at index.
func (l ListValue) Get(i int) any {
	if i < 0 || i >= len(l) {
		return nil
	}
	return l[i]
}

// ToSlice converts ListValue to []any.
func (l ListValue) ToSlice() []any {
	return []any(l)
}
