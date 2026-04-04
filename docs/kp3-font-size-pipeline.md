# How KP3 Computes Font-Size for Inline Elements (sub/sup/small)

This document describes KP3's end-to-end font-size computation pipeline,
with particular focus on how `<sub>`, `<sup>`, and `<small>` elements
receive their final `font-size` in KFX output — especially when nested
inside headings (`h1`–`h6`).

The findings are based on analysis of the decompiled Kindle Previewer source
and decoded ION data files.

## Overview

KP3 does **not** have any heading-descendant replacement mechanism. There are
no special styles like `h1--sub` or `h2--sup`. Instead, KP3 uses standard
CSS-like inheritance with three pipeline stages:

1. **CSS Parsing** (`com/a/a/Dd.java`) — converts CSS font-size values
   (`smaller`, `25%`, `1.5rem`, etc.) to internal `(value, unit)` pairs.

2. **Style Merger** (`stylelist.ion` rules) — when an element inherits a
   parent's font-size AND has its own font-size, the `YJRelativeRuleMerger`
   combines them multiplicatively for `em` units.

3. **Computed Style Visitor** (`yj/style/b/e.java`) — walks the content tree,
   accumulating font-sizes: `em` values compound with parent, `rem` values
   multiply by the document-data font-size.

4. **Post-Processing Relativization** (`Q/a/d/b/i.java`) — divides each
   element's computed font-size by the "most common" font-size across the
   book, producing a `rem` value where `1rem` = most-common body text size.

The merger (stage 2) runs during style rollup of nested HTML containers.
For inline elements like `<sub>` inside a `<p>`, the sub's properties are
stored as **style events** (inline overrides). Whether the merger fires
depends on the rollup context. For style events that introduce new
properties (not merging with an existing value), the value is stored
directly. The critical multiplication happens in stage 3 (the visitor),
where `em`-unit font-sizes always compound with the parent's accumulated
font-size.

## Stage 1: CSS Parsing

**File:** `com/a/a/Dd.java`, method `c()` at lines 44–87

`Dd.c()` converts CSS font-size values to `(value, unit)` pairs using the
`Ad.a(value, unitType)` factory, where unit type codes come from
`Ad.java` line 4:

| Index | Unit code | CSS unit |
|-------|-----------|----------|
| 1     | in        | `in`     |
| 2     | cm        | `cm`     |
| 3     | mm        | `mm`     |
| 4     | pt        | `pt`     |
| 5     | pc        | `pc`     |
| 6     | px        | `px`     |
| 7     | em        | `em`     |
| 8     | ex        | `ex`     |
| 9     | rem       | `rem`    |

The conversion rules:

### Keywords (case 0 — named values)

| Keyword    | Value | Unit | Source line |
|------------|-------|------|-------------|
| `xx-small` | 7.0   | pt   | 50          |
| `x-small`  | 7.5   | pt   | 52          |
| `small`    | 10.0  | pt   | 54          |
| `medium`   | 12.0  | pt   | 56          |
| `large`    | 13.5  | pt   | 58          |
| `x-large`  | 18.0  | pt   | 60          |
| `xx-large` | 24.0  | pt   | 62          |
| `xxx-large`| 36.0  | pt   | 64          |
| `larger`   | 1.2   | em   | 66          |
| `smaller`  | 0.8333| em   | 68          |

The `smaller` value is exactly `1/1.2 = 0.8333333333333334` (IEEE 754 double).

### Percentage (case 5)

```java
// Dd.java:83-86
case 5:  // PERCENT
    double var3 = var2.c() / 100.0;  // 25% → 0.25
    var1 = Ad.a(var3, 7);            // unit 7 = em
```

**Percentages are converted to `em`**: `font-size: 25%` → `(0.25, UNIT_EM)`.
This is critical — the percentage is lost, and downstream code only sees `em`.

### Dimensioned values (case 1–4, 6–8)

Values with explicit units (`1.5rem`, `16px`, `12pt`) are preserved as-is
with their declared unit.

### Relevance to sub/sup/small

The HTML element handlers set font-size before `Dd.c()` runs:

