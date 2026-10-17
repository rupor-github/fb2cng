# Image Handling

This guide brings together image behavior across output formats, related configuration,
and practical troubleshooting. **Vector** artwork stays sharp when zoomed; **rasterization**
converts it to pixels. Raster source images include JPEG, PNG, GIF, BMP, TIFF, and WebP.

Three choices are independent: the output's required image format, optional processing,
and the size at which an image is displayed. Disabling optimization does not disable
compatibility conversions, and keeping vectors does not bypass layout sizing.

## Format Capabilities

The following describes normal output with `rasterize_svg: false` and
`remove_transparency: false`. Cover exceptions are described [below](#covers).

| Output | Raster source images | SVG body/inline images | Transparent backgrounds |
|---|---|---|---|
| EPUB2 / EPUB3 | Normally retain their format | Retain SVG | Preserved where the reader supports them |
| KEPUB / KEPUB3 | WebP becomes lossless PNG; others normally retain their format | Retain SVG | Preserved where the reader supports them |
| KFX / AZW8 | Convert decodable non-JPEG sources to JPEG | Supported artwork stays vector-based; otherwise JPEG | Preserved in native vectors; raster output is white-backed |
| PDF | Embed decoded raster images; JPEG can remain JPEG | Supported artwork stays vector-based; otherwise raster | Preserved in native vectors; raster output is white-backed |
| Markdown, `external` | Normally retain their format in separate files | Retain SVG in a separate file | Depends on the Markdown viewer |
| Markdown, `embedded` | Normalize to JPEG embedded in the document | Rasterize to JPEG | Flattened onto white |
| TXT / Markdown, `placeholder` | Readable placeholders, no image files | Readable placeholders | Not applicable |

"Retain" does not mean byte-for-byte preservation when resizing, transparency removal,
or optimization changes an image. EPUB and Markdown reader support varies; packaging a
format does not guarantee that every reader displays it. For predictable raster handling,
prefer JPEG or PNG. Animated source formats are not preserved as animations when decoded
and re-encoded.

## Configuration

Image settings are under `document.images`. These are the current defaults:

```yaml
document:
  images:
    use_broken: false
    remove_transparency: false
    rasterize_svg: false
    scale_factor: 1.0
    optimize: true
    jpeg_quality_level: 95
    markdown: placeholder
    screen:
      width: 1264
      height: 1680
      dpi: 300
    cover:
      generate: false
      default_image_path: ""
      resize: stretch
```

Use `fbc dumpconfig --default config.yaml` to obtain the complete current configuration.

| Setting | Effect and interaction |
|---|---|
| `rasterize_svg` | `true` forces SVG rasterization independently of `optimize`. `false` allows the output's normal vector handling **and** any required rasterization or fallback. |
| `remove_transparency` | Flattens supported transparent raster sources onto white. Also selects white-backed raster output for SVG in PDF and Kindle. Retained SVG in EPUB/KEPUB or external Markdown is unchanged unless rasterization is also requested. |
| `optimize` | Enables optional JPEG/PNG recompression. Does not control resizing, transparency removal, or required format conversions. |
| `jpeg_quality_level` | Quality from 40 to 100 whenever JPEG is encoded, including required conversions. Does not change vector detail or PNG quality. |
| `scale_factor` | A positive ratio for non-cover image resizing where applicable; `1.0` leaves size unchanged. See [sizing exceptions](#sizing-and-resolution). |
| `screen.width`, `screen.height` | Target dimensions in pixels for cover processing and some SVG sizing. Together with `dpi`, also determine PDF page size. |
| `screen.dpi` | PDF physical page sizing and JPEG density information; increasing it does not create extra image detail. |
| `cover.generate` | Supply a fallback cover when the FB2 specifies none. Automatically enabled for Kindle output. |
| `cover.default_image_path` | Optional fallback cover file, used instead of the built-in artwork when a fallback is needed. Does not replace an existing FB2 cover. |
| `cover.resize` | Cover resizing policy, separate from body `scale_factor` and format conversion. See [covers](#covers). |
| `markdown` | Choose readable placeholders, separate assets, or embedded images for Markdown. Ignored by other outputs. |
| `use_broken` | `false` substitutes a broken-image placeholder when preparation fails. `true` retains the failed image rather than repairing it; it does **not** enable placeholders. |

### Which Settings Take Precedence?

Required output compatibility is not optional: Kindle and embedded Markdown still convert
raster images to JPEG with `optimize: false`; KEPUB still converts WebP to PNG. Setting
`cover.resize: none` disables resizing, not format conversion.

For SVG, the combination is:

| Output / placement | Both settings `false` | Only `remove_transparency: true` | Only `rasterize_svg: true` | Both `true` |
|---|---|---|---|---|
| EPUB/KEPUB; external Markdown | SVG | SVG, unchanged | PNG, transparency preserved | PNG on white |
| Kindle, non-cover | Native vectors if supported; otherwise JPEG on white | JPEG on white | JPEG on white | JPEG on white |
| Kindle, cover | JPEG on white | JPEG on white | JPEG on white | JPEG on white |
| PDF | Native vectors if supported; otherwise raster on white | Raster on white | Raster on white | Raster on white |
| Embedded Markdown | JPEG on white | JPEG on white | JPEG on white | JPEG on white |

TXT and Markdown placeholder mode emit no image resources, so these choices have no visible
effect there. The SVG table describes source artwork; the [built-in fallback cover](#covers)
has its own preparation policy.

For transparent PNG, GIF, TIFF, and WebP sources, `remove_transparency: true` flattens onto
white. Outside JPEG-normalizing outputs, flattened GIF/TIFF/WebP becomes PNG. Opaque images
are not converted solely because this setting is enabled. PDF raster output is white-backed
even with the setting disabled.

With `optimize: true`, source JPEGs are optionally re-encoded when their detected quality
exceeds `jpeg_quality_level`; existing lower-quality JPEGs are not improved by a higher value.
PNG recompression is lossless; PDF skips this optional PNG recompression. Resizing or a
required conversion can still trigger encoding, even when optimization is disabled. JPEG
encoding also detects grayscale when optimization is enabled.

## SVG Artwork

EPUB/KEPUB and external Markdown pass retained SVG to the reader. Their ability to display
SVG features therefore depends on that reader. PDF and Kindle instead keep supported SVG
artwork vector-based by default, with no extra opt-in.

### Native Vector Support

Both PDF and Kindle support:

- SVG paths, including curves and arcs, plus rectangles, circles, ellipses, lines,
  polygons, and polylines.
- Groups, inherited presentation properties, and supported inline styles.
- Solid named/hex colors, fill/stroke opacity, and nonzero fills.
- Butt/round/square caps, miter/round/bevel joins, custom miter limits, and dash patterns
  including positive and negative offsets.

| Capability | PDF | Kindle KFX/AZW8 |
|---|---|---|
| Translation, axis scaling, pure rotation | Native | Native |
| General affine transforms, including skew and rotation combined with scaling | Native | Raster fallback for unverified combinations |
| Even-odd fills | Native | Raster fallback |
| SVG covers | Native when supported | JPEG |
| SVG placements with nonempty alt text | Can remain vector-based | JPEG to retain alt text |
| Built-in broken/not-found placeholders | Native vectors | Native vectors for body images |

For native output, use a positive root `viewBox`. Gradients, clipping/masks/filters,
references such as `use`, SVG text/fonts, embedded images, CSS rules/classes, `currentColor`,
nested viewports, non-scaling strokes, and composited element/group opacity are unsupported.
Excessive coordinates or artwork complexity can also prevent native output.

### Raster Fallback

An unsupported feature causes the **whole image** to be rasterized rather than partially
drawn as vectors. Native artwork and its fallback retain the same layout dimensions.
Kindle uses JPEG on white; PDF raster output is also white-backed.

The SVG rasterizer is not a browser-complete renderer: fallback cannot guarantee correct
handling of every unsupported feature. If complex artwork renders incorrectly, simplify
the SVG or convert it to PNG/JPEG in an external graphics tool before conversion.

Debug logs distinguish unsupported-artwork fallback from configuration-requested
rasterization and identify the affected image. Check output in the intended reader;
successful structural validation alone does not guarantee correct appearance.

## Sizing And Resolution

**Displayed size and raster resolution are different.** Layout, CSS, page size, and reader
behavior can constrain the displayed image even when its source contains more pixels.
Vectors do not need extra pixels to stay sharp when enlarged.

- Raster body images use `scale_factor` during preparation. Values below 1 reduce pixel
  dimensions; values above 1 enlarge them but cannot recover missing detail.
- For Kindle SVG body artwork, `screen.width` establishes the initial sizing, preserving
  aspect ratio, followed by `scale_factor`. Native and raster versions use the same sizing
  policy; the vector drawing itself is not reduced to that pixel resolution.
- Optional SVG rasterization for EPUB/KEPUB and external Markdown, and required SVG
  rasterization for embedded Markdown, starts at `screen.width`, preserving aspect ratio.
  Cover resizing or body `scale_factor` then applies.
- SVG retained unchanged in EPUB/KEPUB or external Markdown is not resized by
  `scale_factor` or `cover.resize`; its displayed size is controlled by layout and the reader.
- **PDF SVG sizing is different:** native vectors and raster fallback retain intrinsic SVG
  dimensions. `screen.width`, `cover.resize`, and `scale_factor` do not add SVG resizing.
  PDF layout still fits artwork into its available space. Raster source images still use
  the normal preparation settings.
- Built-in placeholders start at their intrinsic size instead of expanding to screen
  width; the broken-image placeholder is also exempt from SVG body scaling.

For SVG rasterization, missing viewBox dimensions fall back to 2048 per missing dimension;
very large initial rasterizations are capped for memory safety. Supply sensible SVG
dimensions rather than relying on fallback sizing.

For PDF, `screen.width`, `screen.height`, and `screen.dpi` set the physical page size:
`points = pixels × 72 / dpi`. See [PDF page size](formats/pdf.md#page-size).
Changing these settings changes pages as well as image-related sizing; they are not a
general-purpose "higher quality" switch.

## Covers

Only the first FB2 cover is used. With `cover.generate: true`, an FB2 with no specified
cover receives `cover.default_image_path`, or the built-in cover if that path is empty.
Kindle output always enables this fallback. Other outputs default to `generate: false`.

The built-in fallback cover stays vector-based in PDF when supported and not forced to
rasterize; other image-bearing outputs use a JPEG version. A custom SVG fallback cover
follows the same rules as any source SVG cover: retained SVG in EPUB/KEPUB, native vectors
when supported in PDF, and JPEG in Kindle.

### Cover Resizing

These modes apply to raster covers and to rasterized SVG covers outside PDF:

| `cover.resize` | Behavior |
|---|---|
| `none` | Leave prepared dimensions unchanged. Kindle treats this as `keepAR`. |
| `keepAR` | If shorter than `screen.height`, enlarge to that height while preserving aspect ratio. Does not shrink a taller cover or guarantee it fits screen width. |
| `stretch` | Resize to exactly `screen.width` × `screen.height`; can distort aspect ratio. This is the default. |
| `fit` | Enlarge or shrink to fit within the screen width/height while preserving aspect ratio; does not crop. |

Body `scale_factor` does not apply to covers. Retained SVG and PDF SVG sizing exceptions
are described [above](#sizing-and-resolution). A KEPUB WebP cover still becomes PNG with
`resize: none`; Kobo covers are not forced to JPEG.

### Omitting Covers

Use `--no-cover` for EPUB2/3, KEPUB/KEPUB3, or PDF:

```bash
fbc convert --to pdf --no-cover book.fb2
```

This omits the dedicated cover and cover metadata even if generation is enabled.
An image also referenced in the body remains a body illustration. Kindle requires a cover
and ignores this flag with a warning. TXT and Markdown omit the dedicated cover already.

## Markdown Images

Set `document.images.markdown`:

| Mode | Result |
|---|---|
| `placeholder` | Readable text such as `[Image: illustration]`; no assets. This is the default. |
| `external` | Separate files in a book-specific directory such as `images-<book-id>/`, referenced from the `.md` file. Keep this directory with the document. |
| `embedded` | JPEG data embedded as `data:` image URIs; no separate assets. Viewer support varies. |

External SVG stays SVG unless `rasterize_svg: true` requests PNG. Embedded SVG always
becomes JPEG on white; embedded raster images follow the same JPEG policy.
Markdown does not render vignettes or a dedicated cover.

## Vignettes And Placeholders

Decorative vignettes are configured separately under `document.vignettes`. Each position
can be omitted, set to `builtin`, or set to an image file path:

```yaml
document:
  vignettes:
    book:
      title_top: builtin
      title_bottom: builtin
    chapter:
      title_top: builtin
      title_bottom: builtin
      end: builtin
    section:
      title_top: builtin
      title_bottom: builtin
      end: builtin
```

Book, chapter, and nested-section decorations follow the same output image policies as
other artwork. TXT and Markdown ignore them. Native vector strokes retain their authored
widths. Kindle rasterization of built-in vignettes makes their strokes heavier for
readability; external SVGs, placeholders, native vectors, and optional PNG rasterization
do not receive this adjustment.

Missing image references use a not-found placeholder; image preparation failures normally
use a broken-image placeholder. These remain vector-based where supported. Kindle cover
placeholders and raster-forcing policies use raster output. `use_broken: true` leaves a
failed source image for the reader to handle; it may still be invisible or unusable.

## Examples

Save a fragment in `images.yaml` and use it with
`fbc -c images.yaml convert --to epub3 book.fb2`, changing the output format as needed.
Fragments merge with the defaults.

### Prefer Original Artwork

Keep normal vector handling and disable optional recompression:

```yaml
document:
  images:
    rasterize_svg: false
    remove_transparency: false
    optimize: false
```

Required conversions, SVG fallback, and default cover resizing still apply. To avoid
raster cover resizing as well, set `cover.resize: none` (except for Kindle's override).

### Force Raster Artwork For Reader Compatibility

```yaml
document:
  images:
    rasterize_svg: true
    optimize: false
```

This works independently of optimization. EPUB/KEPUB and external Markdown receive PNG;
Kindle and embedded Markdown receive JPEG; PDF receives raster artwork.

### Make Transparent Artwork White-Backed

For EPUB/KEPUB or external Markdown, request both settings so SVG is flattened too:

```yaml
document:
  images:
    rasterize_svg: true
    remove_transparency: true
```

For PDF and Kindle, `remove_transparency: true` alone already selects raster SVG on white.

### Reduce Raster Image Size

```yaml
document:
  images:
    optimize: true
    jpeg_quality_level: 75
    scale_factor: 0.75
    cover:
      resize: fit
```

This trades raster detail for size. Vector artwork is not simplified by lowering JPEG
quality; PDF SVG dimensions are unaffected by `scale_factor`. Cover `fit` can enlarge small
covers; use `none` outside Kindle if you do not want cover resizing.

### Keep Markdown Artwork In Separate Files

```yaml
document:
  images:
    markdown: external
    rasterize_svg: false
```

Move the generated image directory together with the `.md` file.

## Troubleshooting

| Symptom | What to check |
|---|---|
| SVG becomes raster with `rasterize_svg: false` | Output requirements, unsupported native features, `remove_transparency`, Kindle covers, or nonempty Kindle alt text. `false` is not a ban on rasterization. |
| Transparency disappears | Kindle/embedded-Markdown JPEG policy, PDF raster output, or `remove_transparency`. |
| `remove_transparency` does not change SVG in EPUB/Markdown | Retained SVG is unchanged; also enable `rasterize_svg` to flatten it. |
| Cover looks stretched | Default `cover.resize: stretch`; try `fit` to preserve aspect ratio. |
| Changing `scale_factor` does not enlarge SVG | Retained EPUB/Markdown SVG and PDF SVG have different sizing rules; layout may also constrain display size. |
| Image becomes blurry | SVG rasterization, reduced raster dimensions, JPEG quality, or enlargement of a low-resolution source. |
| GIF/BMP/TIFF/WebP fails during resizing | Some processing paths cannot re-encode these formats. Convert the source to PNG/JPEG first; required Kindle/Kobo conversions have their own rules. |
| Markdown images are missing | Check image mode, retain the external asset directory, and check viewer support for SVG or embedded `data:` images. |
| Artwork differs between readers | Test with `rasterize_svg: true`; simplify or externally rasterize complex SVG if necessary. |
| A broken/not-found icon appears | Check the original image data or image reference, not just optimization settings. |

Use `fbc -d convert ...` and inspect the logs for the image ID and fallback/processing
reason. When reporting a problem, include the command, configuration, reader/version, and
the complete [debug report](debugging.md#converter-debug-reports).
