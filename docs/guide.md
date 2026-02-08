# FBC User Guide

## Table of Contents

- [Introduction](#introduction)
- [Installation](#installation)
- [Quick Start](#quick-start)
- [Basic Usage](#basic-usage)
- [Advanced Features](#advanced-features)
- [Configuration](#configuration)
- [MyHomeLib Integration](#myhomelib-integration)
- [Troubleshooting](#troubleshooting)

## Introduction

**fb2cng** (FictionBook to Next Generation) is a complete rewrite of [fb2converter](https://github.com/rupor-github/fb2converter), designed to convert FB2 (FictionBook) files to various e-book formats including EPUB2, EPUB3, KEPUB, and KFX.

### Supported Output Formats

- **EPUB2** - Standard EPUB format with wide device compatibility
- **EPUB3** - Modern EPUB format with enhanced features
- **KEPUB** - EPUB2 optimized for Kobo devices
- **KFX** - Kindle format X (with `.kfx` extension)
- **AZW8** - Kindle format X with `.azw8` extension (same as KFX, different extension, added for convinience - Kindle Previewer 3 can open azw8 files directly and Kindle devices handle them just fine)

### Key Features

- Batch conversion support (directories and archives)
- Flexible configuration via YAML files
- Template-based file naming and metadata formatting
- Custom CSS stylesheet support with font embedding
- Image optimization and cover generation
- Footnotes processing with floating/popup support
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

Supported formats: `epub2`, `epub3`, `kepub`, `kfx`, `azw8`

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

- `--to TYPE` - Output format: `epub2` (default), `epub3`, `kepub`, `kfx`, `azw8`
- `--ebook, --eb` - For Kindle formats, mark output as ebook (EBOK) instead of personal document (PDOC)
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

- `.Title` - Book title
- `.Authors` - Array of author objects with `.FirstName`, `.MiddleName`, `.LastName`
- `.Series` - Array with `.Name` and `.Number`
- `.Language` - Language code
- `.Date` - Publication date
- `.Format` - Output format (epub2, epub3, etc.)
- `.SourceFile` - Original filename (no path/extension)
- `.BookID` - Book UUID
- `.Genres` - Array of genre names

### Metadata Customization

Format book title and author names in metadata:

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
- `.footnote`, `.footnote-title` - Footnote styling
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

**Font Files:** Place fonts in the same directory as the CSS file. They will be automatically embedded in the EPUB.

**Supported Formats:** TTF, OTF, WOFF, WOFF2

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

#### Resource Path Resolution

When referencing resources (fonts, images) in your custom stylesheet:

**Fragment References** - Reference FB2 binary objects:
```css
src: url("#font-id");  /* Resolves to FB2 <binary id="font-id"> */
```

**Relative Paths** - Resolve from current working directory:
```css
src: url("fonts/MyFont.ttf");  /* Loads from ./fonts/MyFont.ttf */
src: url("../shared/fonts/x.ttf");  /* Basename only: fonts/x.ttf in EPUB */
```

**Absolute Paths** - Basename extracted:
```css
src: url("/usr/share/fonts/MyFont.ttf");  /* Becomes fonts/MyFont.ttf in EPUB */
```

**Data URLs** - Kept as-is (already embedded):
```css
src: url("data:font/woff2;base64,...");  /* No processing needed */
```

**HTTP(S) URLs** - Not supported (warning logged):
```css
src: url("https://example.com/font.woff");  /* Cannot be embedded */
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
    jpeq_quality_level: 75
    
    # Reader screen size so images could be adjusted properly
    screen:
      width: 1264
      height: 1680
```

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

### Footnotes Processing

```yaml
document:
  footnotes:
    # Mode: default, float, floatRenumbered
    mode: float
    
    # FB2 bodies to treat as footnotes
    bodies: ["notes", "comments", "примечания", "комментарии"]
    
    # Backlink symbol
    backlinks: "[<]"
    
    # Multi-paragraph indicator
    more_paragraphs: "(~)\u00A0"
    
    # Label template (only used with floatRenumbered mode)
    label_template: |
      {{- if gt .BodyNumber 0 -}}
      {{-   printf "%d" .BodyNumber -}}.
      {{- end -}}
      {{- printf "%d" .NoteNumber -}}
```

**Footnote Modes:**

- **`default`** - Regular hyperlinks to footnotes with no special processing
- **`float`** - Popup/floating footnotes (requires reader support). Preserves original footnote reference text from FB2 file
- **`floatRenumbered`** - Same as `float`, but automatically renumbers all footnotes sequentially and replaces their reference text with formatted labels

**floatRenumbered Mode:**

When using `floatRenumbered` mode, the converter:
1. Assigns sequential numbers to each footnote within each footnote body
2. Updates footnote reference text in the main content to use the formatted label
3. Updates footnote section titles to match the new numbering

This is useful when the original FB2 has inconsistent or non-sequential footnote numbering.

**label_template:**

The `label_template` uses Go template syntax to format how footnote references appear. Available fields:

- `.BodyTitle` (string) - Title of the footnote body (can be empty)
- `.BodyNumber` (int) - 1-based index of the footnote body (0 if only one body)
- `.NoteTitle` (string) - Original footnote title (can be empty)
- `.NoteNumber` (int) - 1-based sequential number of the footnote within its body

**Examples:**

Simple sequential numbering (default):
```yaml
label_template: |
  {{- printf "%d" .NoteNumber -}}
```
Result: `1`, `2`, `3`, ...

Body prefix when multiple footnote bodies exist:
```yaml
label_template: |
  {{- if gt .BodyNumber 0 -}}
  {{-   printf "%d" .BodyNumber -}}.
  {{- end -}}
  {{- printf "%d" .NoteNumber -}}
```
Result: `1.1`, `1.2`, `2.1`, `2.2`, ... (or just `1`, `2`, ... if single body)

Custom format with body title:
```yaml
label_template: |
  {{- if .BodyTitle -}}
  {{-   printf "[%s-%d]" .BodyTitle .NoteNumber -}}
  {{- else -}}
  {{-   printf "[%d]" .NoteNumber -}}
  {{- end -}}
```
Result: `[Notes-1]`, `[Notes-2]`, ... (or `[1]`, `[2]`, ... if no title)

### Table of Contents

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

### Page Map

```yaml
document:
  page_map:
    # Generate page map for navigation
    enable: true
    
    # Page size in characters
    size: 2300
    
    # Adobe Digital Editions support (EPUB2/KEPUB only)
    adobe_de: false
```

### Hyphenation

```yaml
document:
  # Insert soft hyphens for devices without hyphenation
  insert_soft_hyphen: false
```

**Note:** May conflict with built-in reader hyphenation.

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
  
  metainformation:
    transliterate: false
  
  images:
    optimize: true
    jpeq_quality_level: 80
    screen:
      width: 1264
      height: 1680
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

1. **No config file** - Uses built-in defaults
2. **Custom config** - Specify with `-c` flag: `fbc -c myconfig.yaml convert ...`
3. **Merged config** - Your settings override defaults, missing values use defaults

### Getting Default Configuration

```bash
fbc dumpconfig --default default.yaml
```

This provides a complete template with all available options and their default values.

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
    ├───fb2converter
    │       fbc.exe
    │       mhl-connector.exe
    │
    ├───fb2epub
    │       fb2epub.exe  (copy or symlink to mhl-connector.exe)
    │       fb2epub.yaml (optional fbc.exe configuration)
    │       connector.yaml (optional connector configuration)
    │
    └───fb2mobi
            fb2mobi.exe  (copy or symlink to mhl-connector.exe)
            fb2mobi.yaml (optional fbc.exe configuration)
            connector.yaml (optional connector configuration)
```

### Setup Options

**Option 1: Copy executable**
- Copy `mhl-connector.exe` as `fb2epub.exe` and/or `fb2mobi.exe`
- Place `fbc.exe` in system PATH or in `fb2converter` directory

**Option 2: Symlinks (recommended)**
- Create symlinks next to `fbc.exe`
- No PATH modification needed
- Both executables can be anywhere

**Windows (as Administrator):**
```cmd
mklink fb2epub.exe mhl-connector.exe
mklink fb2mobi.exe mhl-connector.exe
```

### Configuration

#### FBC Configuration (Optional)

Place format-specific fbc configuration files in the same directory as the connector:

- `fb2epub.yaml` - Settings for EPUB conversion (passed to fbc.exe)
- `fb2mobi.yaml` - Settings for MOBI/KFX conversion (passed to fbc.exe)

These files use the same format as the main fbc configuration file documented above.

#### Connector Configuration (Optional)

Since passing additional arguments via MyHomeLib is inconvenient, the connector supports an optional `connector.yaml` configuration file. This file should be located in the same directory as the connector executable (next to `fb2epub.exe` or `fb2mobi.exe`).

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
  - If incompatible format is specified, default is used with warning in logs
- `kindle_ebook` (boolean, default: false) - For Kindle outputs, pass `--ebook` to mark as EBOK

**When connector.yaml is not needed:**

In most cases, the connector works fine without configuration. You only need `connector.yaml` if you want to:
- Debug the MyHomeLib integration
- Override the default output format
- Redirect logs to a file for troubleshooting

### Connector Behavior

- `fb2epub.exe` → Converts to EPUB2 by default (or format specified in `connector.yaml`)
- `fb2mobi.exe` → Converts to KFX by default
- `kindle_ebook` in `connector.yaml` adds `--ebook` for `fb2mobi.exe` conversions
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

Russian language discussions available on [4PDA forum](https://4pda.ru/forum/index.php?showtopic=942250).

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