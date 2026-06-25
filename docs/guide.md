# FBC User Guide

## Overview

**fb2cng** converts FB2/FictionBook files to EPUB2, EPUB3, KEPUB, KFX/AZW8, PDF, TXT, and Markdown.

Use this guide for installation, command-line basics, choosing an output format, and finding the right reference document. Format-specific details now live in separate files.

## Quick Start

Convert one book to the default output format, EPUB2:

```bash
fbc convert book.fb2
```

Convert to a specific format:

```bash
fbc convert --to epub3 book.fb2
fbc convert --to kfx book.fb2
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
| `--to TYPE` | `epub2`, `epub3`, `kepub`, `kfx`, `azw8`, `pdf`, `txt`, or `md` |
| `--ebook`, `--eb` | For Kindle formats, mark output as ebook/EBOK instead of personal document/PDOC |
| `--asin ASIN` | For Kindle formats, set ASIN metadata |
| `--output-file FILE`, `-o FILE` | Write a single source book to exact output file path |
| `--nodirs`, `--nd` | Do not preserve input directory structure in output |
| `--overwrite`, `--ow` | Overwrite existing output files |
| `--force-zip-cp ENCODING` | Force archive file-name encoding, such as `windows-1251` or `cp866` |

`SOURCE` can be a single FB2 file, a directory, an archive entry, or a directory inside an archive. Archives inside archives are not supported. `DESTINATION` is always a directory; if omitted, the current directory is used.

`--output-file` is different from `DESTINATION`: it is an exact final file path for single-book conversion. It cannot be used with `DESTINATION`, recursive directory conversion, or archive paths that contain multiple FB2 books. It is useful when another application already decided the expected output file name.

## Supported Formats

| Format | Extension | Best For | Details |
|---|---|---|---|
| EPUB2 | `.epub` | Broad e-reader compatibility | [EPUB and KEPUB](formats/epub.md) |
| EPUB3 | `.epub` | Modern EPUB readers and semantic footnotes | [EPUB and KEPUB](formats/epub.md) |
| KEPUB | `.kepub.epub` | Kobo devices | [EPUB and KEPUB](formats/epub.md) |
| KFX | `.kfx` | Kindle Enhanced Typesetting | [Kindle KFX/AZW8](formats/kindle.md) |
| AZW8 | `.azw8` | KFX payload with Kindle Previewer-friendly extension | [Kindle KFX/AZW8](formats/kindle.md) |
| PDF | `.pdf` | Fixed-page output, outlines, selectable text, printed footnotes | [PDF](formats/pdf.md) |
| TXT | `.txt` | Plain readable UTF-8 text | [TXT and Markdown](formats/text.md) |
| Markdown | `.md` | Semantic plain-text documents with headings, links, tables, and optional images | [TXT and Markdown](formats/text.md) |

## Feature Matrix

| Feature | EPUB2 | EPUB3 | KEPUB | KFX/AZW8 | PDF | TXT | Markdown |
|---|---:|---:|---:|---:|---:|---:|---:|
| Clickable internal links | yes | yes | yes | yes | yes | no | yes |
| Device navigation | NCX | NAV | NCX/Kobo | Kindle nav | PDF outline | no | headings/anchors |
| Optional visible TOC page | yes | yes | yes | yes | yes | text | links |
| CSS styling | yes | yes | yes | mapped to Kindle styles | subset via native renderer | no | no |
| Embedded fonts | yes | yes | yes | yes | yes | no | no |
| Block and inline images | yes | yes | yes | yes | yes | placeholders | placeholder/external/embedded |
| Tables | XHTML | XHTML | XHTML | Kindle table model | native PDF | aligned text | Markdown pipe tables |
| Code blocks | styled XHTML | styled XHTML | styled XHTML | styled text | styled PDF text | plain text | fenced blocks |
| Default footnotes | normal sections | normal sections | normal sections | normal sections | normal sections with optional PDF backlinks | normal sections | normal sections, no generated backlinks |
| Floating footnotes | bidirectional links | EPUB aside/noteref | bidirectional links | Kindle popup footnotes | printed page footnotes | final Notes section | final Notes section with backlinks |
| `floatRenumbered` labels | normalized | normalized | normalized | normalized | page-local printed labels | normalized endnotes | normalized endnotes |
| Footnote backlinks | float modes | float modes | float modes | float modes | default body links and printed notes | no clickable links | float modes |
| Page map/page labels | optional | optional | optional | optional/generated | native pages | no | no |
| Debug validation focus | EPUB structure/checks | EPUB3 nav/aside | Kobo packaging | Kindle Previewer/KFX checks | PDF structure/fonts/layout | text rendering | Markdown links/assets |

See [Footnotes](footnotes.md) for a full mode-by-format matrix.

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
| EPUB/KEPUB behavior | [EPUB and KEPUB](formats/epub.md) |
| Kindle KFX/AZW8 behavior | [Kindle KFX/AZW8](formats/kindle.md) |
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

Set `document.stylesheet_path` to a CSS file. A custom stylesheet replaces the built-in stylesheet, so start by copying `convert/default.css` if you want to modify the default. See [Stylesheets](stylesheets.md).

### Configure Footnotes

Set `document.footnotes.mode` to `default`, `float`, or `floatRenumbered`. See [Footnotes](footnotes.md).

### Configure Markdown Images

Set `document.images.markdown` to `placeholder`, `external`, or `embedded`. See [TXT and Markdown](formats/text.md#markdown-images).

### Configure PDF Page Size

Set `document.images.screen.width`, `height`, and `dpi`. See [PDF](formats/pdf.md#page-size).

## Reference Documents

- [Configuration](config.md)
- [Templates](templates.md)
- [Footnotes](footnotes.md)
- [Stylesheets](stylesheets.md)
- [EPUB and KEPUB](formats/epub.md)
- [Kindle KFX/AZW8](formats/kindle.md)
- [PDF](formats/pdf.md)
- [TXT and Markdown](formats/text.md)
- [MyHomeLib Integration](mhl.md)

## Troubleshooting

Use `--debug` first. The debug report is the fastest way to inspect parsed metadata, normalized content, generated resources, logs, and format-specific diagnostics.

Common checks:

| Problem | What To Check |
|---|---|
| Conversion fails | Run with `--debug`; inspect `fbc-report.zip` and logs |
| Archive paths look wrong | Try `--force-zip-cp windows-1251` or another legacy encoding |
| Output file is not created | Check destination path, overwrite flag, and output-name template |
| Configuration is ignored | Use `dumpconfig active.yaml` to confirm the loaded values |
| Template output is unexpected | Check `.Context`, `.Format`, and template-specific fields in [Templates](templates.md) |
| Fonts or CSS do not apply | Check [Stylesheets](stylesheets.md) and format-specific styling limits |
| Images are missing | Check `document.images.*` settings and debug image diagnostics |

When reporting an issue, include the command line, relevant configuration, converter version, and the debug report when possible.
