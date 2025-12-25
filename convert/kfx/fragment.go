package kfx

import (
	"fmt"
	"sort"
)

// Fragment represents a single KFX fragment with type, id, and value.
type Fragment struct {
	FType int // Fragment type symbol ID (e.g., SymSection for $260)
	FID   int // Fragment ID symbol ID (equals FType for root fragments, or unique ID)
	Value any // Ion value (map[int]any for struct, []any for list, etc.)
}

// IsRoot returns true if this is a root fragment (fid == ftype).
func (f *Fragment) IsRoot() bool {
	return f.FID == f.FType
}

// IsSingleton returns true if this fragment type should only appear once.
func (f *Fragment) IsSingleton() bool {
	return ROOT_FRAGMENT_TYPES[f.FType]
}

// IsRaw returns true if this fragment's payload should be stored as raw bytes.
func (f *Fragment) IsRaw() bool {
	return RAW_FRAGMENT_TYPES[f.FType]
}

// Key returns the fragment key as (ftype, fid) pair.
func (f *Fragment) Key() FragmentKey {
	return FragmentKey{FType: f.FType, FID: f.FID}
}

// String returns a debug representation of the fragment.
func (f *Fragment) String() string {
	if f.IsRoot() {
		return fmt.Sprintf("Fragment(%s)", FormatSymbol(f.FType))
	}
	return fmt.Sprintf("Fragment(%s, id=%s)", FormatSymbol(f.FType), FormatSymbol(f.FID))
}

// FragmentKey uniquely identifies a fragment by type and id.
type FragmentKey struct {
	FType int
	FID   int
}

// String returns a debug representation of the key.
func (k FragmentKey) String() string {
	if k.FType == k.FID {
		return FormatSymbol(k.FType)
	}
	return fmt.Sprintf("%s:%s", FormatSymbol(k.FType), FormatSymbol(k.FID))
}

// FragmentList holds a collection of fragments with indexing.
type FragmentList struct {
	fragments []*Fragment
	byType    map[int][]*Fragment       // ftype -> list of fragments
	byKey     map[FragmentKey]*Fragment // (ftype, fid) -> fragment
}

// NewFragmentList creates an empty fragment list.
func NewFragmentList() *FragmentList {
	return &FragmentList{
		byType: make(map[int][]*Fragment),
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
func (fl *FragmentList) Get(ftype, fid int) *Fragment {
	return fl.byKey[FragmentKey{FType: ftype, FID: fid}]
}

// GetRoot returns a root fragment by type (where fid == ftype).
func (fl *FragmentList) GetRoot(ftype int) *Fragment {
	return fl.Get(ftype, ftype)
}

// GetByType returns all fragments of a given type.
func (fl *FragmentList) GetByType(ftype int) []*Fragment {
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
func (fl *FragmentList) Types() []int {
	types := make([]int, 0, len(fl.byType))
	for t := range fl.byType {
		types = append(types, t)
	}
	sort.Ints(types)
	return types
}

// Remove removes a fragment by key.
func (fl *FragmentList) Remove(ftype, fid int) bool {
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
func NewFragment(ftype, fid int, value any) *Fragment {
	return &Fragment{
		FType: ftype,
		FID:   fid,
		Value: value,
	}
}

// NewRootFragment creates a new root fragment (fid == ftype).
func NewRootFragment(ftype int, value any) *Fragment {
	return &Fragment{
		FType: ftype,
		FID:   ftype,
		Value: value,
	}
}

// StructValue is a helper type for building Ion struct values.
type StructValue map[int]any

// NewStruct creates a new empty struct value.
func NewStruct() StructValue {
	return make(StructValue)
}

// Set sets a field in the struct.
func (s StructValue) Set(field int, value any) StructValue {
	s[field] = value
	return s
}

// SetString sets a string field.
func (s StructValue) SetString(field int, value string) StructValue {
	return s.Set(field, value)
}

// SetInt sets an integer field.
func (s StructValue) SetInt(field int, value int64) StructValue {
	return s.Set(field, value)
}

// SetSymbol sets a symbol field.
func (s StructValue) SetSymbol(field int, symbolID int) StructValue {
	return s.Set(field, SymbolValue(symbolID))
}

// SetList sets a list field.
func (s StructValue) SetList(field int, items []any) StructValue {
	return s.Set(field, items)
}

// Get gets a field value.
func (s StructValue) Get(field int) any {
	return s[field]
}

// GetInt gets an integer field value.
func (s StructValue) GetInt(field int) (int64, bool) {
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
func (s StructValue) GetString(field int) (string, bool) {
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
func (l *ListValue) AddSymbol(symbolID int) *ListValue {
	return l.Add(SymbolValue(symbolID))
}

// AddString adds a string to the list.
func (l *ListValue) AddString(s string) *ListValue {
	return l.Add(s)
}

// SymbolValue represents a symbol value (as opposed to a string).
// When writing, this will be encoded as an Ion symbol, not a string.
type SymbolValue int

// RawValue represents raw bytes for $417/$418 fragments.
type RawValue []byte

// HasKey checks if a field exists in the struct.
func (s StructValue) HasKey(field int) bool {
	_, ok := s[field]
	return ok
}

// GetSymbol gets a symbol field value.
func (s StructValue) GetSymbol(field int) (SymbolValue, bool) {
	v, ok := s[field]
	if !ok {
		return 0, false
	}
	sym, ok := v.(SymbolValue)
	return sym, ok
}

// GetList gets a list field value.
func (s StructValue) GetList(field int) ([]any, bool) {
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
func (s StructValue) GetStruct(field int) (StructValue, bool) {
	v, ok := s[field]
	if !ok {
		return nil, false
	}
	switch val := v.(type) {
	case StructValue:
		return val, true
	case map[int]any:
		return StructValue(val), true
	default:
		return nil, false
	}
}

// SetStruct sets a nested struct field.
func (s StructValue) SetStruct(field int, value StructValue) StructValue {
	return s.Set(field, value)
}

// Delete removes a field from the struct.
func (s StructValue) Delete(field int) StructValue {
	delete(s, field)
	return s
}

// Keys returns all field IDs in the struct.
func (s StructValue) Keys() []int {
	keys := make([]int, 0, len(s))
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
