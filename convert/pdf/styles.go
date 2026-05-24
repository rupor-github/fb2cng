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
	var combinedCSS strings.Builder
	for i := range book.Stylesheets {
		stylesheet := &book.Stylesheets[i]
		if stylesheet.Type != "" && !strings.EqualFold(stylesheet.Type, "text/css") {
			continue
		}
		if strings.TrimSpace(stylesheet.Data) == "" {
			continue
		}
		processed++
		combinedCSS.WriteString(stylesheet.Data)
		combinedCSS.WriteByte('\n')
	}
	warnings := 0
	if combinedCSS.Len() > 0 {
		parsed := parser.Parse([]byte(combinedCSS.String()), "combined stylesheets")
		resolver.parsedStylesheetCSS = parsed.String()
		resolver.hasParsedStylesheet = true
		warnings += len(parsed.Warnings)
		pseudoWarnings := resolver.extractPseudoContent(parsed)
		warnings += len(pseudoWarnings)
		stats := resolver.applyStylesheet(parsed)
		log.Debug("CSS styles loaded",
			zap.Int("rules", stats.Rules),
			zap.Int("styles", stats.Styles),
			zap.Int("warnings", warnings),
			zap.Int("pseudo_content", len(resolver.pseudoContent)))
		for _, warning := range parsed.Warnings {
			log.Debug("CSS conversion warning", zap.String("warning", warning))
		}
		for _, warning := range pseudoWarnings {
			log.Debug("CSS conversion warning", zap.String("warning", warning))
		}
	}
	log.Info("CSS stylesheets processed", zap.Int("stylesheets", processed), zap.Int("warnings", warnings))
	resolver.applyPDFStyleAdjustments()
	return resolver
}