| Element | Handler file       | CSS equivalent    | After Dd.c()         |
|---------|--------------------|-------------------|----------------------|
| `sub`   | `com/a/a/ahB.java` | `font-size:smaller`| `(0.8333, UNIT_EM)` |
| `sup`   | `com/a/a/ahy.java` | `font-size:smaller`| `(0.8333, UNIT_EM)` |
| `small` | `com/a/a/ahJ.java` | `font-size:smaller`| `(0.8333, UNIT_EM)` |

When user CSS overrides `sub { font-size: 25% }`, the user value replaces
the UA default via the standard CSS cascade. The final parsed value for sub
becomes `(0.25, UNIT_EM)`.

## Stage 2: Style Merger

**File:** `com/amazon/yj/style/merger/rules/YJRelativeRuleMerger.java`

### Rule selection

The `stylelist.ion` data (line 78 in decoded text) defines which merger
handles font-size:

```
YJPropertyKey: "font_size,true,*,em"
YJPropertyKeyClass: "com.amazon.yj.style.merger.rules.YJRelativeRuleMerger"
```

This matches `font_size` where `isInheritable=true`, any value, and
`UNIT_EM`. Since `Dd.java` converts all percentages and relative keywords
to `em`, this rule fires for all CSS-originated font-size values.

### Merger algorithm

`YJRelativeRuleMerger.a()` at lines 17–27:

```java
double var9  = existing.value();   // parent/existing font-size
double var11 = incoming.value();   // child/incoming font-size
u var13 = existing.unit();
u var14 = incoming.unit();
d var15 = new d(var9, var13);      // wraps existing
d var16 = new d(var11, var14);     // wraps incoming

f.a(property, var15, var16, parentCtx, currentCtx);  // normalize units
double result = f.a(var15, var16);                    // multiply
return factory.create(result, var16.unit());           // output
```

Steps:
1. Wrap both values as `(value, unit)` pairs
2. Normalize units if they differ (see below)
3. Multiply: `result = existing × incoming` (for `em` units)
4. Return result in the incoming unit

### Unit normalization

**File:** `com/amazon/yj/style/merger/e/f.java`, lines 57–97

When existing and incoming units differ, normalization converts one to
match the other. The priority list (line 341–345):

```java
a.add(u.UNIT_PERCENT);  // priority 0
a.add(u.UNIT_POINT);    // priority 1
a.add(u.UNIT_EM);       // priority 2
a.add(u.UNIT_PIXEL);    // priority 3
```

**`UNIT_REM` is NOT in this list** (index = -1).

The priority comparison at line 100–102:

```java
private static boolean a(int childIdx, int parentIdx) {
    return parentIdx != -1 && (childIdx == -1 || childIdx >= parentIdx);
}
```

When parent=REM (index=-1) and child=EM (index=2):
- `a(childIdx=2, parentIdx=-1)` → `parentIdx == -1` → returns `false`
- This means: convert the **parent** (REM) to the child's unit (EM)

The conversion function for parent REM → EM (lines 117–146):

```java
// var0=1.5, var2=UNIT_REM, var3=parentCtx, var4=currentCtx, ...
if (var2 == UNIT_PIXEL) return var0;
else if (var2 == UNIT_POINT) return var0 * 1.25;
else if (var2 == UNIT_EM) return var0 * var4.a();  // multiply by context font-size
else if (var2 == UNIT_PERCENT) return var0 * var8 / 100.0;
else if (var2 == UNIT_INCH) ...
else return var0;  // ← REM hits this default: raw value unchanged
```

**REM falls to the default case**: the value is returned as-is (1.5 → 1.5),
and the unit is changed to EM. So `(1.5, REM)` becomes `(1.5, EM)`.

### Multiplication

**File:** `com/amazon/yj/style/merger/e/f.java`, lines 217–229

```java
public static double a(d var0, d var1) {
    // After normalization, both should be same unit
    double var2 = var0.a();  // existing value
    double var4 = var1.a();  // incoming value
    if (var0.unit().equals(UNIT_PERCENT)) {
        return var2 * var4 / 100.0;
    } else {
        return var0.unit().equals(UNIT_EM) ? var2 * var4 : var4;
    }
}
```

