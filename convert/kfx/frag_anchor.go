package kfx

import (
	"sync"
)

// anchorTarget holds the EID and optional character offset for an anchor position.
// The Offset field is used for backlink anchors to tell the Kindle viewer exactly
// where within a content entry the footnote reference appears ($143 in KFX).
type anchorTarget struct {
	EID    int
	Offset int // character offset (runes) within the content entry; 0 = start
}

type eidByFB2ID map[string]anchorTarget

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
		target, ok := idToEID[id]
		if !ok || target.EID == 0 {
			continue
		}
		pos := NewStruct().SetInt(SymUniqueID, int64(target.EID))
		if target.Offset > 0 {
			pos.SetInt(SymOffset, int64(target.Offset))
		}
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
