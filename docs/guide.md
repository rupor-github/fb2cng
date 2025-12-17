# FB2CNG User Guide

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
- **KFX** - Kindle format (in development)

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

Supported formats: `epub2`, `epub3`, `kepub`, `kfx`

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

- `--to TYPE` - Output format: `epub2` (default), `epub3`, `kepub`, `kfx`
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

Specify your own CSS stylesheet and embed fonts:

```yaml
document:
  stylesheet_path: "mystyle.css"
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

body {
    font-family: "paragraph";
}
```

Place fonts in the same directory as the CSS file. They will be automatically embedded in the EPUB.

See [stylesheets.md](stylesheets.md) for detailed information on path resolution and resource handling.

### Image Processing

```yaml
document:
  images:
    # Use broken images without processing
    use_broken: false
    
    # Remove PNG transparency (for Kindle eInk)
    remove_png_transparency: false
    
    # Resize all images (1.0 = no change)
    scale_factor: 1.0
    
    # Recompress images
    optimize: true
    
    # JPEG quality 40-100%
    jpeq_quality_level: 75
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
      
      # Target dimensions
      width: 1264
      height: 1680
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
```

**Modes:**
- `default` - Regular hyperlinks
- `float` - Popup footnotes (reader support required)
- `floatRenumbered` - Popup with sequential numbering

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
    cover:
      generate: true
      resize: stretch
      width: 1264
      height: 1680
  
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
fbc dumpconfig --default > default.yaml
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
    │       fb2epub.yaml (optional configuration)
    │
    └───fb2mobi
            fb2mobi.exe  (copy or symlink to mhl-connector.exe)
            fb2mobi.yaml (optional configuration)
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

Place format-specific configuration files in the same directory as the connector:

- `fb2epub.yaml` - Settings for EPUB conversion
- `fb2mobi.yaml` - Settings for MOBI/KFX conversion

If no configuration file exists, defaults are used.

### Debug Mode

Set environment variable to enable debug output:

**Windows:**
```cmd
set FBC_DEBUG=yes
```

### Connector Behavior

- `fb2epub.exe` → Converts to EPUB3
- `fb2mobi.exe` → Converts to KFX
- Automatically uses `--overwrite` flag
- Expects exactly 2 arguments: source file and destination directory

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

#### Memory Issues

**Problem:** Out of memory with large files

**Solutions:**
- Process files individually rather than entire directories
- Disable image optimization: `optimize: false`
- Reduce image scaling: `scale_factor: 0.8`
- Use 64-bit version of the program

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

**License:** See project repository for license information.

**Build Key:** RWTNh1aN8DrXq26YRmWO3bPBx4m8jBATGXt4Z96DF4OVSzdCBmoAU+Vq
