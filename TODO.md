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
5. Instead of gofmt use `goimports-reviser -format -rm-unused -set-alias
   -company-prefixes github.com/rupor-github -excludes vendor ./...` - it is
   always available
6. Module path is `fbc` (not `fb2cng`) - use `fbc/...` for imports

## Selector Coverage Policy (EPUB parity)

We must support both:

1) **Class selectors** (e.g. `.epigraph`, `.link-footnote`) because EPUB generation attaches semantic classes.

2) **Element selectors** (e.g. `p`, `h1`..`h6`, `code`, `table`, `th`, `td`) because EPUB output also relies on plain tags, and user stylesheets may target tags without classes.

3) **Inline tag selectors used by EPUB generator** (no classes): `strong`, `em`, `del`, `sub`, `sup`, `a`, `img`, `span`, `div`, `blockquote`.

Supported selector forms (initial target):
- `tag` (e.g. `p`, `h1`, `strong`)
- `.class` (e.g. `.paragraph`, `.epigraph`)
- `tag.class` (e.g. `blockquote.cite`, `p.has-dropcap`)

Everything else is ignored with warning (descendant/sibling/pseudo selectors).

### Selector → KFX style name resolution

- `.foo` → style name `foo`
- `tag` → style name `tag` (unless we define an explicit alias, see below)
- `tag.foo` → style name `foo` (preferred) with fallback to `tag.foo` only if needed to avoid collisions

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
- Advanced selectors: `::after`, `::before`, `:first-letter`, `:first-line`
- Adjacent sibling selectors (`E + F`, `E ~ F`)
- CSS Grid, Flexbox (very limited)
- Transitions, animations
- Media queries (basic support only)

### Media Queries

Per **Media Query Policy** above: `@media` blocks are ignored.

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

## Phase 1: CSS Parser Foundation

**Goal**: Create a CSS parser that extracts rule information relevant to KFX conversion.

### Tasks

- [ ] **1.1 Research CSS Parsing Libraries**
  - Evaluate Go CSS parsing options:
    - `github.com/gorilla/css` (simple, maintained) - **ARCHIVED, not recommended**
    - `github.com/vanng822/css` (more complete) - **Good candidate**
    - `github.com/andybalholm/cascadia` (selector-focused, no property parsing)
    - `github.com/tdewolff/parse/v2/css` (fast tokenizer, low-level)
    - Custom minimal parser (if libraries are overkill)
  - Decision criteria: minimal dependencies, handles our subset
  - **Note**: `fb2/stylesheet.go` already has regex for `@font-face`, `url()`, `@import` - could extend

- [ ] **1.2 Define Supported CSS Subset**
  - Document which CSS properties we support (based on `default.css` + Amazon KDP support):
    ```
    /* Typography - Fully Supported */
    font-size, font-weight, font-style, font-family, font-variant
    text-indent, text-align, text-decoration, text-transform
    line-height, letter-spacing, word-spacing
    color, vertical-align, white-space
    
    /* Box Model - Fully Supported */
    margin, margin-top, margin-bottom, margin-left, margin-right
    padding, padding-top, padding-bottom, padding-left, padding-right
    
    /* Display/Layout - Partial */
    display (block, inline, none)
    float (left, right)
    
    /* Page Control - Supported */
    page-break-before, page-break-after, page-break-inside
    ```
  - Document which selectors we support:
    - Element selectors: `p`, `h1`-`h6`, `blockquote`, `table`, `code`, `th`, `td`, etc.
    - Class selectors: `.paragraph`, `.epigraph`, `.verse`
    - Combined: `p.has-dropcap`, `div.image`
    - **Grouped selectors**: `h2, h3, h4, h5, h6` (line 52-57 of default.css)
  - Document what we ignore (per Amazon limitations):
    - Complex selectors (descendant like `p code`, sibling, pseudo-elements)
    - Most `@media` queries (except `@media amzn-kf8`)
    - CSS counters, animations, transitions
    - Grid, flexbox
  - **Analysis of default.css needed**: Count selectors by type, identify any edge cases

- [ ] **1.2.1 Analyze default.css Selectors**
  - Create inventory of all selectors in `convert/default.css`:
    - Simple element selectors (e.g., `p`, `h1`, `code`, `table`, `th`, `td`)
    - Simple class selectors (e.g., `.dropcap`, `.body-title`, `.epigraph`)
    - Combined element.class (e.g., `p.has-dropcap`, `span.has-dropcap`, `blockquote.cite`)
    - Grouped selectors (e.g., `h2, h3, h4, h5, h6`)
    - **Descendant selectors** (e.g., `p code`, `.section-title h2.section-title-header`) - decide handling
    - img-prefixed classes (e.g., `img.image-block`, `img.image-inline`)
  - Count: ~80+ rules in default.css, identify which need special handling
  
