# Template Reference

Many configuration fields ending in `_template` use Go templates plus the slim-sprig helper library.

References:

- Go templates: https://pkg.go.dev/text/template#pkg-overview
- slim-sprig helpers: https://go-task.github.io/slim-sprig

## Common Fields

All template contexts include:

| Field | Meaning |
|---|---|
| `.Context` | Template field name, such as `output_name_template`, `label_template`, or `backlink_template` |
| `.Format` | Requested output format: `epub2`, `epub3`, `kepub`, `kfx`, `azw8`, `pdf`, `txt`, or `md` |

## `output_name_template`

Controls generated output file names.

```yaml
document:
  output_name_template: |
    {{- $prefix := "" -}}
    {{- if gt (len .Authors) 0 -}}
    {{-   with first .Authors -}}
    {{-     $prefix = .LastName -}}
    {{-   end -}}
    {{- end -}}
    {{- if $prefix -}}{{ $prefix }} - {{- end -}}{{ .Title -}}
```

Available fields:

| Field | Meaning |
|---|---|
| `.Title` | Book title |
| `.Authors` | Author objects with `.FirstName`, `.MiddleName`, `.LastName` |
| `.Series` | Series objects with `.Name` and `.Number` |
| `.Language` | Language code |
| `.Date` | Publication date |
| `.SourceFile` | Original file name without path or extension |
| `.BookID` | Book UUID/document ID |
| `.Genres` | Genre names |

## Metadata Templates

`document.metainformation.title_template` formats the title stored in output metadata. It uses the same book metadata fields as `output_name_template`.

`document.metainformation.creator_name_template` formats each author and exposes:

| Field | Meaning |
|---|---|
| `.Index` | 0-based author index |
| `.FirstName` | Author first name |
| `.MiddleName` | Author middle name |
| `.LastName` | Author last name |

Example:

```yaml
document:
  metainformation:
    title_template: |
      {{- if gt (len .Series) 0 -}}
      {{-   with first .Series -}}({{ .Name }} - {{ printf "%02d" .Number }}) {{- end -}}
      {{- end -}}
      {{ .Title }}
    creator_name_template: |
      {{- .LastName -}}
      {{- if .FirstName }}, {{ .FirstName }}{{ end -}}
      {{- if .MiddleName }} {{ .MiddleName }}{{ end -}}
```

## `label_template`

Formats logical footnote labels. See [Footnotes](footnotes.md) for mode-specific behavior.

Available fields:

| Field | Meaning |
|---|---|
| `.BodyTitle` | Title of the footnote body, can be empty |
| `.BodyNumber` | 1-based footnote body index; `0` when the book has only one footnote body |
| `.NoteTitle` | Original footnote section title, can be empty |
| `.NoteNumber` | 1-based sequential number inside the footnote body |

Example:

```yaml
document:
  footnotes:
    label_template: |
      {{- if gt .BodyNumber 0 -}}
      {{-   printf "%d.%d" .BodyNumber .NoteNumber -}}
      {{- else -}}
      {{-   printf "%d" .NoteNumber -}}
      {{- end -}}
```

## `backlink_template`

Formats generated return links from footnotes back to the original reference. See [Footnotes](footnotes.md#backlink_template) for cross-format behavior.

Available fields:

| Field | Meaning |
|---|---|
| `.PageNumber` | Exact PDF page number; approximate/generated page-map number for EPUB/KFX when available |
| `.LocationNumber` | Kindle/KFX location number; for Markdown, rendered Markdown block number containing the original reference anchor |
| `.SectionTitle` | Nearest titled section/body containing the reference |
| `.ChapterTitle` | Spine/chapter title containing the reference |
| `.TargetID` | ID of the footnote section being referenced |
| `.RefID` | Unique generated ID of this reference occurrence |
| `.RefNumber` | 1-based occurrence number for this target footnote |
| `.Href` | Generated return href where applicable |
| `.Filename` | Generated content file containing the reference where applicable |

If the template is empty, invalid, or renders an empty string, fb2cng falls back to `[<]`.

Example:

```yaml
document:
  footnotes:
    backlink_template: |
      {{- if and (eq .Format "pdf") .PageNumber -}}
      {{-   printf "[page %d]" .PageNumber -}}
      {{- else if .LocationNumber -}}
      {{-   printf "[loc %d]" .LocationNumber -}}
      {{- else if .SectionTitle -}}
      {{-   printf "[%s]" .SectionTitle -}}
      {{- else -}}
      {{-   "[<]" -}}
      {{- end -}}
```

Markdown note: in Markdown, `.Href` is also used as the actual Markdown backlink target, so visible template text and clickable destination correspond.

## TOC Page Author Template

`document.toc_page.authors_template` formats author text on the optional visible TOC page.

Use it when you want a visible table of contents page to display author names differently from metadata.
