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

It supports normal linked notes, floating notes,
renumbering, backlinks, visible TOC placement, embedded TTF/OTF fonts, and native
non-cover SVG. Display and popup behavior depend on the reader.

Shared image configuration applies, including explicit SVG rasterization and
transparency removal for compatibility. Ordinary covers use Kindle-compatible
raster preparation. See [Image handling](../images.md) and [Footnotes](../footnotes.md).

Debug reports include the final AZW3, the selected APNX when generated, and native
compilation diagnostics. See [Debugging](../debugging.md) for enabling reports.
