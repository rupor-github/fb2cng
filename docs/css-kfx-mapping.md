CSS → KFX Style Mapping Notes
=============================

Derived from: `EpubToKFXConverter-4.0.jar` (Kindle Previewer 3) version 3.101.0

1. Configuration inputs
-----------------------
* `data/stylemap.ion` (loaded via `com.amazon.yjhtmlmapper.e.d`) enumerates every HTML signal that can become a KFX/YJ property.  Each entry uses the ion struct template below:

  ```
  'com.amazon.yj.style.map.entry@1.0'::{
    html_tag: string,
    html_attribute: string,
    html_attribute_value: string,
    html_attribute_value_unit: string,
    yj_property: string,
    yj_value_type: string,
    yj_value: string|list<string>,
    yj_unit: string,
    conversion_factor?: string,
    converter_classname?: string,
    css_styles?: list<struct {style_name:string, style_value:string}>,
    display?: string,
    ignore_for_yj_to_html_mapping?: bool
  }
  ```

  | Field | Purpose |
  | --- | --- |
  | `html_tag` / `html_attribute` / `html_attribute_value` / `html_attribute_value_unit` | Match literal attributes, inline CSS (`style`), or synthesized attributes (e.g., `body-margin-left`). Wildcards and special tags are legal. |
  | `yj_property` | Target KFX property (`com.amazon.B.d.e.h.s`). |
  | `yj_value_type` / `yj_unit` | Encode how to interpret raw values (measure, string, int, bool, composite) and which unit to emit (`px`, `percent`, `color_unit`, etc.). |
  | `yj_value` | Literal value(s) when the HTML side does not supply one (e.g., `<strong>` → `font_weight=ultra_bold`).  Lists are used by transformers. |
  | `conversion_factor` / `converter_classname` | Optional number or helper class used to transform the HTML value prior to emission. |
  | `css_styles` / `display` | Canonical CSS snippet to recreate legacy tags on reverse mapping or to wrap a container with specific layout hints. |
  | `ignore_for_yj_to_html_mapping` | Skip this entry when converting back from KFX to HTML. |

  **Helper `converter_classname` transformers referenced in `stylemap.ion`**

  | Class | Purpose | Typical CSS trigger |
  | --- | --- | --- |
  | `com.amazon.yjhtmlmapper.transformers.BGColorTransformer` | Normalizes legacy `bgcolor`/`background-color` values, selects `FILL_COLOR` vs. `TEXT_BACKGROUND_COLOR`, and rejects unsupported formats (e.g., body `hsl`). | `<body bgcolor=…>`, inline `background-color` needing KFX color tokens. |
  | `com.amazon.yjhtmlmapper.transformers.BGRepeatTransformer` | Collapses `background-repeat`, `background-repeat-x`, and `background-repeat-y` into the enumerated `no_repeat`, `repeat_x`, `repeat_y` codes expected by YJ. | Shorthands like `background-repeat: repeat-x`. |
  | `com.amazon.yjhtmlmapper.transformers.BorderRadiusTransformer` | Splits two-value `border-*-radius` pairs, converts units, and emits the four dedicated `BORDER_RADIUS_*` properties. | `border-top-left-radius`, etc. |
  | `com.amazon.yjhtmlmapper.transformers.ImageBorderTransformer` | Expands comma-delimited stylemap presets into multiple border properties (string vs. measure) for image containers. | Image border presets emitted by stylemap rows listing several `yj_property` names. |
  | `com.amazon.yjhtmlmapper.transformers.LanguageTransformer` | Canonicalizes `lang`/`xml:lang` values through the locale service and writes the normalized tag back into both HTML and KFX. | Any mapping that needs a validated BCP‑47 language token. |
  | `com.amazon.yjhtmlmapper.transformers.LineHeightTransformer` | Handles the special `line-height: normal` case, substituting locale-aware defaults (or `auto` for ruby text) before emitting `LINE_HEIGHT`. | `line-height: normal`. |
  | `com.amazon.yjhtmlmapper.transformers.LinkStyleTransformer` | Extracts the color payload from UA link style entries and synthesizes `color:<hex>` for reverse mapping. | `a:link`, `a:visited`, `a:hover` style injections. |
  | `com.amazon.yjhtmlmapper.transformers.MarginAutoTransformer` | Resolves `margin:auto` into logical start/end properties, checks layout constraints (floats, abs positioning), and falls back to `text-align` for centered images. | `margin-left:auto; margin-right:auto`, logical margin shorthands with `auto`. |
  | `com.amazon.yjhtmlmapper.transformers.MaxCropPercentageTransformer` | Parses 1–4 values, validates they are non-negative and sum ≤ 100 per axis, and emits the composite `MAX_CROP_PERCENTAGE` struct. | `-amazon-max-crop-percentage` style hints for page bleed. |
  | `com.amazon.yjhtmlmapper.transformers.NonBlockingBlockImageTransformer` | Ensures only `<img style="float:snap-block">` entries set the non-blocking flag and skips other tags. | Image floats requesting `snap-block`. |
  | `com.amazon.yjhtmlmapper.transformers.PageBleedTransformer` | Maps keywords (`left/right/top/bottom/all`) to `USER_MARGIN_*_PERCENTAGE = -100` so specified edges extend to the bleed box. | `-amazon-page-bleed: left,right`. |
  | `com.amazon.yjhtmlmapper.transformers.ShapeOutsideTransformer` | Accepts only `polygon()` coordinates in percent, validates each pair, and builds the `BORDER_PATH` point list used for float exclusion zones. | `shape-outside: polygon(…)`. |
  | `com.amazon.yjhtmlmapper.transformers.TextCombineTransformer` | When the layout is vertical‐RL and CSS requests `text-combine-upright`, emits `text_combine=all` plus `writing_mode=horizontal_tb` overrides for the glyph cluster. | `text-combine-upright: all`/`horizontal`. |
  | `com.amazon.yjhtmlmapper.transformers.TextDecorationTransformer` | Aggregates underline/overline/line-through directives and stroke styles (solid/dotted/dashed/double), emitting the per-line YJ properties and pruning redundant inputs. | `text-decoration: underline dotted`, legacy `<u>` overrides. |
  | `com.amazon.yjhtmlmapper.transformers.TextEmphasisStyleTransformer` | Maps CSS emphasis shapes (filled/open circle, sesame, triangle, etc.) or quoted glyphs to the internal emphasis tokens, enforcing regex validation for literal values. | `text-emphasis-style: filled circle` or `'※ '`. |
  | `com.amazon.yjhtmlmapper.transformers.TransformerForWebkitTransform` | Flags unsupported `-webkit-transform` usage—returns no properties for already-filtered absolute elements, otherwise throws `NO_MAPPING_FOR_VALID_STYLE`. | `-webkit-transform` rules that lack a supported KFX equivalent. |
  | `com.amazon.yjhtmlmapper.transformers.UserAgentStyleAddingTransformer` | Injects UA default styles defined in `stylemap.ion`, duplicates value lists per property, remaps logical directions for the current writing mode, and performs unit conversions. | Synthetic UA rows (e.g., defining default margins for `<body>`, `<dd>`, `<center>` wrappers). |
  | `com.amazon.yjhtmlmapper.transformers.WidowsOrphansTransformer` | Converts `widows`/`orphans` counts into the nested `KEEP_LINES_TOGETHER` struct (FIRST/LAST limits) while skipping `<img>` elements. | `widows: 2`, `orphans: 3`. |
  | `com.amazon.yjhtmlmapper.transformers.WritingModeTransformer` | Translates supported writing-mode keywords into `WRITING_MODE`, updates mapper context, and ignores unsupported modes or FLBook content. | `writing-mode: vertical-rl`. |
  | `com.amazon.yjhtmlmapper.transformers.XYStyleTransformer` | Reconciles `*-x`/`*-y` pairs (background position/size) into a single declaration, converting units and handling `auto` semantics. | `background-position-x`, `background-position-y`, `background-size`. |
  | `com.amazon.yjhtmlmapper.transformers.shadow.BoxShadowTransformer` | Parses each `box-shadow` tuple (with up to four arguments), prohibits negative spread, and emits the serialized `shadows` list. | `box-shadow: 0 2px 4px rgba(...)`. |
  | `com.amazon.yjhtmlmapper.transformers.shadow.TextShadowTransformer` | Splits comma-separated `text-shadow` clauses (respecting nested functions), enforces 2–3 arguments, and builds the `text_shadows` descriptor. | `text-shadow: 1px 1px 2px #000, 0 0 1em red`. |