For EM: `result = existing × incoming`.

### When the merger fires

The merger operates during **style rollup** when nested HTML containers
are merged. For a `<sub>` inside a `<p>` inside a heading:

- The heading/paragraph container has a font-size (e.g., from UA stylesheet
  or user CSS)
- The `<sub>` creates a **style event** (inline override)
- Whether the merger fires for the style event depends on whether the event
  has a pre-existing font-size to merge with

For style events that are the first to set font-size in their context, the
incoming value is stored directly (no multiplication at this stage). The
multiplication happens later in the computed style visitor (stage 3).

## Stage 3: Computed Style Visitor

**File:** `com/amazon/yj/style/b/e.java`

### Font-size computation (lines 834–861)

```java
private void a(containerId var1, style var2, computedStyle var3) {
    fontSizeValue = var2.get(FONT_SIZE);
    if (fontSizeValue == null) return;  // inherit parent's computed value

    measure = fontSizeValue.measure();

    if (UNIT_EM == measure.unit()) {
        // EM: compound with parent's accumulated font-size
        parentFontSize = var3.get(FONT_SIZE).measure();
        computed = parentFontSize.value() * measure.value();
        var3.set(FONT_SIZE, (computed, parentFontSize.unit()));

    } else if (UNIT_REM == measure.unit()) {
        // REM: absolute, multiply by document-data font-size
        // this.u is from document_data, typically (1.0, UNIT_EM)
        computed = measure.value() * this.u.value();  // e.g., 1.5 * 1.0 = 1.5
        var3.set(FONT_SIZE, (computed, UNIT_EM));

    } else {
        // Unsupported unit (px, pt, etc.): log error, default to 1.0em
        var2.set(FONT_SIZE, this.i);  // this.i = (1.0, UNIT_EM)
    }
}
```

Key properties:

- **Root/initial font-size**: `1.0em` (set at line 88: `this.i = (1.0, UNIT_EM)`)
- **Document-data font-size** (`this.u`): extracted from `document_data` ION
  entity, expected in `UNIT_EM`. Typically `1.0em`. If missing or non-EM,
  defaults to `1.0em` (lines 846–851)
- **EM compounds**: parent `1.5em` × child `0.25em` = `0.375em`
- **REM is absolute**: `1.5rem × 1.0 = 1.5em` (loses the "rem" unit)
- **Unsupported units** log an error and get replaced with `1.0em`

### Call order within the visitor

The visitor method `a()` at line 560–588 processes each container:

| Step | Line | What |
|------|------|------|
| 1 | 574 | LINE_HEIGHT (em/lh/rem → em) |
| 2 | 576 | Copy FONT_FAMILY, FONT_WEIGHT, FONT_STYLE, LANGUAGE |
| 3 | 580 | Color properties |
| 4 | 582 | **FONT_SIZE computation** |
| 5 | 583 | Inline margins/padding → % |
| 6 | 584 | Border weights → % |
| 7 | 587 | TEXT_INDENT → % |

Font-size is computed at step 4, before margins and text-indent, so the
computed font-size is available for margin scaling.

### Style events (inline elements)

**Method `h()` at lines 948–984** processes style events using a stack:

```java
private void h(containerId var1) {
    List events = getStyleEvents(var1);
    Stack<int> offsetStack;
    Stack<computedStyle> styleStack;
    styleStack.push(this.w);  // parent container's computed style

    for (event : events) {
        // Pop any events that ended before this offset
        while (!offsetStack.empty() && offsetStack.peek() < event.endOffset()) {
            offsetStack.pop();
            styleStack.pop();
        }

        // Clone parent computed style and apply this event's properties
        parentComputed = styleStack.peek();
        childComputed = clone(parentComputed);
        apply(containerId, event.style(), childComputed, isNested);

        styleStack.push(childComputed);
        offsetStack.push(event.endOffset());
        computedStyles.put(event, childComputed);
    }
}
```

For a `<sub>` with `font-size: 0.25em` inside a heading paragraph with
accumulated font-size `1.5em`:

