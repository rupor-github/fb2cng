package text

import (
	"io"
	"sort"
	"strings"
	"unicode/utf8"
)

// A trie uses runes rather than characters for indexing, therefore its child
// key values are integers.
type trie struct {
	leaf     bool           // whether the node is a leaf (the end of an input string).
	value    any            // the value associated with the string up to this leaf node.
	children map[rune]*trie // a map of sub-tries for each child rune value.
}

// newTrie creates and returns a new Trie instance.
func newTrie() *trie {
	t := new(trie)
	t.leaf = false
	t.value = nil
	t.children = make(map[rune]*trie)
	return t
}

// Internal function: adds items to the trie, reading runes from a
// strings.Reader.  It returns the leaf node at which the addition ends.
func (p *trie) addRunes(r io.RuneReader) *trie {
	sym, _, err := r.ReadRune()
	if err != nil {
		p.leaf = true
		return p
	}

	n := p.children[sym]
	if n == nil {
		n = newTrie()
		p.children[sym] = n
	}

	// recurse to store sub-runes below the new node
	return n.addRunes(r)
}

// addString adds a string to the trie. If the string is already present, no
// additional storage happens. Yay!
func (p *trie) addString(s string) {
	if len(s) == 0 {
		return
	}

	// append the runes to the trie -- we're ignoring the value in this invocation
	p.addRunes(strings.NewReader(s))
}

// addValue adds a string to the trie, with an associated value.  If the string
// is already present, only the value is updated.
func (p *trie) addValue(s string, v any) {
	if len(s) == 0 {
		return
	}

	// append the runes to the trie
	leaf := p.addRunes(strings.NewReader(s))
	leaf.value = v
}

// Internal string removal function. Returns true if this node is empty
// following the removal.
func (p *trie) removeRunes(r io.RuneReader) bool {
	sym, _, err := r.ReadRune()
	if err != nil {
		// remove value, remove leaf flag
		p.value = nil
		p.leaf = false
		return len(p.children) == 0
	}

	child, ok := p.children[sym]
	if ok && child.removeRunes(r) {
		// the child is now empty following the removal, so prune it
		delete(p.children, sym)
	}

	return len(p.children) == 0
}

// remove a string from the trie. Returns true if the Trie is now empty.
func (p *trie) remove(s string) bool {
	if len(s) == 0 {
		return len(p.children) == 0
	}

	// remove the runes, returning the final result
	return p.removeRunes(strings.NewReader(s))
}

// Internal string inclusion function.
func (p *trie) includes(r io.RuneReader) *trie {
	rune, _, err := r.ReadRune()
	if err != nil {
		if p.leaf {
			return p
		}
		return nil
	}

	child, ok := p.children[rune]
	if !ok {
		return nil // no node for this rune was in the trie
	}

	// recurse down to the next node with the remainder of the string
	return child.includes(r)
}

// contains tests for the inclusion of a particular string in the Trie.
func (p *trie) contains(s string) bool {
	if len(s) == 0 {
		return false // empty strings can't be included (how could we add them?)
	}
	return p.includes(strings.NewReader(s)) != nil
}

// getValue returns the value associated with the given string.  Double return:
// false if the given string was not present, true if the string was present.
// The value could be both valid and nil.
func (p *trie) getValue(s string) (any, bool) {
	if len(s) == 0 {
		return nil, false
	}

	leaf := p.includes(strings.NewReader(s))
	if leaf == nil {
		return nil, false
	}
	return leaf.value, true
}

// Internal output-building function used by Members()
func (p *trie) buildMembers(prefix string) []string {

	strList := []string{}
	if p.leaf {
		strList = append(strList, prefix)
	}

	// for each child, go grab all suffixes
	for sym, child := range p.children {
		buf := make([]byte, 4)
		numChars := utf8.EncodeRune(buf, sym)
		strList = append(strList, child.buildMembers(prefix+string(buf[0:numChars]))...)
	}
	return strList
}

// members retrieves all member strings, in order.
func (p *trie) members() (members []string) {
	members = p.buildMembers(``)
	sort.Strings(members)
	return
}

// size counts all the nodes of the entire Trie, NOT including the root node.
func (p *trie) size() (sz int) {
	sz = len(p.children)

	for _, child := range p.children {
		sz += child.size()
	}

	return
}

// allSubstrings returns all anchored substrings of the given string within the
// Trie.
func (p *trie) allSubstrings(s string) []string {

	v := []string{}

	for pos, rune := range s {
		child, ok := p.children[rune]
		if !ok {
			// return whatever we have so far
			break
		}

		// if this is a leaf node, add the string so far to the output vector
		if child.leaf {
			v = append(v, s[0:pos+utf8.RuneLen(rune)])
		}
		p = child
	}
	return v
}

// allSubstringsAndValues returns all anchored substrings of the given string
// within the Trie, with a matching set of their associated values.
func (p *trie) allSubstringsAndValues(s string) ([]string, []any) {

	sv := []string{}
	vv := []any{}

	for pos, rune := range s {
		child, ok := p.children[rune]
		if !ok {
			// return whatever we have so far
			break
		}

		// if this is a leaf node, add the string so far and its value
		if child.leaf {
			sv = append(sv, s[0:pos+utf8.RuneLen(rune)])
			vv = append(vv, child.value)
		}
		p = child
	}
	return sv, vv
}