* `data/stylelist.ion` (loaded in `com.amazon.yj.style.merger.d`) selects the **merging rule** used when multiple style sources contribute to the same KFX property.  Each entry matches the template:

  ```
  'com.amazon.yj.htmlstylemerger.yjstylelist.properties@1.0'::{
    YJPropertyKey: "property_name,isMeasure,existingUnit,newUnit,allowWritingModeConvert,sourceIsWrapper,sourceIsContainer,sourceIsInline",
    YJPropertyKeyClass: "fully.qualified.RuleMerger"
  }
  ```

  | Field | Purpose |
  | --- | --- |
  | `property_name` | Target KFX property (e.g., `margin_top`, `font_size`, `text_indent`). |
  | `isMeasure` | `true` if the property stores numeric measures that require unit reconciliation. |
  | `existingUnit` / `newUnit` | Expected units for the currently merged value and the incoming override (`*` means “any”). |
  | `allowWritingModeConvert` | Whether the merger may convert units when writing modes differ. |
  | `sourceIsWrapper` / `sourceIsContainer` / `sourceIsInline` | Booleans (or `*`) indicating which source contexts the rule applies to; used to disambiguate wrappers vs. containers vs. inline tags. |
  | `YJPropertyKeyClass` | One of the merger implementations under `com.amazon.yj.style.merger.rules.*` (cumulative, overriding, relative, baseline, etc.). |

  Each `YJPropertyKey` string supplies comma-delimited toggles; unused slots can be `*` to indicate “don’t care.”  The mapped class determines whether values are summed, overridden, or converted relative to the parent style.

  **Available `YJPropertyKeyClass` implementations**

  | Class | Behavior | Typical properties |
  | --- | --- | --- |
  | `YJOverridingRuleMerger` | Always chooses the incoming value (`var3`) and discards the existing one. | `line_height`, `text_indent`, most non-additive CSS properties. |
  | `YJCumulativeRuleMerger` | Adds measured values after reconciling units; deduplicates string lists. | Margins, paddings, baseline shifts, cumulative layout hints. |
  | `YJCumulativeInSameContainerRuleMerger` | Same as cumulative but asserts both sources share the same container; sums without cross-container adjustments. | Horizontal padding inside identical containers (e.g., left margin accumulation). |
  | `YJRelativeRuleMerger` | Converts relative measures (percent/em) into absolute units using parent context before combining. | `font_size`, other inherited relative measures. |
  | `YJOverrideMaximumRuleMerger` | Chooses whichever value produces the greater extent after optional clamping; can fall back to existing value. | `margin_top`/`margin_bottom` override-max entries, properties with max constraints. |
  | `YJHorizontalPositionRuleMerger` | Special handling for `clear`/float resolution: merges `none/left/right` into `both` semantics. | `CLEAR`. |
  | `YJBaselineStyleRuleMerger` | Prefers non-`normal` baseline styles; ensures both inputs are strings. | `BASELINE_STYLE`. |
