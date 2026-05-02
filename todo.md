# PDF Pipeline — KFX Feature Parity Plan

Tracking progress on reaching full feature parity between PDF and KFX (AZW8)
generation pipelines.

## Completed

- [x] **Performance fix** — Buffered PDF output writes (256KB) to avoid costly
  per-token syscalls. `savePDF()` in `convert/pdf/generate.go`.

- [x] **Infinite layout loop** — Scale down oversized images to fit page content
  height, preventing folio's layout engine from looping.

- [x] **Cyrillic bookmarks/metadata** — Encode PDF text strings as UTF-16BE with
  BOM when they contain non-Latin-1 characters. Fixes mojibake in outlines and
  document properties. `encodePDFTextString()` in `convert/pdf/generate.go`.

- [x] **Configurable screen DPI** — Added `dpi` field to `ScreenConfig`
  (default 300). PDF page size now uses device DPI instead of hardcoded 96.
  Image JFIF metadata also uses config DPI. Separate `CSSPxToPt()` for CSS
  unit conversion (always 96 DPI per spec).

- [x] **CSS margin collapsing** — Extracted KFX collapse algorithm into shared
  `convert/margins/` package. Added UA-style defaults, tree builder, apply
  phase, and per-container kind/flags to the PDF pipeline.

- [x] **Heading font sizes & embedded fonts** — Fixed heading text rendering at
  body size (12pt) by resolving heading style before creating text runs.
  Replaced standard PDF fonts and system fallback with embedded Literata
  (serif), Noto Sans (sans-serif), and Noto Sans Mono (monospace). Fonts are
  gzip-compressed, SIL OFL licensed, downloaded via Taskfile `get-fonts` task.

- [x] **Empty-line margin absorption** — KFX-style margin absorption model
  (`handleEmptyLine` + `consumePendingEmptyLine` + `emptyLineSignal`) replaces
  spacer divs.

- [x] **CSS property gaps** — `letter-spacing`, `hyphens`, `font-size`
  keywords, `margin: auto` centering, `ex` units, `break-before`/`break-after`,
  `min/max-width/height`.

- [x] **Internal links (intra-document navigation)** — FB2 `<a xlink:href="#id">`
  links now navigate to the correct page via `anchorTracker` +
  `anchoredElement` + `internalLinkRewriter` + `newPageProbe`.

- [x] **PDF generation tracer** — `PDFTracer` captures pipeline state events
  (elements, AreaBreaks, anchors, link rewrites, margin trees, outline
  construction) and flushes to `pdf-trace.txt` in the debug report zip.

- [x] **Multi-paragraph titles** — FB2 `<title>` with multiple `<p>` children
  renders as separate block elements matching KFX's `addTitleAsParagraphs`.

- [x] **Outline from plan.TOC** — Manual outline builder (`outlineFinalizer`)
  walks plan.TOC with unlimited nesting depth, bypassing folio's H1-H6 cap.
  Symmetric wrong-nesting trees, UTF-16BE encoded titles.

- [x] **Footnote/body-intro anchors** — Synthetic unit IDs (`a-notes-*`,
  `a-body-*`) now registered as anchors so outline entries resolve correctly.
  Footnote bodies appear as parent entries with nested child sections.

## Phase 4 — Dropcap Improvements

PDF now renders drop caps using folio's Float API at the top level and an
overlay-based simulation inside container Divs.

- [x] Wire up folio `Float(FloatLeft, ...)` for `.dropcap` segments
  At depth 0 the Float element produces true text wrapping via folio's
  `render_plans.go` float tracking.  CSS padding-right on `.dropcap` becomes
  the float margin.
- [x] Overlay fallback for nested containers (depth > 0)
  `Div.PlanLayout` does not track floats — children get full inner width,
  causing overlap.  `emitDropcapOverlay` uses a wrapper Div with
  `padding-left` + `AddOverlay` to position the enlarged character at the
  left edge while the body text wraps at reduced width.  All body lines are
  indented (not just the lines adjacent to the dropcap) — acceptable
  trade-off vs. overlap.
- [x] Empty-line + dropcap vertical alignment fix
  `consumeEmptyLineForDropcap` emits a zero-content spacer Paragraph
  (survives margin collapsing unlike empty Divs) so both Float and body
  Paragraph share the same Y after the empty-line gap.
