# How Amazon's Kindle Previewer Converts `em` to `%`

This document describes how and why CSS `em` units on horizontal margins
(`margin-left`, `margin-right`) and `text-indent` are converted to percentage
values during EPUB-to-KFX conversion by Amazon's Kindle Previewer.

The findings are based on analysis of the decompiled Kindle Previewer source
(`/home/rupor/amazon/src`) and the Calibre KFXOutput/KFXInput plugins.

## Overview

The conversion is **hardcoded and unconditional** for reflowable content. There
is no configuration, flag, or condition that preserves `em` units for horizontal
margins or `text-indent`. By the time KFX data is written, these values are
always stored as `$314` (percentage).

## The Core Conversion Function

**File:** `com/amazon/yj/F/a/b.java`, lines 160-210

This is the single function that converts all length units to `UNIT_PERCENT`:

```java
// Line 161: output unit is ALWAYS percent
u var9 = u.UNIT_PERCENT;

switch (var1) {   // input unit
    case UNIT_EM:
        var8 = var0 * 100.0 / var2;     // em -> %
        break;
    case UNIT_PIXEL:
        var8 = var0 * 100.0 / var10;    // px -> %
        break;
    case UNIT_POINT:
        var8 = var0 * 100.0 / var11;    // pt -> %
        break;
    case UNIT_PERCENT:
    case UNIT_VW:
    case UNIT_VH:
        var8 = var0;                     // already relative, pass through
        var9 = var1;                     // keep original unit
        break;
}
```

### Formula

For `em` the formula is:

    percent = em_value * 100.0 / parent_width_in_ems

Where `parent_width_in_ems` is a constant:

| Writing mode    | Value | Source              |
|-----------------|-------|---------------------|
| `horizontal-tb` | 32.0  | `b.java` field `b`  |
| vertical modes  | 40.0  | `b.java` field `c`  |

Other constants used for `px` and `pt` conversion:

- Default font size: **16.0 px** (field `i`)
- Points per inch: **72.0** (field `j`)
- DPI for relative units: **96.0** (`CONVERSION_TO_RELATIVE_UNITS`)

### Examples

| Input          | Writing mode    | Output   |
|----------------|-----------------|----------|
| `2em`          | horizontal-tb   | `6.25%`  |
| `1.5em`        | horizontal-tb   | `4.6875%`|
| `2em`          | vertical        | `5%`     |

Results are rounded to three decimal places (`b.java:249-251`):

```java
public static double a(double var0) {
    return Math.round(var0 * 1000.0) / 1000.0;
}
```

Results are capped at `100%` unless an `allowExceed` flag is set (lines 201-203).

## Computed Style Visitor Call Order

**File:** `com/amazon/yj/style/b/e.java`, method `a()` at lines 560-588

The visitor processes each container's style properties in a strict order.
The call sequence in the main visitor method is:

| Step | Line | Method | What it does |
|------|------|--------|-------------|
| 1 | 574 | `b(var1, var5, var3)` → 915 | LINE_HEIGHT computation (em/lh/rem → em) |
| 2 | 576 | loop over `D` | Copy FONT_FAMILY, FONT_WEIGHT, FONT_STYLE, LANGUAGE |
| 3 | 580 | `a(var5, var3, var4)` → 863 | Color properties |
| 4 | 582 | **`a(var1, var5, var3)` → 834** | **FONT_SIZE computation** |
| 5 | 583 | **`b(var5, var3, var6, var7)` → 628** | **Inline margins/padding → %** (set `a.e`) |
| 6 | 584 | `a(var5, var3, var6, var7)` → 618 | Border weights → % (set `a.g`) |
| 7 | 587 | **`a(var5, var3, var10, var6, var7)` → 591** | **TEXT_INDENT → %** |

Font-size is computed at **step 4** before margins (**step 5**) and text-indent
(**step 7**). This ordering is essential -- the computed font-size must be
available for the margin font-size multiplication (see §4 below).

## Where the Conversion Is Triggered

### 1. Inline margins and padding (lines 628-646)

**Method `b()`** iterates over `MARGIN_INLINE_START`, `MARGIN_INLINE_END`,
`PADDING_INLINE_START`, `PADDING_INLINE_END` (set `a.e` from
`com/amazon/yj/y/a/a.java`), converts each to `UNIT_PERCENT`:

