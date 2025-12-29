// Package css provides CSS parsing and KFX style conversion functionality.
//
// This package parses a subset of CSS relevant for ebook styling and converts
// it to KFX-native style definitions. It supports:
//
// # Selector types
//
//   - Element selectors: p, h1-h6, code, table, th, td, blockquote, etc.
//   - Class selectors: .paragraph, .epigraph, .verse
//   - Combined: p.has-dropcap, blockquote.cite
//   - Grouped: h2, h3, h4 (split into separate rules)
//   - Descendant: p code, .section-title h2 (ancestor-descendant relationships)
//   - Pseudo-elements: ::before, ::after (with content property)
//
// # CSS Properties to KFX Mapping
//
// Typography:
//   - font-size: Converts to $16 with dimension value (em, px, pt, %)
//   - font-weight: bold→$361, normal→$350, 100-900 numeric values
//   - font-style: italic→$382, normal→$350
//   - font-family: String value stored as-is ($11)
//   - line-height: Ratio or dimension ($42)
//   - letter-spacing: Dimension value ($32)
//   - color: RGB hex, rgb(), keywords → $19
//
// Text Layout:
//   - text-align: left/start→$680, right/end→$681, center→$320, justify→$321
//   - text-indent: Dimension value ($36)
//
// Box Model:
//   - margin-*: Dimension values ($47, $48, $49, $50)
//   - margin shorthand: Expanded to individual properties
//
// Special Properties:
//   - text-decoration: underline→$23, line-through→$27
//   - vertical-align: super/sub → baseline_shift ($31)
//   - float: left→$680, right→$681
//   - page-break-*: always→$352, avoid→$353
//
// # Unsupported (ignored with warning)
//
//   - Sibling selectors (E + F, E ~ F)
//   - Child combinator (E > F)
//   - Attribute selectors ([attr], [attr=value])
//   - Pseudo-classes (:first-child, :hover)
//   - @media blocks (always skipped)
//   - Complex CSS features (animations, transforms, grid, flexbox)
//
// # Usage
//
//	parser := css.NewParser(logger)
//	sheet := parser.Parse(cssBytes)
//
//	converter := css.NewConverter(logger)
//	styles, warnings := converter.ConvertStylesheet(sheet)
//
//	for _, style := range styles {
//	    registry.Register(style)
//	}
package css
