# Kindle KFX/AZW8 Output

This page covers `kfx` and `azw8` output. `azw8` uses the same KFX payload with a different extension for workflows that prefer Kindle Previewer-friendly file names.

## Command Line

```bash
fbc convert --to kfx book.fb2
fbc convert --to azw8 book.fb2
```

Kindle-specific options:

| Option | Meaning |
|---|---|
| `--ebook`, `--eb` | Mark output as ebook/EBOK instead of personal document/PDOC |
| `--asin ASIN` | Set Kindle ASIN metadata for Kindle output |

## Styling Model

KFX output maps the normalized book model and CSS-derived styles into Kindle Enhanced Typesetting structures.

Important differences from EPUB:

- CSS is converted into Kindle style records, not embedded as browser CSS.
- `@media amzn-et` targets KFX/Enhanced Typesetting.
- The bare `body { font-family: ... }` rule can set the Kindle document default font when matching `@font-face` resources exist.
- Negative margins are preserved generally, but KFX avoids negative horizontal margins on dropcap paragraphs because Kindle Previewer lays those out poorly.

See [Stylesheets](../stylesheets.md) for shared stylesheet rules and resource handling.

## Images

- Kindle output requires a cover. If the source book has no valid cover, fb2cng forces cover generation and uses the configured default cover image or the built-in generated cover.
- If `document.images.cover.resize` is `none`, Kindle output treats it as `keepAR` so the generated or source cover is still suitable for Kindle packaging.
- All decodable non-JPEG raster images are converted to JPEG for Kindle compatibility.
- SVG images are rasterized and encoded as JPEG for Kindle output.
- Transparent PNG/GIF content is flattened onto a white background before JPEG encoding.
- Broken-image fallback SVGs are rasterized to JPEG as well, so missing or undecodable images still produce Kindle-compatible resources.

## Footnotes

| Mode | KFX/AZW8 Behavior |
|---|---|
| `default` | Footnotes are normal book sections; no generated backlinks |
| `float` | Footnote content is marked for Kindle popup rendering and backlink paragraphs are generated |
| `floatRenumbered` | Same as `float`, with normalized labels from `label_template` |

KFX resolves `.LocationNumber` for `backlink_template` from generated Kindle positions. See [Footnotes](../footnotes.md) and [Templates](../templates.md#backlink_template).

## Page And Location Maps

KFX/AZW8 can generate Kindle location and page-related navigation data from rendered content positions. Page-map behavior depends on `document.page_map` settings.
