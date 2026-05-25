package pdf

import (
	"strings"

	"go.uber.org/zap"

	"fbc/fb2"
)

func newPDFStyleResolver(book *fb2.FictionBook, log *zap.Logger, tracers ...*pdfStyleTracer) *pdfStyleResolver {
	if log == nil {
		log = zap.NewNop()
	}
	if book == nil {
		return newPDFBaseStyleResolver(log, tracers...)
	}
	return newPDFStyleResolverFromParsed(parsePDFStylesheets(book, log), log, tracers...)
}

func newPDFBaseStyleResolver(log *zap.Logger, tracers ...*pdfStyleTracer) *pdfStyleResolver {
	var tracer *pdfStyleTracer
	if len(tracers) > 0 {
		tracer = tracers[0]
	}
	defaults := defaultPDFStyles()
	resolver := &pdfStyleResolver{styles: clonePDFStyles(defaults), defaults: defaults, dropcaps: make(map[string]pdfDropcapCSSConfig), log: log, tracer: tracer}
	resolver.traceDefaults()
	return resolver
}

func newPDFStyleResolverFromParsed(stylesheets []pdfParsedStylesheet, log *zap.Logger, tracers ...*pdfStyleTracer) *pdfStyleResolver {
	if log == nil {
		log = zap.NewNop()
	}
	resolver := newPDFBaseStyleResolver(log, tracers...)
	warnings := 0
	var stats pdfStylesheetStats
	var parsedCSS strings.Builder
	for _, stylesheet := range stylesheets {
		parsed := stylesheet.parsed
		if parsed == nil {
			continue
		}
		if parsedCSS.Len() > 0 {
			parsedCSS.WriteByte('\n')
		}
		parsedCSS.WriteString(parsed.String())
		resolver.hasParsedStylesheet = true
		warnings += len(parsed.Warnings)
		pseudoWarnings := resolver.extractPseudoContent(parsed)
		warnings += len(pseudoWarnings)
		stylesheetStats := resolver.applyStylesheet(parsed)
		stats.Rules += stylesheetStats.Rules
		stats.Styles += stylesheetStats.Styles
		for _, warning := range parsed.Warnings {
			log.Debug("CSS conversion warning", zap.String("warning", warning))
		}
		for _, warning := range pseudoWarnings {
			log.Debug("CSS conversion warning", zap.String("warning", warning))
		}
	}
	if resolver.hasParsedStylesheet {
		resolver.parsedStylesheetCSS = parsedCSS.String()
		log.Debug("CSS styles loaded",
			zap.Int("rules", stats.Rules),
			zap.Int("styles", stats.Styles),
			zap.Int("warnings", warnings),
			zap.Int("pseudo_content", len(resolver.pseudoContent)))
	}
	log.Info("CSS stylesheets processed", zap.Int("stylesheets", len(stylesheets)), zap.Int("warnings", warnings))
	resolver.applyPDFStyleAdjustments()
	return resolver
}
