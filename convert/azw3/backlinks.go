package azw3

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/beevik/etree"

	"fbc/content"
	"fbc/convert/publication"
)

// compileBacklinks resolves labels against their reference occurrences in the
// final KF8 main flow. Label lengths can move later references (including cycles
// within notes), so recompile until both text and its coordinates agree. Never
// patch a frozen layout or rerender Content, which would allocate new references.
func compileBacklinks(
	ctx context.Context, c *content.Content, pub *publication.Publication,
) (*publication.Publication, *textLayout, error) {
	if c == nil || pub == nil {
		return nil, nil, errors.New("content and publication are required for backlink compilation")
	}
	if len(pub.Backlinks) > 0 {
		count := len(pub.Backlinks)
		pub = copyBacklinkPublication(pub)
		if len(pub.Backlinks) != count {
			return nil, nil, errors.New("generated backlink is not part of the publication documents")
		}
	}
	refsByID := make(map[string]content.BackLinkRef)
	for _, refs := range c.BackLinkIndex {
		for _, ref := range refs {
			refsByID[ref.RefID] = ref
		}
	}
	const maxPasses = 32
	for range maxPasses {
		text, err := compileText(ctx, pub)
		if err != nil {
			return nil, nil, err
		}
		changed := false
		for element, refID := range pub.Backlinks {
			if err := ctx.Err(); err != nil {
				return nil, nil, err
			}
			ref, found := refsByID[refID]
			if !found {
				return nil, nil, fmt.Errorf("generated backlink has no reference context: %q", refID)
			}
			position, err := resolveMainPosition(text, linkTarget{Filename: ref.Filename, ID: refID})
			if err != nil {
				return nil, nil, fmt.Errorf("resolve backlink location: %w", err)
			}
			// Classic MOBI/KF8 locations are 1-based 150-byte main-text units,
			// unlike KFX's 40-position units. CSS/resources do not contribute.
			ref.LocationNumber = int(position.Position/150) + 1
			refsByID[refID] = ref
			label, err := c.BacklinkText(ref)
			if err != nil {
				return nil, nil, err
			}
			if element.Text() != label {
				element.SetText(label)
				changed = true
			}
		}
		if !changed {
			for targetID, refs := range c.BackLinkIndex {
				for i := range refs {
					refs[i].LocationNumber = refsByID[refs[i].RefID].LocationNumber
				}
				c.BackLinkIndex[targetID] = refs
			}
			return pub, text, nil
		}
	}
	return nil, nil, errors.New("AZW3 backlink template locations did not stabilize after 32 compilation passes")
}

// Copy the source documents and their generated-link identities together, leaving
// the shared publication untouched. All non-document resources remain read-only.
func copyBacklinkPublication(source *publication.Publication) *publication.Publication {
	result := *source
	result.Chapters = slices.Clone(source.Chapters)
	result.Backlinks = make(map[*etree.Element]string, len(source.Backlinks))
	copies := make(map[*etree.Document]*etree.Document)
	for i := range result.Chapters {
		chapter := &result.Chapters[i]
		if chapter.Doc == nil {
			continue
		}
		if copy, found := copies[chapter.Doc]; found {
			chapter.Doc = copy
			continue
		}
		original := chapter.Doc
		chapter.Doc = original.Copy()
		copies[original] = chapter.Doc
		type pair struct{ original, copy *etree.Element }
		stack := []pair{{original.Root(), chapter.Doc.Root()}}
		for len(stack) > 0 {
			item := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			if item.original == nil {
				continue // compileText diagnoses missing document roots.
			}
			if refID, found := source.Backlinks[item.original]; found {
				result.Backlinks[item.copy] = refID
			}
			children := item.copy.ChildElements()
			for i, child := range item.original.ChildElements() {
				stack = append(stack, pair{child, children[i]})
			}
		}
	}
	return &result
}