1. Parent computed: `FONT_SIZE = (1.5, UNIT_EM)`
2. Sub's style event: `FONT_SIZE = (0.25, UNIT_EM)`
3. Visitor applies EM rule: `1.5 × 0.25 = 0.375`
4. Sub's computed: `FONT_SIZE = (0.375, UNIT_EM)`

This is the standard CSS inheritance behavior: em-based font-sizes are
relative to the parent's computed font-size.

## Stage 4: Post-Processing Relativization

**File:** `com/amazon/Q/a/d/b/i.java`

### Most-common font-size

Before relativization, KP3 walks the entire content tree to find the
**most frequently occurring** computed font-size, weighted by text length.
This is computed by `com/amazon/Q/a/d/a/a.java` (frequency analysis).

For a typical book where body text dominates:
- `mostCommon.FONT_SIZE = (1.0, UNIT_EM)` (standard body text)

### Font-size relativization (method `b()` at lines 531–547)

```java
void b(style var1, originalValue var2, computedValue var3,
       mostCommonValue var4, boolean isParagraph) {

    if (originalValue != null || isParagraph) {
        double computed  = computedValue.measure().value();   // in EM
        double mostCommon = mostCommonValue.measure().value(); // in EM

        // Both MUST be UNIT_EM (assertions at lines 534, 536)

        double ratio = computed / mostCommon;

        // For paragraph-level styles with explicit font-size:
        // if ratio ≈ 1.0 (within 1e-6), REMOVE font-size entirely
        if (|ratio - 1.0| < 1e-6 && isParagraph
            && originalValue != null && style.has(FONT_SIZE)) {
            style.remove(FONT_SIZE);  // elide, it's the "default"
        } else {
            // Store as REM: 1rem = most-common font-size
            style.set(FONT_SIZE, (ratio, UNIT_REM));
        }
    }
}
```

**Critical**: This redefines `rem` to mean "relative to the most-common
font-size", NOT the CSS root font-size. So:

| Element type | Typical ratio | Meaning |
|--------------|--------------|---------|
| Body text    | ~1.0 rem     | Same as most-common |
| Headings     | >1.0 rem     | Larger than body |
| Sub/sup      | <1.0 rem     | Smaller than body |

### Threshold behavior

- Threshold: `p = 1.0E-6` (field at line 35)
- Only applies to **paragraph-level** styles (`isParagraph = true`)
- Style events (inline elements) have `isParagraph = false`, so any
  ratio that isn't **exactly** 1.0 gets stored as REM
- A value like `1.00001` (= 1 + 1e-5) is 10× the threshold, so it
  would NOT be elided even at paragraph level

### Style events path

For style events (inline `<sub>`, `<sup>`, etc.), the relativization is
called from method `a()` at line 278–305:

```java
void a(contentContainer var1) {
    for (styleEvent : var1.styleEvents()) {
        computedStyle = computedStyles.get(event);
        fixedStyle = fixup(computedStyle, event.style(),
                           isParagraph=false, container);
        event.setStyle(fixedStyle);
    }
}
```

The `isParagraph=false` means:
1. The 1e-6 threshold removal never applies
2. Any font-size present in the style event gets relativized

## Worked Example: sub-in-heading

Given:
- User CSS: `sub { font-size: 25%; vertical-align: baseline; }`
- Heading h1 with computed font-size: `1.5em` (from UA stylesheet + user CSS)
- Body text most-common font-size: `1.0em`

### Stage 1: CSS parsing

`font-size: 25%` → `Dd.c()` case 5: `25.0 / 100.0 = 0.25` → `(0.25, UNIT_EM)`

### Stage 2: Style merger

The sub's font-size `(0.25, UNIT_EM)` is stored in the style event. If the
style event doesn't have a pre-existing font-size to merge with, the value
is stored directly (no merger multiplication at this point).

### Stage 3: Computed style visitor

Parent (heading paragraph) accumulated font-size: `(1.5, UNIT_EM)`

Sub's style event: `(0.25, UNIT_EM)`