- [x] `detectDropcapPatterns()` one-time CSS analysis with debug logging
  (mirrors KFX `css_converter.go:1032`)
- [x] `"float"` recognised in `handledCSSProperties` to suppress warning
- [x] Negative `margin-left` workaround — N/A for PDF
  KFX hack (NNBSP insertion + dropcap-chars extension) compensates for a
  KP3 renderer bug when `margin-left < 0` meets `dropcap_lines`.  PDF uses
  folio Float, which positions correctly regardless of margins; default CSS
  sets `margin: 0` on `.has-dropcap` paragraphs.
- [x] CSS `clear` for float clearing — N/A for Phase 4
  Default CSS does not use `clear`.  Folio already has `Clearable` interface
  support in the render loop; dropcap floats expire naturally after ~1 line.
  General `clear` support can be added later if user CSS requires it.

## Phase 4½ — CSS Infrastructure

- [x] `fbc-pdf` media query — custom `@media fbc-pdf { … }` blocks active
  only during PDF generation.  `MediaContext` struct replaces positional
  booleans in `MediaQuery.Evaluate`.  PDF now evaluates `amzn-kf8` and
  `amzn-et` as true (matching KFX exactly).
- [x] Body style cascade — resolved CSS `body` rule is the root of the
  inherited property cascade for all content elements (font-family,
  font-size, line-height, color, text-align, letter-spacing, hyphens …).
  Page margins derived from body margin/padding with 18 pt fallback.
  Debug log reports the resolved body style at startup.

## Phase 5 — Generated Sections

- [x] Annotation page generation — `addAnnotationPage` renders the
  book-level annotation (`TitleInfo.Annotation`) as a standalone page
  with heading (CSS `annotation-title`) and content wrapper (CSS
  `annotation`).  Controlled by `cfg.Annotation.Enable`; optionally
  appears in the PDF outline via `cfg.Annotation.InTOC`.
- [x] TOC page generation — `addTOCPage` renders a hierarchical table
  of contents with internal links to section anchors.  Title from book
  title + optional authors (via `AuthorsTemplate`).  Entries rendered
  as paragraphs with `InlineLink` segments (reusing `internalLinkRewriter`).
  Nested entries wrapped in `toc-nested` Div containers.  Placement
  before (after cover) or after (end of document) via `TOCPagePlacement`.
  Document ordering: Cover → TOC (before) → Annotation → Content →
  Footnotes → TOC (after).

## Phase 6 — Metadata & Polish

- [x] Add Keywords to PDF metadata — `doc.Info.Keywords` wired from
  `book.Description.TitleInfo.Keywords`.
- [x] Table header row repetition across page breaks — `isHeaderRow()`
  detects all-`<th>` rows and calls folio's `AddHeaderRow()` so headers
  repeat when tables break across pages.
- [x] Handle IMAGE + EMPTYLINE + IMAGE case — three-way empty-line
  handling matching KFX: image+emptyline+image emits a real spacer
  paragraph; non-image+emptyline+image sets `EmptyLineMarginBottom` on
  the previous element; `prevWasImage` tracking in `flowBuilder`.
- [ ] Footnote backlinks in default (non-float) mode
- [ ] Review and fix any remaining visual discrepancies

### Deferred

- Language metadata (`/Lang` catalog entry) — folio has no API for the
  PDF catalog `/Lang` entry.  The catalog dictionary is a local variable
  inside `Document.WriteTo()` with no extension points.  Requires a
  vendored code change (3 lines) or a new post-processing dependency
  (e.g. pdfcpu).  Skipped for now.

## Not Planned (KFX-specific)

These features are specific to the KFX/Kindle format and do not apply to PDF:

- Position maps / location maps
- Font-size percentage compression formula (KP3-specific non-linear scaling)
- `writing-mode` / `text-orientation` / `text-combine-upright` (vertical text)
- `text-emphasis-*` (Japanese text features)
- KFX content chunking and Ion serialization
- Footnote content markers (`position:footer`, `yj.classification:footnote`)
- Float-mode footnote popup rendering
- `overline` text-decoration (folio lacks `DecorationOverline`)
- `border-style` groove/ridge/inset/outset (folio lacks these)
