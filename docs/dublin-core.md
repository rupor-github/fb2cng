# Dublin Core Metadata Expansion

This note summarizes metadata expansion for EPUB2, EPUB3, KFX/AZW8, PDF, and Markdown output from parsed FB2 metadata.

## Source Metadata

FB2 parsing already keeps these useful fields:

| FB2 block | Fields |
|---|---|
| `title-info` | genres, authors, book title, annotation, keywords, date, language, source language, translators, sequences |
| `src-title-info` | original/source title metadata with same shape as `title-info` |
| `document-info` | FB2 document authors, program used, document date, source URLs, source OCR, ID, version, history, publishers |
| `publish-info` | printed title, publisher, city, year, ISBN, sequences |
| `custom-info` | typed free-form values |

## EPUB2 And EPUB3

EPUB uses Dublin Core metadata in `content.opf`. EPUB2 and EPUB3 share the basic `dc:*` elements, but differ in how richer semantics are represented.

Current core metadata already maps title, UUID, language, authors, genres, annotation, cover, and series metadata.

Recommended minimal expansion:

| FB2 source | OPF metadata | Rule |
|---|---|---|
| `TitleInfo.Date` | `dc:date` | Prefer machine date `YYYY-MM-DD`; otherwise normalize display text only when it can be converted to W3CDTF syntax. |
| `PublishInfo.Year` | `dc:date` | Use only when `TitleInfo.Date` is missing and year/date syntax is valid. |
| `PublishInfo.Publisher` | `dc:publisher` | Emit when non-empty. |
| `TitleInfo.Translators` | `dc:contributor` | Mark role as translator. |
| `TitleInfo.Keywords` | additional `dc:subject` | Split comma/semicolon-separated values; keep genres too. |
| `PublishInfo.ISBN` | extra `dc:identifier` | Emit alongside FB2 document UUID, not instead of it. |

Translator names should use the same `document.metainformation.creator_name_template` and `transliterate` settings as author metadata. The template controls only displayed name text; role metadata still marks translators as `trl`. Template `Index` should be relative to the translator list.

Format-specific representation:

| Semantics | EPUB2 | EPUB3 |
|---|---|---|
| author role | `dc:creator opf:role="aut"` | `dc:creator id="creatorN"` plus `meta refines="#creatorN" property="role" scheme="marc:relators">aut</meta>` |
| translator role | `dc:contributor opf:role="trl"` | `dc:contributor id="translatorN"` plus refined `role=trl` meta |
| publication date | `dc:date`; optionally `opf:event="publication"` | `dc:date`; `dcterms:modified` remains generated package-modified timestamp |
| ISBN | `dc:identifier opf:scheme="ISBN"` | `dc:identifier id="isbn"`; if ISBN-10/13 can be detected, add `meta refines="#isbn" property="identifier-type" scheme="onix:codelist5"` with `02` for ISBN-10 or `15` for ISBN-13 |
| FB2 genre subject | plain `dc:subject` plus `meta name="fb2:genre" content="<genre>"` | `dc:subject id="subject-genre-N"` plus refined `authority=FB2` and `term=<genre>` meta |
| keyword subject | plain `dc:subject` | plain `dc:subject` |
| series | existing calibre meta for first sequence | existing `belongs-to-collection`, `collection-type`, `group-position` meta |

Invalid date values should be skipped and logged at debug level where a logger is available. If `TitleInfo.Date` exists but cannot be normalized, do not fall back to `PublishInfo.Year`; the explicit book date wins even when malformed.

ISBN handling should validate checksums before emitting type-specific metadata. Strip spaces and hyphens first. ISBN-10 uses 9 digits plus a digit or `X` check character and modulo-11 checksum. ISBN-13 uses 13 digits and alternating `1,3` checksum weights. Emit generic ISBN metadata only for checksum-valid values; emit ISBN-10/ISBN-13-specific metadata only when the valid type is known. Invalid ISBN values should be skipped and logged at debug level where a logger is available.

