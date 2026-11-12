# EPUB And KEPUB Output

EPUB is a reflowable e-book format: the reader chooses the final page layout.
This page covers `epub2`, `epub3`, `kepub`, and `kepub3` output.

## Command Line

```bash
fbc convert --to epub2 book.fb2
fbc convert --to epub3 book.fb2
fbc convert --to kepub book.fb2
fbc convert --to kepub3 book.fb2
```

EPUB2/3 use `.epub`; both Kobo variants use `.kepub.epub`. EPUB2 is the default
when `--to` is omitted. Shared naming, destination, and overwrite options are
described in the [command-line guide](../guide.md#command-line).

## Choosing A Variant

| Format | Use When |
|---|---|
| `epub2` | You need broad compatibility with older EPUB readers |
| `epub3` | You want modern EPUB semantics, including EPUB3 footnote markup |
| `kepub` | You target Kobo devices and want EPUB2-based Kobo packaging |
| `kepub3` | You target Kobo devices and want EPUB3 package/navigation semantics |

## Configuration

EPUB/KEPUB use shared settings under `document`; there is no separate `epub` or
`kepub` configuration section.

```yaml
document:
  toc_type: normal
  toc_page:
    placement: before
  footnotes:
    mode: float
```

This example enables a visible contents page and floating-note markup; the
defaults are `placement: none` and `mode: default`. See [Configuration](../config.md)
for all shared settings.

## Structure And Navigation

fb2cng converts top-level FB2 sections into EPUB content documents and generates device navigation:

- EPUB2: `toc.ncx`
- EPUB3: `nav.xhtml`
- KEPUB: EPUB2-style package optimized for Kobo handling
- KEPUB3: EPUB3-style package with Kobo XHTML instrumentation

The optional visible TOC page is controlled by `document.toc_page` and is separate from device navigation.

`document.toc_type` controls device-navigation nesting. The visible contents page
retains the full section hierarchy independently of that setting.

With `document.page_map.enable: true` (the default), generated page positions are
included in navigation. These are approximate reading pages, not print-edition
page numbers. Reader support varies.

For EPUB2/KEPUB only, `document.page_map.adobe_de: true` selects Adobe Digital
Editions-specific page navigation instead. This nonstandard option causes
EPUBCheck errors; leave it disabled unless your reader needs it.

## Styling

EPUB and KEPUB variants use XHTML plus CSS. Set `document.stylesheet_path` to replace the default stylesheet.

Custom stylesheet notes:

- Start from [`convert/default.css`](../../convert/default.css) to preserve default coverage.
- Use `@font-face` to embed fonts.
- See [Stylesheets](../stylesheets.md) for URL resolution and resource packaging.

### Section Splitting

Depth-1 sections are always separate content documents. Nested sections can be split based on CSS `page-break-before` rules on `.section-title-hN` classes. See [Stylesheets](../stylesheets.md) for details.

## Images And Fonts

By default, SVG stays SVG and KEPUB variants convert WebP to PNG for Kobo
compatibility; ordinary EPUB does not. See [Image handling](../images.md) for the
format matrix, transparency and rasterization settings, sizing, and covers.
Use `--no-cover` to omit the dedicated cover page and metadata.

Use `@font-face` in a stylesheet to embed fonts; raw TTF/OTF files are the most
portable choice. Relative resources are resolved from the stylesheet location;
unsafe absolute/path-traversal URLs are rejected. See [Stylesheets](../stylesheets.md#fonts).

## Footnotes

| Mode | EPUB2/KEPUB | EPUB3/KEPUB3 |
|---|---|---|
| `default` | Normal linked sections; generated returns enabled by default | Normal linked sections; generated returns enabled by default |
| `float` | Linked notes; generated returns enabled by default | Semantic footnote/popup markup; generated returns enabled by default |
| `floatRenumbered` | Same as `float`, with normalized labels | Same as `float`, with normalized labels |

In EPUB3 and KEPUB3 `default` mode, footnote references are ordinary links, not
popup references. Floating modes add EPUB3 footnote semantics; actual popup
behavior depends on the reader.

The bundled `backlink_template` enables returns in all three modes. A valid
template producing empty or whitespace-only text, such as `{{- "" -}}`, suppresses
generated returns without removing forward links, note targets or EPUB3
`footnote`/`noteref` semantics. Empty notes retain their targets.

Returns provide a way back when a reader jumps to a note instead of opening a
popup. Readers may also use link relationships as popup-recognition heuristics,
especially with EPUB2/KEPUB. Disabling returns can therefore affect recognition,
popup boundaries or navigation; validate with your intended readers.

See [Footnotes](../footnotes.md) for mode details and templates.

## Limitations

- Popup recognition, fonts, SVG display, and page navigation depend on the reader.
- Kobo sentence markup improves Kobo integration but does not make other EPUB
  readers behave like Kobo devices.
- Embedded media formats still need support in the target reader; use JPEG or PNG
  when broad raster-image compatibility matters.

## Debugging And Validation

Generate a [debug report](../debugging.md#converter-debug-reports) with `fbc -d convert ...`.
Validate the book with [EPUBCheck](../debugging.md#epub-and-kepub), then check
appearance, links, and notes in the intended reader. Ordinary output should have
no EPUBCheck errors or warnings; the Adobe page-map option is an explicit exception.
