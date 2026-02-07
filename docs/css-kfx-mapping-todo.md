# CSS-to-KFX Mapping Implementation Status

This document tracks the implementation status of CSS-to-KFX mapping features documented in `css-kfx-mapping.md`, which was derived from KP3 (Kindle Previewer 3) version 3.101.0.

## Transformers

### Fully Implemented

| Transformer | KP3 Purpose | Implementation |
|-------------|-------------|----------------|
| BGColorTransformer | Normalizes `bgcolor`/`background-color`, selects `FILL_COLOR` vs `TEXT_BACKGROUND_COLOR` | `css_converter.go:267-271` - ParseColor + MakeColorValue |
| BGRepeatTransformer | Collapses `background-repeat-x/y` into enumerated codes | `style_mapper_util.go:100-124` - `ConvertBackgroundRepeat()` |
| LineHeightTransformer | Handles `line-height: normal` with locale-aware defaults | `style_mapper_convert.go:10-28` - converts `normal` to 1.2em |
| TextDecorationTransformer | Aggregates underline/overline/line-through with stroke styles | `css_values.go:241-260` - `ConvertTextDecoration()` |
| TextEmphasisStyleTransformer | Maps CSS emphasis shapes to internal tokens | `css_values.go:127-183` - `ConvertTextEmphasisStyle()` |
| WritingModeTransformer | Translates writing-mode keywords | `css_values.go:79-96` - `ConvertWritingMode()` |
| WidowsOrphansTransformer | Converts widows/orphans to `KEEP_LINES_TOGETHER` struct | `style_mapper_convert.go:246-258` |
| BoxShadowTransformer | Parses `box-shadow` tuples into `shadows` list | `style_mapper_convert.go:530-601` - `parseShadows()` |
| TextShadowTransformer | Parses `text-shadow` clauses | `style_mapper_convert.go:530-601` - `parseShadows()` with `isText=true` |
| UserAgentStyleAddingTransformer | Injects UA defaults from stylemap | `style_mapper_stylemap.go:23-25` |
| MaxCropPercentageTransformer | Parses `-amazon-max-crop-percentage` values | `style_mapper_convert.go:450-497` |
| PageBleedTransformer | Maps `-amazon-page-bleed` keywords to `USER_MARGIN_*_PERCENTAGE` | `style_mapper_convert.go:499-528` |
| TextCombineTransformer | Handles `text-combine-upright` for vertical text | `css_values.go:98-108` - `ConvertTextCombine()` |
| BorderRadiusTransformer | Splits two-value `border-*-radius` pairs, emits single or list value | `css_units.go:MakeBorderRadiusValue()` + `style_mapper_convert.go:204-208` |
| ShapeOutsideTransformer | Parses `polygon()` coordinates, validates percent pairs, builds KVG path as flat Ion list | `style_mapper_convert.go` - `parsePolygonPath()` + `convertStyleMapProp()` |

### Partially Implemented

| Transformer | KP3 Purpose | Current Status | Missing |
|-------------|-------------|----------------|---------|
| MarginAutoTransformer | Resolves `margin:auto` to logical properties, checks layout constraints | `style_mapper_convert.go:230-237` - basic `auto` keyword support | Layout constraint checking (floats, abs positioning), `text-align` fallback for centered images |
| XYStyleTransformer | Reconciles `*-x`/`*-y` pairs for background position/size | `style_mapper_convert.go:146-173` | More robust `auto` semantics handling |
| NonBlockingBlockImageTransformer | Sets non-blocking flag for `float:snap-block` on `<img>` | `style_mapper_stylemap.go:224` - basic check | Full tag-specific validation |

### Not Implemented

| Transformer | KP3 Purpose | Priority | Notes |
|-------------|-------------|----------|-------|
| ImageBorderTransformer | Expands comma-delimited presets into multiple border properties | Medium | Needed for image border presets |
| LanguageTransformer | Canonicalizes `lang`/`xml:lang` through locale service, validates BCP-47 | Medium | Only basic passthrough exists (`style_mapper_convert.go:200-203`) |
| LinkStyleTransformer | Extracts color from UA link style entries (`a:link`, `a:visited`, `a:hover`) | Low | Would require pseudo-class support |
| TransformerForWebkitTransform | Flags unsupported `-webkit-transform` usage | Low | Currently only debug log (`style_mapper_core.go:143-146`) |