EM compounding: `1.5 × 0.25 = 0.375` → computed: `(0.375, UNIT_EM)`

### Stage 4: Relativization

`ratio = 0.375 / 1.0 = 0.375` → output: `(0.375, UNIT_REM)` = `0.375rem`

### Actual KP3 output observation

In the test file, the KP3 output shows `font-size: 1.00001rem` for
sub-in-heading. This suggests the actual computation involves values
different from the simplified example above. The `1.00001` value means
the sub's computed font-size is almost exactly equal to the most-common
body font-size. This can happen when:

1. The heading font-size and sub font-factor produce a product close to
   the most-common size (e.g., heading `4.0em` × sub `0.25em` = `1.0em`)
2. IEEE 754 floating-point precision causes a slight deviation from 1.0
3. The ratio doesn't hit the 1e-6 threshold because it's on a style event
   (where the threshold doesn't apply)

The exact value depends on the specific heading font-size in the test file,
the sub's CSS font-size, and any intermediate floating-point rounding. Note
that the heading font-size itself goes through the full pipeline (UA
stylesheet value merged with user CSS, then visitor compounding), so the
accumulated value may not be a round number.

## How fb2cng Differs

### DescendantReplacement mechanism

fb2cng implements a `DescendantReplacement` system that has no equivalent
in KP3. It pre-creates heading-specific inline styles like `h1--sub`,
`h2--sub`, etc. with hardcoded font-sizes. This was designed to handle
the interaction between heading context and inline elements.

**File:** `convert/kfx/style_registry.go`, `DefaultStyleRegistry()` at
lines 563–607

The default descendant replacement styles:

| Style      | Properties |
|------------|-----------|
| `sub`      | `baseline_style: subscript, font_size: 0.75rem` |
| `sup`      | `baseline_style: superscript, font_size: 0.75rem` |
| `small`    | `font_size: 0.8333em` |
| `h1--sub`  | `baseline_style: subscript, font_size: 0.9em` |
| `h2--sub`  | etc. (similar pattern for all h1–h6 × sub/sup/small) |

When a `<sub>` appears inside `<h1>`, `resolveProperties()` (in
`style_context_resolve.go:62-126`) uses `h1--sub` INSTEAD of the user's
merged `sub` style.

**KP3 has NO such mechanism**. It relies entirely on standard CSS
inheritance (em compounding in the visitor).

### Font-size conversion

fb2cng converts font-sizes to `rem` using `PercentToRem()`:

```go
// kp3_units.go:213-220
func PercentToRem(percent float64) float64 {
    if percent > 100 {
        return RoundSignificant(1+(percent-100)/FontSizeCompressionFactor, SignificantFigures)
    }
    return RoundSignificant(percent/100, SignificantFigures)
}
```

For `25%`: `25/100 = 0.25rem`. This is a direct conversion, not the
multi-stage compounding that KP3 performs.

### Impact on sub-in-heading

The current fb2cng output for sub-in-heading: `font-size: 0.40625rem`
- Comes from: heading `1.625rem` × sub `0.25` (user CSS 25%)
- This is the pre-computed product of heading size × sub factor

KP3 output: `font-size: 1.00001rem`
- Comes from: visitor compounding (heading accumulated em × sub em),
  then division by most-common font-size

The discrepancy arises because:
1. fb2cng's `rem` means "fraction of base font-size" (direct conversion)
2. KP3's `rem` means "ratio to most-common font-size" (post-hoc
   relativization)
3. fb2cng pre-multiplies heading × sub during style registry
4. KP3 lets the visitor compound them during tree traversal
5. The heading base font-sizes differ slightly between the two

## KP3 Unit System Reference

### Internal unit enum

**File:** `com/amazon/B/d/e/h/u.java`

| Enum constant  | CSS unit |
|----------------|----------|
| `UNIT_EM`      | `em`     |
| `UNIT_REM`     | `rem`    |
| `UNIT_PERCENT` | `%`      |
| `UNIT_PIXEL`   | `px`     |
| `UNIT_POINT`   | `pt`     |
| `UNIT_INCH`    | `in`     |
| `UNIT_LINE_HEIGHT` | `lh` |
| `UNIT_VW`      | `vw`     |
| `UNIT_VH`      | `vh`     |