```java
for (var6 : com.amazon.yj.y.a.a.e) {
    s var9 = e.a(var6, writingMode, direction);
    if (d.a(this.v, var6)) {
        double var10 = var2.a(b.a(var9)).i().a();  // parent size
        double var7 = this.a(var1, var9, var2, var10, ...);
        // Accumulate parent border margins if present
        a var12 = var2.b();
        if (var12 != null) {
            var13 = var12.a(var9);
            if (var13 != null) var7 += var13.i().a();
        }
    }
    var14 = this.b.a(var7, UNIT_PERCENT);
    var2.a(var9, var14);
}
```

In `horizontal-tb` with LTR direction the logical properties resolve to
`MARGIN_LEFT`, `MARGIN_RIGHT`, `PADDING_LEFT`, `PADDING_RIGHT`.

### 2. Border weights (lines 618-625)

**Method `a()`** iterates over `BORDER_WEIGHT_INLINE_START`,
`BORDER_WEIGHT_INLINE_END` (set `a.g`), converts each to `UNIT_PERCENT`:

```java
for (var6 : com.amazon.yj.y.a.a.g) {
    s var7 = e.a(var6, writingMode, direction);
    double var8 = var2.a(b.a(var7)).i().a();  // parent size
    double var10 = this.a(var1, var7, var2, var8, ...);
    var12 = this.b.a(var10, UNIT_PERCENT);
    var2.a(var7, var12);
}
```

### 3. TEXT_INDENT (lines 591-597)

Always converted to `UNIT_PERCENT`:

```java
if (var1.c(TEXT_INDENT) != null) {
    double var6 = var3.a();  // parent inline size
    double var8 = this.a(var1, TEXT_INDENT, var2, var6, ...);
    var10 = this.b.a(var8, UNIT_PERCENT);
    var2.a(TEXT_INDENT, var10);
}
```

## The Shared Conversion Function

All three callers above invoke the same private method at lines 649-679.
This function handles pre-conversion, the font-size multiplication, and
the final em-to-percent conversion.

### Pre-conversion of REM and LH units (lines 659-663)

Before the main conversion, `rem` is converted to `em` (by dividing by
the computed font-size) and `lh` (line-height units) is converted to `em`
(by multiplying by the computed line-height):

```java
if (var12.b() == UNIT_REM) {
    var12 = b.a(var3, var12, this.b);  // rem -> em: value / computedFontSize
} else if (var12.b() == UNIT_LINE_HEIGHT) {
    var12 = b.b(var3, var12, this.b);  // lh -> em: value * computedLineHeight
}
```

### Font-size multiplication for inline margins (lines 665-671)

When the unit is `em` AND the property is an inline margin or padding
(set `a.e` = {MARGIN_INLINE_START, MARGIN_INLINE_END, PADDING_INLINE_START,
PADDING_INLINE_END}), the value is **multiplied by the computed font-size**
before the em-to-percent formula:

```java
double var13 = var12.a();  // raw em value
if (UNIT_EM == var12.b()
    && a.e.contains(e.a(var2, writingMode, direction))) {
    computedFontSize = var3.a(FONT_SIZE);   // from computed style
    double var16 = computedFontSize.i().a(); // e.g. 1.25 for font-size: 1.25em
    var13 *= var16;  // em_value * font_size_in_em
}
```

This means for margins in `em`, the conversion accounts for the actual
computed font-size of the element, not just a nominal `em` unit.

**What does NOT get multiplied:**

- **TEXT_INDENT**: `a.e.contains(resolveLogical(TEXT_INDENT, ...))` is always
  `false` because `a.e` only contains margin/padding inline properties.
  Text-indent goes through `em × 100 / 32` directly.
- **Border weights**: set `a.g` is {BORDER_WEIGHT_INLINE_START/END}, also
  not in `a.e`.

### Final em-to-percent conversion (lines 673-676)

After the optional font-size multiplication, the value is converted to
percent using the core formula:

```java
u var18 = var12.b();                                       // unit (still em after multiplication)
Double var19 = var4 * b.b(writingMode) / 100.0;            // parentSize * baseWidth / 100
result = b.a(var13, var18, var19, DPI, false, errors);     // → core conversion (§ above)
return result.value() * var4 / 100.0;
```

### Worked example

For an element with `margin-left: 2em` and computed `font-size: 1.25em`
(e.g., parent has `font-size: 125%`), in `horizontal-tb`:

1. `var13 = 2.0` (the raw em value)
2. Property is `MARGIN_LEFT` → resolves to `MARGIN_INLINE_START` → is in `a.e`
3. `computedFontSize = 1.25`
4. `var13 = 2.0 × 1.25 = 2.5` (after multiplication)
5. Core formula: `2.5 × 100.0 / 32.0 = 7.8125%`

Without the multiplication the result would be `2.0 × 100.0 / 32.0 = 6.25%`.