## Style Merging Rules

### All Implemented

| Merger | KP3 Behavior | Implementation |
|--------|--------------|----------------|
| YJOverridingRuleMerger | Always chooses incoming value, discards existing | `style_merger.go:132-134` - `mergeOverride()` |
| YJCumulativeRuleMerger | Adds measured values after unit reconciliation, deduplicates string lists | `style_merger.go:137-171` - `mergeCumulative()` |
| YJCumulativeInSameContainerRuleMerger | Same as cumulative but asserts same container | `stylelist.go:103-109` with container tracking in `style_context_resolve.go` |
| YJRelativeRuleMerger | Converts relative measures (em/percent) using parent context | `style_merger.go:173-197` - `mergeRelative()` |
| YJOverrideMaximumRuleMerger | Chooses greater magnitude after optional clamping | `style_merger.go:201-215` - `mergeOverrideMaximum()` |
| YJHorizontalPositionRuleMerger | Merges `clear` values (`none/left/right` → `both` semantics) | `style_merger.go:292-312` - `mergeHorizontalPosition()` |
| YJBaselineStyleRuleMerger | Prefers non-`normal` baseline styles | `style_merger.go:218-226` - `mergeBaselineStyle()` |

## CSS Normalization

### Implemented

| Normalization | KP3 Purpose | Implementation |
|---------------|-------------|----------------|
| Zero value stripping | Strips `width:0`, `height:0` etc. to avoid phantom rules | `css_converter.go:39-49` - `zeroValueProps` map + `shouldDropZeroValue()` |
| rem handling | Font-size compression to rem | `kp3_units.go` - `PercentToRem()` |
| ex to em conversion | Converts ex units using 0.44em factor (KP3 `com.amazon.yj.F.a.b:24`) | `css_converter.go` - `normalizeCSSProperties()` + `css_units.go` safety net |
| text-decoration normalization | Removes `text-decoration: none` for non-decoration-control elements; preserves it for `<u>`, `<a>`, `<ins>`, `<del>`, `<s>`, `<strike>`, `<br>` where it has semantic meaning (KP3 `h/b.java:373-395`) | `css_converter.go` - `normalizeCSSProperties()` with `textDecorationControlTags` set |

### Partially Implemented

| Normalization | KP3 Purpose | Current Status | Missing |
|---------------|-------------|----------------|---------|
| rem to em conversion | Converts rem to em using document/node font sizes | Font-size uses rem internally | No document-level rem→em conversion for other properties |

### Not Implemented

| Normalization | KP3 Purpose | Priority | Notes |
|---------------|-------------|----------|-------|
| Table/list property filtering | Removes properties that don't apply to element role | Medium | KP3 uses `h.b.u` and `h.b.t` sets |
| List container zero margins | Ensures list containers get zero top/bottom margins unless explicit | Low | `com.amazon.yjhtmlmapper.e.j.f` |
| box-sizing injection | Injects `box-sizing:border-box` for width/height declarations | Low | May not be needed for FB2 conversion |

## CSS Properties Support

### Fully Supported

**Typography:**
- `font-size`, `font-weight`, `font-style`, `font-family`
- `line-height`, `letter-spacing`, `color`, `background-color`

**Text Layout:**
- `text-indent`, `text-align`
- `text-decoration` (underline, line-through)
- `text-emphasis-style`, `text-emphasis-color`

**Box Model:**
- `margin-top/bottom/left/right` (including shorthand)
- `padding-top/bottom/left/right` (including shorthand)

**Dimensions:**
- `width`, `height`, `min-width`, `max-width`, `max-height`

**Borders:**
- `border-style`, `border-width`, `border-color`
- `border-radius` (single values and two-value elliptical pairs)

**Writing Mode:**
- `writing-mode`, `-webkit-writing-mode`
- `text-orientation`, `text-combine-upright`

**Float/Clear:**
- `float`, `clear`

**Page Breaks:**
- `page-break-before/after/inside`
- `break-before/after/inside`

**Table:**
- `border-collapse`, `border-spacing-vertical/horizontal`

