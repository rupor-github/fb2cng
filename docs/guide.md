# FBC User Guide

## Overview

**fb2cng** converts FB2/FictionBook files to EPUB2, EPUB3, KEPUB, KEPUB3, AZW3, KFX/AZW8, PDF, TXT, and Markdown.

Use this guide for installation, command-line basics, choosing an output format,
and finding the right reference document.

## Installation

Download the archive for your operating system and processor from the
[releases page](https://github.com/rupor-github/fb2cng/releases) and unpack it.
Run `fbc` (`fbc.exe` on Windows) from that directory, or add it to `PATH`.
On Linux and macOS, use `./fbc` if the directory is not on `PATH`.

Conversion does not require Calibre, KindleGen, or Amazon software. The
[external validation tools](debugging.md#output-validators) are optional.
For MyHomeLib on Windows, see [MyHomeLib integration](mhl.md).

## Quick Start

Convert one book to the default output format, EPUB2:

```bash
fbc convert book.fb2
```

Convert to a specific format:

```bash
fbc convert --to epub3 book.fb2
fbc convert --to kfx book.fb2
fbc convert --to azw8 --compression lzma book.fb2
fbc convert --to azw3 book.fb2
fbc convert --to pdf book.fb2
fbc convert --to md book.fb2
```

Convert into a destination directory:

```bash
fbc convert book.fb2 /path/to/output/
```

Convert every FB2 file in a directory tree:

```bash
fbc convert /path/to/books/ /path/to/output/
```

Use a custom configuration:

```bash
fbc -c myconfig.yaml convert --to epub3 book.fb2
```

Enable debug reporting:

```bash
fbc -d convert book.fb2
```

Debug mode creates `fbc-report.zip` with logs and diagnostic files.
See [Debugging and validation](debugging.md) for report contents and inspection tools.
To keep logs and reports alongside conversion output, use `.OutputDir` in their
[destination templates](templates.md#artifact-destination-templates).

## Command Line

```text
fbc [global options] command [command options] [arguments]
```

Global options:

| Option | Meaning |
|---|---|
| `--config FILE`, `-c FILE` | Load YAML configuration |
| `--debug`, `-d` | Enable debug report generation |
| `--version`, `-v` | Print version |
| `--help`, `-h` | Show help |

Convert command:

```text
fbc convert [options] SOURCE [DESTINATION]
```

Convert options:

| Option | Meaning |
|---|---|
| `--to TYPE` | `epub2`, `epub3`, `kepub`, `kepub3`, `kfx`, `azw8`, `azw3`, `pdf`, `txt`, or `md` |
| `--ebook`, `--eb` | For Kindle formats, explicitly mark output as ebook/EBOK |
| `--asin ASIN` | For Kindle formats, set ASIN metadata: 10 letters/digits (`A–Z`, `0–9`) |
| `--compression MODE` | Override configured compression: KFX/AZW8 `none` or `lzma`; AZW3 `none`, `standard`, or `huffdic`; ignored with a warning for other formats |
| `--no-cover` | Omit existing or generated covers; ignored with a warning for AZW3 and KFX/AZW8 |
| `--output-file FILE`, `-o FILE` | Write a single source book to exact output file path |
| `--nodirs`, `--nd` | Do not preserve input directory structure in output |
| `--overwrite`, `--ow` | Overwrite existing output files |
| `--force-zip-cp ENCODING` | Force archive file-name encoding, such as `windows-1251` or `cp866` |

`SOURCE` can be a single FB2 file, a directory, an archive entry, or a directory
inside an archive. Directory processing is recursive and does not follow symbolic
links. Archives inside archives are not supported. `DESTINATION` is always a
directory; if omitted, the current directory is used.

For an archive entry or subtree, append its internal path to the archive path:

```bash
fbc convert "books.zip/author/book.fb2" output/
fbc convert "books.zip/author/" output/
```

`--output-file` is different from `DESTINATION`: it is an exact final file path for single-book conversion. It cannot be used with `DESTINATION`, recursive directory conversion, or archive paths that contain multiple FB2 books. It is useful when another application already decided the expected output file name.

Use `--no-cover` to produce EPUB2/3, KEPUB/KEPUB3, or PDF without a dedicated cover page or cover metadata, even when the FB2 supplies a cover or `document.images.cover.generate` is enabled. Images also used in the book body remain ordinary illustrations. TXT and Markdown already skip the dedicated cover. Kindle formats require a cover, so AZW3 and KFX/AZW8 ignore the flag and log a warning.

```bash
fbc convert --no-cover --to pdf book.fb2
```

KFX/AZW8 compression is opt-in. Use `document.kfx.compression: lzma` in YAML or an explicit
command override. `--compression none` disables a configured compression default.

```bash
fbc convert --to azw8 --compression lzma book.fb2
fbc convert --to azw3 --compression huffdic book.fb2
```

AZW3 defaults to `standard` compression (corresponding to KindleGen `-c1`);
`none` corresponds to `-c0`, and `huffdic` to `-c2`. AZW3 is generated natively
without invoking KindleGen. `--compression` accepts the named modes supported by
the selected `--to` format, not KindleGen's arguments; omitting it respects that
format's YAML setting.

See [KFX/AZW8 compression](formats/kfx.md#compression) and
[AZW3 compression](formats/azw3.md#compression) for conversion settings, and
[Debugging and validation](debugging.md) for the standalone utility and envelope diagnostics.

## Supported Formats

| Format | Extension | Best For | Details |
|---|---|---|---|
| EPUB2 | `.epub` | Broad e-reader compatibility | [EPUB and KEPUB](formats/epub.md) |
| EPUB3 | `.epub` | Modern EPUB readers and semantic footnotes | [EPUB and KEPUB](formats/epub.md) |
| KEPUB | `.kepub.epub` | Kobo devices with EPUB2-based packaging | [EPUB and KEPUB](formats/epub.md) |
| KEPUB3 | `.kepub.epub` | Kobo devices with EPUB3 package/navigation semantics | [EPUB and KEPUB](formats/epub.md) |
| KFX | `.kfx` | Kindle Enhanced Typesetting with optional LZMA compression | [Kindle KFX/AZW8](formats/kfx.md) |
| AZW8 | `.azw8` | KFX with optional LZMA compression and Kindle Previewer-friendly extension | [Kindle KFX/AZW8](formats/kfx.md) |
| AZW3 | `.azw3` | Native standalone KF8 with configurable compression and optional APNX | [Native AZW3](formats/azw3.md) |
| PDF | `.pdf` | Fixed-page output, outlines, selectable text, printed footnotes | [PDF](formats/pdf.md) |
| TXT | `.txt` | Plain readable UTF-8 text | [TXT and Markdown](formats/text.md) |
| Markdown | `.md` | Semantic plain-text documents with headings, links, tables, and optional images | [TXT and Markdown](formats/text.md) |

## Feature Matrix

The tables group formats that share behavior: KEPUB follows EPUB2, KEPUB3 follows
EPUB3, and AZW8 uses the same book format as KFX. Kobo packaging and image
differences are listed separately below. **Yes** means the converter supports a
feature, not that every reader displays it identically. **—** means not applicable.

### Content And Appearance

| Feature | EPUB2/KEPUB | EPUB3/KEPUB3 | KFX/AZW8 | AZW3 | PDF | TXT | Markdown |
|---|---|---|---|---|---|---|---|
| Layout | Reflowable | Reflowable | Reflowable | Reflowable | Fixed pages | Viewer-wrapped text | Viewer-rendered markup |
| CSS styling | Reader CSS | Reader CSS | Converted to Kindle styles | Reader KF8 CSS | Native-renderer subset | No | No |
| Embedded fonts | Yes | Yes | Yes | TTF/OTF | TTF/OTF, with limits | No | No |
| Block/inline images | Yes | Yes | Yes | Yes | Yes | Placeholders | Placeholder/external/embedded |
| SVG artwork | Retained SVG | Retained SVG | Native vectors or JPEG fallback | Retained non-cover SVG | Native vectors or raster fallback | Placeholders | External SVG or embedded JPEG; placeholders by default |
| Dedicated cover | Optional | Optional | Required | Required | Optional | Omitted | Omitted |
| Tables | HTML tables | HTML tables | Kindle tables | HTML tables | Native tables | Aligned text | Pipe tables |
| Code blocks | Styled text | Styled text | Styled text | Styled text | Styled text | Plain text | Fenced blocks |
| Poems and quotations | Styled text | Styled text | Styled text | Styled text | Styled text | Plain text | Text/block quotes |
| Drop caps | CSS | CSS | Kindle styles | CSS | Native layout | Ordinary text | Ordinary text |
| Vignettes | Yes | Yes | Yes | Yes | Yes | No | No |
| Optional annotation section | Yes | Yes | Yes | Yes | Yes | Yes | Yes |
| Text transformations | Yes | Yes | Yes | Yes | Yes | Yes | Yes |

Drop caps, vignettes, and a separate annotation section are disabled by default.
`--no-cover` omits covers in EPUB/KEPUB and PDF; Kindle output ignores it with a warning.
KEPUB/KEPUB3 add Kobo sentence markup and convert WebP images to PNG by default;
ordinary EPUB does neither. Image settings can override normal raster encoding.

### Navigation And Output

| Feature | EPUB2/KEPUB | EPUB3/KEPUB3 | KFX/AZW8 | AZW3 | PDF | TXT | Markdown |
|---|---|---|---|---|---|---|---|
| Clickable links | Yes | Yes | Yes | Yes | Yes | No | Yes |
| Reader contents navigation | NCX | EPUB navigation | Kindle navigation | Kindle navigation | PDF outline | No | Headings/anchors only |
| Optional visible contents | Linked page | Linked page | Linked page | Linked page | Linked list | Text list | Linked list |
| Page navigation | Approximate, optional | Approximate, optional | Approximate, optional | Optional APNX sidecar | Physical pages | No | No |
| Output compression | ZIP package | ZIP package | `none` (default) or `lzma` | `standard` (default), `none`, or `huffdic` | Automatic | None | None |
| Output-name template | Yes | Yes | Yes | Yes | Yes | Yes | Yes |
| Metadata title/name templates | Yes | Yes | Yes | Yes | Yes | No; source text | YAML front matter only |
| Debug report | Yes | Yes | Yes | Yes | Yes | Yes | Yes |
| Independent check | EPUBCheck | EPUBCheck | KFXInput + reader | KindleUnpack + reader | qpdf/pdfinfo + viewer | Read text | Check links/assets in viewer |

Approximate page maps are not print-edition page numbers. APNX support, Kindle
popup recognition, fonts, and library thumbnails depend on the reader and transfer
method. See [AZW3](formats/azw3.md) for its specific limits.

### Footnotes

| Feature | EPUB2/KEPUB | EPUB3/KEPUB3 | KFX/AZW8 | AZW3 | PDF | TXT | Markdown |
|---|---|---|---|---|---|---|---|
| `default` | Linked sections | Linked sections | Linked sections; no popups | Linked sections | Linked sections | Normal sections | Linked sections |
| `float` | Linked notes; popups reader-dependent | Semantic popup markup | Kindle popup notes | Plain-text notes; popups reader-dependent | Printed page footnotes | Final Notes section | Final Notes section |
| `floatRenumbered` | Normalized labels | Normalized labels | Normalized labels | Normalized labels | Page-local numeric labels | Normalized endnotes | Normalized endnotes |
| Template-generated returns | All modes; on by default | All modes; on by default | All modes; on by default | All modes; off by default | `default` only; on by default | None | All modes; on by default |
| Empty template result | Suppresses returns | Suppresses returns | Suppresses returns | Suppresses extra returns, not leading title return | Suppresses `default` returns | No effect | Suppresses returns |

AZW3 floating notes retain an independent leading title/label return when a
reference exists. PDF printed references are not clickable and do not have
generated return links. Disabling EPUB/KEPUB returns can affect reader popup
heuristics as well as navigation.

See [Footnotes](footnotes.md) for a full mode-by-format matrix, and
[Image handling](images.md) for image capabilities and configuration interactions.

## Metadata By Format

Metadata is read from FB2 `description` fields and written to each output format where the target format has a matching concept. Metadata dates and ISBNs are normalized when possible; invalid ISBN values are skipped.

`document.metainformation.title_template` controls the exported book title metadata.
`document.metainformation.creator_name_template` controls exported author and
translator names. `document.metainformation.transliterate` applies afterward.
These settings do not rewrite headings or author names in the book text. TXT's
opening title and metadata block use source values; Markdown uses templates in
YAML front matter but keeps its visible title from the source.

| Metadata | EPUB2/KEPUB | EPUB3/KEPUB3 | KFX/AZW8 | AZW3 | PDF | TXT | Markdown |
|---|---|---|---|---|---|---|---|
| Title | Yes, templated | Yes, templated | Yes, templated | Yes, templated | Yes, templated | Source heading | YAML `title` |
| Authors | Yes | Yes | Yes | Yes | Yes | Metadata block | YAML `authors` |
| Translators | Yes, with roles | Yes, with roles | No | Yes, with roles | Yes | No | YAML `translators` |
| Language | Yes | Yes | Yes | Yes | Yes | Metadata block | YAML `language` |
| Date | Yes | Yes | Yes | Yes | Yes | Metadata block | YAML `date` |
| Publisher | Yes | Yes | Yes | Yes | Yes | No | YAML `publisher` |
| Annotation/description | Yes | Yes | Yes | Yes | Yes | No metadata field | YAML `description` |
| Genres | Yes | Yes | No | Yes | Yes | Metadata block | YAML `genres` |
| Keywords | Yes | Yes | No | Yes | Yes | No | YAML `keywords` |
| ISBN | Yes | Yes | Yes | Yes | Yes | No | YAML `isbn` |
| Book identifier | Yes | Yes | Yes | Yes | Yes | No | YAML `identifier` |
| Series | First series, Calibre-compatible | All series as collections | No | Collection metadata | No | Metadata block | YAML `series` |
| Source title/language | No | No | No | No | No | No | YAML `source_title`, `source_language` |

The identifier comes from the FB2 document ID, normalized or generated during
preparation. KFX/AZW8 also derive fallback Kindle catalog identity from it. AZW3
uses local-book metadata by default; a catalog identifier is written only when
`--asin` or `--ebook` is explicitly requested.

AZW3 preserves translator roles and series metadata, but display and library grouping
depend on the reader; embedding these fields does not guarantee Kindle collection grouping.

PDF stores both classic document properties and XMP metadata. Its Subject property
comes from annotation text; genres and keywords are also retained in XMP. Not every
PDF viewer exposes every field. A separate annotation section, controlled by
`document.annotation`, is independent of description metadata in all formats.

## Configuration Basics

Generate a default configuration file:

```bash
fbc dumpconfig --default config.yaml
```

Show the active configuration after defaults and your custom file are merged:

```bash
fbc -c config.yaml dumpconfig active.yaml
```

Use generated defaults as the source of truth for all available fields. Unknown YAML fields are rejected, so stale option names fail fast.

Common configuration areas:

| Area | Reference |
|---|---|
| Config loading and structure | [Configuration](config.md) |
| Output naming, metadata, footnote labels, backlinks | [Templates](templates.md) |
| CSS, fonts, media queries, section splitting | [Stylesheets](stylesheets.md) |
| Footnote modes and templates | [Footnotes](footnotes.md) |
| Images, SVG, transparency, sizing, covers, and vignettes | [Image handling](images.md) |
| EPUB/KEPUB behavior | [EPUB and KEPUB variants](formats/epub.md) |
| Kindle KFX/AZW8 behavior | [Kindle KFX/AZW8](formats/kfx.md) |
| Native AZW3 behavior and APNX | [Native AZW3](formats/azw3.md) |
| PDF behavior | [PDF](formats/pdf.md) |
| TXT/Markdown behavior | [TXT and Markdown](formats/text.md) |

## Common Tasks

### Choose An Output Name

Use `document.output_name_template`. See [Templates](templates.md#output_name_template).

### Write To An Exact Output File

Use `--output-file` when converting one book and the caller expects one exact output path:

```bash
fbc convert --to epub3 --output-file /tmp/output/book.epub book.fb2
```

The short form is equivalent:

```bash
fbc convert --to epub3 -o /tmp/output/book.epub book.fb2
```

In this mode, `fbc` writes exactly that file path and ignores `document.output_name_template` and `--nodirs`. `--overwrite` is still required if the file already exists.

### Customize Metadata

Use `document.metainformation.title_template` and `document.metainformation.creator_name_template`. See [Templates](templates.md#metadata-templates).

### Customize Styling

Set `document.stylesheet_path` to a CSS file. A custom stylesheet replaces the
built-in stylesheet, so start by copying [`convert/default.css`](../convert/default.css)
to modify the default. See [Stylesheets](stylesheets.md).

### Configure Footnotes

Set `document.footnotes.mode` to `default`, `float`, or `floatRenumbered`. See [Footnotes](footnotes.md).

### Configure Images

See [Image handling](images.md) for the output capability matrix, SVG/raster policies,
transparency, covers, resizing, and configuration examples.

### Configure Markdown Images

Set `document.markdown.images` to `placeholder`, `external`, or `embedded`. See [Markdown images](images.md#markdown-images).

### Configure PDF Page Size

Set `document.images.screen.width`, `height`, and `dpi`. See [PDF](formats/pdf.md#page-size).

## Reference Documents

- [Configuration](config.md)
- [Image handling](images.md)
- [Debugging and validation](debugging.md)
- [Templates](templates.md)
- [Footnotes](footnotes.md)
- [Stylesheets](stylesheets.md)
- [EPUB and KEPUB variants](formats/epub.md)
- [Kindle KFX/AZW8](formats/kfx.md)
- [Native AZW3](formats/azw3.md)
- [PDF](formats/pdf.md)
- [TXT and Markdown](formats/text.md)
- [MyHomeLib Integration](mhl.md)

## Troubleshooting

Use `--debug` first. The debug report is the fastest way to inspect parsed metadata, normalized content, generated resources, logs, and format-specific diagnostics.
See [Debugging and validation](debugging.md) for artifact descriptions, all development
utilities and external validators.

Common checks:

| Problem | What To Check |
|---|---|
| Conversion fails | Run with `--debug`; inspect `fbc-report.zip` and logs |
| Archive paths look wrong | Try `--force-zip-cp windows-1251` or another legacy encoding |
| Output file is not created | Check destination path, overwrite flag, and output-name template |
| Configuration is ignored | Use `fbc -c config.yaml dumpconfig active.yaml` to confirm the loaded values |
| Template output is unexpected | Check `.Context`, `.Format`, and template-specific fields in [Templates](templates.md) |
| Fonts or CSS do not apply | Check [Stylesheets](stylesheets.md) and format-specific styling limits |
| Images are missing | Check [image troubleshooting](images.md#troubleshooting) and debug image diagnostics |

When reporting an issue, include the command line, relevant configuration, converter version, and the debug report when possible.
