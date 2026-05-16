package pdf

import (
	"strings"

	"go.uber.org/zap"

	"fbc/css"
	"fbc/fb2"
)

func newPDFStyleResolver(book *fb2.FictionBook, log *zap.Logger, tracers ...*pdfStyleTracer) *pdfStyleResolver {
	if log == nil {
		log = zap.NewNop()
	}
	var tracer *pdfStyleTracer
	if len(tracers) > 0 {
		tracer = tracers[0]
	}
	defaults := defaultPDFStyles()
	resolver := &pdfStyleResolver{styles: clonePDFStyles(defaults), defaults: defaults, tracer: tracer}
	resolver.traceDefaults()
	if book == nil {
		return resolver
	}
	parser := css.NewParser(log)
	for i := range book.Stylesheets {
		stylesheet := &book.Stylesheets[i]
		if stylesheet.Type != "" && !strings.EqualFold(stylesheet.Type, "text/css") {
			continue
		}
		if strings.TrimSpace(stylesheet.Data) == "" {
			continue
		}
		resolver.applyStylesheet(parser.Parse([]byte(stylesheet.Data), "pdf stylesheet"))
	}
	resolver.applyPDFStyleAdjustments()
	return resolver
}