**KFX-Specific:**
- `yj-break-before/after`
- `dropcap-lines/chars`
- `-amazon-max-crop-percentage`
- `-amazon-page-bleed`

### Partial Support

| Property | Status | Notes |
|----------|--------|-------|
| `vertical-align` | Partial | Maps to `baseline_style` or `baseline_shift`, but not all values |
| `white-space` | Partial | Only `nowrap` supported |
| `background` | Partial | Color extraction only, not full shorthand |
| `border` | Partial | Basic width/style/color, not all shorthand forms |

## Stylemap Data

The implementation loads embedded ion files matching KP3's configuration:

| File | Purpose | Status |
|------|---------|--------|
| `data/stylemap.ion` | HTML→KFX property mappings | ✅ Loaded via `ion_data.go` |
| `data/stylelist.ion` | Merge rules per property | ✅ Loaded via `ion_data.go` |
| `data/mapping_ignorable_patterns.ion` | Patterns to skip during mapping | ✅ Loaded via `ion_data.go` |

## Priority TODO List

### High Priority

1. **MarginAutoTransformer completeness**
   - Add layout constraint checking (floats, absolute positioning)
   - Implement `text-align` fallback for centered images

2. ~~**BorderRadiusTransformer two-value pairs**~~ ✅ **DONE**
   - ~~Parse `border-radius: 10px / 20px` syntax~~
   - ~~Split into horizontal/vertical radii~~
   - Implemented in `css_units.go:MakeBorderRadiusValue()` — splits space-separated pairs, emits single dimension or Ion list of two dimensions matching KP3

3. ~~**ShapeOutsideTransformer polygon validation**~~ ✅ **DONE**
   - ~~Parse `polygon()` coordinates~~
   - ~~Validate percent pairs and sum constraints~~
   - Implemented in `style_mapper_convert.go:parsePolygonPath()` — parses polygon(), validates percent units, converts to fractional KVG path as flat Ion list (moveTo/lineTo/closePath commands)

4. ~~**ex to em conversion**~~ ✅ **DONE**
   - ~~Convert `ex` units using 0.5em ratio before mapping~~
   - Implemented in `css_converter.go:normalizeCSSProperties()` using KP3's 0.44 factor

### Medium Priority

5. **ImageBorderTransformer**
   - Implement comma-delimited preset expansion for image borders

6. **LanguageTransformer**
   - Add BCP-47 validation and canonicalization

7. ~~**text-decoration normalization**~~ ✅ **DONE**
   - ~~Skip "remove underline" for `<u>`, `<a>`, `<ins>`, `<del>`, `<s>`, `<strike>`, `<br>`~~
   - Implemented in `css_converter.go:normalizeCSSProperties()` — element-aware stripping of `text-decoration: none` for non-decoration-control tags; preserves it for tags where it has semantic meaning (KP3 `h/b.java:373-395`). For reflowable books (fb2cng's output), `<a>` is always in the control set.

8. **Table/list property filtering**
   - Remove inapplicable properties based on element role (row, cell, caption, etc.)

### Low Priority

9. **LinkStyleTransformer**
   - Would require pseudo-class support (`:link`, `:visited`, `:hover`)

10. **TransformerForWebkitTransform**
    - Emit proper warning instead of debug log

11. **box-sizing injection**
    - May not be needed for FB2 conversion

12. **List container zero margins**
    - Default zero top/bottom margins unless explicitly set

## Architecture Notes

The implementation follows a multi-layer approach similar to KP3:

1. **CSS Parser** (`css_parser.go`) - Parses stylesheet into rules
2. **Style Mapper** (`style_mapper_core.go`, `style_mapper_stylemap.go`) - Applies stylemap lookups
3. **CSS Converter** (`css_converter.go`) - Converts CSS values to KFX symbols/values
4. **Style Merger** (`style_merger.go`, `stylelist.go`) - Merges properties per stylelist rules
5. **Style Registry** (`style_registry*.go`) - Manages final style definitions
6. **Style Context** (`style_context*.go`) - Handles inheritance and cascade resolution

The transformer logic is integrated into conversion functions rather than separate class instances, which is a Go-idiomatic adaptation of KP3's Java class hierarchy.