- [ ] **1.3 Create CSS Types**
  - File: `convert/kfx/css/types.go`
  - Types needed:
    ```go
    type CSSValue struct {
        Value    float64
        Unit     string  // "em", "px", "%", "pt", etc.
        Keyword  string  // "bold", "italic", "center", etc.
        Raw      string  // Original CSS value string
    }
    
    type CSSRule struct {
        Selector   string              // ".paragraph", "h1", etc.
        Properties map[string]CSSValue // "font-size" -> {1.2, "em", "", "1.2em"}
    }
    
    type CSSStylesheet struct {
        Rules     []CSSRule
        FontFaces []CSSFontFace   // @font-face declarations
        // NOTE: @media blocks are always ignored, so we do not represent them.
    }
    
    type CSSFontFace struct {
        Family string
        Src    string
        Style  string // normal, italic
        Weight string // normal, bold, 400, 700
    }
    ```

- [ ] **1.4 Implement CSS Parser**
  - File: `convert/kfx/css/parser.go`
  - Parse CSS text into `CSSStylesheet`
  - Extract relevant rules, skip unsupported constructs
  - Ignore all `@media ... {}` blocks
  - Handle comments, whitespace, basic syntax

- [ ] **1.5 Add Unit Tests**
  - File: `convert/kfx/css/parser_test.go`
  - Test parsing of `default.css` (use `//go:embed` to load actual file)
  - Test that all `@media` blocks are ignored (content inside is skipped)
  - Test grouped selectors (`h2, h3, h4`) are split into separate rules
  - Test edge cases (empty rules, malformed CSS, comments)
  - Test extraction of specific properties

---

## Phase 2: CSS-to-KFX Property Mapping

**Goal**: Create a mapping layer that converts CSS properties to KFX style properties.

### Tasks

- [ ] **2.1 Define CSS-to-KFX Property Map**
  - File: `convert/kfx/css/mapping.go`
  - Create property name mapping based on Amazon's supported properties:
    ```go
    var cssToKFXProperty = map[string]int{
        // Typography
        "font-size":       SymFontSize,       // $16
        "font-weight":     SymFontWeight,     // $13
        "font-style":      SymFontStyle,      // $12
        "font-family":     SymFontFamily,     // $11
        "line-height":     SymLineHeight,     // $42
        "letter-spacing":  SymLetterspacing,  // $32
        "color":           SymTextColor,      // $19
        
        // Text Layout
        "text-indent":     SymTextIndent,     // $36
        "text-align":      SymTextAlignment,  // $34
        
        // Box Model
        "margin-top":      SymMarginTop,      // $47
        "margin-bottom":   SymMarginBottom,   // $49
        "margin-left":     SymMarginLeft,     // $48
        "margin-right":    SymMarginRight,    // $50
        "padding":         SymPadding,        // $51
        
        // Text Decoration
        "text-decoration": -1, // Special handling: underline->$23, line-through->$27
        "vertical-align":  SymBaselineShift,  // $31
    }
    ```

- [ ] **2.2 Implement Unit Conversion**
  - File: `convert/kfx/css/units.go`
  - Convert CSS units to KFX dimension values:
    ```go
    func CSSValueToKFX(css CSSValue) (value float64, unit int, err error) {
        switch css.Unit {
        case "em":
            return css.Value, SymUnitEm, nil       // $308
        case "ex":
            return css.Value, SymUnitEx, nil       // $309
        case "%":
            return css.Value / 100, SymUnitRatio, nil // $310 (percent->ratio)
        case "px":
            return css.Value, SymUnitPx, nil       // $319
        case "pt":
            return css.Value, SymUnitPt, nil       // $318
        case "cm":
            return css.Value, SymUnitCm, nil       // $315
        case "mm":
            return css.Value, SymUnitMm, nil       // $316
        case "in":
            return css.Value, SymUnitIn, nil       // $317
        case "":
            // Unitless - could be ratio for line-height
            return css.Value, SymUnitRatio, nil
        default:
            return 0, 0, fmt.Errorf("unsupported unit: %s", css.Unit)
        }
    }
    ```

- [ ] **2.3 Implement Value Conversion for Complex Properties**
  - File: `convert/kfx/css/values.go`
  - `font-weight`: 
    - CSS: bold, bolder, lighter, normal, 100-900
    - KFX: $361 (bold), $362 (semibold), $363 (light), $364 (medium), $350 (normal)
  - `font-style`:
    - CSS: italic, oblique, normal
    - KFX: $382 (italic), $350 (normal)
  - `text-align`:
    - CSS: left, right, center, justify, start, end
    - KFX: $680 (start), $681 (end), $320 (center), $321 (justify)
  - `text-decoration`:
    - CSS: underline → KFX $23 (underline)
    - CSS: line-through → KFX $27 (strikethrough)
    - CSS: none → clear both
  - `vertical-align`:
    - CSS: super → KFX baseline_shift positive
    - CSS: sub → KFX baseline_shift negative
  - `display`:
    - CSS: none → KFX visibility handling
    - CSS: block, inline → KFX render mode ($601/$602)
  - `page-break-*`:
    - CSS: always → KFX $352 (always)
    - CSS: avoid → KFX $353 (avoid)

