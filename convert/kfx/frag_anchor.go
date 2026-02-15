package kfx

import (
	"sync"
)

type eidByFB2ID map[string]int

// buildAnchorFragments generates $266 anchor fragments for internal navigation.
// Fragment naming uses the actual ID from the source document (e.g., section IDs, note IDs).
// For TOC page links, anchor IDs may match section names (c0, c1, etc.) - this is allowed
// since anchor fragments ($266) use a different fragment type than section fragments ($260).
func buildAnchorFragments(idToEID eidByFB2ID, referenced map[string]bool) []*Fragment {
	var out []*Fragment
	if len(referenced) == 0 || len(idToEID) == 0 {
		return out
	}

	for id := range referenced {
		if id == "" {
			continue
		}
		eid, ok := idToEID[id]
		if !ok || eid == 0 {
			continue
		}
		pos := NewStruct().SetInt(SymUniqueID, int64(eid))
		out = append(out, &Fragment{
			FType:   SymAnchor,
			FIDName: id,
			Value: NewStruct().
				Set(SymAnchorName, SymbolByName(id)).
				SetStruct(SymPosition, pos),
		})
	}

	return out
}

// ExternalLinkRegistry tracks external URLs and maps them to anchor IDs.
// This allows multiple references to the same URL to share a single anchor.
type ExternalLinkRegistry struct {
	mu      sync.Mutex
	urlToID map[string]string // URL -> anchor ID
	counter int               // Counter for generating unique anchor IDs
}

// NewExternalLinkRegistry creates a new external link registry.
func NewExternalLinkRegistry() *ExternalLinkRegistry {
	return &ExternalLinkRegistry{
		urlToID: make(map[string]string),
		// Start counter at a high value to avoid collision with other anchors
		// Using base36 naming like "aEXT0", "aEXT1", etc.
		counter: 0,
	}
}

// Register registers an external URL and returns its anchor ID.
// If the URL was already registered, returns the existing anchor ID.
func (r *ExternalLinkRegistry) Register(url string) string {
	if url == "" {
		return ""
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if id, ok := r.urlToID[url]; ok {
		return id
	}

	// Generate a unique anchor ID using "aEXT" prefix + base36 counter
	// This ensures no collision with internal anchors which use FB2 IDs
	id := "aEXT" + toBase36(r.counter)
	r.counter++
	r.urlToID[url] = id
	return id
}

// BuildFragments creates anchor fragments for all registered external URLs.
// External link anchors have $180 (anchor_name) and $186 (uri) but NO position.
func (r *ExternalLinkRegistry) BuildFragments() []*Fragment {
	r.mu.Lock()
	defer r.mu.Unlock()

	if len(r.urlToID) == 0 {
		return nil
	}

	out := make([]*Fragment, 0, len(r.urlToID))
	for url, id := range r.urlToID {
		out = append(out, &Fragment{
			FType:   SymAnchor,
			FIDName: id,
			Value: NewStruct().
				Set(SymAnchorName, SymbolByName(id)).
				SetString(SymURI, url),
		})
	}
	return out
}
