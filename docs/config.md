# Configuration Reference

fb2cng configuration is YAML. The easiest way to start is to dump the default configuration and edit it.

```bash
fbc dumpconfig --default config.yaml
```

For all configuration details, either generate the default configuration with the command above or inspect the source template at [`config/config.yaml.tmpl`](../config/config.yaml.tmpl).

Show the active configuration after defaults and overrides are merged:

```bash
fbc -c config.yaml dumpconfig active.yaml
```

## Loading Rules

- Defaults are always available.
- A custom config file is merged over the defaults.
- Unknown YAML fields are errors.
- File path fields are sanitized and path traversal is rejected where applicable.
- Template fields are not expanded while loading; they are expanded later in the relevant rendering context.

## Main Structure

```yaml
version: 1
document:
  output_name_template: "..."
  stylesheet_path: "..."
  images: {}
  footnotes: {}
  annotation: {}
  toc_page: {}
  metainformation: {}
  vignettes: {}
  dropcaps: {}
  page_map: {}
  text_transformations: {}
  kfx:
    compression: none
  pdf:
    page_frame: 0
logging: {}
reporting: {}
```

Use `dumpconfig --default` for the complete structure and current defaults.

## Frequently Used Document Settings

| Setting | Purpose | Reference |
|---|---|---|
| `document.output_name_template` | Output file names | [Templates](templates.md#output_name_template) |
| `document.stylesheet_path` | Custom CSS stylesheet | [Stylesheets](stylesheets.md) |
| `document.images.markdown` | Markdown image output mode | [Markdown images](images.md#markdown-images) |
| `document.images.screen` | Target screen profile and PDF page size | [Image sizing](images.md#sizing-and-resolution) |
| `document.images.cover` | Cover fallback and resize behavior | [Covers](images.md#covers) |
| `document.footnotes.mode` | Footnote processing mode | [Footnotes](footnotes.md) |
| `document.footnotes.label_template` | Logical footnote labels | [Templates](templates.md#label_template) |
| `document.footnotes.backlink_template` | Generated backlink text | [Templates](templates.md#backlink_template) |
| `document.toc_type` | Device navigation shape | [Guide](guide.md#feature-matrix) |
| `document.kfx.compression` | Opt-in KFX/AZW8 compression (`none` or `lzma`) | [Kindle compression](formats/kindle.md#compression) |
| `document.pdf.page_frame` | Optional nearly-white PDF outline to discourage automatic margin cropping | [PDF page frame](formats/pdf.md#discouraging-automatic-margin-cropping) |
| `document.toc_page` | Optional visible TOC page | [Guide](guide.md#common-tasks) |
| `document.metainformation.*_template` | Metadata formatting | [Templates](templates.md#metadata-templates) |

## KFX Compression

Both KFX and AZW8 support optional, lossless LZMA compression without encryption. To enable
it persistently:

```yaml
document:
  kfx:
    compression: lzma
```

`document.kfx.compression` defaults to `none` (uncompressed output). Set it to `lzma`
for lossless compression without encryption. It applies equally to KFX and AZW8 and has no effect
on other formats. Compression sizing is [automatic](formats/kindle.md#compression).
An explicitly supplied `convert --kfx-compression none|lzma` overrides
the YAML value; an omitted flag respects YAML. Logs and conversion debug text record the
effective choice/source because archived loaded YAML precedes CLI overrides.

```bash
fbc convert --to azw8 --kfx-compression lzma book.fb2
fbc -c config.yaml convert --to azw8 --kfx-compression none book.fb2
```

## PDF Page Frame

Some PDF readers automatically crop margins and rescale or recenter pages. An
optional nearly-white outline can discourage this behavior:

```yaml
document:
  pdf:
    page_frame: 0.5pt
```

`document.pdf.page_frame` defaults to `0` (disabled). Set a thickness in `pt` or
`px`; pixels use `document.images.screen.dpi`, so `1px` equals `0.24pt` at 300 DPI.
Negative values, unsupported units, nonzero values without units, and thicknesses
as large as either page dimension are rejected.

The outline is drawn inside every page, including covers and blank pages, behind
all content. It does not change layout, pagination, or negative-margin behavior.
It applies only to PDF output. Effectiveness depends on the reader and firmware;
see [PDF page frame](formats/pdf.md#discouraging-automatic-margin-cropping).

## Image Settings

See [Image handling](images.md) for the format capability tables, current defaults,
configuration precedence, SVG support, sizing, covers, Markdown images, and vignettes.
It is the main reference for `document.images` and `document.vignettes`.

## Text Transformations

`document.text_transformations` cleans up common legacy FB2 typography problems.

```yaml
document:
  text_transformations:
    speech:
      enable: true
      from: "‐‑−–—―"
      to: "— "
    dashes:
      enable: true
      from: "‐‑−–—―"
      to: "—"
    dialogue:
      enable: true
      from: "‐‑−–—―"
      to: " "
```

The transformations run in this order: `speech`, then `dashes`, then `dialogue`.

Scope limitation: text transformations apply to regular paragraph content in section bodies. They do not apply to titles, subtitles, poems, cites, epigraphs, tables, or other special structures.

## Logging And Reports

Debug reports are enabled with `--debug`. By default, logs and reports use `{{ .AppName }}` based templates and are written as `fbc.log`, `fbc-panic.log`, and `fbc-report.zip` in the current working directory.

Artifact destination templates support `.Context`, `.AppName`, `.PID`, `.Hostname`, `.Started`,
`.Unique`, `.Command`, `.Format`, `.SourceFile`, and `.OutputDir`. FB2 metadata is not available
because logs and reports are initialized before the source book is parsed.

Use `.OutputDir` to place the file log and report beside a single-book result:

```yaml
logging:
  file:
    mode: overwrite
    destination_template: '{{ .OutputDir }}/{{ .AppName }}.log'
    panic_destination_template: '{{ .OutputDir }}/{{ .AppName }}-panic.log'
reporting:
  destination_template: '{{ .OutputDir }}/{{ .AppName }}-report.zip'
```

It follows `DESTINATION`, the parent of `--output-file`, or the current directory when neither
is specified. For batches and output templates that create subdirectories, it means the
destination root. See [Artifact destination templates](templates.md#artifact-destination-templates).

See [Debugging and validation](debugging.md#converter-debug-reports) for archive contents,
format-specific diagnostics, and the logging behavior of `--debug`.
