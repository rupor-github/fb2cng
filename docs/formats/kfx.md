# KFX/AZW8 Output

KFX is Kindle's reflowable Enhanced Typesetting format. Both `kfx` and `azw8`
generate the same book format; only the extension (`.kfx` or `.azw8`) and the
requested name exposed to templates differ. AZW8 is useful for workflows that
prefer Kindle Previewer-friendly filenames.

For the separate native KF8 format, see [Native AZW3](azw3.md).

Both formats support uncompressed output and **optional, lossless LZMA compression without
encryption**. Compression is disabled by default.

## Command Line

```bash
fbc convert --to kfx book.fb2
fbc convert --to azw8 book.fb2
fbc convert --to azw8 --compression lzma book.fb2
```

Kindle-specific options:

| Option | Meaning |
|---|---|
| `--ebook`, `--eb` | Mark output as ebook/EBOK instead of personal document/PDOC |
| `--asin ASIN` | Set Kindle ASIN metadata for Kindle output |
| `--compression MODE` | Override `document.kfx.compression` with `none` or `lzma` |

`--no-cover` is ignored with a warning because Kindle output requires a cover.
Shared naming, destination, and overwrite options are described in the
[command-line guide](../guide.md#command-line).

## Configuration

`document.kfx` applies to both KFX and AZW8; it currently contains only
`compression`. Navigation, footnotes, images, metadata, and styling use the shared
`document` settings. The requested name (`kfx` or `azw8`) remains available as
`.Format` in templates. See [Configuration](../config.md).

### Compression

KFX/AZW8 compression is lossless: uncompressing recovers the original book file exactly.
The default `none` leaves output uncompressed; `lzma` enables compression without encryption.

```yaml
document:
  kfx:
    compression: lzma
```

```bash
fbc convert --to azw8 --compression lzma book.fb2
fbc -c config.yaml convert --to kfx --compression none book.fb2
```

An explicitly supplied flag wins over YAML; an omitted flag uses configuration. Both formats
use the same setting. AZW3 uses its [own compression modes](azw3.md#compression).
For non-Kindle output the flag is ignored with a warning.

Compression sizing is automatic; no additional tuning is required. It does not change
reading pages, Kindle locations, image quality, or book content.

See [Debugging and validation](../debugging.md) for report contents, inspection utilities,
[compression of existing files](../debugging.md#kfxcompress), and KFXInput validation.

## Structure And Navigation

Books include Kindle contents navigation, internal links, and generated location
positions. `document.toc_type` controls navigation nesting; `document.toc_page`
adds an optional visible contents page.

Page navigation is controlled by `document.page_map`. Generated pages are
approximate reading positions, not print-edition page numbers. No APNX sidecar
is needed. See [page-map settings](../config.md#navigation-and-page-maps).

### Page And Location Maps

Page maps use rendered content positions. Disabling the page map does not disable
Kindle location positions or location-based backlink labels.

## Styling

KFX output maps the normalized book model and CSS-derived styles into Kindle Enhanced Typesetting structures.

Important differences from EPUB:

- CSS is converted into Kindle style records, not embedded as browser CSS.
- `@media amzn-et` targets KFX/Enhanced Typesetting.
- The bare `body { font-family: ... }` rule can set the Kindle document default font when matching `@font-face` resources exist.
- Negative margins are preserved generally, but KFX avoids negative horizontal margins on dropcap paragraphs because Kindle Previewer lays those out poorly.

See [Stylesheets](../stylesheets.md) for shared stylesheet rules and resource handling.

## Images And Fonts

Kindle requires a cover and normalizes decodable raster images to JPEG on white.
See [Image handling](../images.md) for cover generation/resizing, format conversions,
transparency, and configuration precedence.

Use `@font-face` with raw TTF/OTF font files for embedded fonts. A matching
`body { font-family: ... }` rule selects the document default font. Font selection
and display still depend on the Kindle reader.

### Native SVG Artwork

Supported non-cover SVG artwork remains vector-based by default; other artwork uses JPEG
fallback. Check results in Kindle Previewer and on your intended device.
See [SVG artwork](../images.md#svg-artwork) for the supported subset and PDF/Kindle
comparison, or [examples](../images.md#examples) to force raster output for compatibility.

## Footnotes

| Mode | Behavior |
|---|---|
| `default` | Footnotes are normal book sections with generated backlink paragraphs; links navigate to sections and do not open popups |
| `float` | Footnote content is marked for Kindle popup rendering and backlink paragraphs are generated |
| `floatRenumbered` | Same as `float`, with normalized labels from `label_template` |

The bundled `backlink_template` enables generated returns in all three modes. A
valid template producing empty or whitespace-only text, such as `{{- "" -}}`,
suppresses these return paragraphs without removing forward note links or
floating-note popup markers. Empty referenced notes remain addressable. Popup
behavior still depends on the Kindle device or app.

KFX resolves `.LocationNumber` for `backlink_template` from generated Kindle positions, including backlinks from later notes to earlier notes. See [Footnotes](../footnotes.md) and [Templates](../templates.md#backlink_template).

## Limitations

- CSS is supported through a conversion model, not a browser layout engine; not
  every property or selector has an equivalent Kindle style.
- Unsupported SVG features fall back to raster artwork; fallback is not a
  browser-complete SVG renderer either.
- Popup behavior and appearance vary between Kindle Previewer, devices, and apps.
- Conversion and inspection tools do not remove DRM from encrypted books.

## Debugging And Validation

Generate a [debug report](../debugging.md#kfxazw8) with `fbc -d convert ...`.
Use [kfxdump](../debugging.md#kfxdump) for inspection and
[KFXInput](../debugging.md#kfx-and-azw8) for an independent check. Inspect all
diagnostics, including any native-vector compatibility reports, then open the
book in Kindle Previewer and the intended reader.
