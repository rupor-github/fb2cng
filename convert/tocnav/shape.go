package tocnav

import (
	"math"

	"fbc/common"
)

// Item is a flat navigation item with an effective source nesting level.
// Level values are compared relative to each other; lower numbers are closer
// to the root. Title/Href are intentionally minimal so EPUB and KFX builders can
// use the same shaping logic without depending on each other's data types.
type Item struct {
	ID    string
	Title string
	Href  string
	Level int
	EID   int
}

// Node is a shaped navigation item with nested children.
type Node struct {
	Item     Item
	Children []*Node
}

type stackItem struct {
	level int
	node  *Node
}

// Shape returns a navigation tree for the requested TOC hierarchy type.
//
// This mirrors fb2converter's NCX shaping behavior:
//   - normal preserves ordinary section nesting, but keeps the first/book title
//     at the same top level as first-level chapters.
//   - old_kindle allows only shallow nesting for old Kindle/kindlegen readers.
//   - flat promotes entries to top-level siblings.
func Shape(items []Item, tocType common.TOCType) []*Node {
	if len(items) == 0 {
		return nil
	}

	levelLimit, depthBarrier := shapeLimits(tocType)

	var (
		roots   []*Node
		history []stackItem
		prev    *Item
	)

	addNode := func(parent *Node, item Item) *Node {
		node := &Node{Item: item}
		if parent == nil {
			roots = append(roots, node)
		} else {
			parent.Children = append(parent.Children, node)
		}
		return node
	}

	peek := func() (stackItem, bool) {
		if len(history) == 0 {
			return stackItem{}, false
		}
		return history[len(history)-1], true
	}

	pop := func() {
		if len(history) == 0 {
			return
		}
		history[len(history)-1] = stackItem{}
		history = history[:len(history)-1]
	}

	push := func(level int, node *Node) {
		history = append(history, stackItem{level: level, node: node})
	}

	for i := range items {
		item := items[i]
		if prev == nil {
			push(item.Level, addNode(nil, item))
			prev = &item
			continue
		}

		switch {
		case prev.Level < item.Level: // going in
			if item.Level < levelLimit || len(history) > depthBarrier {
				pop()
			}
		case prev.Level == item.Level: // same level
			pop()
		case prev.Level > item.Level: // going out
			for top, ok := peek(); ok && top.level >= item.Level; top, ok = peek() {
				pop()
			}
		}

		var parent *Node
		if top, ok := peek(); ok {
			parent = top.node
		}
		push(item.Level, addNode(parent, item))
		prev = &item
	}

	return roots
}

func shapeLimits(tocType common.TOCType) (levelLimit int, depthBarrier int) {
	const oldKindleMaxLevel = 2

	switch tocType {
	case common.TOCTypeFlat:
		return math.MaxInt, 1
	case common.TOCTypeOldKindle:
		return oldKindleMaxLevel, 1
	default:
		return oldKindleMaxLevel, math.MaxInt
	}
}
