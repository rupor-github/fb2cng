# Footnotes

Footnotes are detected from FB2 bodies whose names match `document.footnotes.bodies`.

```yaml
document:
  footnotes:
    mode: default
    bodies: ["notes", "comments", "примечания", "комментарии"]
```

## Modes

`document.footnotes.mode` controls conversion behavior.

```yaml
document:
  footnotes:
    mode: float
```

| Mode | Meaning |
|---|---|
| `default` | Keep notes as ordinary linked document sections, with generated backlinks where supported |
| `float` | Convert notes to popup/printed/endnote behavior where the format supports it |
| `floatRenumbered` | Same target behavior as `float`, but normalize logical note labels using `label_template` |

## Mode By Format

| Format | `default` | `float` | `floatRenumbered` |
|---|---|---|---|
| EPUB2 | Normal footnote sections with backlinks | Bidirectional links | Bidirectional links with normalized labels |
| EPUB3 | Normal footnote sections with backlinks | EPUB `aside` / `noteref` markup | EPUB `aside` / `noteref` markup with normalized labels |
| KEPUB | Normal footnote sections with backlinks | Bidirectional links | Bidirectional links with normalized labels |
| KFX/AZW8 | Normal linked sections with backlink paragraphs; no popup markers | Kindle popup footnotes with backlink paragraphs | Popup footnotes with normalized labels and backlinks |
| PDF | Normal linked sections with generated return links | Printed page footnotes | Printed page footnotes with page-local labels |
| TXT | Normal sections | Final `Notes` section | Final `Notes` section with normalized labels |
| Markdown | Normal linked sections with backlinks | Final `Notes` section with backlinks | Final `Notes` section with normalized labels and backlinks |

## Backlinks

Generated backlinks are controlled by `document.footnotes.backlink_template`.

Formats that generate backlinks:

| Format | Default mode | Float modes |
|---|---:|---:|
| EPUB2/KEPUB | yes | yes |
| EPUB3 | yes | yes |
| KFX/AZW8 | yes | yes |
| PDF | yes, in footnote body sections | printed footnotes use page/link handling |
| TXT | no clickable links | no clickable links |
| Markdown | yes | yes |

In `default` mode, generated backlinks do not imply popup/floating behavior. For example, KFX/AZW8 default footnote links navigate to normal footnote sections; only float modes emit Kindle popup markers.

See [Templates](templates.md#backlink_template) for all fields.

Markdown-specific notes:

- Backlinks are generated in `default`, `float`, and `floatRenumbered` modes.
- `.Href` is the actual Markdown backlink target.
- `.Filename` is the output `.md` file name when known.
- `.LocationNumber` is the 1-based rendered Markdown block number containing the original reference anchor.
- Empty referenced notes still get generated anchors and backlinks.

## Labels

`label_template` controls logical footnote labels in `floatRenumbered` mode for EPUB/KFX/AZW8/TXT/Markdown. PDF printed footnotes have page-local printed labels; for PDF, `label_template` formats title text appended after that page-local label.

See [Templates](templates.md#label_template) for fields and examples.

Markdown preserves inline-image footnote labels instead of replacing them with plain text. This supports books where visible note markers are images.

## More Paragraph Indicator

When a floating note has more than one visible content element, some readers may show only the first one in a popup. `document.footnotes.more_paragraphs` prepends an indicator to the first visible item.

```yaml
document:
  footnotes:
    more_paragraphs: "(~) "
```

Paragraphs, images, and tables count as visible content elements. The indicator is styled with `.footnote-more`; the default stylesheet shows it for EPUB and hides it for KFX.

## Empty Notes

Empty referenced notes are handled intentionally:

- KFX/AZW8 register the footnote anchor and can still generate backlink paragraphs.
- Markdown emits a generated heading/anchor and backlink even when the note body has no rendered content.
- Default modes keep empty notes as ordinary document sections where the format supports anchors.

This keeps references clickable even for empty or placeholder notes.
