<table>
<tr>
<td width="96" valign="middle"><img src="docs/books.svg" width="96" alt="fb2cng"/></td>
<td valign="middle"><h1>FB2 converter to EPUB2/3, KEPUB, AZW8/KFX, PDF, TXT/MD</h1></td>
</tr>
</table>

### A complete rewrite of [fb2converter](https://github.com/rupor-github/fb2converter)
[![GitHub Release](https://img.shields.io/github/release/rupor-github/fb2cng.svg)](https://github.com/rupor-github/fb2cng/releases)

### Format highlights

| Format | Essential features |
|---|---|
| EPUB2 | Broad e-reader compatibility, XHTML/CSS styling, NCX navigation, linked footnote sections, SVG preservation, optional visible TOC page |
| EPUB3 | Modern EPUB package with `nav.xhtml`, semantic `aside`/`noteref` footnotes in floating modes, XHTML/CSS styling, SVG preservation |
| KEPUB | Kobo-oriented EPUB packaging, Kobo-friendly navigation, XHTML/CSS styling, linked footnotes, SVG preservation |
| KFX/AZW8 | Direct Kindle Enhanced Typesetting output without Calibre or Amazon software, Kindle style records, floating/popup footnotes, generated location/page navigation, Kindle-compatible image conversion |
| PDF | Native PDF 1.4 generation, fixed target page size, selectable/searchable text, embedded fonts, outlines/bookmarks, internal links, tables, images, drop caps, printed page footnotes |
| TXT | UTF-8 readable plain text, plain headings, aligned text tables, image placeholders, readable note labels, collected final notes for floating footnotes |
| Markdown | Semantic UTF-8 Markdown, YAML front matter, headings, links, anchors, pipe tables, fenced code blocks, collected endnotes with backlinks, placeholder/external/embedded image modes |

EPUB2/3 and KEPUB output is intended to pass the latest [epubcheck](https://www.w3.org/publishing/epubcheck/) without errors or warnings.

> **Note:** Direct KFX and PDF generation are relatively new features, so various hiccups may still occur. If you encounter any issues, please [create an issue](https://github.com/rupor-github/fb2cng/issues) on GitHub for investigation. The generator aims to preserve maximum compatibility with Kindle Previewer 3 behavior, while also intentionally supporting capabilities that Amazon's processing pipeline does not expose. Examples include predictable handling of negative margins, richer drop-cap rendering, and condensed text controlled through `html`/`body` line-height processing. Good luck trying to produce something like [this](docs/page.jpg) or [this](docs/page2.jpg) via "Send To Kindle".

### Documentation

[User guide](docs/guide.md)

[Russian discussion forum](https://4pda.to/forum/index.php?showtopic=942250#)

### Installation:

Download from the [releases page](https://github.com/rupor-github/fb2cng/releases) and unpack it in a convenient location.