### Unit priority for merger normalization

**File:** `com/amazon/yj/style/merger/e/f.java`, lines 341–345

```
[UNIT_PERCENT, UNIT_POINT, UNIT_EM, UNIT_PIXEL]
```

Priority = position in list (lower index = lower priority). Units NOT in the
list (like `UNIT_REM`, `UNIT_LINE_HEIGHT`) get index `-1`.

When units mismatch, the unit with lower priority gets converted to the
higher-priority unit. If both are missing (both -1), an error is logged.

### Conversion functions in the merger

**File:** `com/amazon/yj/style/merger/e/f.java`

**Forward conversion** (lines 117–146) — convert value FROM `var2` unit:

| Source unit | Formula | Notes |
|-------------|---------|-------|
| PIXEL       | `var0` (unchanged) | |
| POINT       | `var0 × 1.25` | |
| EM          | `var0 × contextFontSize` | `var4.a()` = computed px |
| PERCENT     | `var0 × basePercent / 100` | base depends on property |
| INCH        | `var0 × pxPerInch × 1.25` | |
| (default)   | `var0` (unchanged) | **REM hits this** |

**Reverse conversion** (lines 148–183) — convert value TO `var3` unit:

| Target unit | Formula | Notes |
|-------------|---------|-------|
| PERCENT     | `var1 / basePercent × 100` | |
| PIXEL       | `var1` (unchanged) | |
| POINT       | `var1 × 0.75` | |
| EM          | `var1 / contextFontSize` | `var5.a()` = computed px |
| (default)   | `var1` (unchanged) | |

### REM handling summary

Throughout the KP3 pipeline, REM is handled inconsistently:

1. **CSS parsing**: REM values are parsed with their correct unit (type 9)
2. **Merger normalization**: REM is NOT in the priority list, so when REM
   meets EM, the REM value is converted to EM using the **default case**
   (value unchanged, unit replaced). This effectively treats `1.5rem` as
   `1.5em`, which is incorrect but produces reasonable results when the
   document-data font-size is 1.0em.
3. **Computed style visitor**: REM values are handled correctly —
   `value × documentDataFontSize` → result in EM
4. **Post-processing**: All computed EM values are divided by the
   most-common EM value and stored as REM. This REM has a completely
   different meaning ("relative to most-common") than CSS rem.

### KFX output unit codes

| KFX Symbol | CSS Unit | Usage in KFX |
|------------|----------|-------------|
| `$505`     | `rem`    | Font-size (always, after relativization) |
| `$308`     | `em`     | Text-indent, margins, padding |
| `$314`     | `%`      | Margins (sometimes), text-indent (conditionally) |
| `$310`     | `lh`     | Line-height, vertical margins |

## Merger Rules Reference

**File:** `/home/rupor/amazon/data/stylelist.txt`

| Property | Rule | Behavior |
|----------|------|----------|
| `font_size,true,*,em` | `YJRelativeRuleMerger` | Multiplicative: existing × incoming |
| `baseline_style,*` | `YJBaselineStyleRuleMerger` | Special baseline logic |
| `line_height,true` | `YJOverridingRuleMerger` | Incoming replaces existing |
| `text_indent,true` | `YJOverridingRuleMerger` | Incoming replaces existing |
| `margin_left,true,...,false` | `YJCumulativeRuleMerger` | Additive: existing + incoming |
| `margin_top,true,...,true` | `YJOverrideMaximumRuleMerger` | max(existing, incoming) |
| `*,false` | `YJOverridingRuleMerger` | Non-inheritable: incoming replaces |

The key insight: `font_size` uses `YJRelativeRuleMerger` (multiplicative)
only when the incoming unit is `em`. Since `Dd.java` converts percentages
and relative keywords to `em`, this rule fires for all standard CSS
font-size values. REM font-sizes would NOT match this rule (they'd fall
to the `*,false` catchall), but in practice, REM values go through the
computed style visitor path instead of the merger.

