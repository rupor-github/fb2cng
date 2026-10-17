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

- Kindle output requires a cover. If the source book has no valid cover, fb2cng forces cover generation and uses the configured default cover image or the built-in generated cover.
- If `document.images.cover.resize` is `none`, Kindle output treats it as `keepAR` so the generated or source cover is still suitable for Kindle packaging.
- All decodable non-JPEG raster images, including PNG, GIF, BMP, TIFF, and WebP, are converted to JPEG for Kindle compatibility. This applies to covers as well as body images and does not depend on `document.images.optimize`.
- Supported SVG body, inline, table and vignette artwork remains vector-based by default, without an additional opt-in. SVG covers remain JPEG.
- Transparent PNG, GIF, TIFF, and WebP content is flattened onto a white background before JPEG encoding, even when `document.images.remove_transparency` is disabled.
- Built-in broken/not-found placeholders remain vector-based for body images. Covers, forced rasterization and transparency removal retain JPEG placeholders.

### Native SVG Artwork

Supported SVG artwork stays sharp when zoomed and retains transparent backgrounds.
Paths and basic shapes, groups, inherited solid colors, supported inline styles, fill/stroke
opacity, standard stroke caps/joins, miter limits, and dash patterns are supported.
Translation, horizontal/vertical scaling, and pure rotation are supported too.

Artwork using even-odd fills, skew or rotation combined with scaling, CSS classes/rules,
gradients, references, clipping/masks/filters, SVG text, embedded images, nested viewports,
non-scaling strokes, `currentColor`, or composited element/group opacity is rendered as a
whole JPEG instead. This fallback is not a browser-complete SVG renderer. Images with
nonempty alt text also use JPEG to preserve that text.

`document.images.rasterize_svg: true` forces raster output. `remove_transparency: true`
also selects JPEG on white. Unsupported-feature fallback logs the image ID, filename and
reason at debug level, separately from configuration-requested rasterization.

Vector artwork and raster fallback use the same layout dimensions and image sizing settings.
Authored vector stroke widths are preserved. Check the result in Kindle Previewer and on
your intended device; support can vary between readers. If artwork displays incorrectly,
try `rasterize_svg: true` and include a [debug report](../debugging.md) when reporting it.

## Footnotes

| Mode | KFX/AZW8 Behavior |
|---|---|
| `default` | Footnotes are normal book sections with generated backlink paragraphs; links navigate to sections and do not open popups |
| `float` | Footnote content is marked for Kindle popup rendering and backlink paragraphs are generated |
| `floatRenumbered` | Same as `float`, with normalized labels from `label_template` |

KFX resolves `.LocationNumber` for `backlink_template` from generated Kindle positions, including backlinks from later notes to earlier notes. See [Footnotes](../footnotes.md) and [Templates](../templates.md#backlink_template).

## Page And Location Maps

KFX/AZW8 can generate Kindle location and page-related navigation data from rendered content positions. Page-map behavior depends on `document.page_map` settings.