* `com.amazon.yjhtmlmapper.e.a` also consumes `data/mapping_ignorable_patterns.ion`.  Entries follow:

  ```
  'mapping_ignorable'::{
    Tag: string,
    Style: string,
    Value: string,
    Unit: string
  }
  ```

  | Field | Purpose |
  | --- | --- |
  | `Tag` | HTML tag name or `*` wildcard. |
  | `Style` | Attribute or CSS property name (supports wildcards). |
  | `Value` | Literal value to ignore (can be `*`).  Used to drop namespace-like patterns (`xmlns:*`, `epub:*`, `amzn*`). |
  | `Unit` | Optional unit constraint; when blank the filter acts regardless of unit. |

  Any combination in this list is skipped during HTML→KFX mapping to suppress warnings for known ignorable metadata.
* The mapper honours `style_mapping_dir` and `style_merger_dir` environment variables to override the default `data/*.ion` search paths.

2. Core objects and entry points
--------------------------------
* `com.amazon.yjhtmlmapper.f.a` describes a single HTML wrapper/container/tag with three parts: the DOM name (`a()`), literal attributes (`b()`), and the computed CSS map (`c()`).  Adapters (see `adapter/common/n/a/d`) build a stack of these objects for every KFX container.
* `com.amazon.yjhtmlmapper.f.c` (implemented by `com.amazon.yjhtmlmapper.e.k`) is the two‑way mapper.  Its forward path (`a(List<f.a>, …)`) turns one or more wrappers into a list of `com.amazon.yjhtmlmapper.f.d` objects (each represents a KFX property + value set).  The reverse path (`a(List<f.d>, f, …)`) uses the same mapping tables to reconstruct HTML when needed.
* `com.amazon.yjhtmlmapper.e.c` converts a wrapper’s CSS map into a list of `com.amazon.yjhtmlmapper.e.c` keys (`html_tag`, `html_attribute`, value/unit metadata) that can be matched against `stylemap.ion`.  It expands every CSS declaration into:
  - Tag‑level matches (the element itself),
  - Explicit attribute matches (`style` or legacy attributes such as `align`),
  - Synthetic “special_tag” matches that allow stylemap entries to fire for global properties like `dir`.