## File Index

| File | Role |
|------|------|
| `com/a/a/Dd.java` | CSS font-size parser (keywords, %, units) |
| `com/a/a/Ad.java` | CSS value factory (unit types, `a(value, unitType)`) |
| `com/a/a/ahB.java` | `<sub>` HTML handler (sets font-size: smaller) |
| `com/a/a/ahy.java` | `<sup>` HTML handler (sets font-size: smaller) |
| `com/a/a/ahJ.java` | `<small>` HTML handler (sets font-size: smaller) |
| `com/amazon/yj/style/merger/rules/YJRelativeRuleMerger.java` | Multiplicative merger for font-size |
| `com/amazon/yj/style/merger/e/f.java` | Merger math: unit normalization + multiplication |
| `com/amazon/yj/style/merger/e/g.java` | Property → RET_TYPE mapping (which base to use) |
| `com/amazon/yj/style/merger/d/d.java` | Value+unit wrapper class |
| `com/amazon/yj/style/merger/d/a.java` | Return type enum (RET_WIDTH_PERCENT, RET_FONTSIZE_PERCENT, RET_NONE) |
| `com/amazon/yj/style/b/e.java` | **Computed style visitor** (font-size at lines 834–861) |
| `com/amazon/Q/a/d/b/i.java` | **Post-processing relativization** (font-size at lines 531–547) |
| `com/amazon/Q/a/d/a/a.java` | Most-common style computation (frequency analysis) |
| `com/amazon/B/d/e/h/u.java` | Unit enum (UNIT_EM, UNIT_REM, etc.) |
| `com/amazon/B/d/e/h/s.java` | Property enum (FONT_SIZE, LINE_HEIGHT, etc.) |

### Data files

| File | Content |
|------|---------|
| `/home/rupor/amazon/data/stylelist.txt` | Decoded merger rule definitions |
| `/home/rupor/amazon/data/stylemap.txt` | Decoded CSS→KFX property mappings |
| `/home/rupor/amazon/data/mapping_ignorable_patterns.txt` | Decoded ignorable patterns |

### fb2cng source files

| File | Relevance |
|------|-----------|
| `convert/kfx/style_registry.go` | DescendantReplacement defaults (lines 563–607) |
| `convert/kfx/style_context_resolve.go` | resolveProperties() with descendant logic (lines 62–126) |
| `convert/kfx/style_registry_css.go` | propagateToHeadingDescendants() |
| `convert/kfx/kp3_units.go` | PercentToRem(), font-size compression |
| `convert/kfx/css_values.go` | ConvertVerticalAlign (baseline handling) |
| `convert/kfx/style_merger.go` | mergeBaselineStyle, selectMergeRule |
| `convert/kfx/stylelist.go` | Stylelist rules including YJBaselineStyleRuleMerger |

## Blanket Ignorable Patterns for sub/sup

**File:** `/home/rupor/amazon/data/mapping_ignorable_patterns.txt`,
lines 1147–1158

```
Tag: "sub", Style: "*", Value: "*", Unit: "*"
Tag: "sup", Style: "*", Value: "*", Unit: "*"
```

These blanket patterns cause KP3 to suppress "No Mapping found" warnings
for ANY unmapped CSS property on `<sub>` and `<sup>` elements. However,
they do NOT prevent mapped properties from being processed.

In KP3's architecture (`com/amazon/yjhtmlmapper/e/k.java`, lines 237–255),
the ignorable check only fires in the "no mapping found" error path. CSS
properties that have valid stylemap entries (like `font-size`, `vertical-align`,
`position`, `bottom`) are processed normally regardless of these patterns.

In fb2cng, the `isIgnorable()` guard in `style_mapper_stylemap.go` checks
`KFXPropertySymbol(propName) != SymbolUnknown` — if the property is a known
KFX property, it's exempt from blanket filtering. Properties like `position`
and `bottom` are NOT in `cssToKFXProperty` (they go through the stylemap
path), so they do get caught by the blanket filter on sub/sup elements.
The practical effect is similar — these properties are cosmetic adjustments
that don't significantly affect rendering.
