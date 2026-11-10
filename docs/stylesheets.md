# Stylesheets

fb2cng uses CSS stylesheets for EPUB, KEPUB variants, AZW3, KFX/AZW8, and PDF output. TXT and Markdown ignore visual CSS styling, though text transformations and normalized content still apply before rendering.

Set a custom stylesheet with:

```yaml
document:
  stylesheet_path: "mystyle.css"
```

A custom stylesheet replaces the built-in stylesheet completely. Start from `convert/default.css` if you want to modify the default behavior instead of writing all rules from scratch.

## Default Classes

The built-in stylesheet covers the normalized FB2 structure.

Document structure:

- `.body-title`, `.chapter-title`, `.section-title` - title containers with page-break control.
- `.section-title-h2` through `.section-title-h6` - depth-specific section title wrappers.
- `.body-title-header`, `.chapter-title-header`, `.section-title-header` - title text.
- `.section-subtitle` - section subtitles.
- `.toc-title` - visible table-of-contents title.

Content:

- `p` - ordinary paragraphs.
- `h1` through `h6` - headings.
- `code` - inline/code-block text.
- `.dropcap` - decorative first letter.
- `.has-dropcap` - paragraph containing a drop cap.
- `.emptyline` - FB2 empty-line spacing.

Poetry and quotes:

- `.poem`, `.poem-title`, `.poem-subtitle`.
- `.stanza`, `.stanza-title`.
- `.verse`.
- `.cite`.
- `.epigraph`, `.epigraph-subtitle`.
- `.text-author`.

Images and decorations:

- `.image` - image container.
- `img.image-block` - block image.
- `img.image-inline` - inline image.
- `img.image-vignette` - decorative vignette.
- `.vignette-book-title-top`, `.vignette-book-title-bottom`.
- `.vignette-chapter-title-top`, `.vignette-chapter-title-bottom`, `.vignette-chapter-end`.
- `.vignette-section-title-top`, `.vignette-section-title-bottom`, `.vignette-section-end`.

Footnotes and links:

- `.footnote` - footnote body/content.
- `.footnote-title` - footnote title.
- `.footnote-title-first`, `.footnote-title-next` - split/generated footnote title paragraph parts.
- `.footnote-separator` - separator before PDF printed footnotes.
- `.footnote-more` - multi-paragraph popup indicator.
- `.link-footnote` - footnote references.
- `.link-backlink` - generated return links.
- `.link-external`, `.link-internal`, `.link-toc`.

## Media Queries

The default stylesheet uses output-specific media queries:

| Query | Applies To |
|---|---|
| `@media amzn-mobi` | Kindle MOBI-style compatibility rules |
| `@media amzn-kf8` | Kindle AZW3/KF8 and KFX-compatible rules |
| `@media amzn-et` | Kindle Enhanced Typesetting / KFX rules |
| `@media fbc-pdf` | Native PDF renderer rules |

Media query conditions can be combined with `and`, `not`, `only`, and comma-separated
alternatives. A leading `not` negates the whole query; `and not` negates one condition.

```css
@media fbc-pdf {
    body {
        font-size: 120%;
        line-height: 130%;
    }
}

@media amzn-kf8 and not amzn-et {
    p {
        margin: 0 -8pt 0.3em -8pt;
    }
}
```

For **AZW3**, media selection uses KF8 capabilities, not Enhanced Typesetting:

| Condition | AZW3 |
|---|---|
| `amzn-kf8` | Matches |
| `amzn-et` | Does not match |
| `amzn-mobi` | Does not match; AZW3 is not MOBI7 |
| `fbc-pdf` | Does not match |
| `amzn-kf8 and not amzn-et` | Matches |
| `not amzn-et` | Matches |

Matching output-specific blocks are selected in stylesheet order before fonts/images
and section page breaks are processed. Nonmatching blocks cannot introduce resources
or override KF8 section-splitting rules. Combined conditions, comma-separated alternatives,
and nested output-specific blocks are supported. Reader-dependent conditions such as
`screen and (min-width: 600px)` remain conditional rather than assuming a device size.

EPUB/KEPUB keep their media queries for the reader. KFX/AZW8 continue to use their
Enhanced Typesetting context; PDF keeps its existing PDF-specific context.

AZW3, KFX/AZW8, and PDF use one media-query evaluator with these unchanged profiles:

