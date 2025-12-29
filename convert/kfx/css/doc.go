// Package css provides CSS parsing and KFX style conversion functionality.
//
// This package parses a subset of CSS relevant for ebook styling and converts
// it to KFX-native style definitions. It supports:
//
// Selector types:
//   - Element selectors: p, h1-h6, code, table, th, td, blockquote, etc.
//   - Class selectors: .paragraph, .epigraph, .verse
//   - Combined: p.has-dropcap, blockquote.cite
//   - Grouped: h2, h3, h4 (split into separate rules)
//   - Descendant: p code, .section-title h2 (ancestor-descendant relationships)
//   - Pseudo-elements: ::before, ::after (with content property)
//
// CSS properties (see TODO.md for full matrix):
//   - Typography: font-size, font-weight, font-style, text-align, text-indent, line-height
//   - Box model: margin-*, padding
//   - Display: display, float
//   - Page control: page-break-before, page-break-after, page-break-inside
//   - Pseudo-element: content (string values for ::before/::after)
//
// Unsupported (ignored with warning):
//   - Sibling selectors (E + F, E ~ F)
//   - Child combinator (E > F)
//   - Attribute selectors ([attr], [attr=value])
//   - Pseudo-classes (:first-child, :hover)
//   - @media blocks (always skipped)
//   - Complex CSS features (animations, transforms, grid, flexbox)
package css
