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
| KEPUB3 | Normal footnote sections with backlinks | EPUB `aside` / `noteref` markup | EPUB `aside` / `noteref` markup with normalized labels |
| KFX/AZW8 | Normal linked sections with backlink paragraphs; no popup markers | Kindle popup footnotes with backlink paragraphs | Popup footnotes with normalized labels and backlinks |
| AZW3 | Normal footnote sections; template-controlled backlinks, off by default | Single-paragraph plain-text notes with a leading title return and optional trailing backlinks | Same behavior with normalized labels |
| PDF | Normal linked sections with generated return links | Printed page footnotes | Printed page footnotes with page-local labels |
| TXT | Normal sections | Final `Notes` section | Final `Notes` section with normalized labels |
| Markdown | Normal linked sections with backlinks | Final `Notes` section with backlinks | Final `Notes` section with normalized labels and backlinks |

## Backlinks

Generated backlinks are controlled by `document.footnotes.backlink_template`.
A valid template producing empty or whitespace-only text suppresses generated
returns in every supporting format. The bundled template disables them for AZW3
and enables them for other formats; TXT does not generate backlinks.

Formats that generate backlinks:

| Format | Default mode | Float modes |
|---|---:|---:|
| EPUB2/KEPUB | yes | yes |
| EPUB3/KEPUB3 | yes | yes |
| KFX/AZW8 | yes | yes |
| AZW3 | template-controlled, off by default | leading title return; trailing backlinks template-controlled, off by default |
| PDF | yes, in footnote body sections | printed footnotes use page/link handling |
| TXT | no clickable links | no clickable links |
| Markdown | yes | yes |

In `default` mode, generated backlinks do not imply popup/floating behavior. For example, KFX/AZW8 default footnote links navigate to normal footnote sections; only float modes emit Kindle popup markers.

For AZW3, a valid `backlink_template` that produces empty or whitespace-only text
suppresses additional generated backlinks in **all three modes**. The default
template produces empty text for AZW3 to avoid clutter and confusing Kindle's
popup detection. A custom nonempty result enables these links, including in
`default` mode. Existing custom templates are not automatically rewritten.

This does not disable the leading title/label return link in AZW3 float modes.
In `default`, authored links and normal note formatting are preserved.

For PDF, a valid empty or whitespace-only result suppresses generated returns in
`default` mode without disabling forward links to notes. Printed footnotes in
`float` and `floatRenumbered` do not generate these backlinks and are unaffected.
The bundled template still enables PDF backlinks by default.

For Markdown, a valid empty or whitespace-only result suppresses generated returns
in all three modes. Forward note links, note anchors and reference anchors remain.
The bundled template still enables Markdown backlinks by default.

For KFX/AZW8, a valid empty or whitespace-only result suppresses generated return
paragraphs in all three modes. Forward note links and floating-note popup markers
remain; empty notes retain an addressable target. The bundled template still
enables KFX/AZW8 backlinks by default.

For EPUB/KEPUB, empty or whitespace-only results suppress generated returns in
all three modes. Forward links, note targets and EPUB3 `footnote`/`noteref`
semantics remain. Returns are enabled by default for reader compatibility;
disabling them can affect return navigation and reader-dependent popup recognition
or boundaries, especially in EPUB2. Validate with the readers you use.

Missing or invalid templates still fall back to `[<]`, including execution errors.
See [Templates](templates.md#backlink_template).

Popup behavior still depends on the Kindle device or app. See [Native AZW3](formats/azw3.md).

See [Templates](templates.md#backlink_template) for all fields.

Markdown-specific notes:

- Backlinks are generated in `default`, `float`, and `floatRenumbered` modes.
- `.Href` is the actual Markdown backlink target.
- `.Filename` is the output `.md` file name when known.
- `.LocationNumber` is the 1-based rendered Markdown block number containing the original reference anchor.
- Empty referenced notes still get generated anchors and, when enabled, backlinks.

## Labels

`label_template` controls logical footnote labels in `floatRenumbered` mode for EPUB/KEPUB/AZW3/KFX/AZW8/TXT/Markdown. PDF printed footnotes have page-local printed labels; for PDF, `label_template` formats title text appended after that page-local label.

See [Templates](templates.md#label_template) for fields and examples.

Markdown preserves inline-image footnote labels instead of replacing them with plain text. This supports books where visible note markers are images.

## AZW3 Floating Notes

AZW3 `float` and `floatRenumbered` place each complete note in one paragraph:

- When a clickable reference exists, the note title (`float`) or normalized label
  (`floatRenumbered`) is a return link to the first reference, followed by a
  nonbreaking space and the note text.
- In `float`, every nonempty original title is enclosed in square brackets, such as
  `[Примечание 1]`, to distinguish it from the plain-text body. Titles already
  enclosed in `[]` are not wrapped again. This also applies to unlinked titles;
  bracketing does not create a return link without a destination. Forward-reference
  captions are unchanged. In `floatRenumbered`, `label_template` controls the label
  without automatic brackets, whether linked or unlinked.
- Formatting is removed. Former paragraphs, verses, and table rows are separated
  by literal newlines; table cells use spaces. Explicit empty lines are retained.
- Images become `[img:alt text]`, or `[img:]` when they have no usable alt text.
- Authored links become `[link:caption]`, or `[link:original destination]` when
  the captions are empty. Already-bracketed captions, such as `[2]`, are preserved
  without another wrapper. If source brackets surround the link element instead,
  they are reused for a single `[link:caption]`, not `[[link:caption]]`, including
  across formatting boundaries. These are plain-text placeholders, not clickable
  links, including references to other notes. Both placeholder rules apply in
  `float` and `floatRenumbered`; default-mode images and links remain intact.
- When enabled by `backlink_template`, occurrence-specific trailing return links
  follow the text after a newline inside the same paragraph. Empty template
  results omit those links and their separators. The leading return uses the
  note title/label instead and remains clickable even when trailing links are
  suppressed. Notes without references keep an unlinked title.

Other output formats keep their own note formatting and link behavior.

## More Paragraph Indicator

When a floating note has more than one visible content element, some readers may show only the first one in a popup. `document.footnotes.more_paragraphs` prepends an indicator to the first visible item.

```yaml
document:
  footnotes:
    more_paragraphs: "(~) "
```

Paragraphs, images, and tables count as visible content elements. The indicator is styled with `.footnote-more`; the default stylesheet shows it for EPUB/KEPUB and AZW3 (`not amzn-et`), and hides it for KFX/AZW8 (`amzn-et`).

For AZW3, CSS controls whether the indicator text is included during conversion:
`.footnote-more { display: none; }` suppresses it. Overriding this with
`.footnote-more { display: inline; }` includes it when the flattened note has
multiple nonempty lines and `more_paragraphs` is nonempty. The result remains
plain text, without a styling wrapper.

## Empty Notes

Empty referenced notes are handled intentionally:

- KFX/AZW8 retain the footnote target even when generated returns are suppressed.
- Markdown emits a generated heading/anchor and, when enabled, backlinks even when the note body has no rendered content.
- Default modes keep empty notes as ordinary document sections where the format supports anchors.

This keeps references clickable even for empty or placeholder notes.
