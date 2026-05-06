<h1>
    <img src="docs/books.svg" style="vertical-align:middle; width:8%" align="absmiddle"/>
    <span style="vertical-align:middle;">&nbsp;&nbsp;FB2 converter to EPUB2/3, KEPUB, AZW8/KFX</span>
</h1>

### A complete rewrite of [fb2converter](https://github.com/rupor-github/fb2converter)
[![GitHub Release](https://img.shields.io/github/release/rupor-github/fb2cng.svg)](https://github.com/rupor-github/fb2cng/releases)

* Produces EPUB2/3 and KEPUB files which pass latest [epubcheck](https://www.w3.org/publishing/epubcheck/) with no error/warnings
* Generates KFX/AZW8 files directly without any Amazon's software (much faster than using Kindle Previewer 3 with Calibre plugins)

> **Note:** Direct KFX generation is a relatively new feature, so various hiccups may still occur. The generator aims to preserve maximum compatibility with Kindle Previewer 3 behavior, while also intentionally supporting capabilities that Amazon's processing pipeline does not expose. Examples include predictable handling of negative margins, richer drop-cap rendering, and condensed text controlled through `html`/`body` line-height processing. If you encounter any issues, please [create an issue](https://github.com/rupor-github/fb2cng/issues) on GitHub for investigation.

### Documentation

[User guide](docs/guide.md)

[Russian discussion forum](https://4pda.to/forum/index.php?showtopic=942250#)

### Installation:

Download from the [releases page](https://github.com/rupor-github/fb2cng/releases) and unpack it in a convenient location.
