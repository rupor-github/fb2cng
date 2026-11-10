package metainfo

import (
	"fmt"
	"strconv"

	"github.com/beevik/etree"

	"fbc/fb2"
)

// AddSequences writes EPUB3 collection metadata for every sequence, preserving
// source order and the existing series/set distinction for nested sequences.
func AddSequences(metadata *etree.Element, sequences []fb2.Sequence) {
	counter := 1
	for i := range sequences {
		counter = addSequence(metadata, &sequences[i], counter, 1)
	}
}

func addSequence(metadata *etree.Element, seq *fb2.Sequence, idCounter, level int) int {
	collectionID := fmt.Sprintf("collection%d", idCounter)
	idCounter++
	meta := metadata.CreateElement("meta")
	meta.CreateAttr("property", "belongs-to-collection")
	meta.CreateAttr("id", collectionID)
	meta.SetText(seq.Name)
	collType := "series"
	if level > 1 {
		collType = "set"
	}
	meta = metadata.CreateElement("meta")
	meta.CreateAttr("refines", "#"+collectionID)
	meta.CreateAttr("property", "collection-type")
	meta.SetText(collType)
	if seq.Number != nil {
		meta = metadata.CreateElement("meta")
		meta.CreateAttr("refines", "#"+collectionID)
		meta.CreateAttr("property", "group-position")
		meta.SetText(strconv.Itoa(*seq.Number))
	}
	for i := range seq.Children {
		idCounter = addSequence(metadata, &seq.Children[i], idCounter, level+1)
	}
	return idCounter
}