## KFX/AZW8

KFX has no Dublin Core layer. Book-level values are stored as `kindle_title_metadata` key/value pairs.

Current KFX metadata already writes ASIN/content IDs, content type, cover image, sample/font flags, title, authors, language, publisher, and description.

Recommended expansion:

| FB2 source | KFX key | Rule |
|---|---|---|
| `TitleInfo.Date` | `issue_date` | Prefer machine date `YYYY-MM-DD`; otherwise normalize display text only when it can be converted to date syntax. |
| `PublishInfo.Year` | `issue_date` | Use only when `TitleInfo.Date` is missing and year/date syntax is valid. |
| `PublishInfo.ISBN` | `ISBN` | Emit raw ISBN when non-empty. |
| normalized ISBN-10/13 | `ISBN-10`, `ISBN-13` | Optional; only emit when format is obvious and validated. |

Avoid native KFX metadata for translators, subjects, series, source title, and source URLs unless a concrete Kindle consumer is known. Kindle Previewer and KFXInput/KFXOutput expose no stable native keys for those fields; unknown keys are likely ignored.

## PDF

PDF keeps legacy document metadata in the Info dictionary and richer metadata in an XMP metadata stream referenced from the catalog.

Info dictionary metadata remains intentionally small for broad reader compatibility:

| FB2 source | PDF Info key |
|---|---|
| metadata title template or `TitleInfo.BookTitle` | `Title` |
| `TitleInfo.Authors` | `Author` |
| `TitleInfo.Annotation` | `Subject` |
| `TitleInfo.Keywords` | `Keywords` |
| generator | `Creator`, `Producer` |

XMP metadata uses Dublin Core and PDF/XMP fields:

| FB2 source | XMP metadata |
|---|---|
| metadata title template or `TitleInfo.BookTitle` | `dc:title` |
| `TitleInfo.Authors` | `dc:creator` |
| `TitleInfo.Translators` | `dc:contributor` |
| `TitleInfo.Annotation` | `dc:description` |
| `TitleInfo.Genres` plus `TitleInfo.Keywords` | `dc:subject` |
| `PublishInfo.Publisher` | `dc:publisher` |
| normalized `TitleInfo.Date`, fallback `PublishInfo.Year` | `dc:date` |
| `DocumentInfo.ID`, validated `PublishInfo.ISBN` | `dc:identifier` |
| `TitleInfo.Lang` | `dc:language` |
| generator | `xmp:CreatorTool`, `pdf:Producer` |
| `TitleInfo.Keywords` | `pdf:Keywords` |

PDF author and translator names use the same `document.metainformation.creator_name_template` and `transliterate` settings as EPUB/KFX/Markdown metadata.

## Markdown Front Matter

Markdown already emits YAML front matter with title, authors, series, language, date, and genres when available. It also applies metadata title/creator templates like EPUB/KFX.

Possible front matter expansion, aligned with EPUB where practical:

| FB2 source | YAML key | Rule |
|---|---|---|
| `TitleInfo.Annotation` | `description` | Plain text annotation. |
| `PublishInfo.Publisher` | `publisher` | String. |
| `TitleInfo.Translators` | `translators` | List of formatted names. |
| `TitleInfo.Keywords` | `subjects` or `keywords` | Prefer one list; include split keywords and/or genres deliberately. |
| `PublishInfo.ISBN` | `isbn` | String. |
| `DocumentInfo.ID` | `identifier` | FB2 UUID, useful for traceability. |
| `SrcTitleInfo.BookTitle` | `source_title` | Optional original/source title. |
| `TitleInfo.SrcLang` | `source_language` | Optional when not `und`. |

Keep Markdown front matter readable and stable. Avoid dumping `document-info`, `history`, `custom-info`, and output/share rules unless a dedicated provenance section is needed.
