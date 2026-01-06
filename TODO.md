# CSS-to-KFX Stylesheet Implementation Plan

## Overview

This document outlines a phased approach to implement CSS stylesheet support
for KFX output generation. The goal is to translate the `default.css` rules
(and custom user stylesheets) into KFX-native styles, ensuring consistent
formatting between EPUB and KFX outputs.

1. Pass validation via `testdata/input.py` (KFXInput plugin)
2. Implementation docs are in `docs/`
3. Follow project coding standards (Go idioms, structured logging, etc.)
4. Use latest Go features - ALWAYS
5. For any function or method if present context should always be first argument and zap.Logger last one 
6. Instead of gofmt use `goimports-reviser -format -rm-unused -set-alias
   -company-prefixes github.com/rupor-github -excludes vendor ./...` - it is
   always available
7. Module path is `fbc` (not `fb2cng`) - use `fbc/...` for imports

## Selector Coverage Policy (EPUB parity)

We must support both:

1) **Class selectors** (e.g. `.epigraph`, `.link-footnote`) because EPUB generation attaches semantic classes.

2) **Element selectors** (e.g. `p`, `h1`..`h6`, `code`, `table`, `th`, `td`) because EPUB output also relies on plain tags, and user stylesheets may target tags without classes.

3) **Inline tag selectors used by EPUB generator** (no classes): `strong`, `em`, `del`, `sub`, `sup`, `a`, `img`, `span`, `div`, `blockquote`.

Supported selector forms (initial target):
- `tag` (e.g. `p`, `h1`, `strong`)
- `.class` (e.g. `.paragraph`, `.epigraph`)
- `tag.class` (e.g. `blockquote.cite`, `p.has-dropcap`)
- `ancestor descendant` (e.g. `p code`, `.section-title h2`)
- `selector::before`, `selector::after` (pseudo-elements with `content` property)

Sibling selectors (`+`, `~`) are ignored with warning.

### Selector → KFX style name resolution

- `.foo` → style name `foo`
- `tag` → style name `tag` (unless we define an explicit alias, see below)
- `tag.foo` → style name `foo` (preferred) with fallback to `tag.foo` only if needed to avoid collisions
- `ancestor descendant` → style name based on rightmost part (e.g., `p code` → `p--code`, `.section-title h2` → `section-title--h2`)

**Important aliases for EPUB parity:**
- `p` should map to the KFX base paragraph style (currently `paragraph`). We should define an alias rule so that `p { ... }` feeds into style `paragraph` unless explicitly overridden by a more specific selector.
- `em` → `emphasis`, `del` → `strikethrough` (match existing KFX inline style names).

## Amazon Kindle CSS Support Reference

### Official Documentation Sources