For `text-indent: 2em` with the same font-size, the multiplication does NOT
apply, so the result is `2.0 × 100.0 / 32.0 = 6.25%` regardless of font-size.

## Font-Size Computation

**File:** `com/amazon/yj/style/b/e.java`, lines 834-861

Font-size is computed at step 4 of the visitor (line 582), before margins
and text-indent are processed. The computation handles three cases:

```java
private void a(containerId var1, style var2, computedStyle var3) {
    fontSizeValue = var2.get(FONT_SIZE);
    if (fontSizeValue == null) return;     // inherit parent's computed value

    measure = fontSizeValue.measure();
    if (UNIT_EM == measure.unit()) {
        // Compound with parent: computed = parentFontSize × emValue
        parentFontSize = var3.get(FONT_SIZE).measure();
        computed = parentFontSize.value() * measure.value();
        var3.set(FONT_SIZE, new Value(computed, parentFontSize.unit()));
    } else if (UNIT_REM == measure.unit()) {
        // Absolute: computed = remValue × documentDataFontSize
        // documentDataFontSize = 1.0em from document_data
        if (this.u == null) {
            // No font-size in document_data: log error, default to 1.0em
            this.u = this.i;  // 1.0em
        }
        computed = measure.value() * this.u.value();
        var3.set(FONT_SIZE, new Value(computed, UNIT_EM));
    } else {
        // Unsupported unit: log error, set to 1.0em
        var2.set(FONT_SIZE, this.i);
    }
}
```

Key properties:

- Root/initial font-size is `1.0em` (set at line 88: `this.i = this.b.a(1.0, UNIT_EM)`)
- `em` font-sizes **compound**: 1.25em × 0.8em parent = 1.0em computed
- `rem` font-sizes are **absolute**: remValue × documentDataFontSize (1.0em)
- Unsupported units (px, pt, etc.) log an error and default to 1.0em
- The computed style inherits the parent's font-size when no explicit value is set

## Conditions

### When the conversion does NOT happen

1. **Fixed layout** books (`isFixedLayout = true`). All relativization is
   disabled in `com/amazon/adapter/a/a/b/h.java`, lines 59-76:

   ```java
   if (this.a.D().j()) {  // isFixedLayout
       var8.e(false);   // relativizeUnits = false
       var8.f(false);   // fontSizeRelativize = false
       var8.g(false);   // lineHeightRelativize = false
       var8.h(false);   // textIndentRelativize = false
       var8.i(false);   // marginRelativize = false
       var8.q(false);   // paddingRelativize = false
   }
   ```

2. **Zero values** (`e.java:656-657`). Values of exactly `0.0` are
   short-circuited and returned as `0.0` without going through the conversion.

3. **Null properties**. If the property is not present in the style, no
   conversion is attempted.

### When the conversion ALWAYS happens

For all **reflowable** content, the em-to-percent conversion runs
unconditionally. There is no flag or configuration to disable it.

## Configuration Flags (Unused for This Conversion)

The preprocessor configuration (`com/amazon/html/c/b/e.java`) has several
relativization flags:

| Flag                    | Field | Default | Getter        |
|-------------------------|-------|---------|---------------|
| `relativizeUnits`       | `h`   | `true`  | `k()`         |
| `fontSizeRelativize`    | `i`   | `false` | `l()` = i OR h|
| `lineHeightRelativize`  | `j`   | `false` | `m()` = j OR h|
| `textIndentRelativize`  | `k`   | `false` | `n()` = k     |
| `marginRelativize`      | `l`   | `false` | `o()` = l     |
| `paddingRelativize`     | `m`   | --      | `A()` = h OR m|

Note that `fontSizeRelativize` and `lineHeightRelativize` are OR'd with the
master `relativizeUnits` flag, but `textIndentRelativize` and
`marginRelativize` are **not**. They default to `false` and are only set
to `true` when the app-level `CompleteRelativization` flag is on
(`com/amazon/adapter/common/app/c.java`).

These flags are serialized to JSON and sent to an external preprocessor
(Chromium/PhantomJS-based). They do NOT control the Java-side em-to-percent
conversion in the computed style visitor -- that runs unconditionally.

The preprocessor outputs an `isHorizontalPropertyRelativized` flag
(`com/amazon/html/c/b/f.java:153-154`), but in the mapper context construction
(`com/amazon/adapter/common/n/a/g.java:12`) this flag is **overridden to
`true`** regardless of the preprocessor output:

```java
var5.b().a(true);  // isHorizontalPropertyRelativized = true always
```