| Output | `amzn-kf8` | `amzn-et` | `fbc-pdf` | `screen` | `print` |
|---|---|---|---|---|---|
| AZW3 | true | false | false | true | false |
| KFX/AZW8 | true | true | false | true | false |
| PDF | true | true | true | true | false |

`all` matches every profile and `amzn-mobi` matches none. PDF deliberately uses Kindle/ET
compatibility and screen styling; producing a PDF does not select print media.

Device dimensions, orientation, and similar features are not inferred from the image
screen settings. AZW3 retains unresolved conditions for its reader. KFX/PDF keep their
existing false fallback for unsupported conditions, with ordinary negation still applying.
Debug logs identify the query, unsupported condition, active profile, and actual decision;
KFX/PDF style traces also record unsupported conditions. These diagnostics do not add warnings
or enable a device-specific layout profile.

This does not make the stylesheet parser a browser-complete CSS engine. In particular,
nested at-rules that survive into KFX/PDF stylesheet parsing remain a parser limitation
and are reported at debug level. Existing section-splitting behavior is unchanged.

## Section Splitting

FB2 books have hierarchical sections. Top-level sections are always separate content files in EPUB-like formats. Nested sections normally render inline, but fb2cng can split nested sections into separate files when the stylesheet requests a page break.

Each nested section title wrapper gets a depth-specific class:

- `.section-title-h2`
- `.section-title-h3`
- `.section-title-h4`
- `.section-title-h5`
- `.section-title-h6`

If one of these classes has `page-break-before: always`, sections at that depth are split into separate files.

Default behavior splits depth-2 sections:

```css
.section-title-h2 {
    page-break-before: always;
}
```

Disable nested section splitting:

```css
.section-title-h2 {
    page-break-before: auto;
}
```

Split both depth 2 and depth 3:

```css
.section-title-h2,
.section-title-h3 {
    page-break-before: always;
}
```

Notes:

- Depth-1 sections are always separate files.
- Splitting is recursive.
- TOC hierarchy is preserved even when nested sections are split.

## Fonts

Use `@font-face` for custom fonts.

```css
@font-face {
    font-family: "paragraph";
    src: url("PTSerif-Regular.ttf");
}

@font-face {
    font-family: "paragraph";
    src: url("PTSerif-Bold.ttf");
    font-weight: bold;
}

body {
    font-family: "paragraph";
}
```

Regular TrueType/OpenType files (`.ttf`, `.otf`) are the most portable choice. PDF has stricter font parsing and embedding limits; see [PDF](formats/pdf.md#fonts).

## KFX Body Font Handling

For KFX/AZW8, a bare `body { font-family: "..."; }` rule has special meaning when matching `@font-face` resources exist:

- It sets the Kindle document default font.
- Styles using that family are normalized to Kindle's `default` font keyword.
- Styles without any font family inherit the body font automatically.
- Other font families, such as a dedicated dropcap font, remain separate.

The selector must be exactly `body` and the font family must have resolvable font resources.

## Negative Margins

fb2cng preserves negative CSS margin values where the target format can use them. KFX has one important exception: negative horizontal margins are avoided on dropcap paragraphs because Kindle Previewer 3 / KFX layouts render that combination poorly.

## Resource Path Resolution

Stylesheet URLs can refer to FB2 binary IDs, relative files, or data URLs.

| URL Type | Example | Behavior |
|---|---|---|
| Fragment ID | `url("#font-id")` | Resolves to an FB2 binary object |
| Relative path | `url("fonts/x.ttf")` | Resolves relative to the stylesheet |
| Nested traversal | `url("../fonts/x.ttf")` | Rejected |
| Absolute path | `url("/usr/share/fonts/x.ttf")` | Rejected |
| Data URL | `url("data:font/ttf;base64,...")` | Kept as-is where usable |
| HTTP(S) URL | `url("https://example.com/x.ttf")` | Warned and skipped |

`@import` can load additional stylesheet files, but imported files' own resources are not processed recursively.

## EPUB Resource Organization

EPUB-like outputs place resources by MIME type:

| Resource | EPUB Location |
|---|---|
| Fonts | `OEBPS/fonts/` |
| Images and other resources | `OEBPS/other/` |

Path traversal and absolute paths are rejected before packaging.
