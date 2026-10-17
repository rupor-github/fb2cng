# Kindle KFX/AZW8 Output

This page covers `kfx` and `azw8` output. `azw8` uses the same KFX payload with a different extension for workflows that prefer Kindle Previewer-friendly file names.

Both formats support uncompressed output and **optional, lossless LZMA compression without
encryption**. Compression is disabled by default.

## Command Line

```bash
fbc convert --to kfx book.fb2
fbc convert --to azw8 book.fb2
fbc convert --to azw8 --kfx-compression lzma book.fb2
```

Kindle-specific options:

| Option | Meaning |
|---|---|
| `--ebook`, `--eb` | Mark output as ebook/EBOK instead of personal document/PDOC |
| `--asin ASIN` | Set Kindle ASIN metadata for Kindle output |
| `--kfx-compression MODE` | Override `document.kfx.compression` with `none` or `lzma` |

## Compression

KFX/AZW8 compression is lossless: uncompressing recovers the original book file exactly.
The default `none` leaves output uncompressed; `lzma` enables compression without encryption.

```yaml
document:
  kfx:
    compression: lzma
```

```bash
fbc convert --to azw8 --kfx-compression lzma book.fb2
fbc -c config.yaml convert --to kfx --kfx-compression none book.fb2
```

An explicitly supplied flag wins over YAML; an omitted flag uses configuration. Both formats
use the same setting. For other output formats the flag is ignored with a warning.

Compression sizing is automatic; no additional tuning is required. It does not change
reading pages, Kindle locations, image quality, or book content.

See [Debugging and validation](../debugging.md) for report contents, inspection utilities,
[compression of existing files](../debugging.md#kfxcompress), and KFXInput validation.

## Styling Model

KFX output maps the normalized book model and CSS-derived styles into Kindle Enhanced Typesetting structures.

Important differences from EPUB:

- CSS is converted into Kindle style records, not embedded as browser CSS.
- `@media amzn-et` targets KFX/Enhanced Typesetting.
- The bare `body { font-family: ... }` rule can set the Kindle document default font when matching `@font-face` resources exist.
- Negative margins are preserved generally, but KFX avoids negative horizontal margins on dropcap paragraphs because Kindle Previewer lays those out poorly.

See [Stylesheets](../stylesheets.md) for shared stylesheet rules and resource handling.

## Images

Kindle requires a cover and normalizes decodable raster images to JPEG on white.
See [Image handling](../images.md) for cover generation/resizing, format conversions,
transparency, and configuration precedence.

### Native SVG Artwork

Supported non-cover SVG artwork remains vector-based by default; other artwork uses JPEG
fallback. Check results in Kindle Previewer and on your intended device.
See [SVG artwork](../images.md#svg-artwork) for the supported subset and PDF/Kindle
comparison, or [examples](../images.md#examples) to force raster output for compatibility.

## Footnotes

| Mode | KFX/AZW8 Behavior |
|---|---|
| `default` | Footnotes are normal book sections with generated backlink paragraphs; links navigate to sections and do not open popups |
| `float` | Footnote content is marked for Kindle popup rendering and backlink paragraphs are generated |
| `floatRenumbered` | Same as `float`, with normalized labels from `label_template` |

KFX resolves `.LocationNumber` for `backlink_template` from generated Kindle positions, including backlinks from later notes to earlier notes. See [Footnotes](../footnotes.md) and [Templates](../templates.md#backlink_template).

## Page And Location Maps

KFX/AZW8 can generate Kindle location and page-related navigation data from rendered content positions. Page-map behavior depends on `document.page_map` settings.
