package pdf

import (
	"strings"

	"go.uber.org/zap"

	"fbc/css"
	"fbc/fb2"
)

type pdfParsedStylesheet struct {
	stylesheet *fb2.Stylesheet
	parsed     *css.Stylesheet
}

func parsePDFStylesheets(book *fb2.FictionBook, log *zap.Logger) []pdfParsedStylesheet {
	if book == nil {
		return nil
	}
	parser := css.NewParser(log)
	parsed := make([]pdfParsedStylesheet, 0, len(book.Stylesheets))
	for i := range book.Stylesheets {
		stylesheet := &book.Stylesheets[i]
		if stylesheet.Type != "" && !strings.EqualFold(stylesheet.Type, "text/css") {
			continue
		}
		if strings.TrimSpace(stylesheet.Data) == "" {
			continue
		}
		parsed = append(parsed, pdfParsedStylesheet{
			stylesheet: stylesheet,
			parsed:     parser.Parse([]byte(stylesheet.Data), "pdf stylesheet"),
		})
	}
	return parsed
}
