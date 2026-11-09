package azw3

import (
	"bytes"
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/beevik/etree"

	"fbc/css"
)

// lowerSVGResources resolves SVG dependencies after all embed/flow numbers have
// been allocated. Self-contained SVG stays byte-identical. Malformed SVG retained
// by use_broken is opaque: preserving it does not promise reader renderability.
func lowerSVGResources(ctx context.Context, filename string, data []byte, resources *resourceLayout) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	doc := etree.NewDocument()
	if err := doc.ReadFromBytes(data); err != nil || doc.Root() == nil {
		return bytes.Clone(data), nil
	}
	changed := false
	rewrite := func(value string, kind resourceKind) (string, error) {
		if strings.HasPrefix(strings.TrimSpace(value), "#") {
			return value, nil // Local paint servers, masks, clips and use references.
		}
		target, local, err := resolveTextTarget(filename, value)
		if err != nil {
			return "", err
		}
		if !local {
			return value, nil
		}
		if target.ID != "" {
			return "", fmt.Errorf("unsupported external SVG fragment URI %q", value)
		}
		uri, _, err := lowerResourceURI("", (&url.URL{Path: target.Filename}).String(), resources, kind)
		if err != nil {
			return "", err
		}
		changed = true
		return uri, nil
	}
	rewriteValue := func(value string) (string, error) {
		var rewriteErr error
		result := css.RewriteValueURLsUnquoted(value, func(original string) string {
			if rewriteErr != nil {
				return original
			}
			var result string
			result, rewriteErr = rewrite(original, resourceAny)
			return result
		})
		return result, rewriteErr
	}
	stack := []*etree.Element{doc.Root()}
	for len(stack) > 0 {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		elem := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		stack = append(stack, elem.ChildElements()...)
		for _, attr := range elem.Attr {
			value := attr.Value
			var err error
			if (elem.Tag == "image" || elem.Tag == "use") && attr.Key == "href" {
				value, err = rewrite(value, resourceImage)
			} else {
				value, err = rewriteValue(value)
			}
			if err != nil {
				return nil, err
			}
			if value != attr.Value {
				elem.CreateAttr(attr.FullKey(), value)
			}
		}
		if elem.Tag == "style" {
			for _, token := range elem.Child {
				text, ok := token.(*etree.CharData)
				if !ok {
					continue
				}
				value, err := rewriteValue(text.Data)
				if err != nil {
					return nil, err
				}
				text.Data = value
			}
		}
	}
	if !changed {
		return bytes.Clone(data), nil
	}
	result, err := doc.WriteToBytes()
	if err != nil {
		return nil, fmt.Errorf("serialize lowered SVG: %w", err)
	}
	return result, nil
}
