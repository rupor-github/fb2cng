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

KFX/AZW8 compression is lossless: uncompressing recovers the complete original CONT bytes.
The default `none` produces an ordinary CONT file. `lzma` wraps the exact same inner book
bytes in a version-2 plaintext ProtectedData envelope, with independently compressed
pages sized automatically from the serialized container. Compression adds no encryption,
keys, or vouchers; the DRMION signature identifies the envelope format, not whether it is encrypted.

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

### Adaptive Page Size

Page size depends on the **decoded CONT size**, including images/fonts and other serialized
book data, rather than the input FB2 size:

| Decoded container size | Page size |
|---|---:|
| Up to 1 MiB | 4 KiB |
| Over 1 MiB, up to 4 MiB | 16 KiB |
| Over 4 MiB, up to 8 MiB | 32 KiB |
| Over 8 MiB | 64 KiB |

The converter tries **4, 16, 32, then 64 KiB**, choosing the smallest size for which
`ceil(decoded_container_bytes / page_size) <= 256`. If none meets that target, it uses
64 KiB. Thresholds in the table are inclusive; 1 MiB is 1,048,576 bytes.

Containers over 16 MiB can have more than 256 pages. Larger pages reduce per-page reader
overhead for large books; small books retain the original 4-KiB profile. The final page may
be shorter. Logs, envelope dumps, and debug reports show the actual size and count.

For example, a 14,515,521-byte container uses 64-KiB pages: **222 compression pages**
instead of 3,544 at 4 KiB. Compression pages are byte blocks; reading pages and Kindle
location/page maps are controlled separately.

Sizing is automatic for conversion and kfxcompress, including diagnostic uncompressed
envelopes; no separate page-size setting is required.

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

- Kindle output requires a cover. If the source book has no valid cover, fb2cng forces cover generation and uses the configured default cover image or the built-in generated cover.
- If `document.images.cover.resize` is `none`, Kindle output treats it as `keepAR` so the generated or source cover is still suitable for Kindle packaging.
- All decodable non-JPEG raster images are converted to JPEG for Kindle compatibility.
- SVG images are rasterized and encoded as JPEG for Kindle output.
- Transparent images are flattened onto a white background before JPEG encoding.
- Broken-image fallback SVGs are rasterized to JPEG as well, so missing or undecodable images still produce Kindle-compatible resources.

## Footnotes

| Mode | KFX/AZW8 Behavior |
|---|---|
| `default` | Footnotes are normal book sections with generated backlink paragraphs; links navigate to sections and do not open popups |
| `float` | Footnote content is marked for Kindle popup rendering and backlink paragraphs are generated |
| `floatRenumbered` | Same as `float`, with normalized labels from `label_template` |

KFX resolves `.LocationNumber` for `backlink_template` from generated Kindle positions, including backlinks from later notes to earlier notes. See [Footnotes](../footnotes.md) and [Templates](../templates.md#backlink_template).

## Page And Location Maps

KFX/AZW8 can generate Kindle location and page-related navigation data from rendered content positions. Page-map behavior depends on `document.page_map` settings.
