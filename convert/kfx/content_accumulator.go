package kfx

import "fmt"

// MaxContentFragmentSize is the maximum size in bytes for a content fragment's content_list.
// Some consumers validate that content fragments don't exceed 8192 bytes.
// This is separate from the container's ChunkSize ($412) which is used for streaming/compression.
const MaxContentFragmentSize = 8192

// ContentAccumulator manages content fragments with automatic chunking.
// Each paragraph/text entry is a separate item in content_list.
// When accumulated size exceeds MaxContentFragmentSize, a new fragment is created.
// ContentAccumulator accumulates paragraph content into named content fragments,
// automatically splitting into chunks when size limits are exceeded.
//
// Fragment naming pattern: "content_{N}" where N is sequential (e.g., content_1, content_2, content_3).
// This human-readable format is used instead of base36 for better debuggability.
type ContentAccumulator struct {
	counter     int                 // Global counter for sequential naming
	currentName string              // Current content fragment name
	currentList []string            // Current content list (each entry is one paragraph)
	currentSize int                 // Current accumulated size in bytes
	fragments   map[string][]string // All completed content fragments
}

// NewContentAccumulator creates a new content accumulator.
// Fragment names follow pattern "content_{N}" with sequential numbering.
func NewContentAccumulator(startCounter int) *ContentAccumulator {
	name := fmt.Sprintf("content_%d", startCounter)
	return &ContentAccumulator{
		counter:     startCounter,
		currentName: name,
		currentList: make([]string, 0),
		currentSize: 0,
		fragments:   make(map[string][]string),
	}
}

// Add adds a paragraph/text entry to the accumulator.
// Each call adds one entry to content_list. Creates new chunk if size limit exceeded.
// Returns the content name and offset for the added text.
func (ca *ContentAccumulator) Add(text string) (name string, offset int) {
	textSize := len(text)

	// Check if we need to start a new chunk
	// Start new chunk if current is non-empty and adding this would exceed limit
	if ca.currentSize > 0 && ca.currentSize+textSize > MaxContentFragmentSize {
		ca.finishCurrentChunk()
	}

	name = ca.currentName
	offset = len(ca.currentList)
	ca.currentList = append(ca.currentList, text)
	ca.currentSize += textSize

	return name, offset
}

// finishCurrentChunk saves the current chunk and starts a new one with sequential naming.
func (ca *ContentAccumulator) finishCurrentChunk() {
	if len(ca.currentList) > 0 {
		ca.fragments[ca.currentName] = ca.currentList
	}

	ca.counter++
	ca.currentName = fmt.Sprintf("content_%d", ca.counter)
	ca.currentList = make([]string, 0)
	ca.currentSize = 0
}

// Finish completes accumulation and returns all content fragments.
func (ca *ContentAccumulator) Finish() map[string][]string {
	// Save current chunk if it has content
	if len(ca.currentList) > 0 {
		ca.fragments[ca.currentName] = ca.currentList
	}
	return ca.fragments
}

// buildContentFragmentByName creates a content ($145) fragment with string name.
// The name parameter comes from ContentAccumulator and follows the pattern "content_{N}"
// with sequential numbering. This human-readable naming convention
// is maintained throughout the conversion for easier debugging and inspection.
func buildContentFragmentByName(name string, contentList []string) *Fragment {
	// Use string-keyed map for content with local symbol names
	// The "name" field value should be a symbol, not a string
	content := map[string]any{
		"$146": anySlice(contentList), // content_list
		"name": SymbolByName(name),    // name as symbol value
	}

	return &Fragment{
		FType:   SymContent,
		FIDName: name,
		Value:   content,
	}
}

// anySlice converts []string to []any
func anySlice(s []string) []any {
	result := make([]any, len(s))
	for i, v := range s {
		result[i] = v
	}
	return result
}
