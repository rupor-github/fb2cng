# PDF Output

PDF output is generated natively as fixed-page PDF 1.4.

```bash
fbc convert --to pdf book.fb2
```

## Main Characteristics

- Fixed page size derived from `document.images.screen.width`, `height`, and `dpi`.
- Selectable/searchable text with embedded Unicode fonts.
- Internal links, PDF outline/bookmarks, cover images, block images, tables, poems, epigraphs, drop caps, and vignettes.
- Native printed page footnotes in `float` and `floatRenumbered` modes.
- No full complex-script layout for Arabic/Indic shaping, bidi text, vertical writing, or CJK typography.

## Page Size

```yaml
document:
  images:
    screen:
      width: 1264
      height: 1680
      dpi: 300
```

Formula:

```text
PDF points = pixels * 72 / dpi
```

The same `screen` block also affects image sizing decisions, so keep it aligned with your target device/profile.

## Discouraging Automatic Margin Cropping

Some readers automatically crop blank margins and rescale or recenter pages. An
optional nearly-white outline can discourage this behavior:

```yaml
document:
  pdf:
    page_frame: 0.5pt
```

`0` disables the outline (the default). Nonzero thicknesses require `pt` or `px`;
`px` uses `document.images.screen.dpi`. For example, `1px` is `0.24pt` at 300 DPI.
Negative values and thicknesses as large as either page dimension are rejected.

The opaque RGB `(0.996, 0.996, 0.996)` outline is drawn entirely inside every
physical page, including covers and blank pages, behind all content. It does not
change margins, pagination, or existing clipping, including with negative margins.
Other output formats ignore it.

This is a device-dependent workaround, not a guarantee: readers may treat the
nearly-white outline as blank. On Kindles it works, however different firmware
versions behave differently, older will add margins around this outline, newer
do not. `0.5pt` is a suggested starting thickness.

## Styling

PDF uses the same `stylesheet_path` mechanism as other formats. A custom stylesheet replaces the default stylesheet.

Use `@media fbc-pdf` for PDF-only rules:

```css
@media fbc-pdf {
    body {
        font-size: 10.5pt;
        line-height: 1.2;
    }
}
```

Commonly useful CSS properties include font family/size/style/weight, line height, color, text alignment, margins, padding, borders, page breaks, widows, and orphans.

PDF also understands generated pseudo-content used by the default stylesheet, such as footnote reference decorations on `.link-footnote::before` and `.link-footnote::after`.

## SVG Artwork

Supported SVG covers, illustrations, vignettes, and placeholders remain vector-based by
default, retaining transparency and sharpness when zoomed. Other SVG artwork uses raster
fallback on white; raster source images are also white-backed.

See [Image handling](../images.md#svg-artwork) for the supported subset, configuration
interactions, and PDF/Kindle comparison. PDF SVG artwork retains intrinsic sizing rather
than applying body scaling or cover resizing; see [sizing and resolution](../images.md#sizing-and-resolution).

## Fonts

PDF font handling is stricter than EPUB/KFX because fb2cng parses fonts and writes PDF font resources itself.

Supported:

- TrueType outlines (`.ttf`) as PDF Type0 / CIDFontType2 fonts.
- TrueType subsetting when allowed by font embedding flags.
- OpenType CFF/CFF2 outlines (`.otf`) as PDF Type0 / CIDFontType0 fonts, embedded whole.

Limitations:

- Browser font containers are not decoded; use raw `.ttf` or `.otf` files.
- Type 1 fonts, bitmap-only fonts, and fonts without TrueType/CFF/CFF2 outlines are skipped.
- Color emoji/color fonts are not rendered as color glyphs.
- Synthetic bold/italic is not generated; provide real font faces.
- Complex shaping and bidi layout are limited.

Run with `--debug` to inspect font diagnostics. See [Debugging and validation](../debugging.md#pdf)
for PDF report artifacts and [PDF validation](../debugging.md#pdf-validation) for structural/metadata checks.

## Footnotes

| Mode | PDF Behavior |
|---|---|
| `default` | Footnotes are ordinary linked sections with generated return links in footnote bodies using `backlink_template` |
| `float` | Source footnotes become printed page footnotes at the bottom of the reference page |
| `floatRenumbered` | Printed page footnotes use page-local numeric labels; `label_template` formats extra title text |

Printed footnotes that do not fit continue on later pages with continuation markers.

See [Footnotes](../footnotes.md) for cross-format behavior.
