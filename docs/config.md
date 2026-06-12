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
logging: {}
reporting: {}
```

Use `dumpconfig --default` for the complete structure and current defaults.

## Frequently Used Document Settings

| Setting | Purpose | Reference |
|---|---|---|
| `document.output_name_template` | Output file names | [Templates](templates.md#output_name_template) |
| `document.stylesheet_path` | Custom CSS stylesheet | [Stylesheets](stylesheets.md) |
| `document.images.markdown` | Markdown image output mode | [TXT and Markdown](formats/text.md#markdown-images) |
| `document.images.screen` | Target screen profile and PDF page size | [PDF](formats/pdf.md#page-size) |
| `document.images.cover` | Cover fallback and resize behavior | [Format guides](guide.md#reference-documents) |
| `document.footnotes.mode` | Footnote processing mode | [Footnotes](footnotes.md) |
| `document.footnotes.label_template` | Logical footnote labels | [Templates](templates.md#label_template) |
| `document.footnotes.backlink_template` | Generated backlink text | [Templates](templates.md#backlink_template) |
| `document.toc_type` | Device navigation shape | [Guide](guide.md#feature-matrix) |
| `document.toc_page` | Optional visible TOC page | [Guide](guide.md#common-tasks) |
| `document.metainformation.*_template` | Metadata formatting | [Templates](templates.md#metadata-templates) |

## Image Settings

```yaml
document:
  images:
    use_broken: false
    remove_transparency: false
    scale_factor: 1.0
    optimize: true
    jpeg_quality_level: 75
    markdown: placeholder
    screen:
      width: 1264
      height: 1680
      dpi: 300
    cover:
      generate: true
      default_image_path: ""
      resize: stretch
```

Important notes:

- Kindle output normalizes decodable raster images to JPEG.
- SVG stays SVG in EPUB but is rasterized for Kindle output.
- Markdown rasterizes SVG when `document.images.markdown: embedded` is used.
- For Kindle output, `cover.resize: none` is effectively treated as `keepAR`.

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

Debug reports are enabled with `--debug` and stored as `fb2cng-report.zip` next to the generated output. Use this when diagnosing parsing, image handling, stylesheets, fonts, footnotes, or output-format behavior.
