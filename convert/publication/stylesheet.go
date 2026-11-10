package publication

import (
	"fmt"
	"strings"

	"fbc/common"
	"fbc/css"
	"fbc/fb2"
)

// Stylesheet contains combined CSS and its resources in source stylesheet order.
// URLs refer to publication-relative filenames, without an output-container prefix.
type Stylesheet struct {
	Data      []byte
	Resources []fb2.StylesheetResource
}

// AssembleStylesheet preserves the normalized cascade and resolves CSS URLs without
// writing a container. CSS-only resources, including fonts, are retained separately
// from body images.
func AssembleStylesheet(styles []fb2.Stylesheet, format common.OutputFmt) (Stylesheet, error) {
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
		if format == common.OutputFmtAzw3 {
			// KF8 readers consume CSS directly. Do not sort declarations or discard
			// selectors/at-rules through the native renderers' restricted AST.
			data, err := css.RewriteStylesheetURLs([]byte(style.Data), rewrite)
			if err != nil {
				return Stylesheet{}, fmt.Errorf("assemble AZW3 stylesheet: %w", err)
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