* `com.amazon.yjhtmlmapper.e.k` loads every stylemap entry into two hash maps: HTML key → KFX definition (used for `DefaultStyleTransformer`) and the reverse (used when emitting HTML).  During mapping it:
  - Validates each HTML key against the content feature catalog (`com.amazon.yjhtmlmapper.b.c`) to make sure the current wrapper/container/tag is legal for the active layout mode.
  - Instantiates either `DefaultStyleTransformer` (for single property copies) or one of the specialized transformers under `com.amazon.yjhtmlmapper.transformers.*` (for compound CSS such as text-decoration, backgrounds, writing-mode, etc.).
  - Specialized transformers share the abstract base `SpecialStyleTransformer`, which itself extends `StyleTransformer` but threads two additional collaborators into each helper: the `StyleMapperContext` (`com.amazon.yjhtmlmapper.c.c`) and the set of pending style rows associated with the current wrapper.  This lets every helper reuse the same HTML/KFX style handles while also querying document-level layout data (writing mode, element role, parent stacks) or pruning sibling CSS keys when emitting compound properties.
* `com.amazon.yjhtmlmapper.g.a` post‑processes the emitted `f.d` list so it can be consumed by the style merger:
  - Deduplicates properties (latest wins unless `com.amazon.yjhtmlmapper.i.b.bN` marks them as composite).
  - Converts percentage based width/height into viewport units when wrappers request page‑relative alignment.
  - Drops unsupported properties for special tags (e.g., `TEXT_COLOR` on `<img>`), fills list/table defaults, and enforces opacity bounds.
* `com.amazon.yj.style.merger.b` is the final stage that injects the mapped KFX properties into the YJ document (`com.amazon.B.d.f.c`).  It consults `com.amazon.yj.style.merger.d` to pick the right rule class per property and perform cross‑container merges (wrappers + containers + inline tags).

3. Normalization before lookup
------------------------------
The mapper aggressively normalizes CSS so wrappers, containers, and inline tags use the same units and the stylemap can match deterministic keys:

* `com.amazon.yjhtmlmapper.h.b.a(f.a, …)` orchestrates cleanup:
  - Converts `rem` to `em` (needs both document font size and computed font size for the node).
  - Converts `ex` to `em` using `com.amazon.yjhtmlmapper.i.c`.
  - Strips zero inline sizes so properties such as `width:0` don’t create phantom rules.
  - Removes table/list/table-child properties that do not apply to the current element (see the `u` and `t` sets in `h.b`).
  - Normalizes `text-decoration` so “remove underline” is ignored for tags that always control their own decoration (`<u>`, `<a>`, `<ins>`, `<del>`, `<s>`, `<strike>`, `<br>`).
  - Ensures list containers get zero top/bottom margins unless explicitly set (`com.amazon.yjhtmlmapper.e.j.f`).
* `com.amazon.yjhtmlmapper.c.c` tracks document‑level context: block/inline writing mode, text direction, whether we are inside the root body, current computed styles (`c.b`), and whether wrappers already forced `display`/`box-sizing`.  It lets `h.b` decide when (for example) `box-sizing:border-box` must be injected for width/height declarations.
* `com.amazon.yjhtmlmapper.e.b` inspects the wrapper stack to detect layout situations:
  - `a() / b()` report whether width/height are still `auto`.
  - `o()…w()` expose table sub roles (row, column, caption, etc.).
  - Flags like `i()` (float), `f()` (absolute positioning), `x()` (orthogonal writing mode) are later consumed by `g.a` when deciding whether to add `FIT_WIDTH`, `FIT_TIGHT`, or viewport units.

4. Mapping and transformers
---------------------------
* For each normalized HTML key, `e.k` looks up the corresponding `e` definition and picks a transformer:
  - `DefaultStyleTransformer` simply copies/scales the value using the metadata from `e` (value type, unit, optional conversion factor).
  - Specialized transformers (TextDecoration, BGColor, MarginAuto, WritingMode, XYStyle, BorderRadius, ShapeOutside, WidowsOrphans, etc.) interpret multi-token CSS values, emit multiple KFX properties, or mutate additional wrappers.