- [Attributes and Tags Supported by Enhanced Typesetting](https://kdp.amazon.com/en_US/help/topic/GB5GDY7WAJDN9GFK) - Official Amazon Enhanced Typesetting support
- [HTML and CSS Tags Supported in Kindle Format 8](https://kdp.amazon.com/en_US/help/topic/GG5R7N649LECKP7U) - Official KF8 CSS reference
- [Amazon Kindle Publishing Guidelines PDF](https://kindlegen.s3.amazonaws.com/AmazonKindlePublishingGuidelines.pdf) - Comprehensive publishing guidelines
- [MobileRead Wiki: Kindle Format 8 CSS](https://wiki.mobileread.com/wiki/Kindle_Format_8_CSS) - Community reference

### KF8/KFX Supported CSS Properties (Amazon Official)

#### Typography Properties
| CSS Property | KF8 Support | Enhanced Typesetting | Notes |
|--------------|-------------|---------------------|-------|
| `font-family` | ✅ | ✅ | Via `@font-face` embedding |
| `font-size` | ✅ | ✅ | em, px, pt, % supported |
| `font-style` | ✅ | ✅ | italic, normal |
| `font-weight` | ✅ | ✅ | bold, normal, numeric |
| `font-variant` | ✅ | ✅ | small-caps |
| `line-height` | ✅ | ✅ | |
| `letter-spacing` | ✅ | ✅ | |
| `word-spacing` | ✅ | ✅ | |
| `text-align` | ✅ | ✅ | left, right, center, justify |
| `text-indent` | ✅ | ✅ | First-line indentation |
| `text-decoration` | ✅ | ✅ | underline, line-through |
| `text-transform` | ✅ | ✅ | uppercase, lowercase, capitalize |
| `text-shadow` | ✅ | Limited | |
| `vertical-align` | ✅ | ✅ | super, sub, baseline |
| `white-space` | ✅ | ✅ | |
| `color` | ✅ | ✅ | |
| `direction` | ✅ | ✅ | RTL support |

#### Box Model Properties
| CSS Property | KF8 Support | Enhanced Typesetting | Notes |
|--------------|-------------|---------------------|-------|
| `margin` | ✅ | ✅ | All sides |
| `margin-top/bottom/left/right` | ✅ | ✅ | |
| `padding` | ✅ | ✅ | All sides |
| `padding-top/bottom/left/right` | ✅ | ✅ | |
| `width` | ✅ | ✅ | |
| `height` | ✅ | ✅ | |
| `max-width/height` | ✅ | ✅ | |
| `min-width/height` | ✅ | ✅ | |

#### Border Properties
| CSS Property | KF8 Support | Enhanced Typesetting | Notes |
|--------------|-------------|---------------------|-------|
| `border` | ✅ | ✅ | Shorthand |
| `border-color/style/width` | ✅ | ✅ | |
| `border-top/bottom/left/right` | ✅ | ✅ | |
| `border-radius` | ✅ | Limited | |
| `border-collapse` | ✅ | ✅ | Tables |
| `border-spacing` | ✅ | ✅ | Tables |

#### Background Properties
| CSS Property | KF8 Support | Enhanced Typesetting | Notes |
|--------------|-------------|---------------------|-------|
| `background` | ✅ | Limited | |
| `background-color` | ✅ | ✅ | |
| `background-image` | ✅ | Limited | |
| `background-position/repeat/size` | ✅ | Limited | |

#### Layout & Display
| CSS Property | KF8 Support | Enhanced Typesetting | Notes |
|--------------|-------------|---------------------|-------|
| `display` | ✅ | ✅ | block, inline, none, etc. |
| `float` | ✅ | ✅ | left, right |
| `clear` | ✅ | ✅ | |
| `position` | Limited | Limited | |
| `visibility` | ✅ | ✅ | |
| `overflow` | ✅ | Limited | |
| `z-index` | ✅ | Limited | |

#### Page/Print Properties
| CSS Property | KF8 Support | Enhanced Typesetting | Notes |
|--------------|-------------|---------------------|-------|
| `page-break-before` | ✅ | ✅ | always, avoid |
| `page-break-after` | ✅ | ✅ | always, avoid |
| `page-break-inside` | ✅ | ✅ | avoid |

#### Not Supported / Limited
- CSS counters (`counter-increment`, `counter-reset`)
- Advanced selectors: `:first-letter`, `:first-line`
- Adjacent sibling selectors (`E + F`, `E ~ F`)
- CSS Grid, Flexbox (very limited)
- Transitions, animations
- Media queries (basic support only)

#### Pseudo-elements (Supported)
- `::before` - Content inserted before element (requires `content` property)
- `::after` - Content inserted after element (requires `content` property)
- Note: Only `content` property with string values is supported for pseudo-elements

### Media Queries

Per **Media Query Policy** above: `@media` blocks are ignored with the following exception:

- [x] **Handle Amazon-specific @media queries**: During KFX generation, `amzn-kf8` and `amzn-et` are set to true. Parse and apply rules from `@media amzn-kf8` and `@media amzn-et` blocks. Rules from `@media amzn-mobi` are always ignored. Rules inside qualifying media queries should be processed and merged with regular stylesheet rules. The CSS parser must properly evaluate media query expressions including logical operators (e.g., `@media amzn-kf8 and not amzn-et` evaluates to false during KFX generation since both amzn-kf8 and amzn-et are true).

---

## Current State Analysis

### EPUB Pipeline (Working Reference)
1. **Stylesheet Loading**: `convert/run.go` loads `default.css` (embedded) or custom CSS via `env.Cfg.Document.StylesheetPath`
2. **Stylesheet Processing**: `fb2/stylesheet.go` normalizes stylesheets, resolves external resources (fonts, etc.)
3. **CSS Usage**: `convert/epub/generate.go` writes the CSS directly to `stylesheet.css` in the EPUB
4. **Element Styling**: `convert/epub/xhtml.go` applies CSS classes to XHTML elements (e.g., `class="paragraph"`, `class="h1"`)

### KFX Pipeline (Current)
1. **Style Definition**: `convert/kfx/frag_style.go` defines `StyleDef`, `StyleBuilder`, and `StyleRegistry`
2. **Hardcoded Styles**: `DefaultStyleRegistry()` creates hardcoded KFX-native styles
3. **Style Application**: `convert/kfx/frag_storyline.go` references styles by name in content entries
4. **No CSS Parsing**: CSS is completely ignored; styles are duplicated in Go code
5. **Entry Point**: `convert/kfx/generate.go` `buildFragments()` creates style registry at line 89

### Existing CSS Infrastructure (Can Be Reused)
1. **Stylesheet Loading**: `fb2/stylesheet.go` - `NormalizeStylesheets()` processes CSS, resolves `url()` refs, handles `@font-face`
2. **Resource Types**: `fb2/types.go` - `Stylesheet`, `StylesheetResource` structs already defined
3. **Regex Patterns**: `fb2/stylesheet.go` has `urlPattern`, `fontFacePattern`, `importPattern` - could be extended
4. **Content Access**: `content.Content.Book.Stylesheets` contains processed CSS (raw string in `Data` field)

### Key Differences
| Aspect | EPUB | KFX |
|--------|------|-----|
| Style Format | CSS text | KFX binary properties (symbols) |
| Style Reference | CSS class names | Style symbols ($157) |
| Units | CSS units (em, px, %) | KFX dimension values ($306/$307) |
| Inheritance | CSS cascade | KFX parent_style ($158) |
| Properties | CSS properties | KFX property symbols |

---

## Implementation Phases

---

## Phase 1: CSS Parser Foundation ✅

**Goal**: Create a CSS parser that extracts rule information relevant to KFX conversion.

**Status**: COMPLETED - Using `github.com/tdewolff/parse/v2/css` as lexer.

### Tasks

- [x] **1.1 Research CSS Parsing Libraries**
  - Selected `github.com/tdewolff/parse/v2/css` - fast tokenizer with proper CSS grammar support
  - Provides `Parser` with grammar types: BeginRulesetGrammar, DeclarationGrammar, etc.
  - Handles comments, @-rules, and complex value parsing natively

- [x] **1.2 Define Supported CSS Subset**
  - Implemented in `convert/kfx/css/types.go` and `doc.go`
  - Supported selectors: element, class, element.class, grouped, descendant, ::before/::after
  - Unsupported: sibling (`+`, `~`), child combinator (`>`), attribute selectors, pseudo-classes
  - @media blocks are skipped entirely

- [x] **1.2.1 Analyze default.css Selectors**
  - Parser successfully handles all 87 rules from default.css with 0 warnings
  - Descendant selectors like `p code` and `.section-title h2.section-title-header` are fully supported

- [x] **1.3 Create CSS Types**
  - File: `convert/kfx/css/types.go`
  - Types: `CSSValue`, `Selector`, `PseudoElement`, `CSSRule`, `CSSFontFace`, `Stylesheet`
  - `Selector.StyleName()` generates KFX style names:
    - Simple: `p` → `p`, `.foo` → `foo`, `p.foo` → `foo`
    - Descendant: `p code` → `p--code`, `.section-title h2` → `section-title--h2`
    - Pseudo: `selector::before` → `selector--before`

- [x] **1.4 Implement CSS Parser**
  - File: `convert/kfx/css/parser.go`
  - Uses tdewolff/parse lexer for tokenization
  - Handles grouped selectors, descendant selectors, @font-face, skips @media blocks
  - Parses dimension values (em, px, %, pt), keywords, colors

- [x] **1.5 Add Unit Tests**
  - File: `convert/kfx/css/parser_test.go`
  - 14 tests covering: element/class/combined selectors, grouped selectors,
    ::before/::after pseudo-elements, @media skipping, @font-face parsing,
    numeric values, shorthand properties, comments, default.css parsing
  - Test extraction of specific properties

---

## Phase 2: CSS-to-KFX Property Mapping ✅

**Goal**: Create a mapping layer that converts CSS properties to KFX style properties.

**Status**: COMPLETED - All conversion functions implemented and tested.

### Tasks

- [x] **2.1 Define CSS-to-KFX Property Map**
  - File: `convert/kfx/css/mapping.go`
  - Implemented `cssToKFXProperty` map with all supported CSS properties
  - Helper functions: `KFXPropertySymbol()`, `IsShorthandProperty()`, `IsSpecialProperty()`

- [x] **2.2 Implement Unit Conversion**
  - File: `convert/kfx/css/units.go`
  - `CSSValueToKFX()` converts CSS units to KFX dimension values
  - Supports: em, ex, %, px, pt, cm, mm, in, unitless
  - `MakeDimensionValue()` creates KFX struct values

- [x] **2.3 Implement Value Conversion for Complex Properties**
  - File: `convert/kfx/css/values.go`
  - `ConvertFontWeight()`: bold/normal/100-900 → KFX symbols
  - `ConvertFontStyle()`: italic/oblique/normal → KFX symbols
  - `ConvertTextAlign()`: left/right/center/justify/start/end → KFX symbols
  - `ConvertTextDecoration()`: underline/line-through/none
  - `ConvertVerticalAlign()`: super/sub/baseline → baseline_shift
  - `ConvertDisplay()`: block/inline/none → render mode
  - `ConvertFloat()`: left/right/none → KFX symbols
  - `ConvertPageBreak()`: always/avoid/auto → KFX symbols
  - `ParseColor()`: hex/rgb()/keywords → RGB values

- [x] **2.4 Handle Shorthand Properties**
  - Margin shorthand expansion (1-4 values)
  - `expandBoxShorthand()` handles all box model patterns

- [x] **2.5 Create Style Converter**
  - File: `convert/kfx/css/converter.go`
  - `Converter` type with `ConvertRule()` and `ConvertStylesheet()` methods
  - Merges rules with same selector
  - Returns warnings for unsupported properties

- [x] **2.6 Add Unit Tests**
  - File: `convert/kfx/css/converter_test.go`
  - 10+ test functions covering all conversion logic
  - Tests for font-weight, font-style, text-align, text-decoration
  - Tests for vertical-align, color parsing, unit conversion
  - Tests for shorthand expansion and rule merging

---

## Phase 3: Stylesheet Integration ✅

**Goal**: Integrate CSS parsing into the KFX generation pipeline.

**Status**: COMPLETED - CSS parsing integrated, default styles aligned with EPUB/CSS conventions.

### Tasks

- [x] **3.1 Modify StyleRegistry**
  - File: `convert/kfx/frag_style.go`
  - `RegisterFromCSS(styles []StyleDef)` method added
  - Registers CSS-converted styles into the registry
  - Later rules override earlier ones for the same style name

- [x] **3.2 Create CSS-Aware Default Registry**
  - Function: `NewStyleRegistryFromCSS(cssData []byte, log *zap.Logger) (*StyleRegistry, []string)`
  - Starts with `DefaultStyleRegistry()` as base
  - Parses CSS using `NewParser()` and converts via `NewConverter()`
  - Overlays CSS styles on top of defaults
  - Returns warnings for unsupported CSS features

- [x] **3.3 Selector-to-StyleName Mapping**
  - Implemented in `css_types.go` `Selector.StyleName()` method
  - CSS `.paragraph` → KFX style name `paragraph`
  - CSS `p` → KFX style name `p` (element styles)
  - CSS `tag.class` → KFX style name `class` (class takes precedence)
  - CSS `ancestor descendant` → KFX style name `ancestor--descendant`
  - CSS `selector::before` → KFX style name `selector--before`

- [x] **3.4 Update KFX Generator**
  - File: `convert/kfx/generate.go`
  - `buildStyleRegistry()` function added (lines 370-407)
  - Combines all `text/css` stylesheets from `c.Book.Stylesheets`
  - Calls `NewStyleRegistryFromCSS()` for CSS-based initialization
  - Falls back to `DefaultStyleRegistry()` if no stylesheets
  - Logs warnings at debug level

- [x] **3.5 Handle Style Name Normalization**
  - Style names from CSS selectors are used directly
  - `SymbolByName()` handles custom style name registration
  - Symbols are collected via `collectSymbolsFromValue()` in `container.go`
  - Local symbols start at ID 852 (after YJ_symbols max)

- [x] **3.6 Add Integration Tests**
  - `TestNewStyleRegistryFromCSS` - Tests full CSS parsing and registry creation
  - `TestNewStyleRegistryFromCSS_Empty` - Tests fallback to defaults
  - `TestStyleRegistryBuildFragments` - Tests fragment generation from registry
  - Tests verify CSS properties override defaults
  - Tests verify default styles are preserved

- [x] **3.7 Ensure Default Styles for All Generated Classes**
  - Audited all CSS classes generated by EPUB converter
  - Aligned style names between KFX and CSS/EPUB:
    - Changed default paragraph style from "paragraph" to "p" (matches CSS `p { }` selector)
    - Changed empty line style from "empty-line" to "emptyline" (matches CSS `.emptyline` class)
  - Added missing styles to `DefaultStyleRegistry()`:
    - `code` - for code blocks
    - `sub`, `sup` - subscript/superscript with baseline_style property
    - `body-title`, `chapter-title`, `section-title` - title wrappers
    - `body-title-header`, `chapter-title-header`, `section-title-header` - title headers
    - `date` - date elements
    - `link-external`, `link-internal`, `link-footnote`, `link-backlink` - link styles
  - Cross-referenced with `default.css` class definitions
  - All KFX style names now match EPUB/CSS conventions

- [x] **3.8 Implement Inline Style Events for All FB2 Inline Formatting**
  - Modified `addParagraphWithImages()` in `frag_storyline.go` to handle all inline segment types:
    - `InlineStrong` → creates style event with "strong" style (bold)
    - `InlineEmphasis` → creates style event with "emphasis" style (italic)
    - `InlineStrikethrough` → creates style event with "strikethrough" style
    - `InlineSub` → creates style event with "sub" style (subscript)
    - `InlineSup` → creates style event with "sup" style (superscript)
    - `InlineCode` → creates style event with "code" style
    - `InlineNamedStyle` → creates style event with custom style from FB2
    - `InlineLink` → creates style event with "link-footnote" or "link-external" + link target
  - Added new KFX symbols: `SymBaselineStyle` ($44), `SymSuperscript` ($370), `SymSubscript` ($371)
  - Added `BaselineStyle()` method to StyleBuilder
  - Generated KFX now passes validation with inline formatting properly applied

- [x] **3.9 Context-Specific Styles and Full Poem/Cite Handling (EPUB Parity)**
  - Added `context` parameter to `processFlowItem()` for context-aware styling
  - Subtitles now use context-specific styles matching EPUB (e.g., `section-subtitle`, `cite-subtitle`, `annotation-subtitle`, `epigraph-subtitle`)
  - Fixed `processPoem()` to handle all poem content like EPUB:
    - Added `poem.Subtitles` processing with `poem-subtitle` style
    - Added `poem.Epigraphs` processing
    - Added `poem.Date` processing with `date` style
    - Fixed stanza titles to use `stanza-title` instead of `poem-title`
    - Added stanza subtitle handling with `stanza-subtitle` style
  - Fixed `processCite()` to process all flow items with `cite` context (not just paragraphs)
  - Added missing default styles to `DefaultStyleRegistry()`:
    - `section-subtitle`, `cite-subtitle`, `annotation-subtitle`, `epigraph-subtitle` - context-specific subtitles
    - `poem-subtitle` - poem subtitles with left margin
    - `stanza-title` - stanza titles (bold, centered)
    - `stanza-subtitle` - stanza subtitles

---

## Phase 4: Style Inheritance and Cascading ✅

**Goal**: Implement proper style inheritance to match CSS cascade behavior.

**Status**: COMPLETED - Style inheritance implemented with full flattening for KFX.

### Tasks

- [x] **4.1 Implement Parent Style Support**
  - Styles use `Inherit(parentName)` to specify parent
  - Inheritance resolution flattens all properties from parent chain
  - Child properties override parent properties
  - Handles circular inheritance detection

- [x] **4.2 Create Inheritance Resolution**
  - `resolveInheritance()` in `frag_style.go` builds inheritance chain
  - Walks from child → parent → grandparent
  - Merges properties from root to child (child overrides)
  - Returns fully-flattened StyleDef with all properties

- [x] **4.3 Handle Base Styles**
  - `p` is the base style with: LineHeight, TextIndent, TextAlign
  - All text-based styles inherit from `p`: subtitle, poem, cite, epigraph, etc.
  - `h1`-`h6` have complete properties (no inheritance needed)
  - Specialized styles like `verse`, `stanza` inherit from `poem`
  - Title headers inherit from `p` for line-height

- [x] **4.4 Dynamic Style Inference**
  - `EnsureStyle()` now uses `inferParentStyle()` for unknown styles
  - Pattern matching: `xxx-subtitle` → inherits `subtitle`
  - Pattern matching: `xxx-title` → inherits `title` (if exists)
  - Falls back to `p` for unknown styles

- [x] **4.5 KFX-Specific Flattening**
  - Unlike CSS cascade, KFX expects complete styles on each element
  - `BuildFragments()` resolves all inheritance before generating fragments
  - Each style fragment contains all properties (no parent_style reference)
  - This ensures Kindle viewers display content correctly

---

## Phase 5: Advanced Features

**Goal**: Handle special CSS features and edge cases.

### Tasks

- [ ] **5.1 Handle @font-face**
  - Parse font-face declarations from CSS
  - Extract font-family, src, font-weight, font-style
  - **Note**: `fb2/stylesheet.go` already extracts @font-face and resolves `url()` refs
  - KFX font embedding requires $264 (font_data) fragments - investigate format
  - Document limitations vs EPUB font embedding

- [ ] **5.2 Page Break Properties**
  - `page-break-before: always` → section break in KFX
  - `page-break-after: always` → section break
  - `page-break-inside: avoid` → keep content together
  - Map to KFX equivalents ($131 first, $132 last for keep_together)

- [ ] **5.3 Drop Cap Support**
  - Parse `.dropcap` and `.has-dropcap` styles
  - Map to KFX dropcap_lines ($125) and dropcap_chars ($126)
  - Handle float: left for drop caps

- [ ] **5.4 Complex Selectors (Limited)**
  - Evaluate need for basic descendant selectors
  - `.poem .verse` - verse inside poem context
  - Document as unsupported if too complex

---

## Phase 6: Validation and Documentation

**Goal**: Ensure robustness and document the feature.

### Tasks

- [ ] **6.1 Create Validation Layer**
  - Warn about unsupported CSS properties (log level: WARN)
  - Warn about complex selectors (log level: DEBUG)
  - Log style mapping decisions (log level: DEBUG)
  - Report statistics: X styles parsed, Y converted, Z warnings

- [ ] **6.2 Update Documentation**
  - Update `docs/stylesheets.md`:
    - Add KFX support section
    - Document CSS property support matrix
    - Document selector support limitations
    - Reference Amazon KDP documentation
  - Add examples of custom stylesheets for KFX
  - Document `@media amzn-kf8` usage

- [ ] **6.3 Add End-to-End Tests**
  - Convert same book to EPUB and KFX
  - Verify style application consistency
  - Test with various stylesheet configurations
  - Test with `default.css` modifications

- [ ] **6.4 Performance Testing**
  - Measure CSS parsing overhead
  - Ensure no regression in conversion speed
  - Cache parsed stylesheets if needed

- [ ] **6.5 Backward Compatibility**
  - Ensure existing KFX output remains valid
  - Feature flag if needed during development
  - Regression tests for existing functionality

---

## File Structure

```
convert/kfx/
├── css/
│   ├── doc.go             # Package documentation
│   ├── types.go           # CSS data structures (CSSValue, CSSRule, CSSStylesheet)
│   ├── parser.go          # CSS text parser
│   ├── parser_test.go     # Parser unit tests
│   ├── mapping.go         # CSS→KFX property symbol mapping
│   ├── units.go           # Unit conversion (em, px, pt → KFX dimension values)
│   ├── values.go          # Keyword value conversion (bold→$361, italic→$382, etc.)
│   ├── converter.go       # CSSRule → StyleDef converter
│   └── converter_test.go  # Converter tests
├── frag_style.go          # (modified) Add NewStyleRegistryFromCSS()
├── generate.go            # (modified) Call CSS-based registry initialization
└── ...

Existing files to reference (not modify in Phase 1):
├── fb2/stylesheet.go      # CSS resource resolution (already working)
├── fb2/types.go           # Stylesheet, StylesheetResource types
└── convert/default.css    # Embedded default stylesheet
```

---

## CSS Property Support Matrix (Implementation Target)

### Fully Supported (Phase 2)
| CSS Property | KFX Symbol | Value Mapping |
|--------------|------------|---------------|
| `font-size` | `$16` | em, px, pt, % → dimension |
| `font-weight` | `$13` | bold→$361, normal→$350, 700→$361 |
| `font-style` | `$12` | italic→$382, normal→$350 |
| `text-align` | `$34` | left→$680, right→$681, center→$320, justify→$321 |
| `text-indent` | `$36` | dimension value |
| `line-height` | `$42` | ratio or dimension |
| `margin-top` | `$47` | dimension value |
| `margin-bottom` | `$49` | dimension value |
| `margin-left` | `$48` | dimension value |
| `margin-right` | `$50` | dimension value |
| `color` | `$19` | RGB color value |

### Partially Supported (Phase 5)
| CSS Property | KFX Symbol | Notes |
|--------------|------------|-------|
| `font-family` | `$11` | Requires font embedding |
| `text-decoration` | `$23`/`$27` | underline/strikethrough only |
| `vertical-align` | `$31` | super/sub only |
| `page-break-*` | `$131`/`$132` | always/avoid only |
| `float` | `$140` | left/right for images/dropcaps |

### Pseudo-elements (Supported)
| Pseudo-element | Support | Notes |
|----------------|---------|-------|
| `::before` | ✅ | `content` property with string values |
| `::after` | ✅ | `content` property with string values |

### Not Supported (Document)
| CSS Property | Reason |
|--------------|--------|
| `background-image` | KFX limitation |
| `border-*` | Complex, rarely used in books |
| `position` | KFX reflowable limitation |
| `transform` | Not applicable to ebooks |
| `animation` | Not supported in KFX |

---

## Risks and Mitigations

| Risk | Impact | Mitigation |
|------|--------|------------|
| CSS parser complexity | High | Use minimal subset, proven library |
| KFX property gaps | Medium | Document limitations, graceful fallback |
| Breaking existing output | High | Feature flag, comprehensive regression tests |
| Performance impact | Low | Lazy parsing, one-time per book |
| Amazon format changes | Medium | Reference official docs, version testing |

---

## Success Criteria

1. ✅ `default.css` rules correctly applied in KFX output
2. ✅ Custom stylesheet paths work for KFX (not just EPUB)
3. ✅ Style names consistent between EPUB and KFX
4. ✅ No regression in existing KFX functionality
5. ✅ Clear documentation of supported CSS subset
6. ✅ Warning logs for unsupported CSS features
7. ✅ Amazon KDP CSS compatibility documented

---

## KPV (Kindle Previewer) Parity Fixes ✅

**Goal**: Make KFX output render correctly on Kindle devices by matching Amazon's Kindle Previewer EPUB-to-KFX conversion patterns.

**Status**: COMPLETED - KFX files now render all content properly on Kindle devices.

### Issues Identified and Fixed

- [x] **Footnote Body Processing**
  - **Problem**: Footnote bodies were being processed as separate storylines (443 storylines for footnotes alone)
  - **Solution**: Skip footnote bodies in main loop, process at end into single consolidated storyline
  - **Files**: `convert/kfx/frag_storyline.go`
  - **Result**: Reduced from 451 storylines to 8 (7 main + 1 footnotes)

- [x] **Footnote Anchor Registration**
  - **Problem**: Links to footnotes failed validation ("Referenced fragment is missing" for `n_XX` anchors)
  - **Solution**: Register footnote section IDs in `idToEID` map during footnote body processing
  - **Files**: `convert/kfx/frag_storyline.go`

- [x] **Image Alt Text ($584)**
  - **Problem**: KPV includes `$584` (alt_text) field for all images, we were missing it
  - **Solution**: Added `AltText` field to `ContentRef`, pass alt text from FB2 source elements
  - **Files**: `convert/kfx/frag_storyline.go`
  - **Symbols**: `SymAltText` ($584)

- [x] **Footnote Link Display ($616/$617)**
  - **Problem**: KPV marks footnote links with `$616: $617` (yj.display: yj.note)
  - **Solution**: Added `IsFootnoteLink` to `StyleEventRef`, detect using `FootnotesIndex` (like EPUB does)
  - **Files**: `convert/kfx/frag_storyline.go`, `convert/kfx/symbols.go`
  - **Symbols**: `SymYjDisplay` ($616), `SymYjNote` ($617)

- [x] **Heading Level Semantics ($790)**
  - **Problem**: KPV includes `$790` (yj.semantics.heading_level) on title elements
  - **Solution**: Added `HeadingLevel` to `ContentRef`, `styleToHeadingLevel()` function maps styles to levels
  - **Files**: `convert/kfx/frag_storyline.go`, `convert/kfx/symbols.go`
  - **Symbols**: `SymYjHeadingLevel` ($790)
  - **Mapping**: `body-title`/`chapter-title` → 1, `h1`-`h6` → 1-6, `section-title` → 2

- [x] **Position Map Format**
  - **Problem**: We used compressed `[base, count]` pairs for EID ranges, KPV uses flat EID lists
  - **Solution**: Changed `compressEIDs()` to return flat sorted EID list
  - **Files**: `convert/kfx/frag_positionmaps.go`

- [x] **Section Page Template Format** (CRITICAL FIX)
  - **Problem**: Our page templates used container type ($270) with layout/float/dimensions
  - **KPV Format**: `{$155: eid, $159: "$269", $176: storyline_name}` (text type, no extra fields)
  - **Our Format**: `{$155: eid, $159: "$270", $176: storyline_name, $140: "$320", $156: "$326", $56: 600, $57: 800}`
  - **Solution**: Simplified `NewPageTemplateEntry()` to match KPV's minimal format with text type ($269)
  - **Files**: `convert/kfx/frag_storyline.go`
  - **Result**: Kindle now renders all storyline content, not just first page per TOC entry

### KFX Structure Comparison (Before/After)

| Component | Before Fix | After Fix | KPV Reference |
|-----------|-----------|-----------|---------------|
| Storylines | 451 | 8 | ~49 (per XHTML file) |
| Page template $159 | $270 (container) | $269 (text) | $269 (text) |
| Page template fields | 7 fields | 3 fields | 3 fields |
| Position map $181 | `[[base, count]]` | `[eid, eid, ...]` | `[eid, eid, ...]` |
| Footnote links | No $616 | $616: $617 | $616: $617 |
| Headings | No $790 | $790: level | $790: level |
| Images | No $584 | $584: alt_text | $584: alt_text |

### Files Modified

- `convert/kfx/frag_storyline.go` - Main content generation, footnote handling, page templates
- `convert/kfx/frag_positionmaps.go` - Position map EID format
- `convert/kfx/symbols.go` - Added new KFX symbols
- `convert/kfx/generate.go` - Pass FootnotesIndex to generateStoryline
- `convert/kfx/generated_sections.go` - Updated Build() calls

---
