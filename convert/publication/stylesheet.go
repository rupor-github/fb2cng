package publication

import (
	"fmt"
	"strings"

	"fbc/css"
	"fbc/fb2"
)

// Stylesheet contains combined CSS and its resources in source stylesheet order.
// URLs refer to publication-relative filenames, without an output-container prefix.
type Stylesheet struct {
	Data      []byte
	Resources []fb2.StylesheetResource
}

// StylesheetRewriter lets an adapter preserve syntax beyond the shared CSS AST.
type StylesheetRewriter func(data []byte, rewriteURL func(string) string) ([]byte, error)

// AssembleStylesheet preserves the normalized cascade and resolves CSS URLs without
// writing a container. A nil rewriter uses the shared CSS parser/serializer.
func AssembleStylesheet(styles []fb2.Stylesheet, rewriteStylesheet StylesheetRewriter) (Stylesheet, error) {
	var finalCSS strings.Builder
	var resources []fb2.StylesheetResource
	for _, style := range styles {
		if style.Type != "text/css" {
			continue
		}
		urlMap := make(map[string]string, len(style.Resources))
		for _, resource := range style.Resources {
			urlMap[resource.OriginalURL] = resource.Filename
		}
		resources = append(resources, style.Resources...)
		rewrite := func(originalURL string) string {
			if newPath, ok := urlMap[originalURL]; ok {
				return newPath
			}
			return originalURL
		}
		if rewriteStylesheet != nil {
			data, err := rewriteStylesheet([]byte(style.Data), rewrite)
			if err != nil {
				return Stylesheet{}, fmt.Errorf("assemble publication stylesheet: %w", err)
			}
			finalCSS.Write(data)
			finalCSS.WriteByte('\n')
			continue
		}
		sheet := css.NewParser(nil).Parse([]byte(style.Data), "")
		sheet.RewriteURLs(rewrite)
		// strings.Builder.Write cannot fail.
		_, _ = sheet.WriteTo(&finalCSS)
		finalCSS.WriteString("\n")
	}
	return Stylesheet{Data: []byte(finalCSS.String()), Resources: resources}, nil
}