* When a stylemap entry declares `css_styles`, `e.d`/`e.l` wrap them into `com.amazon.yjhtmlmapper.f.a` instances so wrappers (e.g., `<center>`, `<address>`, `<u>`) can be re-created with known inline CSS.
* Examples pulled from `stylemap.txt`:
  - `body` + `body-margin-left` in `%|px|pt|em|...` map to `conv.body_margin_left` measured properties; the same attribute suffixed with `-importance` maps to `conv.body_margin_left_importance` with value type `string`.
  - `<center>` with no explicit attribute maps directly to `text_alignment = center` and also records `css_styles = text-align:center`, `display:block` so wrappers built later inherit the same visual effect.
  - Inline emphasis tags (`<strong>`, `<em>`, `<cite>`, `<var>`, `<tt>`, `<code>` etc.) map to `font_weight`, `font_style`, or `font_family` string properties with optional CSS hints for reverse mapping.
  - Definition list defaults (e.g., `<dd>`) rely on `UserAgentStyleAddingTransformer` to inject `margin-left:40px`, because the CSS is not always present in the source but is required for layout parity.
* `com.amazon.yjhtmlmapper.e.j` (accessed through `com.amazon.yjhtmlmapper.f.c.b()`) decides whether wrappers should be emitted as `<div>`, `<span>`, or list/table containers.  It compares `display`/CSS maps across existing wrappers so identical combinations can be reused and so inline wrappers remain `<span>` while block wrappers default to `<div>`.

5. Wrappers vs. containers vs. inline tags
------------------------------------------
* **Wrappers**: Every KFX container (TEXT/BOX/IMAGE/etc., see `com.amazon.yjhtmlmapper.f.f`) can require synthetic wrappers even if the original HTML did not declare one.  `adapter/common/n/a/d` builds the initial `f.a` list (often just the source element), and `e.j` may replace it with a canonical `<div>`/`<span>` wrapper whose CSS map is composed of:
  - Inline CSS from the source element,
  - `css_styles` pulled directly from stylemap (so legacy tags like `<center>` maintain behavior),
  - Mapper injected values (e.g., `display:block` for text containers, zeroed list margins, etc.).
  Wrappers are validated against `com.amazon.yjhtmlmapper.b.c` so unsupported style/tag combinations raise `NON_VALIDATED_STYLE_USED` or `UNSUPPORTED_CONTENT_FEATURE` warnings early.
* **Containers**: `com.amazon.yjhtmlmapper.c.b` remembers the computed styles for the current container stack so descendant wrappers can inherit or reuse them.  `g.a` adds container-only KFX properties such as `FIT_WIDTH`, `FIT_TIGHT`, `BOX_ALIGN`, `TABLE_COLUMN_SPAN`, and the viewport-relative width/height substitutions when percentage margins are used in page-aligned containers.  Table-related scrubbing (`h.b.u`, `h.b.t`) ensures container wrappers only advertise properties that actually apply to their role (row, cell, caption, etc.).
* **Individual tags**: Pure inline styles (e.g., `<strong style="color:red">`) are expanded by `e.c` into exact attribute/value/unit tuples so the corresponding `stylemap` row fires.  If the HTML key contains `special_tag`, the mapper knows to look up synthetic entries (for `dir`, `lang`, `tabindex`, etc.).  The resulting KFX properties are merged using the rule specified in `stylelist`:
  - Margins/paddings rely on `YJCumulativeRuleMerger` or `YJCumulativeInSameContainerRuleMerger` to sum values (after unit conversion via `com.amazon.yj.style.merger.e.f`).
  - Logical font sizes use `YJRelativeRuleMerger` to resolve `em`/`percent` chains.
  - Baseline shifts use `YJCumulativeRuleMerger` with percentage-aware handling (`baseline_shift,true,percent,percent`).
  - Clear/floats go through `YJHorizontalPositionRuleMerger`, which collapses `none/left/right` pairs into the expected `both` semantics.
  - Line-height, padding/margin overrides, background offsets, etc., fall back to `YJOverridingRuleMerger`.

