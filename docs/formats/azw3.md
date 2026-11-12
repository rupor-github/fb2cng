# Native AZW3 Output

`azw3` creates a standalone Kindle KF8 book directly, without KindleGen, Calibre,
or Amazon software at conversion time. It is distinct from KFX/AZW8 and does not
produce MOBI7 or combined MOBI/KF8 files.

Rendering, popup footnotes, font selection, and page-map support depend on the
Kindle device or app used to read the book.

## Command Line

```bash
fbc convert --to azw3 book.fb2
fbc convert --to azw3 --ebook --asin B012345678 book.fb2
fbc convert --to azw3 --output-file output/chosen.azw3 book.fb2
```

Output naming, metadata, footnote labels/backlinks, and visible-TOC templates receive
`.Format = "azw3"`. `--ebook` and `--asin` have the same meaning as for the other
Kindle formats. `--no-cover` is ignored with a warning; a missing cover is generated.
Shared naming, destination, and overwrite options are described in the
[command-line guide](../guide.md#command-line).

The cover inside a book and its library thumbnail are separate reader features.
The book contains an embedded cover; the reader controls how it is reached from the
text. The initial reading target remains the book text.
By default, native AZW3 uses local-book metadata without catalog identifiers or an
explicit document-type marker. This allows compatible Kindle readers to use the embedded
library thumbnail. `--ebook` explicitly selects EBOK, and `--asin` explicitly supplies
a catalog identifier (with PDOC unless `--ebook` is also used). Those choices are retained,
but may require device thumbnail-cache support when sideloading. Switching between
PDOC and EBOK alone does not guarantee library thumbnails.

## Configuration

AZW3-specific settings are under `document.azw3`:

```yaml
document:
  azw3:
    compression: standard
    apnx: none
```

These are the defaults. Navigation, styling, images, metadata, and footnotes use
the shared `document` settings. See [Configuration](../config.md).

### Compression

Select native compression in YAML, or override it for one conversion with `--compression`:

```yaml
document:
  azw3:
    compression: standard
```

| Value | Corresponding KindleGen argument | Behavior |
|---|---|---|
| `none` | `-c0` | Uncompressed text |
| `standard` | `-c1` | PalmDOC compression; default |
| `huffdic` | `-c2` | Huffman/dictionary compression |

These are corresponding compression modes for comparison; AZW3 generation remains
native and does not invoke KindleGen. Use the named values with `fbc convert --compression`,
not the KindleGen arguments.

```bash
fbc convert --to azw3 --compression huffdic book.fb2
fbc -c config.yaml convert --to azw3 --compression none book.fb2
```

An explicitly supplied flag wins over YAML; an omitted flag uses configuration.
AZW3 does not accept `lzma`, which is specific to KFX/AZW8.
Compression is lossless and does not change navigation or page-map destinations.

### Optional APNX Page Map

APNX selection is **configuration-file only**, with no command-line override:

```yaml
document:
  azw3:
    apnx: eink
  page_map:
    enable: true
    size: 2300
```

For a final book named `chosen.azw3`:

| Value | Output beside the book |
|---|---|
| `none` | No sidecar; default |
| `app` | `chosen.apnx` |
| `eink` | `chosen.sdr/chosen.apnx` |

These are approximate reading pages, not print-edition page numbers. Disabling
`page_map.enable` suppresses a requested APNX with a warning. APNX consumption
depends on the target reader and how the book is transferred.
The separate cover image has no APNX text position; its synthetic page entry is
omitted without renumbering the remaining page labels.
APNX also supports default local-book metadata; generating a page map does not add
catalog identifiers back into the book.

Without `--overwrite`, an existing book or selected sidecar prevents publication.
Generation failures preserve existing outputs. Unrelated `.sdr` files are preserved;
unselected old APNX files are not deleted or refreshed. Book and sidecar publication
is not atomic as a pair: an interruption during publication can leave a mismatched
pair. A failed rollback reports any retained APNX recovery file.

## Structure And Navigation

AZW3 includes Kindle contents navigation and internal links. `document.toc_type`
controls navigation nesting; `document.toc_page` adds an optional visible contents
page that retains the full section hierarchy. Top-level sections are separate
content documents; CSS can request additional [section splitting](../stylesheets.md#section-splitting).

The initial reading target is the book text; `document.open_from_cover` does not
change it. Approximate page navigation requires the [optional APNX sidecar](#optional-apnx-page-map).

## Styling

AZW3 uses reader-rendered HTML and CSS, not the KFX style conversion model.
Set `document.stylesheet_path` to replace the built-in stylesheet.
CSS media selection enables `amzn-kf8` and disables
`amzn-et`, `amzn-mobi`, and `fbc-pdf`; in particular, `amzn-kf8 and not amzn-et`
matches AZW3. See [stylesheet media queries](../stylesheets.md#media-queries).

CSS declaration order, repeated properties, selectors, and reader-dependent
conditions are retained. Actual CSS support depends on the Kindle reader.

Statically resolved `page-break-before: always` and `page-break-after: always` rules
become native page breaks, including between titled footnote bodies. Rules that
cannot be resolved during conversion remain for the reader to interpret.

## Images And Fonts

Shared [image settings](../images.md) apply. Raster images and covers use
Kindle-compatible JPEG preparation; non-cover SVG is retained by default and
displayed by the reader. Set `document.images.rasterize_svg: true` when a reader
cannot display the artwork reliably.

Use `@font-face` with raw TTF/OTF files to embed fonts. Whether a Kindle uses them
depends on its font settings and support for the supplied font.

## Footnotes

| Mode | Behavior |
|---|---|
| `default` | Normal linked sections, preserving note formatting; generated returns off by default |
| `float` | One plain-text paragraph per note, with a leading title return when a reference exists |
| `floatRenumbered` | Same as `float`, with normalized labels from `label_template` |

In all three modes,
additional generated backlinks are controlled by `backlink_template`: the default
AZW3 result is empty (no extra links), while custom nonempty results enable them.
Float modes use one plain-text paragraph per note with a leading title/label
return link, which is independent of this template. Any enabled additional returns
follow the body. See [Footnotes](../footnotes.md#azw3-floating-notes).
Display and popup behavior depend on the reader.

Backlink-template `.LocationNumber` uses the generated AZW3 reference location,
not the location of the note. Custom format-specific templates must include
`azw3` in their Kindle-format branch to use it.

## Limitations

Kindle popup recognition and boundaries remain inconsistent
and vary with firmware versions.
Some notes navigate instead of floating, and popups may include following notes or
join paragraphs. Full return links follow the note body, but readers may still
include them in popups. Shared notes with multiple references need particular care;
popup support and navigation must be checked on the target reader.

Floating notes lose rich formatting: images and authored links become readable
plain-text placeholders. Default-mode notes retain their formatting and links.
Library thumbnails and APNX support also depend on the reader and transfer method.

## Debugging And Validation

Debug reports include the final AZW3, the selected APNX when generated, and native
compilation diagnostics. Generate a [debug report](../debugging.md#azw3) with
`fbc -d convert ...`, then inspect the book with
[KindleUnpack](../debugging.md#kindleunpack). Successful decoding does not prove
rendering or popup compatibility; check the original book in the intended Kindle reader.
