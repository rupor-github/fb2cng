# Native AZW3 Output

`azw3` creates a standalone Kindle KF8 book directly, without KindleGen, Calibre,
or Amazon software at conversion time. It is distinct from KFX/AZW8 and does not
produce MOBI7 or combined MOBI/KF8 files.

Rendering, popup footnotes, font selection, and page-map support depend on the
Kindle device or app used to read the book.

```bash
fbc convert --to azw3 book.fb2
fbc convert --to azw3 --ebook --asin B012345678 book.fb2
fbc convert --to azw3 --output-file output/chosen.azw3 book.fb2
```

Output naming, metadata, footnote labels/backlinks, and visible-TOC templates receive
`.Format = "azw3"`. `--ebook` and `--asin` have the same meaning as for the other
Kindle formats. `--no-cover` is ignored with a warning; a missing cover is generated.

The cover inside a book and its library thumbnail are separate reader features.
The book contains an embedded cover; the reader controls how it is reached from the
text. The initial reading target remains the book text.
By default, native AZW3 uses local-book metadata without catalog identifiers or an
explicit document-type marker. This lets readers such as Kindle Voyage use the embedded
library thumbnail. `--ebook` explicitly selects EBOK, and `--asin` explicitly supplies
a catalog identifier (with PDOC unless `--ebook` is also used). Those choices are retained,
but may require device thumbnail-cache support when sideloading. Switching between
PDOC and EBOK alone does not guarantee library thumbnails.

## Compression

Select native compression in YAML, or override it for one conversion with `--compression`:

```yaml
document:
  azw3:
    compression: standard
```

| Value | Behavior |
|---|---|
| `none` | Uncompressed text |
| `standard` | PalmDOC compression; default |
| `huffdic` | Huffman/dictionary compression |

```bash
fbc convert --to azw3 --compression huffdic book.fb2
fbc -c config.yaml convert --to azw3 --compression none book.fb2
```

An explicitly supplied flag wins over YAML; an omitted flag uses configuration.
AZW3 does not accept `lzma`, which is specific to KFX/AZW8.
Compression is lossless and does not change navigation or page-map destinations.

## Optional APNX Page Map

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

## Content And Resources

AZW3 uses the shared publication content and HTML5-style footnote semantics, not
the KFX style conversion model. CSS media selection enables `amzn-kf8` and disables
`amzn-et`, `amzn-mobi`, and `fbc-pdf`; in particular, `amzn-kf8 and not amzn-et`
matches AZW3. See [stylesheet media queries](../stylesheets.md#media-queries).

Native CSS retains declaration order, repeated properties, selectors, and conditional
rules for the reader; resource URLs are rewritten for the book. Empty HTML elements
such as blank-line containers and link anchors receive explicit closing tags for
Kindle HTML parsing, without changing other output formats.

Statically resolved `page-break-before: always` and `page-break-after: always` rules
are materialized as native breaks, including between titled footnote bodies. This
does not change converter-controlled document splitting. Unresolved pagination or
visibility rules are retained for the reader and identified in the debug trace.

It supports normal linked notes, floating notes,
renumbering, backlinks, visible TOC placement, embedded TTF/OTF fonts, and native
non-cover SVG. Display and popup behavior depend on the reader.

Known limitations: on Kindle Voyage, `floatRenumbered` popups are inconsistent;
some notes navigate instead of opening a popup, and a popup may include the next note.
AZW3 does not currently populate backlink-template `.LocationNumber`; use page or
section labels as a fallback.

Shared image configuration applies, including explicit SVG rasterization and
transparency removal for compatibility. Ordinary covers use Kindle-compatible
raster preparation. See [Image handling](../images.md) and [Footnotes](../footnotes.md).

Debug reports include the final AZW3, the selected APNX when generated, and native
compilation diagnostics. See [Debugging](../debugging.md) for enabling reports.