6. Putting it together
----------------------
1. Adapter code assembles one or more `f.a` wrappers for each KFX container, capturing both source CSS and any synthetic hints already known for that container.
2. `h.b` and `e.c` normalize those wrappers into deterministic HTML keys and CSS maps, injecting/removing declarations so only valid combinations reach the mapper.
3. `e.k` walks through every normalized key, uses `stylemap.ion` + (optional) `SpecialStyleTransformer` to emit the canonical list of KFX properties (`f.d`).  The same stage records any additional wrappers (`css_styles`) that should wrap the KFX container on reverse-mapping.
4. `g.a` scrubs/augments the property list based on container context (viewport alignment, list/table semantics, image exceptions) and enforces numeric bounds.
5. `com.amazon.yj.style.merger.b`, guided by `stylelist.ion`, merges container-level styles, wrapper styles, and inline tag styles so the final YJ/KFX style set respects CSS cascade semantics (sum margins/paddings, choose larger of competing auto margins, keep explicit overrides, etc.).

7. Quick extraction from KP3 configs (`stylemap.txt`, `stylelist.txt`)
---------------------------------------------------------------------
* `converter_classname` entries present: `UserAgentStyleAddingTransformer`, `BGColorTransformer`, `BGRepeatTransformer`, `BorderRadiusTransformer`, `ImageBorderTransformer`, `LineHeightTransformer`, `MarginAutoTransformer`, `MaxCropPercentageTransformer`, `NonBlockingBlockImageTransformer`, `PageBleedTransformer`, `ShapeOutsideTransformer`, `TextCombineTransformer`, `TextDecorationTransformer`, `TextEmphasisStyleTransformer`, `TransformerForWebkitTransform`, `WritingModeTransformer`, `XYStyleTransformer`, `WidowsOrphansTransformer`, `LanguageTransformer`, shadow transformers (`shadow.BoxShadowTransformer`, `shadow.TextShadowTransformer`), `LinkStyleTransformer`. Many rows include `css_styles` to inject UA defaults (e.g., body/dd/center wrappers).
* `stylelist` mergers observed: `YJHorizontalPositionRuleMerger` for `yj.float_clear`; cumulative for `layout_hints`; margins/paddings mix of cumulative/override/override-maximum with a special cumulative-in-same-container for `margin_left`; `font_size` relative (em) and catch-all `*,true,*,percent` relative; `baseline_shift` cumulative; default override for wildcard entries.
* Rule behaviors confirmed in KP3 Java:
  - `YJCumulativeRuleMerger` reconciles units (including writing-mode conversions when enabled), sums measures, and deduplicates string lists; the “same container” variant asserts identical parents and just sums.
  - `YJRelativeRuleMerger` converts the incoming value using the parent context before combining (e.g., percent → px against parent size).
  - `YJOverrideMaximumRuleMerger` picks the greater magnitude after optional clamping, keeping the original when equal; unit mismatches fall back to parent-aware conversion when allowed.
  - `YJHorizontalPositionRuleMerger` collapses `clear` pairs so `none` + `left/right` resolves to the non-`none` side, conflicting sides become `both`.
  - `YJBaselineStyleRuleMerger` prefers non-`normal` values; otherwise the incoming value wins.
* Raw `stylelist.txt` entries (KP3 3.101.0) for precise wiring:
  - `yj.float_clear,false,*,*,true` → `YJHorizontalPositionRuleMerger`
  - `layout_hints,false` → cumulative
  - `margin_top,true,*,*,*,*,true` → cumulative; `margin_top,true,*,*,true` → override-maximum; `margin_top,true` → override
  - `margin_bottom,true,*,*,*,true` → override-maximum; `margin_bottom,true` → override
  - `margin_left,true,*,*,*,*,*,false` → cumulative; `margin_left,true,*,*,*,*,*,true` → cumulative-in-same-container; `margin_right,true` → cumulative
  - `padding_top,true,*,*,*,*,true` → cumulative; `padding_top,true,*,*,true` → cumulative; `padding_top,true` → override
  - `padding_bottom,true,*,*,*,true` → cumulative; `padding_bottom,true` → override; `padding_left/right,true` → cumulative
  - `baseline_style,*` → baseline-style rule
  - `font_size,true,*,em` and catch-all `*,true,*,percent` → relative
  - `baseline_shift,true,percent,percent` → cumulative
  - Wildcards: `*,false` → override (non-measure defaults); `*,true` → override (measure defaults)

This pipeline ensures wrappers, containers, and individual tags all travel through the same normalization + mapping machinery, but the configuration files let us fine-tune (a) which CSS signals produce which KFX properties, and (b) how competing declarations are merged once they land in the YJ document.
