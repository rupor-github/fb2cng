# TXT And Markdown Output

TXT and Markdown output are generated from the same normalized FB2 model as e-book formats, but they intentionally avoid CSS/layout features that do not make sense in plain text.

## TXT Output

TXT output:

- Produces UTF-8 readable plain text.
- Uses plain headings, blank lines, aligned text tables, image placeholders, and readable footnote labels.
- Does not create external assets or embedded images.
- Does not have clickable links because the file format has no link model.
- Collects floating notes into a final `Notes` section.

## Markdown Output

Markdown output:

- Produces UTF-8 semantic Markdown with headings, paragraphs, links, tables, block quotes, fenced code blocks, and optional image output.
- Starts with YAML front matter containing title, authors, series, language, date, and genres when available.
- Uses `document.metainformation.title_template` and `creator_name_template` for YAML front matter title and author values.
- Emits explicit anchors for section and note targets.
- Uses Markdown pipe tables, including inside collected endnotes.
- Preserves code listings as fenced code blocks when a paragraph is entirely code.
- Supports placeholder, external, or embedded images.
- Ignores CSS-only visual decorations, custom fonts, and dropcap styling.

## Markdown Images

Configure image handling with `document.images.markdown`.

```yaml
document:
  images:
    markdown: placeholder
```

| Mode | Behavior |
|---|---|
| `placeholder` | Render readable placeholders such as `[Image: cover]` |
| `external` | Write assets next to the `.md` file and reference them with Markdown image syntax |
| `embedded` | Embed image data as `data:` URIs |

External assets are placed in a book-specific directory such as `images-<book-id>/` to avoid collisions. SVG images are rasterized when embedded.

## Markdown Links And Anchors

Markdown emits explicit HTML anchors before headings/targets so internal links remain stable even when titles are duplicated or non-Latin.

Visible TOC pages use Markdown links to these explicit anchors. TXT renders the same content as plain readable text without links.

## Footnotes

| Mode | TXT | Markdown |
|---|---|---|
| `default` | Footnotes are normal sections | Footnotes are normal linked sections; no generated backlinks |
| `float` | Referenced notes are collected into final `Notes` | Referenced notes are collected into final `Notes` with backlinks |
| `floatRenumbered` | Same as `float`, with normalized labels | Same as `float`, with normalized labels and backlinks |

Markdown float-mode details:

- Footnote references receive stable anchors such as `<a id="ref-note-1-1"></a>`.
- Generated endnotes link back to those anchors using `backlink_template`.
- `.Href` in `backlink_template` is the actual Markdown backlink target.
- `.LocationNumber` is the 1-based rendered Markdown block number containing the original reference anchor.
- Empty referenced footnotes still get generated endnote anchors and backlinks.
- Inline images used as visible footnote labels are preserved.

See [Footnotes](../footnotes.md) and [Templates](../templates.md#backlink_template).

## Limitations

- Vignettes and CSS-only decorations are ignored.
- Drop caps are rendered as ordinary text.
- Fonts and stylesheets do not affect TXT/Markdown rendering, except that text transformations and normalized document content still apply before rendering.
- Markdown viewer support varies for embedded `data:` images and raw HTML anchors.
