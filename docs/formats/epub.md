# EPUB And KEPUB Output

This page covers `epub2`, `epub3`, and `kepub` output.

## Choosing A Variant

| Format | Use When |
|---|---|
| `epub2` | You need broad compatibility with older EPUB readers |
| `epub3` | You want modern EPUB semantics, including EPUB3 footnote markup |
| `kepub` | You target Kobo devices and want Kobo-specific packaging/behavior |

## Structure And Navigation

fb2cng converts top-level FB2 sections into EPUB content documents and generates device navigation:

- EPUB2: `toc.ncx`
- EPUB3: `nav.xhtml`
- KEPUB: EPUB2-style package optimized for Kobo handling

The optional visible TOC page is controlled by `document.toc_page` and is separate from device navigation.

When `document.page_map.adobe_de` is enabled, EPUB output can include Adobe Digital Editions-compatible page navigation so ADE-based readers can expose generated page positions in their navigation UI. This navigation is ADE-specific rather than standard EPUB markup, so it can produce errors during `epubcheck` validation.

## Styling

EPUB and KEPUB use XHTML plus CSS. Set `document.stylesheet_path` to replace the default stylesheet.

Custom stylesheet notes:

- Start from `convert/default.css` if you want to preserve default coverage.
- Use `@font-face` to embed fonts.
- See [Stylesheets](../stylesheets.md) for URL resolution and resource packaging.

## Section Splitting

Depth-1 sections are always separate content documents. Nested sections can be split based on CSS `page-break-before` rules on `.section-title-hN` classes. See [Stylesheets](../stylesheets.md) for details.

## Footnotes

| Mode | EPUB2/KEPUB | EPUB3 |
|---|---|---|
| `default` | Footnotes are normal linked sections | Footnotes are normal linked sections |
| `float` | Bidirectional links for reader compatibility | EPUB `aside` and `noteref` markup |
| `floatRenumbered` | Same as `float`, with normalized labels | Same as `float`, with normalized labels |

See [Footnotes](../footnotes.md) for mode details and templates.

## Images And Fonts

- SVG images stay SVG in EPUB output.
- Font resources are stored under `OEBPS/fonts/`.
- Other resources are stored under `OEBPS/other/`.
- Relative stylesheet resources are resolved from the stylesheet location; unsafe absolute/path-traversal references are rejected.
