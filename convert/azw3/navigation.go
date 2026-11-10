package azw3

import (
	"context"
	"errors"
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"

	"fbc/common"
	"fbc/convert/publication"
	"fbc/convert/tocnav"
)

type navigationLayout struct {
	TOC   [][]byte
	Guide [][]byte
	Start *textPosition // Reassembled byte coordinate for the later EXTH StartOffset.
}

type navigationNode struct {
	Node       *tocnav.Node
	Parent     uint32
	Level      uint32
	FirstChild uint32
	LastChild  uint32
	Position   textPosition
	Length     uint32
}

// compileNavigation consumes frozen text positions without modifying the text or
// source navigation. KF8 stores siblings contiguously in breadth-first order;
// byte spans still describe reading-order subtrees rather than adjacent records.
func compileNavigation(
	ctx context.Context, pub *publication.Publication, text *textLayout, tocType common.TOCType,
) (*navigationLayout, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if pub == nil || text == nil || text.MainLength == 0 || len(text.Documents) == 0 {
		return nil, errors.New("publication and frozen text layout are required")
	}
	if tocType != common.TOCTypeNormal && tocType != common.TOCTypeOldKindle && tocType != common.TOCTypeFlat {
		return nil, fmt.Errorf("unsupported KF8 TOC hierarchy %d", tocType)
	}
	if uint64(len(pub.Navigation)) > math.MaxUint32 {
		return nil, errors.New("too many KF8 navigation entries")
	}
	var mainLength uint64
	for _, doc := range text.Documents {
		mainLength += uint64(len(doc.Data))
	}
	if mainLength != uint64(text.MainLength) {
		return nil, errors.New("frozen main-text length does not match its documents")
	}
	resolve := func(href string) (textPosition, error) {
		target, local, err := resolveTextTarget("nav.xhtml", href)
		if err != nil {
			return textPosition{}, err
		}
		if !local {
			return textPosition{}, fmt.Errorf("navigation destination %q is not local", href)
		}
		return resolveMainPosition(text, target)
	}

	roots := tocnav.Shape(pub.Navigation, tocType)
	var nodes []navigationNode
	for _, root := range roots {
		nodes = append(nodes, navigationNode{Node: root, Parent: missingRecord, FirstChild: missingRecord, LastChild: missingRecord})
	}
	for i := 0; i < len(nodes); i++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		node := nodes[i].Node
		position, err := resolve(node.Item.Href)
		if err != nil {
			return nil, fmt.Errorf("TOC entry %q: %w", node.Item.Title, err)
		}
		nodes[i].Position = position
		if len(node.Children) == 0 {
			continue
		}
		nodes[i].FirstChild = uint32(len(nodes))
		nodes[i].LastChild = uint32(len(nodes) + len(node.Children) - 1)
		level := nodes[i].Level + 1
		for _, child := range node.Children {
			nodes = append(nodes, navigationNode{
				Node: child, Parent: uint32(i), Level: level, FirstChild: missingRecord, LastChild: missingRecord,
			})
		}
	}
	if err := setNavigationSpans(ctx, nodes, len(roots), text.MainLength); err != nil {
		return nil, err
	}
	result := &navigationLayout{}
	if len(nodes) > 0 {
		labels := make([]string, len(nodes))
		for i, node := range nodes {
			labels[i] = node.Node.Item.Title
		}
		stringsData, offsets, err := encodeStrings(labels)
		if err != nil {
			return nil, fmt.Errorf("encode KF8 TOC labels: %w", err)
		}
		width := len(strconv.FormatUint(uint64(len(nodes)-1), 16))
		entries := make([]indexEntry, len(nodes))
		for i, node := range nodes {
			values := map[byte][]uint32{
				1: {node.Position.Position}, 2: {node.Length}, 3: {offsets[i]}, 4: {node.Level},
				6: {node.Position.FID, node.Position.Offset},
			}
			if node.Parent != missingRecord {
				values[21] = []uint32{node.Parent}
			}
			if node.FirstChild != missingRecord {
				values[22], values[23] = []uint32{node.FirstChild}, []uint32{node.LastChild}
			}
			entries[i] = indexEntry{Key: fmt.Sprintf("%0*X", width, i), Values: values}
		}
		result.TOC, err = encodeIndex([]indexTag{{1, 1}, {2, 1}, {3, 1}, {4, 1}, {21, 1}, {22, 1}, {23, 1}, {6, 2}},
			entries, uint32(len(stringsData)))
		if err != nil {
			return nil, fmt.Errorf("encode KF8 TOC index: %w", err)
		}
		result.TOC = append(result.TOC, stringsData...)
	}

	landmarks := slices.Clone(pub.Landmarks)
	slices.SortFunc(landmarks, func(a, b publication.Landmark) int {
		return strings.Compare(guideType(a.Kind), guideType(b.Kind))
	})
	var entries []indexEntry
	var labels []string
	for i, landmark := range landmarks {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		kind := guideType(landmark.Kind)
		if kind != "text" && kind != "toc" && kind != "cover" {
			return nil, fmt.Errorf("unsupported publication landmark %q", landmark.Kind)
		}
		if i > 0 && guideType(landmarks[i-1].Kind) == kind {
			return nil, fmt.Errorf("duplicate KF8 guide landmark %q", kind)
		}
		position, err := resolve(landmark.Href)
		if err != nil {
			return nil, fmt.Errorf("guide landmark %q: %w", kind, err)
		}
		if kind == "text" {
			result.Start = &position
		}
		entries = append(entries, indexEntry{Key: kind, Values: map[byte][]uint32{6: {position.FID, position.Offset}}})
		labels = append(labels, landmark.Title)
	}
	if len(entries) > 0 {
		stringsData, offsets, err := encodeStrings(labels)
		if err != nil {
			return nil, fmt.Errorf("encode KF8 guide labels: %w", err)
		}
		for i := range entries {
			entries[i].Values[1] = []uint32{offsets[i]}
		}
		result.Guide, err = encodeIndex([]indexTag{{1, 1}, {6, 2}}, entries, uint32(len(stringsData)))
		if err != nil {
			return nil, fmt.Errorf("encode KF8 guide index: %w", err)
		}
		result.Guide = append(result.Guide, stringsData...)
	}
	return result, nil
}

func guideType(kind string) string {
	if kind == "bodymatter" {
		return "text"
	}
	return kind
}

func setNavigationSpans(ctx context.Context, nodes []navigationNode, roots int, mainLength uint32) error {
	var pending, ancestors []uint32
	for i := roots - 1; i >= 0; i-- {
		pending = append(pending, uint32(i))
	}
	var previous uint32
	for len(pending) > 0 {
		if err := ctx.Err(); err != nil {
			return err
		}
		index := pending[len(pending)-1]
		pending = pending[:len(pending)-1]
		node := &nodes[index]
		if node.Position.Position < previous {
			return errors.New("KF8 navigation destinations are not in reading order")
		}
		previous = node.Position.Position
		for len(ancestors) > 0 {
			parent := &nodes[ancestors[len(ancestors)-1]]
			if parent.Level < node.Level {
				break
			}
			parent.Length = node.Position.Position - parent.Position.Position
			ancestors = ancestors[:len(ancestors)-1]
		}
		ancestors = append(ancestors, index)
		if node.FirstChild != missingRecord {
			for child := node.LastChild; ; child-- {
				pending = append(pending, child)
				if child == node.FirstChild {
					break
				}
			}
		}
	}
	for _, index := range ancestors {
		nodes[index].Length = mainLength - nodes[index].Position.Position
	}
	return nil
}
