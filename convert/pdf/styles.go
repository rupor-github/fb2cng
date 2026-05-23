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
	resolver := &pdfStyleResolver{styles: clonePDFStyles(defaults), defaults: defaults, dropcaps: make(map[string]pdfDropcapCSSConfig), log: log, tracer: tracer}
	resolver.traceDefaults()
	if book == nil {
		return resolver
	}
	parser := css.NewParser(log)
	processed := 0
	warnings := 0
	for i := range book.Stylesheets {
		stylesheet := &book.Stylesheets[i]
		if stylesheet.Type != "" && !strings.EqualFold(stylesheet.Type, "text/css") {
			continue
		}
		if strings.TrimSpace(stylesheet.Data) == "" {
			continue
		}
		processed++
		parsed := parser.Parse([]byte(stylesheet.Data), "pdf stylesheet")
		warnings += len(parsed.Warnings)
		for _, warning := range parsed.Warnings {
			log.Debug("CSS conversion warning", zap.String("warning", warning))
		}
		resolver.applyStylesheet(parsed)
	}
	log.Info("CSS stylesheets processed", zap.Int("stylesheets", processed), zap.Int("warnings", warnings))
	resolver.applyPDFStyleAdjustments()
	return resolver
}
