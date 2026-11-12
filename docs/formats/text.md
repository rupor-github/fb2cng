# TXT And Markdown Output

TXT and Markdown output are generated from the same normalized FB2 model as e-book formats, but they intentionally avoid CSS/layout features that do not make sense in plain text.

## Command Line

```bash
fbc convert --to txt book.fb2
fbc convert --to md book.fb2
```

The output extensions are `.txt` and `.md`. Shared naming, destination, and
overwrite options are described in the [command-line guide](../guide.md#command-line).

## Configuration

Both formats use the shared footnote, annotation, visible-contents, and text
transformation settings. Markdown adds `document.markdown.images`; TXT has no
format-specific configuration section. See [Configuration](../config.md).

```yaml
document:
  markdown:
    images: external
  footnotes:
    mode: floatRenumbered
```

This example exports Markdown images as separate files and collects notes with
normalized labels. The defaults are placeholder images and `default` footnotes.

## Structure And Navigation

Both formats preserve headings and readable content order. Neither has a separate
device-navigation map or page map. `document.toc_page.placement` adds a contents
list before or after the main text; `document.toc_type` controls its nesting.
TXT lists are plain text; Markdown entries link to explicit anchors.

### Markdown Links And Anchors

Markdown emits explicit HTML anchors before headings/targets so internal links
do not depend on a viewer's heading-name rules, including for repeated or
non-Latin titles. Visible contents entries link to these anchors. TXT renders
the same content as plain readable text without links.

### TXT Output

TXT output:

- Produces UTF-8 readable plain text.
- Uses plain headings, blank lines, aligned text tables, image placeholders, and readable footnote labels.
- Does not create external assets or embedded images.
- Does not have clickable links because the file format has no link model.
- Collects floating notes into a final `Notes` section.

### Markdown Output

Markdown output:

- Produces UTF-8 semantic Markdown with headings, paragraphs, links, tables, block quotes, fenced code blocks, and optional image output.
- Starts with YAML front matter containing available book metadata; see the
  [metadata matrix](../guide.md#metadata-by-format) for the complete list.
- Uses `document.metainformation.title_template` and `creator_name_template` for YAML front matter title and author values.
- Emits explicit anchors for section and note targets.
- Uses Markdown pipe tables, including inside collected endnotes.
- Preserves code listings as fenced code blocks when a paragraph is entirely code.
- Supports placeholder, external, or embedded images.
- Ignores CSS-only visual decorations, custom fonts, and dropcap styling.

## Styling

TXT uses plain headings, spacing, and text tables. Markdown uses semantic markup
that the viewer styles. Custom CSS, font selection, vignettes, and visual drop-cap
rules do not affect either output. Shared text transformations still run before
rendering.

## Images And Fonts

TXT always uses image placeholders. Markdown supports placeholders, external
assets, and embedded images. Neither format embeds fonts or a dedicated cover.

### Markdown Images

`document.markdown.images` selects `placeholder` (the default), `external`, or `embedded`.
See [Markdown images](../images.md#markdown-images) for asset placement and viewer
compatibility, and [Image handling](../images.md) for SVG, transparency, and sizing rules.

## Footnotes

| Mode | TXT | Markdown |
|---|---|---|
| `default` | Footnotes are normal sections | Footnotes are normal linked sections with backlinks |
| `float` | Referenced notes are collected into final `Notes` | Referenced notes are collected into final `Notes` with backlinks |
| `floatRenumbered` | Same as `float`, with normalized labels | Same as `float`, with normalized labels and backlinks |

Markdown backlink details:

- Footnote references receive stable anchors such as `<a id="ref-note-1-1"></a>`.
- Generated footnote backlinks link back to those anchors using `backlink_template`.
- A valid empty or whitespace-only template result suppresses backlinks in all three
  modes without removing forward links, note anchors or reference anchors. For
  example, use `{{- "" -}}`; an empty YAML value is not valid. The bundled template
  keeps Markdown backlinks enabled by default.
- `.Href` in `backlink_template` is the actual Markdown backlink target.
- `.LocationNumber` is the 1-based rendered Markdown block number containing the original reference anchor.
- Empty referenced footnotes still get generated anchors and, when enabled, backlinks.
- Inline images used as visible footnote labels are preserved.

See [Footnotes](../footnotes.md) and [Templates](../templates.md#backlink_template).

## Limitations

- Vignettes and CSS-only decorations are ignored.
- Drop caps are rendered as ordinary text.
- Fonts and stylesheets do not affect TXT/Markdown rendering, except that text transformations and normalized document content still apply before rendering.
- Markdown viewer support varies for embedded `data:` images and raw HTML anchors.
- Pipe-table rendering requires a Markdown viewer with table support; it is not
  part of basic CommonMark. Complex FB2 table layout is simplified.

## Debugging And Validation

Generate a [debug report](../debugging.md#epubkepub-and-txtmarkdown) with
`fbc -d convert ...` and compare the text with the prepared book. No dedicated
external validator is supplied for these formats. Open Markdown in the intended
viewer and check links, tables, and images; keep external image directories with
the document when moving or sharing it.