The only place this flag is consumed (`com/amazon/yjhtmlmapper/h/b.java:266`)
is a minor cleanup function that removes zero-width inline-size values -- it
has nothing to do with the em-to-percent conversion itself.

## KFX Internal Unit Codes

For reference, the KFX format stores units using symbol codes. The mapping
is in `KFXInput/kfxlib/yj_to_epub_properties.py`, lines 631-647:

| KFX Symbol | CSS Unit |
|------------|----------|
| `$308`     | `em`     |
| `$314`     | `%`      |
| `$311`     | `vw`     |
| `$312`     | `vh`     |
| `$319`     | `px`     |
| `$318`     | `pt`     |
| `$505`     | `rem`    |
| `$310`     | `lh`     |

After Kindle Previewer converts a reflowable EPUB, margins and text-indent
that were originally in `em` are stored as `$314` (percentage) in the KFX
output. The KFXInput plugin faithfully maps `$314` back to `%` -- there is no
reverse conversion from `%` to `em`.

## Implications for Round-Tripping

When converting EPUB -> KFX -> EPUB:

1. The original `em` values for horizontal margins and `text-indent` are
   **permanently lost** during the EPUB-to-KFX step.
2. The KFXInput plugin outputs them as `%` values.
3. To recover approximate `em` values, one could divide the percentage by
   `(100 / 32)` = `3.125` for horizontal-tb, but this would not account
   for font-size-dependent margin scaling (see §4 above).

## Implications for fb2cng

fb2cng emits `em` units directly for horizontal margins, padding, and
text-indent in its KFX output (since commit `2ac8d97`). This means:

1. **KP3 applies the font-size multiplication itself** when it processes
   the KFX data. fb2cng does not need to replicate this step.
2. If fb2cng were converting em→% instead, it **would** need to replicate
   the font-size multiplication for margins (but NOT for text-indent) to
   match KP3 behavior. That was the previously missing step.
3. By emitting raw `em` values, fb2cng lets KP3 handle the font-size-aware
   conversion correctly. The em values also scale with the viewer font-size
   setting, unlike `%` values which are viewport-relative.

The property sets used by fb2cng (see `kp3_units.go`) align with the
analysis above:

| Property | fb2cng unit | KP3 computed style unit |
|----------|-------------|-------------------------|
| margin-left/right | `em` ($308) | `%` ($314) after visitor |
| padding-left/right | `em` ($308) | `%` ($314) after visitor |
| text-indent | `em` ($308) | `%` ($314) after visitor |
| margin-top/bottom | `lh` ($310) | not relativized |
| font-size | `rem` ($505) | `em` after visitor |

## File Index

| File | Role |
|------|------|
| `com/amazon/yj/F/a/b.java` | Core em-to-percent conversion formula |
| `com/amazon/yj/style/b/e.java` | Computed style visitor (triggers conversion) |
| `com/amazon/yj/y/a/a.java` | Property sets (see below) |
| `com/amazon/yj/y/a/e.java` | Writing-mode-aware property resolution |
| `com/amazon/html/c/b/e.java` | Preprocessor input configuration |
| `com/amazon/html/c/b/f.java` | Preprocessor output (isHorizontalPropertyRelativized) |
| `com/amazon/adapter/a/a/b/h.java` | Config construction, fixed-layout override |
| `com/amazon/adapter/common/app/c.java` | App-level relativization config |
| `com/amazon/adapter/common/n/a/g.java` | Mapper context construction |
| `com/amazon/yjhtmlmapper/h/b.java` | HTML pre-processing (rem-to-em, flag check) |
| `com/amazon/yjhtmlmapper/c/b.java` | Mapper context (isHorizontalPropertyRelativized) |
| `com/amazon/B/d/e/h/u.java` | Unit enum (UNIT_EM, UNIT_PERCENT, etc.) |

### Property Sets (`com/amazon/yj/y/a/a.java`)

| Field | Contents | Used for |
|-------|----------|----------|
| `a` | MARGIN_BLOCK_START, MARGIN_BLOCK_END | Vertical margins |
| `b` | MARGIN_INLINE_START, MARGIN_INLINE_END | Horizontal margins |
| `c` | PADDING_INLINE_START, PADDING_INLINE_END | Horizontal padding |
| `d` | PADDING_BLOCK_START, PADDING_BLOCK_END | Vertical padding |
| `e` | `b` ∪ `c` (all inline margins + padding) | **Font-size multiplication check** |
| `f` | `a` ∪ `d` (all block margins + padding) | Block spacing |
| `g` | BORDER_WEIGHT_INLINE_START/END | Border weight conversion |