- [ ] **2.4 Handle Shorthand Properties**
  - `margin: 1em` → expand to all four margins
  - `margin: 1em 2em` → top/bottom, left/right
  - `margin: 1em 2em 3em 4em` → top, right, bottom, left
  - Same for `padding`, `border`

- [ ] **2.5 Create Style Converter**
  - File: `convert/kfx/css/converter.go`
  - Function: `ConvertCSSRuleToStyleDef(rule CSSRule) (StyleDef, []string)`
  - Returns StyleDef and list of warnings for unsupported properties
  - Apply property mapping
  - Handle compound properties
  - Handle shorthand expansions

- [ ] **2.6 Add Unit Tests**
  - File: `convert/kfx/css/converter_test.go`
  - Test individual property conversions
  - Test shorthand property expansion
  - Test keyword value mapping
  - Test edge cases and error handling

---

## Phase 3: Stylesheet Integration

**Goal**: Integrate CSS parsing into the KFX generation pipeline.

### Tasks

- [ ] **3.1 Modify StyleRegistry**
  - File: `convert/kfx/frag_style.go`
  - Add method: `RegisterFromCSS(stylesheet *CSSStylesheet)`
  - Parse CSS rules and create corresponding `StyleDef` entries
  - Maintain style name mapping (CSS selector → KFX style name)
  - Handle `@media amzn-kf8` rules with priority

- [ ] **3.2 Create CSS-Aware Default Registry**
  - New function: `NewStyleRegistryFromCSS(css []byte) (*StyleRegistry, error)`
  - Parse the stylesheet
  - Build registry from CSS rules
  - Merge with hardcoded defaults for any missing essential styles
  - Return warnings for unsupported CSS features

- [ ] **3.3 Selector-to-StyleName Mapping**
  - CSS `.paragraph` → KFX style name `paragraph`
  - CSS `p` → KFX style name `p` (element styles)
  - CSS `h1` → KFX style name `h1`
  - CSS `.body-title-header` → KFX style name `body-title-header`
  - Handle selector specificity for combined selectors

- [ ] **3.4 Update KFX Generator**
  - File: `convert/kfx/generate.go`
  - In `buildFragments()` (around line 88-89):
    - Replace `styles := DefaultStyleRegistry()` with CSS-based initialization
    - Access stylesheet from `c.Book.Stylesheets` (already normalized by `fb2/stylesheet.go`)
    - Create `StyleRegistry` from CSS using new function
    - Fall back to `DefaultStyleRegistry()` if CSS parsing fails
    - Log warnings for unsupported CSS
    - Ensure all referenced styles exist

- [ ] **3.5 Handle Style Name Normalization**
  - CSS class `.body-title-header` → KFX style `body-title-header`
  - Ensure style names are valid KFX symbols
  - Handle collisions and reserved names
  - Create symbol registration for custom style names

- [ ] **3.6 Add Integration Tests**
  - Test full pipeline with `default.css`
  - Test with custom stylesheet
  - Verify style fragments match expected KFX structure
  - Compare styling results with EPUB output

---

## Phase 4: Style Inheritance and Cascading

**Goal**: Implement proper style inheritance to match CSS cascade behavior.

### Tasks

- [ ] **4.1 Implement Parent Style Support**
  - Use KFX `$158` (parent_style) for inheritance
  - Map CSS selector relationships to inheritance:
    - `.verse` could inherit from `.poem` (if defined)
    - `h2` inherits base heading properties
  - Handle element.class patterns (e.g., `p.verse` inherits from `p`)

- [ ] **4.2 Create Inheritance Resolution**
  - Build inheritance tree from CSS selectors
  - Element styles as base (p, h1, div, etc.)
  - Class styles override element styles
  - Generate minimal style definitions (only overridden properties)

- [ ] **4.3 Handle Base Styles**
  - `paragraph` style as base for text elements
  - `h1`-`h6` hierarchy with decreasing sizes
  - Container styles for structural elements
  - Ensure all used styles have definitions

- [ ] **4.4 Specificity Handling**
  - Simple specificity: element < class < element.class
  - Later rules override earlier (source order)

- [ ] **4.5 Add Tests for Inheritance**
  - Test property inheritance chains
  - Test override behavior
  - Test multi-level inheritance
  - Test specificity ordering

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

### Not Supported (Document)
| CSS Property | Reason |
|--------------|--------|
| `background-image` | KFX limitation |
| `border-*` | Complex, rarely used in books |
| `position` | KFX reflowable limitation |
| `transform` | Not applicable to ebooks |
| `animation` | Not supported in KFX |
| `::before`/`::after` | Pseudo-elements not supported |

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
