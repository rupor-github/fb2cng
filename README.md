<table>
<tr>
<td width="96" valign="middle"><img src="docs/books.svg" width="96" alt="fb2cng"/></td>
<td valign="middle"><h1>Converter of FB2 files to EPUB2/3, KEPUB/KEPUB3, AZW3, AZW8/KFX, PDF, TXT/MD</h1></td>
</tr>
</table>

### A complete rewrite of [fb2converter](https://github.com/rupor-github/fb2converter)
[![GitHub Release](https://img.shields.io/github/release/rupor-github/fb2cng.svg)](https://github.com/rupor-github/fb2cng/releases)

### Format highlights

| Format | Essential features |
|---|---|
| [EPUB2](docs/formats/epub.md) | Broad e-reader compatibility, XHTML/CSS styling, NCX navigation, linked footnote sections, SVG preservation, optional visible TOC page |
| [EPUB3](docs/formats/epub.md) | Modern EPUB navigation, semantic popup-note markup in floating modes, XHTML/CSS styling, SVG preservation |
| [KEPUB/KEPUB3](docs/formats/epub.md) | Kobo-oriented EPUB2 or EPUB3 packaging, XHTML/CSS styling with Kobo sentence markup, SVG preservation, WebP-to-PNG image conversion |
| [KFX/AZW8](docs/formats/kfx.md) | Direct Kindle Enhanced Typesetting output without Calibre or Amazon software, optional lossless LZMA compression, CSS-derived styling, floating/popup footnotes, generated location/page navigation, Kindle-compatible image conversion |
| [AZW3](docs/formats/azw3.md) | Direct standalone KF8 without KindleGen, configurable compression, HTML/CSS styling, linked or plain-text floating notes, embedded fonts, retained non-cover SVG, and optional APNX page maps |
| [PDF](docs/formats/pdf.md) | Native PDF 1.4 generation, fixed target page size, selectable/searchable text, embedded fonts, outlines/bookmarks, internal links, tables, images, drop caps, printed page footnotes |
| [TXT](docs/formats/text.md) | UTF-8 readable plain text, plain headings, aligned text tables, image placeholders, readable note labels, collected final notes for floating footnotes |
| [Markdown](docs/formats/text.md) | Semantic UTF-8 Markdown, YAML front matter, headings, links, anchors, pipe tables, fenced code blocks, collected endnotes with optional backlinks, placeholder/external/embedded image modes |

EPUB2/3 and KEPUB/KEPUB3 output is intended to pass the latest [epubcheck](https://www.w3.org/publishing/epubcheck/) without errors or warnings.

Kindle KFX/AZW8 output aims to follow Amazon's Kindle Publishing Guidelines as closely as possible.

Reader support varies, especially for popup notes, fonts, and SVG artwork. See the
[feature matrices](docs/guide.md#feature-matrix) for cross-format details and defaults.

### Documentation

[User guide](docs/guide.md)

[Image handling](docs/images.md)

[Debugging and validation](docs/debugging.md)

[Russian discussion forum](https://4pda.to/forum/index.php?showtopic=942250#)

### Installation

Download from the [releases page](https://github.com/rupor-github/fb2cng/releases) and unpack it in a convenient location.
See the [quick start](docs/guide.md#quick-start) for conversion commands.

### Contributing

Please read the [contribution guidelines](CONTRIBUTING.md) before opening an issue or pull request.
