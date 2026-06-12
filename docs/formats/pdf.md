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

Run with `--debug` to inspect font diagnostics.

## Footnotes

| Mode | PDF Behavior |
|---|---|
| `default` | Footnotes are ordinary linked sections; return links can be generated in footnote bodies using `backlink_template` |
| `float` | Source footnotes become printed page footnotes at the bottom of the reference page |
| `floatRenumbered` | Printed page footnotes use page-local numeric labels; `label_template` formats extra title text |

Printed footnotes that do not fit continue on later pages with continuation markers.

See [Footnotes](../footnotes.md) for cross-format behavior.
