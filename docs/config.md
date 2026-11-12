# Configuration Reference

fb2cng configuration is UTF-8 YAML. The easiest way to start is to dump the default configuration and edit it.

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
- You can supply only the fields you want to change; a complete copy is not required.
- Unknown YAML fields are errors.
- File paths are checked where required. Paths inside stylesheet URLs have additional
  [resource-path restrictions](stylesheets.md#resource-path-resolution).
- Template fields are not expanded while loading; they are expanded later in the relevant rendering context.

Save the YAML fragments in this guide as a configuration file and select it with
`fbc -c config.yaml convert ...`. Format-specific settings are ignored by other
outputs, but the whole configuration must still be valid.

Relative file paths in YAML, such as `stylesheet_path`, cover files, and vignette
files, are relative to the directory where you run `fbc`, not the configuration
file's directory. Relative URLs inside a stylesheet are instead resolved from
the stylesheet's directory. Use absolute YAML paths when invoking `fbc` from
different working directories.

## Main Structure

```yaml
version: 1
document:
  output_name_template: '{{ .Title }}'
  # stylesheet_path: mystyle.css
  fix_zip: false
  open_from_cover: false
  toc_type: normal
  file_name_transliterate: false
  insert_soft_hyphen: false
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
  azw3:
    compression: standard
    apnx: none
  pdf:
    page_frame: 0
  markdown:
    images: placeholder
logging: {}
reporting: {}
```

This is a structural example, not a copy of every default: its short naming
template uses only the title. Empty mappings such as `images: {}` leave defaults
in place. Use `dumpconfig --default` for the complete structure and current defaults.

## Frequently Used Document Settings

| Setting | Purpose | Reference |
|---|---|---|
| `document.output_name_template` | Output file names | [Templates](templates.md#output_name_template) |
| `document.stylesheet_path` | Custom CSS stylesheet | [Stylesheets](stylesheets.md) |
| `document.markdown.images` | Markdown image output mode | [Markdown images](images.md#markdown-images) |
| `document.images.screen` | Target screen profile and PDF page size | [Image sizing](images.md#sizing-and-resolution) |
| `document.images.cover` | Cover fallback and resize behavior | [Covers](images.md#covers) |
| `document.footnotes.mode` | Footnote processing mode | [Footnotes](footnotes.md) |
| `document.footnotes.label_template` | Logical footnote labels | [Templates](templates.md#label_template) |
| `document.footnotes.backlink_template` | Generated backlink text | [Templates](templates.md#backlink_template) |
| `document.toc_type` | Contents-navigation nesting | [Navigation](#navigation-and-page-maps) |
| `document.kfx.compression` | Opt-in KFX/AZW8 compression (`none` or `lzma`) | [KFX/AZW8 compression](formats/kfx.md#compression) |
| `document.azw3.compression` | Native AZW3 compression (`none`, `standard`, or `huffdic`) | [AZW3 compression](formats/azw3.md#compression) |
| `document.azw3.apnx` | Config-only APNX sidecar (`none`, `eink`, or `app`) | [AZW3 page map](formats/azw3.md#optional-apnx-page-map) |
| `document.pdf.page_frame` | Optional nearly-white PDF outline to discourage automatic margin cropping | [PDF page frame](formats/pdf.md#discouraging-automatic-margin-cropping) |
| `document.toc_page` | Optional visible contents page/list | [Navigation](#navigation-and-page-maps) |
| `document.metainformation.*_template` | Metadata formatting | [Templates](templates.md#metadata-templates) |

## Output Names And Compatibility

The settings below are under `document`.

| Setting | Default | Purpose |
|---|---|---|
| `output_name_template` | Author/title template | Generated output basename; see [Templates](templates.md#output_name_template). Ignored with `--output-file`. |
| `file_name_transliterate` | `false` | Transliterate generated output names after template expansion; separate from metadata transliteration. |
| `stylesheet_path` | Built-in stylesheet | Replace the stylesheet for styled formats; see [Stylesheets](stylesheets.md). |
| `fix_zip` | `false` | Use older ZIP packaging for EPUB/KEPUB readers that cannot handle normal ZIP data descriptors. |
| `open_from_cover` | `false` | Request a cover opening target in EPUB/KEPUB and KFX/AZW8; the reader may ignore it. AZW3 starts at the text. |
| `insert_soft_hyphen` | `false` | Insert optional hyphenation hints in EPUB/KEPUB/AZW3 and KFX/AZW8 text. Can conflict with reader hyphenation. |

TXT/Markdown ignore visual CSS and this hyphenation switch. PDF uses its own
layout hyphenation; `insert_soft_hyphen` is not a universal reader-layout switch.

## Navigation And Page Maps

| Setting | Default | Purpose |
|---|---|---|
| `document.toc_type` | `normal` | `normal` preserves ordinary nesting, `old_kindle` limits nesting for older Kindle compatibility, and `flat` promotes entries to siblings. |
| `document.toc_page.placement` | `none` | Add visible contents `before` or `after` the main text; `none` disables it. |
| `document.toc_page.authors_template` | Author-name template | Author text used in the visible contents heading where supported; see [Templates](templates.md#toc-page-author-template). |
| `document.toc_page.include_chapters_without_title` | `false` | Include otherwise untitled chapters in contents/navigation. |
| `document.page_map.enable` | `true` | Generate approximate page navigation where supported. |
| `document.page_map.size` | `2300` | Approximate page size in Unicode code points; minimum `500`. |
| `document.page_map.adobe_de` | `false` | EPUB2/KEPUB-only Adobe page map; nonstandard and causes EPUBCheck errors. |

Device contents and a visible contents page are separate features. EPUB/KEPUB and
AZW3 visible contents retain their full section hierarchy independently of
`toc_type`. PDF, TXT, and Markdown visible contents use the selected navigation
shape. KFX/AZW8 visible contents also retain their own content hierarchy.
PDF outline bookmarks retain the section hierarchy rather than using `toc_type`.

Page maps apply to EPUB/KEPUB and KFX/AZW8. AZW3 requires an explicit
[`document.azw3.apnx`](formats/azw3.md#optional-apnx-page-map) choice as well as
`page_map.enable`. These are generated reading pages, not print-edition pages.
PDF has physical pages; TXT and Markdown have no page map. Disabling a page map
does not remove Kindle locations.

## Footnote Settings

All settings below are under `document.footnotes`.

| Setting | Default | Purpose |
|---|---|---|
| `mode` | `default` | Ordinary sections; `float` and `floatRenumbered` select format-specific popup, printed, or endnote behavior. |
| `bodies` | `notes`, `comments`, `примечания`, `комментарии` | FB2 body names treated as notes. |
| `label_template` | Numeric labels; descriptive PDF title text | Format normalized labels or printed-note title text. |
| `backlink_template` | AZW3 off; other supporting formats on | Generated returns. A valid empty template result suppresses them. |
| `more_paragraphs` | `"(~) "` | Indicator for a multi-part floating note; visibility depends on format and CSS. |

Use `backlink_template: '{{- "" -}}'` to suppress generated returns; a literal
empty YAML string is rejected. This does not remove AZW3's independent leading
title return or alter PDF printed notes. See [Footnotes](footnotes.md) for the
complete mode matrix and [Templates](templates.md#backlink_template) for fields.

## Metadata And Optional Content

| Setting | Default | Purpose |
|---|---|---|
| `document.metainformation.title_template` | Series-prefix/title template | Exported metadata title, not source headings. |
| `document.metainformation.creator_name_template` | Last name, first name, middle name (space-separated), or nickname | Exported author/translator name formatting. |
| `document.metainformation.transliterate` | `false` | Transliterate exported metadata title/names after template expansion. |
| `document.annotation.enable` | `false` | Add a separate section containing the FB2 annotation. |
| `document.annotation.title` | `Annotation` | Heading for that section. |
| `document.annotation.in_toc` | `true` | Include the enabled annotation section in contents/navigation. |
| `document.dropcaps.enable` | `false` | Enable decorative initial letters in styled output. |
| `document.dropcaps.ignore_symbols` | Quotes, punctuation, and digits | Skip these leading characters when choosing a drop cap. |

See [metadata support](guide.md#metadata-by-format) for fields available in each
format. TXT uses source metadata rather than metadata templates; Markdown applies
them to front matter. A visible annotation is independent of description metadata.
TXT/Markdown render drop caps as ordinary text and ignore vignettes.

## Format-Specific Settings

These are the complete format-specific sections; other behavior uses shared settings.

| Setting | Default | Accepted values |
|---|---|---|
| `document.kfx.compression` | `none` | `none`, `lzma`; applies to both KFX and AZW8 |
| `document.azw3.compression` | `standard` | `none`, `standard`, `huffdic` |
| `document.azw3.apnx` | `none` | `none`, `eink`, `app`; configuration-file only |
| `document.pdf.page_frame` | `0` | Disabled (`0`) or a thickness in `pt`/`px` |
| `document.markdown.images` | `placeholder` | `placeholder`, `external`, `embedded` |

There are no `document.epub`, `document.kepub`, `document.azw8`, or `document.txt`
sections. The old `document.images.markdown` key is rejected; move it to
`document.markdown.images`.

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
on other formats. Compression sizing is [automatic](formats/kfx.md#compression).
An explicitly supplied `convert --compression none|lzma` overrides
the YAML value; an omitted flag respects YAML. Logs and conversion debug text record the
effective choice and whether it came from YAML or the command line, because the
archived configuration is captured before CLI overrides.

```bash
fbc convert --to azw8 --compression lzma book.fb2
fbc -c config.yaml convert --to azw8 --compression none book.fb2
```

## AZW3 Compression

`document.azw3.compression` selects `none` (KindleGen `-c0`), `standard`
(KindleGen `-c1`, the default), or `huffdic` (KindleGen `-c2`). These are mode
correspondences; native AZW3 generation does not invoke KindleGen.
An explicitly supplied `convert --compression` overrides it for AZW3; omitting the
flag respects YAML. KFX/AZW8 modes are separate: `lzma` is not valid for AZW3, and
`standard`/`huffdic` are not valid for KFX/AZW8.

```bash
fbc convert --to azw3 --compression huffdic book.fb2
fbc -c config.yaml convert --to azw3 --compression none book.fb2
```

See [AZW3 compression](formats/azw3.md#compression). APNX selection remains YAML-only.

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
It is the main reference for every `document.images` and `document.vignettes`
field, as well as `document.markdown.images`.

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

They are all **disabled by default**; the example deliberately enables them.
`speech` replaces a leading dialogue dash and following spaces, `dashes`
normalizes space-surrounded dashes, and `dialogue` replaces the spaces before a
leading dialogue dash. Each enabled rule requires nonempty `from` and `to` strings.

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
