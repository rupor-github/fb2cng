# FBC User Guide

## Table of Contents

- [Introduction](#introduction)
  - [Supported Output Formats](#supported-output-formats)
  - [Key Features](#key-features)
- [Installation](#installation)
- [Quick Start](#quick-start)
  - [Convert a single file](#convert-a-single-file)
  - [Convert to a specific format](#convert-to-a-specific-format)
  - [Convert with output directory](#convert-with-output-directory)
  - [Convert all FB2 files in a directory](#convert-all-fb2-files-in-a-directory)
- [Basic Usage](#basic-usage)
  - [Command Line Syntax](#command-line-syntax)
  - [Global Options](#global-options)
  - [Convert Command](#convert-command)
  - [Examples](#examples)
- [Advanced Features](#advanced-features)
  - [Configuration Management](#configuration-management)
  - [Templates](#templates)
  - [File Naming Templates](#file-naming-templates)
  - [Metadata Customization](#metadata-customization)
  - [Custom Stylesheets](#custom-stylesheets)
  - [Image Processing](#image-processing)
  - [Cover Image Configuration](#cover-image-configuration)
  - [PDF Generation](#pdf-generation)
  - [Footnotes Processing](#footnotes-processing)
  - [Device Table of Contents](#device-table-of-contents)
  - [Table of Contents Page](#table-of-contents-page)
  - [Annotation Page](#annotation-page)
  - [Vignettes (Decorative Images)](#vignettes-decorative-images)
  - [Dropcaps (Drop Capitals)](#dropcaps-drop-capitals)
  - [Text Transformations](#text-transformations)
  - [Page Map](#page-map)
  - [Hyphenation](#hyphenation)
  - [Logging Configuration](#logging-configuration)
  - [Debug Reporting](#debug-reporting)
- [Configuration](#configuration)
  - [Configuration File Structure](#configuration-file-structure)
  - [Configuration Loading](#configuration-loading)
  - [Configuration Best Practices](#configuration-best-practices)
- [MyHomeLib Integration](#myhomelib-integration)
  - [Installation Structure](#installation-structure)
  - [Setup Options](#setup-options)
  - [Configuration](#configuration-1)
  - [Connector Behavior](#connector-behavior)
- [Troubleshooting](#troubleshooting)
  - [Enable Debug Mode](#enable-debug-mode)
  - [Common Issues](#common-issues)
  - [Getting Help](#getting-help)
  - [Log Files](#log-files)
  - [Performance Tips](#performance-tips)

## Introduction

**fb2cng** (FictionBook converter - Next Generation) is a complete rewrite of [fb2converter](https://github.com/rupor-github/fb2converter), designed to convert FB2 (FictionBook) files to various e-book formats including EPUB2, EPUB3, KEPUB, KFX/AZW8, and PDF.

### Supported Output Formats

- **EPUB2** - Standard EPUB format with wide device compatibility
- **EPUB3** - Modern EPUB format with enhanced features
- **KEPUB** - EPUB2 optimized for Kobo devices
- **KFX** - Kindle format X (with `.kfx` extension)
- **AZW8** - Kindle format X with `.azw8` extension (same as KFX, different extension, added for convenience - Kindle Previewer 3 can open azw8 files directly and Kindle devices handle them just fine)
- **PDF** - Native fixed-page PDF 1.4 output with embedded fonts, outlines, links, images, and printed page footnotes

### Key Features

- Batch conversion support (directories and archives)
- Flexible configuration via YAML files
- Template-based file naming and metadata formatting
- Custom CSS stylesheet support with font embedding
- Image optimization and cover generation
- Footnotes processing with floating/popup support and PDF printed footnotes
- Native PDF generation with embedded fonts and fixed-page layout
- Automatic vignettes and dropcaps styling
- Automatic hyphenation support
- Debug reporting for troubleshooting
- MyHomeLib integration (Windows specific)

## Installation

1. Download the latest release from the [releases page](https://github.com/rupor-github/fb2cng/releases)
2. Unpack the archive to a convenient location
3. The main executable is `fbc` (or `fbc.exe` on Windows)

No additional dependencies are required - the executable is self-contained.

## Quick Start

### Convert a single file

```bash
fbc convert book.fb2
```

This converts `book.fb2` to EPUB2 format in the current directory.

### Convert to a specific format

```bash
fbc convert --to epub3 book.fb2
```

Supported formats: `epub2`, `epub3`, `kepub`, `kfx`, `azw8`, `pdf`

### Convert with output directory

```bash
fbc convert book.fb2 /path/to/output/
```

### Convert all FB2 files in a directory

```bash
fbc convert /path/to/books/ /path/to/output/
```

## Basic Usage

### Command Line Syntax

```
fbc [global options] command [command options] [arguments]
```

### Global Options

- `--config FILE, -c FILE` - Load configuration from YAML file
- `--debug, -d` - Enable debug mode, produces diagnostic report archive
- `--version, -v` - Print version information
- `--help, -h` - Show help

### Convert Command

```
fbc convert [options] SOURCE [DESTINATION]
```

#### Convert Options

- `--to TYPE` - Output format: `epub2` (default), `epub3`, `kepub`, `kfx`, `azw8`, `pdf`
- `--ebook, --eb` - For Kindle formats, mark output as ebook (EBOK) instead of personal document (PDOC)
- `--asin ASIN` - For Kindle formats (`kfx`, `azw8`), set ASIN used in metadata (10 chars, `A-Z0-9`; for books it's often the ISBN-10)
- `--nodirs, --nd` - Don't preserve input directory structure in output
- `--overwrite, --ow` - Overwrite existing files
- `--force-zip-cp ENCODING` - Force encoding for non-UTF8 archive file names (e.g., `windows-1251`, `cp866`)

#### SOURCE Formats

The SOURCE parameter is flexible and supports:

**Single file:**
```bash
fbc convert mybook.fb2
```

**Directory (recursive):**
```bash
fbc convert /path/to/books/
```
Processes all `.fb2` files in directory and subdirectories (symlinks not followed).

**Archive with specific file:**
```bash
fbc convert archive.zip/path/inside/book.fb2
```

**Archive directory (recursive):**
```bash
fbc convert archive.zip/path/inside/
```
Processes all `.fb2` files under the specified path in archive.

**Note:** Archives inside archives are not supported.

#### DESTINATION

- Always a directory path
- Output filenames are generated automatically based on metadata
- If omitted, uses current working directory
- Directory structure preserved unless `--nodirs` is specified

### Examples

**Convert directory preserving structure:**
```bash
fbc convert --to epub3 ~/Books/FB2/ ~/Books/EPUB/
```

**Convert archive with overwrite:**
```bash
fbc convert --ow --to kepub books.zip ~/Kobo/
```

**Convert to Kindle with explicit ASIN:**
```bash
fbc convert --to kfx --asin B012345678 book.fb2
```

**Convert to PDF:**
```bash
fbc convert --to pdf book.fb2
```

**Convert to PDF with a custom configuration:**
```bash
fbc -c pdf.yaml convert --to pdf book.fb2
```

**Convert with custom configuration:**
```bash
fbc -c myconfig.yaml convert --to epub3 book.fb2
```

**Debug mode conversion:**
```bash
fbc -d convert book.fb2
```
Creates `fb2cng-report.zip` with logs and diagnostic information.

## Advanced Features

### Configuration Management

#### Dump Default Configuration

```bash
fbc dumpconfig --default myconfig.yaml
```

This creates a YAML file with all default settings that you can customize.

#### Dump Active Configuration

```bash
fbc -c myconfig.yaml dumpconfig active.yaml
```

Shows the actual configuration being used (defaults merged with your custom settings).

### Templates

Many configuration fields (those with `_template` in their name) use the
[Go template language](https://pkg.go.dev/text/template#pkg-overview) to give you full control over
file naming, metadata formatting, footnote labels, and more. If you are not familiar with Go
templates, the linked documentation provides a complete reference for the syntax including actions,
pipelines, variables, and built-in functions.

In addition to the standard Go template functions, all templates have access to the
[slim-sprig](https://go-task.github.io/slim-sprig) function library which provides many useful
string, math, list, and other helper functions (such as `first`, `cat`, `contains`, etc.).

All template contexts include these common fields:

- `.Context` - Template field name, such as `output_name_template`, `label_template`, or `backlink_template`.
- `.Format` - Requested output format: `epub2`, `epub3`, `kepub`, `kfx`, `azw8`, or `pdf`.

### File Naming Templates

Control output filenames using Go templates in configuration:

```yaml
document:
  output_name_template: |
    {{- $all := "" -}}
    {{- if gt (len .Authors) 0 -}}
    {{-   with first .Authors -}}
    {{-     $all = .LastName -}}
    {{-     if .FirstName }}{{ $all = (cat $all .FirstName) }}{{- end -}}
    {{-   end -}}
    {{-   $all = cat $all "-" -}}
    {{- end -}}
    {{- cat $all .Title -}}
```

#### Available Template Variables

- `.Context` - Template field name (`output_name_template`)
- `.Format` - Output format (`epub2`, `epub3`, `kepub`, `kfx`, `azw8`, `pdf`)
- `.Title` - Book title
- `.Authors` - Array of author objects with `.FirstName`, `.MiddleName`, `.LastName`
- `.Series` - Array with `.Name` and `.Number`
- `.Language` - Language code
- `.Date` - Publication date
- `.SourceFile` - Original filename (no path/extension)
- `.BookID` - Book UUID
- `.Genres` - Array of genre names

### Metadata Customization

Format book title and author names in metadata. `title_template` uses the same book metadata fields as `output_name_template`; `creator_name_template` is applied to each author and exposes `.Context`, `.Format`, `.Index`, `.FirstName`, `.MiddleName`, and `.LastName`.

```yaml
document:
  metainformation:
    # Add series prefix to title: "(SRN - 01) Book Title"
    title_template: |
      {{- if gt (len .Series) 0 -}}
      {{-   with first .Series -}}
      {{-     "(" }}{{ .Name }}{{ " - " }}{{ printf "%02d" .Number }}{{ ") " }}
      {{-   end -}}
      {{- end -}}
      {{ .Title }}
    
    # Format author as "LastName, FirstName MiddleName"
    creator_name_template: |
      {{- .LastName -}}
      {{- if .FirstName }}, {{.FirstName}}{{ end -}}
      {{- if .MiddleName }} {{.MiddleName}}{{ end -}}
```

### Custom Stylesheets

fb2cng includes a comprehensive default stylesheet that styles all FB2 elements. You can customize appearance by providing your own CSS file.

#### Using Custom Stylesheets

```yaml
document:
  stylesheet_path: "mystyle.css"
```

**Important:** Your custom stylesheet **replaces** the default stylesheet entirely. You should either:
- Copy and modify the default stylesheet ([`convert/default.css`](../convert/default.css)) as a starting point
- Write a complete stylesheet that covers all the elements you need styled

#### Default Stylesheet Classes

The built-in stylesheet ([`convert/default.css`](../convert/default.css)) provides comprehensive styling for:

**Document Structure:**
- `.body-title`, `.chapter-title`, `.section-title` - Title containers with page break control
- `.section-title-h2` through `.section-title-h6` - Depth-specific section title classes (see [Section Splitting](#section-splitting-via-css) below)
- `.body-title-header`, `.chapter-title-header`, `.section-title-header` - Title text styling
- `.section-subtitle` - Section subtitles
- `.toc-title` - Table of contents title

**Content Elements:**
- `p` - Paragraphs with text indentation and justification
- `h1` - `h6` - Headings (h1: 140%, h2-h6: 120%)
- `code` - Code blocks with monospace font
- `.dropcap` - Decorative large first letter
- `.has-dropcap` - Paragraphs containing drop caps

**Poetry and Quotes:**
- `.poem`, `.poem-title`, `.poem-subtitle` - Poem containers
- `.stanza`, `.stanza-title` - Poem sections
- `.verse` - Individual poem lines
- `.cite` - Block quotations
- `.epigraph`, `.epigraph-subtitle` - Quotations at chapter starts

**Images:**
- `.image` - Image container
- `img.image-block` - Block images (responsive sizing)
- `img.image-inline` - Inline images
- `img.image-vignette` - Decorative vignettes (100% width)

**Vignettes (Decorative Images):**
- `.vignette-book-title-top/bottom` - Book title decorations
- `.vignette-chapter-title-top/bottom` - Chapter title decorations
- `.vignette-chapter-end` - Chapter end decorations
- `.vignette-section-title-top/bottom` - Section title decorations
- `.vignette-section-end` - Section end decorations

**Annotations and Metadata:**
- `.annotation`, `.annotation-title` - Book descriptions
- `.text-author` - Author attribution
- `.date` - Date elements

**Footnotes:**
- `.footnote` - Footnote body/content styling
- `.footnote-title` - Footnote title styling
- `.footnote-title-first`, `.footnote-title-next` - Split/generated footnote title paragraph parts
- `.footnote-separator` - Separator line before PDF printed page footnotes
- `.footnote-more` - Multi-paragraph indicator
- `.link-footnote` - Superscript footnote references
- `.link-backlink` - Return links from footnotes

**Links:**
- `.link-external` - External hyperlinks (underlined)
- `.link-internal` - Internal document links (no underline)
- `.link-toc` - Table of contents links

**Special:**
- `.emptyline` - Spacing elements
- `.unexpected` - Error/unexpected content (strikethrough)

**Media Queries:**
- `@media amzn-mobi` - Kindle MOBI-specific styles
- `@media amzn-kf8` - Kindle KF8-specific styles
- `@media amzn-et` - Kindle KFX / Enhanced Typesetting-specific styles
- `@media fbc-pdf` - fb2cng native PDF-specific styles

Media query conditions can be combined with `and`, `not`, and the generic `all` / `screen` media types. Examples:

```css
@media fbc-pdf {
    body {
        font-size: 120%;
        line-height: 130%;
    }
}

@media not fbc-pdf {
    .screen-only-tweak {
        margin-left: -8pt;
    }
}

@media amzn-kf8 and not amzn-et {
    p {
        margin: 0 -8pt 0.3em -8pt;
    }
}
```

#### Section Splitting via CSS

FB2 books have a hierarchical section structure: top-level sections (depth 1) are always placed into separate XHTML files inside the EPUB. Nested sections (depths 2-6) are normally rendered inline within their parent's file.

fb2cng can split nested sections into their own XHTML files based on CSS `page-break-before` rules. When a `.section-title-hN` class has `page-break-before: always`, all sections at depth N are split into separate files instead of being inlined.

**How it works:**

Each nested section's title wrapper gets a depth-specific CSS class: `.section-title-h2` for depth 2, `.section-title-h3` for depth 3, and so on up to `.section-title-h6`. fb2cng parses the stylesheet and checks which of these classes have `page-break-before: always`. Sections at those depths are then extracted into their own XHTML files during EPUB generation.

**Default behavior:**

The default stylesheet includes:

```css
.section-title-h2 {
    page-break-before: always;
}
```

This means depth-2 sections (direct children of top-level chapters) are split into separate files by default. Deeper sections (depth 3+) remain inline.

**Customization examples:**

Split sections at depths 2 and 3 into separate files:
```css
.section-title-h2 {
    page-break-before: always;
}
.section-title-h3 {
    page-break-before: always;
}
```

Disable all section splitting (keep everything inline):
```css
.section-title-h2 {
    page-break-before: auto;
}
```

Split only at depth 3, not depth 2:
```css
.section-title-h2 {
    page-break-before: auto;
}
.section-title-h3 {
    page-break-before: always;
}
```

**Notes:**
- Depth 1 sections (top-level chapters) are always in separate files regardless of CSS — this only controls depths 2 through 6.
- Splitting is recursive: if both depth 2 and depth 3 have `page-break-before: always`, depth-3 sections inside a depth-2 section will each get their own file.
- The TOC hierarchy is preserved regardless of splitting — nested sections appear as children of their parent in the table of contents even when they are in separate files.
- When using a custom stylesheet (`stylesheet_path`), it replaces the default entirely. If you want section splitting, include the appropriate `.section-title-hN` rules in your custom CSS.

#### Customization Examples

**Override paragraph styling:**
```css
/* Your custom stylesheet */
p {
    text-indent: 2em;  /* Larger indent than default 1em */
    margin-bottom: 0.5em;
}
```

**Customize headings:**
```css
h1 {
    font-size: 180%;  /* Larger than default 140% */
    color: #333;
}

.chapter-title-header {
    text-align: left;  /* Override default center alignment */
}
```

**Change drop cap appearance:**
```css
.dropcap {
    font-size: 4em;  /* Larger than default 3.5em */
    color: #800000;
}
```

#### Stylesheet with Fonts

Create a CSS file with font references:

```css
@font-face {
    font-family: "paragraph";
    src: url("PTSerif-Regular.ttf");
}

@font-face {
    font-family: "paragraph";
    src: url("PTSerif-Italic.ttf");
    font-style: italic;
}

@font-face {
    font-family: "paragraph";
    src: url("PTSerif-Bold.ttf");
    font-weight: bold;
}

body {
    font-family: "paragraph";
}

.dropcap {
    font-family: "paragraph";
    font-weight: bold;
}
```

**Font Files:** Place fonts in the same directory as the CSS file. They will be automatically embedded in output formats that support font embedding, including EPUB/KFX and PDF.

**Supported Font Files:** Use regular TrueType/OpenType font files (`.ttf`, `.otf`) for best cross-format behavior. PDF output requires raw font files that the native PDF renderer can parse; see [PDF Font Embedding Limits](#pdf-font-embedding-limits) for details.

#### Body Font-Family (KFX special handling)

The `body { font-family: "..."; }` rule triggers special handling during KFX conversion. When fb2cng detects a `font-family` on the bare `body` selector **and** corresponding `@font-face` resources exist for that family, several things happen:

1. **Document default font** — The font family name tells the Kindle reading system which embedded font is the document default.

2. **Style normalization** — Every style in the book is post-processed:
   - If a style's `font-family` matches the body font, it is replaced with the keyword `"default"` (telling the reader to use the document default font).
   - If a style has **no** `font-family` set, `"default"` is added automatically so it inherits the body font.
   - If a style uses a **different** font family (e.g., `"dropcaps"` or `monospace`), it is kept as-is.

This means you don't need to repeat `font-family` on every element — setting it on `body` is sufficient, and all styles will automatically reference it. Non-body fonts (like a dedicated dropcap font) work alongside the body font without conflict.

**Important:** The `body` rule is only recognized as a body font declaration when:
- The selector is exactly `body` (no class, no descendant)
- The font family name has matching `@font-face` resources with actual font data

#### Negative Margins

fb2cng now preserves negative CSS margin values, including in KFX output. This allows custom stylesheets to use negative margins when needed instead of having them stripped during conversion.

**Important KFX caveat:** Kindle Previewer 3 / the KFX rendering engine lays out drop caps badly when the dropcap paragraph itself has negative horizontal margins. For KFX output, fb2cng avoids negative horizontal margins on dropcap paragraphs while preserving the dropcap styling itself.

#### Resource Path Resolution

When referencing resources (fonts, images) in your custom stylesheet:

**Fragment References** - Reference FB2 binary objects:
```css
src: url("#font-id");  /* Resolves to FB2 <binary id="font-id"> */
```

**Relative Paths** - Resolve from the stylesheet's directory:
```css
src: url("fonts/MyFont.ttf");  /* Loads from ./fonts/MyFont.ttf */
```

**Security Constraints** — paths that escape the stylesheet's directory are rejected:
```css
src: url("../shared/fonts/x.ttf");   /* REJECTED — path traversal */
src: url("/usr/share/fonts/x.ttf");  /* REJECTED — absolute path */
```
A warning is logged and the `url()` reference is dropped from the output.

**Data URLs** - Kept as-is for formats/readers that can consume them directly:
```css
src: url("data:font/ttf;base64,...");  /* Already embedded in CSS */
```

**HTTP(S) URLs** - Not supported (warning logged):
```css
src: url("https://example.com/font.ttf");  /* Cannot be embedded */
```

**Note:** Resources are organized in EPUB by type:
- Fonts → `OEBPS/fonts/`
- Images and other resources → `OEBPS/other/`

See [stylesheets.md](stylesheets.md) for complete technical details on path resolution and resource handling.

### Image Processing

```yaml
document:
  images:
    # Use broken images without processing
    use_broken: false
    
    # Remove transparency from PNG/GIF images (for Kindle eInk)
    remove_transparency: false
    
    # Resize all images (1.0 = no change)
    scale_factor: 1.0
    
    # Recompress images
    optimize: true
    
    # JPEG quality 40-100%
    jpeg_quality_level: 75
    
    # Reader screen profile used for image sizing and PDF page size
    screen:
      width: 1264
      height: 1680
      dpi: 300
```

**Image Options:**

- **`use_broken`** - Controls what happens when an image cannot be decoded or processed.
  - `false` - Replace the bad image with the built-in "broken image" placeholder. This is the safer default because the output stays readable.
  - `true` - Keep the original image data untouched and let the reader/device decide whether it can display it.
  - Use `true` only when the source book contains unusual images that `fb2cng` cannot decode but your target reader may still support.

- **`remove_transparency`** - Flattens transparent PNG/GIF images onto a white background.
  - `false` - Keep transparency where the output format and reader can handle it.
  - `true` - Remove transparency to avoid rendering problems on Kindle eInk devices.
  - For Kindle output this behavior is applied automatically, even if the option is `false`.
  - Use this when transparent illustrations, logos, or UI-like graphics show dark boxes, missing areas, or other artifacts on eInk readers.

- **`scale_factor`** - Resizes all non-cover images by a multiplier.
  - `1.0` - Keep original size.
  - `< 1.0` - Shrink images to reduce file size.
  - `> 1.0` - Enlarge images.
  - Use values below `1.0` when books contain oversized images and output size matters. Avoid enlarging unless the source images are intentionally small, because this cannot add detail.

- **`optimize`** - Re-encodes supported raster images to reduce size and normalize output.
  - JPEG images are only re-encoded when the detected source quality is higher than `jpeg_quality_level`.
  - PNG images are re-encoded with best compression.
  - Grayscale JPEGs are detected and encoded as grayscale when possible.
  - For Kindle output, some image conversion still happens even with `optimize: false` because Kindle-compatible output requires JPEG raster images and rasterized SVG.
  - Use `true` for most books. Set it to `false` if you want to preserve original files as much as possible or if a specific image starts failing only after processing.

- **`jpeg_quality_level`** - Target JPEG quality used when JPEG images must be re-encoded.
  - Lower values create smaller files with more visible compression artifacts.
  - Higher values preserve more detail but increase output size.
  - For Kindle output this also affects PNG/GIF-to-JPEG conversion and SVG rasterization, not just optimized source JPEGs.
  - A practical range is usually `70-85`; use higher values for image-heavy books, comics, or covers where artifacts are more noticeable.

- **`screen.width` / `screen.height` / `screen.dpi`** - Target device screen profile.
  - `width` and `height` are device pixels.
  - `dpi` is dots per inch; the default is `300`.
  - The screen profile is available to all output formats and is used anywhere the converter needs to translate between device pixels, physical size, and output layout.
  - Affects cover resizing and image sizing decisions.
  - Used when rasterizing SVG images for Kindle output.
  - Especially important for PDF: `width`, `height`, and `dpi` define the fixed PDF page size using `points = pixels * 72 / dpi`.
  - Set these values to your main target device/profile when you care about exact sizing; otherwise the defaults are a reasonable general-purpose baseline.

**Format Notes:**

- Kindle output normalizes decodable raster images to JPEG.
- SVG images stay as SVG in EPUB output, but are rasterized for Kindle output.
- For Kindle output, `cover.resize: none` is effectively treated as `keepAR`.
- Cover images are not affected by `scale_factor`; they use the separate `cover.resize` rules below.

### Cover Image Configuration

```yaml
document:
  images:
    cover:
      # Generate cover if missing
      generate: true
      
      # Use custom default cover
      default_image_path: "mycover.jpg"
      
      # Resize mode: none, keepAR, stretch
      resize: stretch
```

**Cover Options:**

- **`generate`** - Create a fallback cover when the source book has no cover.
  - Use this when your library software or reader expects every book to have a cover.
  - For Amazon/Kindle output this is effectively always enabled automatically.

- **`default_image_path`** - Path to a custom image used when a generated/default cover is needed.
  - Use this when you want a branded fallback cover instead of the built-in one.

- **`resize`** - Controls how the cover is fitted to `screen.width` and `screen.height`.
  - **`none`** - Keep the cover at its original size. Use this when your source covers are already prepared for the target device. For Kindle output this is effectively changed to `keepAR`.
  - **`keepAR`** - Resize only when the cover is shorter than the target height, preserving aspect ratio. Use this when you want to avoid distortion but still prevent undersized covers.
  - **`stretch`** - Force the cover to exactly match the configured width and height. Use this when you need a predictable full-screen cover and do not mind possible distortion.

### PDF Generation

fb2cng can generate native PDF directly:

```bash
fbc convert --to pdf book.fb2
```

PDF output is a fixed-page format. Unlike EPUB/KFX, the converter performs pagination itself, embeds font resources, writes PDF outlines/navigation, creates internal links, and lays out images and printed footnotes before writing the final file.

**Main PDF characteristics:**

- Writes PDF 1.4 for broad reader/device compatibility.
- Page size is derived from `document.images.screen.width`, `height`, and `dpi`.
- Text is selectable/searchable and rendered with embedded Unicode fonts.
- Built-in fonts cover common Latin/Cyrillic/European text plus symbol/math fallback fonts.
- Custom `@font-face` fonts from the stylesheet can be embedded.
- TrueType fonts are subset when allowed by the font embedding flags; restricted fonts or CFF/CFF2 fonts may be embedded whole instead.
- Internal links, table of contents outlines, cover images, block images, tables, poems, epigraphs, drop caps, and vignettes are supported.
- Complex Arabic/Indic shaping, CJK typography, and bidi layout are not currently supported.

#### PDF Page Size

PDF page size is calculated from the configured screen size:

```yaml
document:
  images:
    screen:
      width: 1264
      height: 1680
      dpi: 300
```

The formula is:

```text
PDF points = pixels * 72 / dpi
```

With the default values this gives approximately `303.36 × 403.2` PDF points. Increase `width`/`height` or lower `dpi` for larger pages; decrease them or raise `dpi` for smaller pages. The same `screen` block is also used by image processing, so keep it aligned with the device/profile you are targeting.

#### PDF Styling

PDF uses the same `stylesheet_path` mechanism as other formats. A custom stylesheet replaces the default stylesheet entirely, so if you customize PDF output, start by copying [`convert/default.css`](../convert/default.css) and editing it.

Use `@media fbc-pdf` for PDF-only stylesheet rules. In the PDF renderer, `fbc-pdf` matches; the renderer also evaluates styles in an Enhanced Typesetting-like context, so `amzn-kf8` and `amzn-et` media queries match as well, while `amzn-mobi` does not. Put PDF-specific overrides in `@media fbc-pdf` (or combine it with other conditions) when you need rules that must apply only to native PDF output.

Commonly useful CSS properties for PDF include:

- `font-family`, `font-size`, `font-style`, `font-weight`, `line-height`, `letter-spacing`
- `color`, `background-color`, `text-decoration`
- `text-align`, `text-indent`, `white-space`, `hyphens`
- `margin`, `padding`, `width`, `min-width`, `max-width`
- `border`, `border-width`, `border-style`, `border-color`
- `page-break-before`, `page-break-after`, `page-break-inside`
- `orphans`, `widows`

PDF also understands generated pseudo-content used by the default stylesheet, such as footnote reference decorations on `.link-footnote::before` / `.link-footnote::after`.

The default stylesheet also contains PDF-specific selectors inside `@media fbc-pdf`. These are useful when customizing printed footnotes:

- `.footnote` - printed footnote text size, line-height, and compact spacing.
- `.footnote-title` - printed footnote title/label style; the default keeps it with following content using `page-break-after: avoid`.
- `.footnote-title-first`, `.footnote-title-next` - title fragments used when an FB2 footnote title contains several title paragraphs.
- `.footnote .link-backlink` - return-link styling when generated return links are present in default-mode PDF footnote bodies.
- `.footnote-separator` - separator rule inserted above the printed footnote area in PDF `float` / `floatRenumbered` modes. The default uses a thin top border and small vertical margins.

Example override:

```css
@media fbc-pdf {
    .footnote-separator {
        border-top: 0.75pt solid #666;
        margin-top: 0.3em;
        margin-bottom: 0.2em;
        width: 35%;
    }
}
```

Example PDF-focused configuration:

```yaml
document:
  stylesheet_path: "pdf.css"
  images:
    screen:
      width: 1264
      height: 1680
      dpi: 300
  footnotes:
    mode: float
```

Example `pdf.css` fragment:

```css
body {
    font-family: "PT Serif", serif;
}

@media fbc-pdf {
    body {
        font-size: 10.5pt;
        line-height: 1.2;
    }

    p {
        text-align: justify;
        text-indent: 1.2em;
        hyphens: auto;
    }
}

@font-face {
    font-family: "PT Serif";
    src: url("PTSerif-Regular.ttf");
}
```

#### PDF Font Embedding Limits

PDF font handling is stricter than EPUB/KFX because fb2cng must parse the font, shape the text, build PDF font dictionaries, and write the embedded font program itself.

**How PDF fonts are selected:**

- PDF reads `@font-face` declarations from the parsed stylesheet and matches them by `font-family`, `font-style`, and `font-weight`.
- Keep `@font-face` declarations at the top stylesheet level for predictable PDF embedding. Use `@media fbc-pdf` for rules that apply the font, not for hiding the font declaration itself.
- `src: url(...)` must resolve to an FB2 binary object (`url("#font-id")`) or to a file relative to the stylesheet.
- `local(...)` sources are ignored.
- If `src` contains several `url(...)` entries, the PDF loader uses the first resolvable URL.
- `format(...)` hints are not used; the actual font data is parsed and classified.
- The PDF renderer uses only family + bold + italic. Numeric weight is folded to regular/bold (`600` and above is bold), and variable font axes or optical-size settings are not applied.

**Supported PDF font programs:**

- **TrueType outlines** (`glyf` table, usually `.ttf`) are embedded as PDF Type0 / CIDFontType2 fonts with `Identity-H` encoding.
- TrueType fonts are subset to the used glyphs when the font permits subsetting.
- If the font has the OpenType `fsType` no-subsetting flag, fb2cng embeds the whole font instead of a subset.
- **OpenType CFF/CFF2 outlines** (usually `.otf`) can be embedded as PDF Type0 / CIDFontType0 fonts with `FontFile3` / `OpenType` data.
- CFF/CFF2 subsetting is not implemented, so these fonts are embedded whole. This can significantly increase PDF size.
- CFF2 embedding is supported by the writer, but compatibility has not been validated across every old PDF reader/device.

**Unsupported or limited cases:**

- Compressed browser font container formats are not decoded by the native PDF font loader. Use raw `.ttf` or `.otf` files for PDF.
- Type 1 fonts, bitmap-only fonts, and fonts without TrueType/CFF/CFF2 outlines are skipped for PDF and the renderer falls back to built-in fonts.
- Color emoji/color font tables are not rendered as color glyphs. Use monochrome outline fonts for reliable PDF output.
- Synthetic bold or synthetic italic is not generated. Provide separate `@font-face` entries for regular, bold, italic, and bold-italic if you need exact variants.
- If a requested family or variant is missing, fb2cng logs a warning and falls back to the closest embedded variant or to a built-in font.
- Built-in fallback covers many symbols/math characters, but it is not a full CJK/emoji fallback system. If your book needs a script outside the built-in coverage, provide a font that covers that script.
- Complex Arabic/Indic shaping, bidirectional layout, vertical writing, and CJK typography are outside the current PDF renderer's scope.
- OpenType shaping is used for supported left-to-right runs in a font that covers the whole run. Advanced CSS font feature controls are not exposed.

**Embedding rights and diagnostics:**

fb2cng reads the font `OS/2 fsType` embedding flags and reports them in debug/log output. The no-subsetting flag changes behavior as described above; other restrictive flags are reported so you can verify that you are allowed to embed the font. The converter cannot grant font embedding rights — use fonts whose license permits your intended output.

When troubleshooting PDF fonts, run with `--debug`. The report can include parsed stylesheet information, font fallback warnings, missing glyph diagnostics, and PDF font resource details.

#### PDF Footnotes

PDF has special behavior for `document.footnotes.mode`:

- `default` keeps footnotes as ordinary linked document sections. The footnote body can contain generated return links formatted by `backlink_template`.
- `float` and `floatRenumbered` render source footnotes as printed page footnotes at the bottom of the page where they are referenced. If a note does not fit, it is continued on later pages with continuation markers.
- In PDF `float` mode, visible reference labels from the source are preserved; the printed footnote title uses that source label, falling back to page-order numbering only when the reference has no visible label.
- In PDF `floatRenumbered` mode, references on each page are relabeled with page-local numeric labels (`1`, `2`, `3`, ... for that page). `label_template` does not choose that page-local number; it formats additional text appended to the printed footnote title after it.

See [Footnotes Processing](#footnotes-processing) for details and examples.

### Footnotes Processing

Footnotes are detected from FB2 bodies whose names match `document.footnotes.bodies`. By default this covers common English and Russian body names:

```yaml
document:
  footnotes:
    mode: default
    bodies: ["notes", "comments", "примечания", "комментарии"]
```

The conversion behavior is controlled by `document.footnotes.mode`:

```yaml
document:
  footnotes:
    # default, float, floatRenumbered
    mode: float
```

#### Footnote Modes

- **`default`** - Do not convert notes to popup/printed notes. References remain ordinary internal links to footnote sections.
  - EPUB/KFX: footnotes are normal book content/navigation entries.
  - PDF: footnote bodies are normal PDF pages/sections; return links can be generated in the footnote body using `backlink_template`.

- **`float`** - Convert detected notes to floating/popup-style notes where the target format supports that concept.
  - EPUB2/KEPUB: uses bidirectional links (`A -> B` and generated `B -> A`) for reader compatibility.
  - EPUB3: uses EPUB footnote/aside markup.
  - KFX/AZW8: marks footnote content for Kindle popup rendering and appends generated return-link paragraphs to footnote sections.
  - PDF: renders notes as printed page footnotes at the bottom of the page where the reference appears. Visible reference labels from the source are preserved; if a reference has no visible label, the printed note title falls back to page-order numbering.

- **`floatRenumbered`** - Same target-format behavior as `float`, but normalizes footnote numbering.
  - EPUB/KFX/AZW8: reference text and footnote titles are rewritten with `label_template` during content preparation.
  - PDF: main references are still assigned page-local numbers during PDF layout; `label_template` is used as extra title text in the printed footnote area, not as the visible page-local reference number.

Use `floatRenumbered` when the source FB2 has missing, inconsistent, duplicated, or non-sequential note labels.

#### `backlink_template`

`backlink_template` formats generated return links from footnote bodies back to the place where the note was referenced.

Current configuration uses the key **`backlink_template`**. Earlier configurations used **`backlink`** as a plain literal string; that field is now fully replaced by `backlink_template`, and unknown fields are rejected by the configuration loader. To keep exact old behavior, put the old literal string directly into the template value:

```yaml
# old:
# backlink: "↩"

# new equivalent:
backlink_template: "↩"
```

```yaml
document:
  footnotes:
    backlink_template: |
      {{- if eq .Format "pdf" -}}
      {{-   if .PageNumber -}}
      {{-     printf "[page\u00A0%d]" .PageNumber -}}
      {{-   else -}}
      {{-     "[<]" -}}
      {{-   end -}}
      {{- else if or (eq .Format "kfx") (eq .Format "azw8") -}}
      {{-   if .LocationNumber -}}
      {{-     printf "[loc\u00A0%d]" .LocationNumber -}}
      {{-   else if .PageNumber -}}
      {{-     printf "[page\u00A0%d]" .PageNumber -}}
      {{-   else if .SectionTitle -}}
      {{-     printf "[%s]" .SectionTitle -}}
      {{-   else -}}
      {{-     "[<]" -}}
      {{-   end -}}
      {{- else -}}
      {{-   if .PageNumber -}}
      {{-     printf "[page\u00A0%d]" .PageNumber -}}
      {{-   else if .SectionTitle -}}
      {{-     printf "[%s]" .SectionTitle -}}
      {{-   else -}}
      {{-     "[<]" -}}
      {{-   end -}}
      {{- end -}}
```

Available fields:

- `.Context` (string) - template field name (`backlink_template`)
- `.Format` (string) - output format: `epub2`, `epub3`, `kepub`, `kfx`, `azw8`, `pdf`
- `.PageNumber` (int) - exact page number for PDF; approximate/generated page-map number for EPUB/KFX when available
- `.LocationNumber` (int) - generated Kindle/KFX location number for KFX/AZW8 when available
- `.SectionTitle` (string) - nearest titled section/body containing the original reference, when known
- `.ChapterTitle` (string) - spine/chapter title containing the original reference, when known
- `.TargetID` (string) - ID of the footnote section being referenced
- `.RefID` (string) - unique generated ID of this reference occurrence
- `.RefNumber` (int) - 1-based occurrence number for this target footnote
- `.Href` (string) - generated return href where applicable
- `.Filename` (string) - generated content file containing the original reference where applicable

If the template is empty, invalid, or renders an empty string, fb2cng falls back to `[<]`.

Examples:

```yaml
# Compact universal marker
backlink_template: "[<]"
```

```yaml
# Prefer exact PDF page numbers, Kindle locations, then section titles
backlink_template: |
  {{- if and (eq .Format "pdf") .PageNumber -}}
  [page {{ .PageNumber }}]
  {{- else if .LocationNumber -}}
  [loc {{ .LocationNumber }}]
  {{- else if .SectionTitle -}}
  [{{ .SectionTitle }}]
  {{- else -}}
  [<]
  {{- end -}}
```

#### `more_paragraphs`

When a floating footnote contains more than one visible content element, some readers may show only the first one in the popup. The `more_paragraphs` setting prepends a visual indicator to the first visible footnote content so the reader knows there is additional content. Paragraphs, images, and tables are treated as visible content elements.

```yaml
document:
  footnotes:
    more_paragraphs: "(~) "
```

The default value is the string `(~)` followed by a non-breaking space. The indicator is styled with `.footnote-more`. The default stylesheet shows it for EPUB and hides it for KFX:

```css
@media not amzn-et {
    .footnote-more {
        text-decoration: none;
        font-weight: bold;
        color: gray;
    }
}

@media amzn-et {
    .footnote-more {
        display: none;
    }
}
```

To show or hide it differently, override `.footnote-more` in your custom stylesheet.

#### `label_template`

`label_template` formats logical footnote labels. It uses Go template syntax plus slim-sprig helpers.

```yaml
document:
  footnotes:
    label_template: |
      {{- if eq .Format "pdf" -}}
      {{-   $bodyTitle := default "Notes" .BodyTitle -}}
      {{-   $noteTitle := trim .NoteTitle -}}
      {{-   $noteNumber := printf "%d" .NoteNumber -}}
      {{-   $label := "" -}}
      {{-   if gt .BodyNumber 0 -}}
      {{-     $label = printf "%d.%s" .BodyNumber $noteNumber -}}
      {{-   else -}}
      {{-     $label = $noteNumber -}}
      {{-   end -}}
      {{-   if and $noteTitle (regexMatch "[^0-9[:space:]]" $noteTitle) -}}
      {{-     printf "%s: %s" $bodyTitle $noteTitle -}}
      {{-   else -}}
      {{-     printf "%s: %s" $bodyTitle $label -}}
      {{-   end -}}
      {{- else -}}
      {{-   if gt .BodyNumber 0 -}}
      {{-     printf "%d" .BodyNumber -}}.
      {{-   end -}}
      {{-   printf "%d" .NoteNumber -}}
      {{- end -}}
```

Available fields:

- `.Context` (string) - template field name (`label_template`)
- `.Format` (string) - output format: `epub2`, `epub3`, `kepub`, `kfx`, `azw8`, `pdf`
- `.BodyTitle` (string) - title of the footnote body, can be empty
- `.BodyNumber` (int) - 1-based index of the footnote body; set to `0` when the book has only one footnote body
- `.NoteTitle` (string) - original footnote section title, can be empty
- `.NoteNumber` (int) - 1-based sequential number of the footnote within its body

How `label_template` is used depends on output format and footnote mode:

| Output / mode | How `label_template` is used |
|---|---|
| EPUB/KFX/AZW8 `float` | Not used for renumbering; original source labels remain. |
| EPUB/KFX/AZW8 `floatRenumbered` | Rewrites main reference text and footnote titles during content preparation. |
| PDF `default` | Not used for printed page labels because footnotes are ordinary linked sections. |
| PDF `float` | Does not relabel visible references. It formats the suffix appended to each printed footnote title after the source reference label (or fallback page-order label if the source label is empty). |
| PDF `floatRenumbered` | References are relabeled with page-local numbers during PDF layout; this template formats the title suffix appended after that page-local label. |

For PDF printed footnotes, think of the rendered footnote title as:

```text
<printed-note-label><NBSP><label_template result>
```

In `float`, `<printed-note-label>` is the source reference label when present. In `floatRenumbered`, it is the page-local number assigned during PDF layout.

For example, with the default template and `floatRenumbered`, the second footnote on a PDF page whose logical note number is `17` may be printed as:

```text
2 Notes: 17
```

Here `2` is the page-local printed-footnote label and `Notes: 17` is the `label_template` result. If you want PDF printed footnote titles to contain only the printed-note label, keep the field present but render an empty string:

```yaml
document:
  footnotes:
    label_template: '{{- "" -}}'
```

If you want multiple footnote bodies to be visible in printed footnote titles:

```yaml
label_template: |
  {{- if .BodyTitle -}}
  {{-   printf "%s-%d" .BodyTitle .NoteNumber -}}
  {{- else if gt .BodyNumber 0 -}}
  {{-   printf "%d.%d" .BodyNumber .NoteNumber -}}
  {{- else -}}
  {{-   printf "%d" .NoteNumber -}}
  {{- end -}}
```

Result examples:

- single body: `1`, `2`, `3`
- multiple bodies without titles: `1.1`, `1.2`, `2.1`
- titled body: `Comments-1`, `Comments-2`

PDF printed footnote example: prefer a non-numeric source note title when present, otherwise fall back to body title plus generated number. This is useful when note titles sometimes contain meaningful text (`Translator note`) and sometimes contain only original numbering (`17`).

```yaml
document:
  footnotes:
    label_template: |
      {{- $bodyTitle := default "Notes" .BodyTitle -}}
      {{- $noteTitle := trim .NoteTitle -}}
      {{- $noteNumber := printf "%d" .NoteNumber -}}
      {{- $label := "" -}}
      {{- if gt .BodyNumber 0 -}}
      {{-   $label = printf "%d.%s" .BodyNumber $noteNumber -}}
      {{- else -}}
      {{-   $label = $noteNumber -}}
      {{- end -}}
      {{- if and $noteTitle (regexMatch "[^0-9[:space:]]" $noteTitle) -}}
      {{-   printf "%s: %s" $bodyTitle $noteTitle -}}
      {{- else -}}
      {{-   printf "%s: %s" $bodyTitle $label -}}
      {{- end -}}
```

In PDF `floatRenumbered`, this would be appended after the page-local printed label. For example, the second printed footnote on a page could become `2 Notes: 17` or `2 Notes: Translator note` depending on the source note title.

### Device Table of Contents

fb2cng always creates machine-readable navigation from the book's section structure. This is the TOC shown by the reading system's built-in navigation UI:

- EPUB2: `toc.ncx`
- EPUB3: `nav.xhtml`
- KFX/AZW8: KFX book navigation
- PDF: PDF document outline/bookmarks

Use `document.toc_type` to control how deeply nested this device navigation should be:

```yaml
document:
  # normal, old_kindle, flat
  toc_type: normal
```

Available values:

- `normal` - preserve the book's section nesting. This is the default and gives modern readers the full hierarchy.
- `old_kindle` - keep a shallow hierarchy compatible with old Kindle/kindlegen navigation behavior.
- `flat` - make all navigation entries top-level siblings.

This setting affects only machine/device navigation, including PDF outlines/bookmarks. Generated visible TOC pages inside the book content keep their normal/full structure regardless of `toc_type`.

### Table of Contents Page

The TOC (table of contents) itself is always generated automatically from the book's section structure. The settings below control an optional **TOC page** — a visible page rendered inside the book content that lists chapters as clickable links. This is separate from the TOC metadata used by the reading system's built-in navigation and from the `document.toc_type` setting described above.

```yaml
document:
  toc_page:
    # Placement: none, before, after
    placement: before
    
    # Include chapters without titles
    include_chapters_without_title: false
    
    # Format authors for TOC page
    authors_template: |
      {{- range .Authors -}}
      {{ .FirstName }} {{ .LastName }}
      {{- end -}}
```

### Annotation Page

When enabled, fb2cng generates an annotation page from the book's `<annotation>` metadata in the FB2 file. This is rendered as a separate chapter in the output, typically containing the book's description or summary as provided by the author or publisher.

```yaml
document:
  annotation:
    # Create annotation chapter
    enable: true
    
    # Chapter title
    title: "About this book"
    
    # Show in TOC
    in_toc: true
```

### Vignettes (Decorative Images)

```yaml
document:
  vignettes:
    book:
      title_top: builtin
      title_bottom: builtin
    chapter:
      title_top: /path/to/image.png
      title_bottom: builtin
      end: builtin
    section:
      title_top: builtin
```

Options: `builtin` (use default), file path, or omit to disable.

### Dropcaps (Drop Capitals)

```yaml
document:
  dropcaps:
    enable: true
    ignore_symbols: "'\"-.…0123456789‒–—«»""<>"
```

**Note:** Drop caps work best with regular paragraph margins. Negative margins are supported, but some readers - most notably KFX/Kindle - still do not render the combination of drop caps and negative margins perfectly in all cases. For KFX output, fb2cng avoids negative horizontal margins on dropcap paragraphs while preserving the dropcap styling itself.

### Text Transformations

Fix common issues in old FB2 files:

```yaml
document:
  text_transformations:
    # Fix speech dashes at paragraph start
    speech:
      enable: true
      from: "‐‑−–—―"
      to: "— "
    
    # Normalize dashes surrounded by spaces
    dashes:
      enable: true
      from: "‐‑−–—―"
      to: "—"
    
    # Fix dialogue formatting
    dialogue:
      enable: true
      from: "‐‑−–—―"
      to: " "
```

These transformations are intended for cleanup of legacy FB2 markup where various dash-like characters and spacing conventions are used inconsistently.

The default values in the sample configuration are not arbitrary. They reflect the defaults that have proven practical over years of real-world usage with older FB2 libraries and reader workflows.

fb2cng applies them in this order: `speech`, then `dashes`, then `dialogue`.

All three options have the same structure:

- `enable`: turn the transformation on or off
- `from`: a set of characters to match; every character in this string is treated as an allowed source variant
- `to`: replacement text inserted by the transformation

By default, `from: "‐‑−–—―"` means "treat any of these dash-like Unicode characters as equivalent input".

**Important scope limitation:** text transformations are applied only to regular paragraph content in section bodies. They are not applied to titles, subtitles, poems, cites, epigraphs, tables, or other special structures.

#### `speech`

Normalizes direct-speech markers at the **start of a text segment**.

It only matches when the very first character is one of the `from` characters. fb2cng then removes that opening dash and any spaces immediately following it, and replaces the whole prefix with `to`.

With the default configuration:

- `—Text` -> `— Text`
- `-   Text` -> `— Text`
- `  — Text` -> unchanged, because the dash is not the first character
- `Text - speech marker` -> unchanged, because this transformation only works at the beginning

Use this when old FB2 files start dialogue paragraphs with inconsistent dash characters or missing space after the opening dash.

#### `dashes`

Normalizes dashes that are surrounded by whitespace on both sides.

fb2cng scans the text and replaces any character from `from` only when it has whitespace before it and whitespace after it. This is useful for cases such as author-speech separators or spaced dashes in the middle of a sentence.

With the default configuration:

- `word - word` -> `word — word`
- `word – word` -> `word — word`
- `word-word` -> unchanged
- `— word` -> unchanged
- `word —` -> unchanged

Use this when the source mixes hyphen-minus, en dash, em dash, and similar characters between words.

#### `dialogue`

Normalizes whitespace immediately **before** an interior dialogue dash.

fb2cng looks for a run of whitespace followed by one of the `from` characters. When found, it keeps the dash itself and replaces the preceding whitespace with `to`.

With the default configuration:

- `"Hello,"   — said John` -> `"Hello," — said John`
- `"Hello,"\t— said John` -> `"Hello," — said John`
- `"Hello,"— said John` -> unchanged, because there is no whitespace before the dash

This is primarily useful for dialogue punctuation conventions where spacing before the dash must be normalized consistently. It can also help enforce Russian line-breaking rules by using a non-breaking space before the dialogue dash.

### Page Map

```yaml
document:
  page_map:
    # Generate page map for navigation
    enable: true
    
    # Page size in characters (Unicode code points), min 500
    size: 2300
    
    # Use Adobe RMSDK proprietary page-map.xml (EPUB2/KEPUB only)
    adobe_de: false
```

When page map is enabled, fb2cng inserts page markers into the document content at approximately every `size` Unicode code points and generates navigation metadata so that readers can display page numbers.

**How page numbers are generated:**

By default (`adobe_de: false`), page map data is written as a standard `<pageList>` element inside the NCX file. This is the EPUB-compliant approach and works with most modern reading systems.

**`adobe_de` option:**

Some e-reader devices are based on the Adobe RMSDK (Reading Mobile SDK). These include **Kobo** e-readers, older Sony Readers, and various other devices that use the Adobe rendering engine under the hood. The Adobe RMSDK does not support the standard NCX `<pageList>` for page navigation. Instead, it uses its own proprietary mechanism: a separate `page-map.xml` file referenced from the `<spine>` element via a non-standard `page-map` attribute.

Setting `adobe_de: true` switches fb2cng to this proprietary mode. When enabled:

1. A `page-map.xml` file is generated inside the EPUB package containing all page markers
2. The OPF manifest includes a `page-map` item with media type `application/oebps-page-map+xml`
3. The `<spine>` element gets a `page-map` attribute pointing to this item

This breaks strict EPUB compliance — EpubCheck will report `ERROR(RSC-005)` because the `page-map` attribute on `<spine>` is not part of the EPUB specification. However, it is the only way to get page number navigation working on Adobe RMSDK-based devices.

**Note:** This setting is only relevant for EPUB2 and KEPUB output formats. For EPUB3 and KFX, page map data is handled through their own native mechanisms regardless of this setting.

### Hyphenation

When enabled, `document.insert_soft_hyphen` inserts **soft hyphens** (Unicode character `U+00AD`) into words throughout the book text before it is written to reflowable output formats. A soft hyphen is an invisible character that marks a position where a word *may* be broken across lines with a hyphen. If the reading system does not need to break the word at that point, the soft hyphen remains invisible and has no effect. This allows reading systems that lack built-in hyphenation to still display properly hyphenated text.

fb2cng uses the Liang/Knuth hyphenation algorithm (the same algorithm used by TeX) with a set of built-in language-specific pattern dictionaries sourced from the [hyph-utf8](http://ctan.math.utah.edu/ctan/tex-archive/language/hyph-utf8/tex/patterns/txt) project. The appropriate dictionary is selected automatically based on the book's language metadata.

**In most cases this feature should not be used for EPUB/KFX.** Modern e-readers (Kindle, Kobo, Apple Books, etc.) include their own built-in hyphenation engines. Enabling soft hyphen insertion on these devices can cause conflicts — the reader's hyphenator may produce double hyphens, incorrect line breaks, or other visual artifacts. This option exists primarily for older devices that have no hyphenation support of their own.

```yaml
document:
  # Insert soft hyphens for devices without hyphenation
  insert_soft_hyphen: false
```

#### PDF line breaking and hyphenation

PDF is different from reflowable formats: fb2cng must decide all line breaks while creating the fixed PDF pages. For PDF layout, the internal hyphenator is always available to the paragraph breaker so it can find better word break points when CSS allows automatic hyphenation. This does **not** depend on `document.insert_soft_hyphen`; PDF does not need to insert soft hyphen characters into the source text for a reader to process later.

You can control PDF hyphenation with CSS:

```css
@media fbc-pdf {
    p {
        hyphens: auto;   /* allow dictionary hyphenation */
    }
}
```

Supported values for PDF are:

- `hyphens: auto` - allow dictionary hyphenation and explicit soft hyphen break points.
- `hyphens: manual` - use only explicit soft hyphens already present in the source text.
- `hyphens: none` - disable dictionary and explicit soft-hyphen breaks for that element.

To disable normal PDF word hyphenation for body paragraphs:

```css
@media fbc-pdf {
    p {
        hyphens: none;
    }
}
```

For short labels, code-like text, table headers, or other content that should not wrap at spaces either, also use `white-space: nowrap`:

```css
@media fbc-pdf {
    th,
    .no-wrap {
        hyphens: none;
        white-space: nowrap;
    }
}
```

Important limitation: PDF has an emergency wrapping path for text that still cannot fit into the available line width. Very long unbreakable words, oversized inline content, or narrow table cells may still be split to keep the PDF layout finite and avoid impossible lines. `hyphens: none` disables hyphenation opportunities; it is not an absolute guarantee that arbitrarily long text can never be emergency-wrapped.

### Logging Configuration

```yaml
logging:
  console:
    # Level: none, normal, debug
    level: normal
  
  file:
    destination: fb2cng.log
    level: debug
    # Mode: append, overwrite
    mode: overwrite
```

### Debug Reporting

```yaml
reporting:
  destination: fb2cng-report.zip
```

Enable with `--debug` flag. Creates archive with:
- Complete debug logs
- Configuration dump
- Processing artifacts
- KFX internals dump (when generating KFX)
- Error information

## Configuration

### Configuration File Structure

Configuration files use YAML format. Here's a minimal example:

```yaml
version: 1

document:
  open_from_cover: true
  toc_type: normal
  
  metainformation:
    transliterate: false
  
  images:
    optimize: true
    jpeg_quality_level: 80
    screen:
      width: 1264
      height: 1680
      dpi: 300
    cover:
      generate: true
      resize: stretch
  
  footnotes:
    mode: float

logging:
  console:
    level: normal
  file:
    destination: fb2cng.log
    level: debug
    mode: overwrite
```

### Configuration Loading

fb2cng always starts from the built-in default configuration embedded into the program.

When you pass `-c myconfig.yaml`, that file is loaded on top of the defaults:

1. Embedded defaults are loaded first
2. Values present in your custom YAML override the corresponding default values
3. Values you do not specify remain at their default values

So the configuration actually used during conversion is always the **merged active configuration**, not "your file only".

Examples:

- No config file: `fbc convert book.fb2` -> use embedded defaults as-is
- Custom config: `fbc -c myconfig.yaml convert book.fb2` -> use defaults plus your overrides
- Dump default config template: `fbc dumpconfig --default default.yaml`
- Dump merged active config: `fbc -c myconfig.yaml dumpconfig active.yaml`
- Dump merged active config to screen: `fbc -c myconfig.yaml dumpconfig`

### Configuration Best Practices

1. Start with default configuration and modify only what you need
2. Use comments to document your customizations
3. Keep separate configs for different use cases (e.g., `kindle.yaml`, `kobo.yaml`)
4. Test with `--debug` flag when trying new settings
5. Validate templates with small test files first

## MyHomeLib Integration

fb2cng includes the `mhl-connector` utility for integration with MyHomeLib library management software.

### Installation Structure

```
MyHomeLib Installation Directory
│   MyHomeLib.exe
│
└───converters
    ├───converter
    │       fbc.exe
    │       mhl-connector.exe
    │
    ├───fb2epub
    │       fb2epub.exe  (copy or symlink to mhl-connector.exe)
    │       fb2epub.yaml (optional fbc.exe configuration)
    │       connector.yaml (optional connector configuration)
    │
    ├───fb2mobi
    │       fb2mobi.exe  (copy or symlink to mhl-connector.exe)
    │       fb2mobi.yaml (optional fbc.exe configuration)
    │       connector.yaml (optional connector configuration)
    │
    └───fb2pdf
            fb2pdf.cmd  (required by MyHomeLib for PDF; wrapper that starts fb2pdf.exe)
            fb2pdf.exe  (copy or symlink to mhl-connector.exe)
            fb2pdf.yaml (optional fbc.exe configuration)
            connector.yaml (optional connector configuration)
```

### Setup Options

**Option 1: Copy executable**
- Copy `mhl-connector.exe` as `fb2epub.exe`, `fb2mobi.exe`, and/or `fb2pdf.exe`
- For PDF, also create `fb2pdf.cmd` next to `fb2pdf.exe`; MyHomeLib starts the `.cmd` file for PDF conversion
- Place `fbc.exe` in system PATH or in `converter` directory

**Option 2: Symlinks (recommended)**
- Create symlinks next to `fbc.exe`
- For PDF, also create `fb2pdf.cmd` next to the `fb2pdf.exe` symlink; MyHomeLib starts the `.cmd` file for PDF conversion
- No PATH modification needed
- Both executables can be anywhere

**Windows (as Administrator):**
```cmd
cd converters\fb2epub
mklink fb2epub.exe ..\converter\mhl-connector.exe
cd ..\fb2mobi
mklink fb2mobi.exe ..\converter\mhl-connector.exe
cd ..\fb2pdf
mklink fb2pdf.exe ..\converter\mhl-connector.exe
```

For PDF conversion, MyHomeLib expects `fb2pdf.cmd` rather than `fb2pdf.exe`. Create `fb2pdf.cmd` in the same directory as `fb2pdf.exe`:

```cmd
@echo off
setlocal

set "EXE=%~dpn0.exe"

if not exist "%EXE%" (
    echo Executable not found: "%EXE%" 1>&2
    exit /b 1
)

"%EXE%" %*
exit /b %ERRORLEVEL%
```

### Configuration

#### FBC Configuration (Optional)

Place format-specific fbc configuration files in the same directory as the connector:

- `fb2epub.yaml` - Settings for EPUB conversion (passed to fbc.exe)
- `fb2mobi.yaml` - Settings for MOBI/KFX conversion (passed to fbc.exe)
- `fb2pdf.yaml` - Settings for PDF conversion (passed to fbc.exe)

These files use the same format as the main fbc configuration file documented above.

#### Connector Configuration (Optional)

Since passing additional arguments via MyHomeLib is inconvenient, the connector supports an optional `connector.yaml` configuration file. This file should be located in the same directory as the connector executable (next to `fb2epub.exe`, `fb2mobi.exe`, or `fb2pdf.exe`).

**Configuration structure:**

```yaml
# Content should be UTF-8!
version: 1

# Redirect connector logs to a file (optional)
# log_destination: connector.log

# Pass debug flag to fbc (optional, default: false)
debug: false

# Output format specification (optional)
# When not specified or not compatible, defaults will be used
# output_format: epub3

# Mark Kindle output as ebook (EBOK) instead of personal document (PDOC)
kindle_ebook: false
```

**Available fields:**

- `log_destination` (string, optional) - Path to log file for connector diagnostics. If not specified, logs go to console
- `debug` (boolean, default: false) - Enable debug mode for fbc.exe, generates diagnostic report
- `output_format` (string, optional) - Override default output format. Allowed values:
  - For `fb2epub.exe`: `epub2`, `epub3`, `kepub`
  - For `fb2mobi.exe`: `kfx`, `azw8`
  - For `fb2pdf.exe`: `pdf`
  - If incompatible format is specified, default is used with warning in logs
- `kindle_ebook` (boolean, default: false) - For Kindle outputs (`kfx`, `azw8`) pass `--ebook` to mark as EBOK; this is only relevant for `fb2mobi.exe`

**When connector.yaml is not needed:**

In most cases, the connector works fine without configuration. You only need `connector.yaml` if you want to:
- Debug the MyHomeLib integration
- Override the default output format
- Redirect logs to a file for troubleshooting

### Connector Behavior

- `fb2epub.exe` → Converts to EPUB2 by default (or format specified in `connector.yaml`)
- `fb2mobi.exe` → Converts to KFX by default
- `fb2pdf.exe` → Converts to PDF by default
- `kindle_ebook` is only meaningful for Kindle output formats (`kfx`, `azw8`), so it is only used by `fb2mobi.exe`
- When `kindle_ebook: true` is set for `fb2mobi.exe`, the connector adds `--ebook` so Kindle output is marked as EBOK instead of PDOC
- If `output_format` is not supported by the current connector target, the connector falls back to that target's default format and writes a warning to the logs
- Example: `output_format: epub3` with `fb2mobi.exe` falls back to `kfx`
- Example: `output_format: azw8` with `fb2epub.exe` falls back to `epub2`
- Example: `output_format: epub3` with `fb2pdf.exe` falls back to `pdf`
- Automatically uses `--overwrite` flag
- Expects exactly 2 arguments: source and destination files
- Logs to console by default (or to file if specified in `connector.yaml`)

## Troubleshooting

### Enable Debug Mode

Always use debug mode when investigating issues:

```bash
fbc -d convert book.fb2
```

This creates `fb2cng-report.zip` with complete diagnostic information.

### Common Issues

#### Conversion Fails

**Problem:** "Unable to process file"

**Solutions:**
- Check FB2 file is valid XML
- Try with `--debug` to see detailed error
- Check file encoding (should be UTF-8)
- Verify file isn't corrupted

#### Archive Processing Issues

**Problem:** "Cannot read archive" or wrong filenames

**Solutions:**
- Use `--force-zip-cp` with correct encoding:
  - Russian Windows archives: `--force-zip-cp windows-1251`
  - DOS archives: `--force-zip-cp cp866`
- Check archive isn't password protected
- Verify archive isn't corrupted

#### Output File Not Found

**Problem:** Conversion succeeds but file missing

**Solutions:**
- Check destination directory exists and is writable
- Without `--overwrite`, existing files aren't replaced
- Check `output_name_template` isn't creating invalid paths
- Look in subdirectories (structure may be preserved)

#### Configuration Not Working

**Problem:** Settings seem ignored

**Solutions:**
- Verify YAML syntax (use online YAML validator)
- Check indentation (spaces, not tabs)
- Dump active config: `fbc -c myconfig.yaml dumpconfig`
- Look for error messages about configuration

#### Template Errors

**Problem:** "Unable to execute template"

**Solutions:**
- Check template syntax against [Go template docs](https://pkg.go.dev/text/template)
- Verify variables exist (see template variables sections)
- Test with simple template first
- Use `--debug` to see processed values

#### Font/Stylesheet Issues

**Problem:** Fonts not embedded or stylesheet not applied

**Solutions:**
- Verify file paths are correct and relative to CSS file
- Check font files exist and are valid
- See [stylesheets.md](stylesheets.md) for path resolution rules
- Font URLs should use `url("filename.ttf")` format

#### Image Problems

**Problem:** Images missing or broken in output

**Solutions:**
- Set `use_broken: true` to include problematic images
- Check `optimize: false` if image processing causes issues
- Verify source images aren't already corrupted
- Check log for specific image processing errors

### Getting Help

When reporting issues, include:

1. **Version information:** `fbc --version`
2. **Command used:** Full command line with all options
3. **Debug report:** Output from `fbc -d convert ...`
4. **Configuration:** Your YAML config file (if used)
5. **Sample file:** Small FB2 that reproduces the problem (if possible)

Visit the [GitHub repository](https://github.com/rupor-github/fb2cng) to:
- Report bugs in Issues
- Request features
- Contribute code
- View source code

Russian language discussions available on [4PDA forum](https://4pda.to/forum/index.php?showtopic=942250).

### Log Files

Check logs for detailed information:

**Console output:**
- Shows INFO level messages by default
- Change with `logging.console.level` in config

**File log:**
- Default: `fb2cng.log` in current directory
- Contains DEBUG level by default
- Configure with `logging.file` settings

**Debug report:**
- Created with `--debug` flag
- ZIP archive with complete diagnostic data
- Location: `fb2cng-report.zip` (configurable)

### Performance Tips

1. **Batch processing:** Process multiple files in one command
2. **Skip optimization:** Disable if images already optimized
3. **Disable features:** Turn off unused features (vignettes, dropcaps, etc.)
4. **Simplified templates:** Complex templates slow processing
5. **SSD storage:** Use fast storage for source and destination

---

**Version:** For specific version features, see `fbc --version` and release notes.
