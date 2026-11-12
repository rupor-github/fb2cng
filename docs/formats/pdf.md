# PDF Output

PDF output is generated natively as fixed-page PDF 1.4.

## Command Line

```bash
fbc convert --to pdf book.fb2
```

The output extension is `.pdf`. Shared naming, destination, overwrite, and
`--no-cover` options are described in the [command-line guide](../guide.md#command-line).

## Configuration

`document.pdf.page_frame` is the PDF-specific setting. Page dimensions come from
`document.images.screen`; typography, footnotes, images, and metadata use shared
settings under `document`. See [Configuration](../config.md).

### Page Size

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

### Discouraging Automatic Margin Cropping

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

The nearly-white outline is drawn entirely inside every
physical page, including covers and blank pages, behind all content. It does not
change margins, pagination, or existing clipping, including with negative margins.
Other output formats ignore it.

This is a device-dependent workaround, not a guarantee: readers may treat the
nearly-white outline as blank or add their own margins around it. Kindle firmware
versions differ in this respect. `0.5pt` is a suggested starting thickness.

## Structure And Navigation

PDF has fixed pages with selectable/searchable text, internal links, and
outline bookmarks. It supports tables, poems, quotations, images, drop caps,
vignettes, and printed page footnotes.

The PDF outline follows the book's section hierarchy. `document.toc_type`
controls the generated visible contents list, not the outline.
`document.toc_page.placement` adds that list before or after the main text;
the default is `none`. Physical PDF pages are not controlled by the approximate
`document.page_map` setting used for reflowable e-books.

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

## Images And Fonts

Raster images are embedded with white-backed transparency; decodable JPEGs can
be embedded without re-encoding. Cover pages are optional and can be omitted with
`--no-cover`. Shared [image settings](../images.md) control preparation.

### SVG Artwork

Supported SVG covers, illustrations, vignettes, and placeholders remain vector-based by
default, retaining transparency and sharpness when zoomed. Other SVG artwork uses raster
fallback on white; raster source images are also white-backed.

See [Image handling](../images.md#svg-artwork) for the supported subset, configuration
interactions, and PDF/Kindle comparison. PDF SVG artwork retains intrinsic sizing rather
than applying body scaling or cover resizing; see [sizing and resolution](../images.md#sizing-and-resolution).

### Fonts

The renderer includes default fonts. Use `@font-face` and matching font-family
rules in a custom stylesheet to supply alternatives.

PDF font handling is stricter than EPUB/KFX because fb2cng parses fonts and writes PDF font resources itself.

Supported:

- TrueType outlines (`.ttf`), embedded with Unicode text support.
- TrueType subsetting when allowed by font embedding flags.
- OpenType CFF/CFF2 outlines (`.otf`), embedded whole.

Limitations:

- Browser font containers are not decoded; use raw `.ttf` or `.otf` files.
- Type 1 fonts, bitmap-only fonts, and fonts without TrueType/CFF/CFF2 outlines are skipped.
- Color emoji/color fonts are not rendered as color glyphs.
- Synthetic bold/italic is not generated; provide real font faces.
- Complex shaping and bidi layout are limited.

Run with `--debug` to inspect font diagnostics.

## Footnotes

| Mode | Behavior |
|---|---|
| `default` | Footnotes are ordinary linked sections with generated return links in footnote bodies using `backlink_template` |
| `float` | Source footnotes become printed page footnotes at the bottom of the reference page |
| `floatRenumbered` | Printed page footnotes use page-local numeric labels; `label_template` formats extra title text |

Printed footnotes that do not fit continue on later pages with continuation markers.
Their reference labels are printed text, not clickable links, and they do not
generate return links.

In `default` mode, a valid `backlink_template` that produces empty or whitespace-only
text, such as `{{- "" -}}`, suppresses generated return links without disabling
forward links to notes. The bundled template keeps returns enabled, normally
showing the reference page number. This setting does not affect printed footnotes
in floating modes.

See [Footnotes](../footnotes.md) for cross-format behavior.

## Limitations

- Layout is fixed at conversion time; changing the reader's font size does not
  reflow the book as it would in EPUB or Kindle formats.
- CSS support is a native-renderer subset, not browser-complete layout.
- Complex shaping, bidirectional text, vertical writing, and CJK typography are
  limited. Font embedding alone does not provide these layout capabilities.
- Very wide tables and artwork still need to fit the selected page size; inspect
  dense pages in the target viewer.

## Debugging And Validation

Generate a [debug report](../debugging.md#pdf) with `fbc -d convert ...` to inspect
font and layout diagnostics. Run the [PDF checker](../debugging.md#pdf-validation)
with `--require-metadata`, then inspect page layout, links, images, and printed
footnote continuations in the intended viewer.
