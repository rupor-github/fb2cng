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
| `document.kfx.compression` | Opt-in KFX/AZW8 compression (`none` or `lzma`) | [Kindle compression](formats/kindle.md#compression) |
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

## Image Settings

```yaml
document:
  images:
    use_broken: false
    remove_transparency: false
    rasterize_svg: false
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

- `optimize` enables optional JPEG/PNG recompression. It does not control resizing, transparency removal, or output-format compatibility conversions.
- With `optimize: true`, JPEGs are re-encoded if their detected quality exceeds `jpeg_quality_level`; PNGs are recompressed using best compression, except for PDF output, which skips optional PNG recompression. Grayscale detection is applied during JPEG encoding when optimization is enabled.
- Kindle output normalizes decodable raster images to JPEG.
- KEPUB/KEPUB3 converts decodable WebP images to lossless PNG for Kobo compatibility. Transparency is preserved unless `remove_transparency` is enabled. Ordinary EPUB2/EPUB3 output does not enable this Kobo-specific conversion.
- These format conversions apply to covers and body images, even with `optimize: false`.
- `remove_transparency: true` flattens transparent PNG, GIF, TIFF, and WebP images onto a white background. Kindle output applies this automatically before JPEG encoding, regardless of the setting.
- For EPUB/KEPUB output, flattened GIF, TIFF, and WebP images become PNG: the image encoder supports only PNG and JPEG, and PNG avoids introducing JPEG artifacts. Opaque images are not converted merely because `remove_transparency` is enabled.
- `rasterize_svg: true` explicitly selects raster rendering, independently of `optimize`. With `false` (the default), each backend keeps its normal behavior; this does not prohibit raster fallback or required rasterization.
- SVG normally stays SVG in EPUB/KEPUB and external Markdown. When forced to raster, it becomes PNG with transparency preserved unless `remove_transparency: true` requests a white background.
- Kindle keeps supported non-cover SVG artwork vector-based; unsupported artwork, covers, forced rasterization and transparency removal use JPEG on white. Embedded Markdown always rasterizes SVG to JPEG on white. Existing raster-to-JPEG compatibility rules remain unchanged.
- Optional SVG rasterization uses `screen.width`, preserving aspect ratio, then applies cover resizing or body `scale_factor`. Placeholders retain intrinsic sizing before those transformations. SVGs without viewBox dimensions fall back to 2048 per missing dimension; raster dimensions are capped at 8192.
- PDF normally keeps supported SVG artwork vector-based, preserving transparency. Unsupported features trigger whole-image raster fallback. `rasterize_svg: true` forces raster output; `remove_transparency: true` also selects white-background raster fallback for SVG. Both representations retain PDF's intrinsic SVG dimensions and existing sizing behavior (no additional screen-width, cover, or scale-factor transformation). PDF raster output still flattens onto white. See [PDF SVG support](formats/pdf.md#svg-artwork) for the supported subset.
- Built-in vignette stroke adjustments apply only to Kindle and embedded-Markdown rasterization, not native vectors or optional PNG rasterization. Generated default covers use native vectors in PDF and JPEG elsewhere; external SVG covers follow the output's cover policy.
- For Kindle output, `cover.resize: none` is effectively treated as `keepAR`.
- For KEPUB/KEPUB3, `cover.resize: none` disables cover resizing but does not prevent WebP-to-PNG conversion. Covers are not forced to JPEG.

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
