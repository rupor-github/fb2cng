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

How this template works:

- `{{ ... }}` marks a Go template action. Text outside actions is copied to the result.
- The `-` in `{{-` or `-}}` trims whitespace next to the action. This keeps generated file names from accumulating newlines and indentation from the YAML block.
- `$prefix := ""` creates a template variable named `$prefix` and initializes it to an empty string.
- `len .Authors` returns the number of authors in the book metadata.
- `gt (len .Authors) 0` means "greater than zero". The surrounding `if` checks whether at least one author exists.
- `first .Authors` is a slim-sprig helper that returns the first author object from the list.
- `with first .Authors` changes the current dot (`.`) to that first author for the duration of the block, so `.LastName` means the first author's last name inside the block.
- `$prefix = .LastName` assigns the first author's last name to the previously created `$prefix` variable.
- `if $prefix` checks whether `$prefix` is non-empty. If it is, the template writes the prefix followed by ` - `.
- `.Title` writes the book title.

In plain English, the sample means: "If the book has at least one author, start the file name with the first author's last name and a separator, then append the book title."

Advanced example with first author name parts and language-specific "et al" text:

```yaml
document:
  output_name_template: |
    {{- $all := "" -}}
    {{- if gt (len .Authors) 0 -}}
    {{-   with first .Authors -}}
    {{-     $all = .LastName -}}
    {{-     if .FirstName }}{{ $all = (cat $all .FirstName) }}{{- end -}}
    {{-     if .MiddleName }}{{ $all = (cat $all .MiddleName) }}{{- end -}}
    {{-   end -}}
    {{-   if gt (len .Authors) 1 -}}
    {{-     if eq .Language "ru" }}{{ $all = (cat $all "и др") }}{{- else -}}{{ $all = (printf "%s%s" $all ", et al") }}{{- end -}}
    {{-   end -}}
    {{-   $all = cat $all "-" -}}
    {{- end -}}
    {{- if $all -}}
    {{-   cat $all .Title -}}
    {{- else -}}
    {{-   .Title -}}
    {{- end -}}
```

How the advanced template works:

- `$all := ""` creates an accumulator variable that will eventually contain the author prefix.
- `if gt (len .Authors) 0` runs the author-prefix logic only when the book has at least one author.
- `with first .Authors` selects the first author and makes that author the current dot (`.`).
- `$all = .LastName` starts the prefix with the first author's last name.
- `if .FirstName` checks whether the first author has a first name. If yes, `$all = (cat $all .FirstName)` appends it.
- `if .MiddleName` does the same for the middle name.
- `cat` is a slim-sprig helper that concatenates values with spaces between arguments. So `cat "Иванов" "Иван"` produces `Иванов Иван`.
- After the `with` block ends, dot (`.`) returns to the full book metadata context.
- `if gt (len .Authors) 1` checks whether there is more than one author.
- `eq .Language "ru"` checks whether the book language is Russian.
- For Russian books, `cat $all "и др"` appends `и др` to mean "and others".
- For non-Russian books, `printf "%s%s" $all ", et al"` appends `, et al` without inserting an extra space before the comma. `printf` is used here instead of `cat` because `cat` always inserts spaces between arguments.
- `$all = cat $all "-"` appends a hyphen separator after the author prefix.
- The final `if $all` chooses between two outputs: if an author prefix exists, concatenate it with the title; otherwise output only `.Title`.

In plain English, the advanced sample means: "Build a file name from the first author's last/first/middle name, add `и др` or `, et al` when there are multiple authors, add a hyphen separator, and then append the book title. If there are no authors, use only the title."

Advanced example using a list of filename parts:

```yaml
document:
  output_name_template: |
    {{- $parts := list -}}
    {{- if gt (len .Authors) 0 -}}
    {{-   with first .Authors -}}
    {{-     $author := .LastName -}}
    {{-     if .FirstName -}}
    {{-       $author = printf "%s %s." $author (first (splitList "" .FirstName)) -}}
    {{-     end -}}
    {{-     $parts = append $parts $author -}}
    {{-   end -}}
    {{- end -}}
    {{- if gt (len .Series) 0 -}}
    {{-   with first .Series -}}
    {{-     if gt .Number 0 -}}
    {{-       $parts = append $parts (printf "%02d" .Number) -}}
    {{-     end -}}
    {{-   end -}}
    {{- end -}}
    {{- $parts = append $parts .Title -}}
    {{- printf "%s_%s" (join " " $parts) .SourceFile -}}
```

How this template works:

- `$parts := list` creates an empty list. This template builds the file name by appending pieces to that list.
- `if gt (len .Authors) 0` checks whether any authors exist.
- `with first .Authors` selects the first author and makes that author the current dot (`.`).
- `$author := .LastName` creates a local author string initialized to the first author's last name.
- `if .FirstName` checks whether the first author has a first name.
- `splitList "" .FirstName` splits the first name into individual characters.
- `first (splitList "" .FirstName)` takes the first character, effectively creating an initial.
- `printf "%s %s." $author ...` formats the last name plus first initial, such as `Иванов И.` or `Doe J.`.
- `$parts = append $parts $author` appends the author string to the `$parts` list.
- After the author block, `if gt (len .Series) 0` checks whether the book has series metadata.
- `with first .Series` selects the first series entry.
- `if gt .Number 0` checks whether the series number is present and greater than zero.
- `printf "%02d" .Number` formats the series number as two digits, such as `01`, `02`, or `12`.
- `$parts = append $parts ...` appends that formatted series number to the filename parts.
- `$parts = append $parts .Title` always appends the book title.
- `join " " $parts` joins all accumulated parts with spaces.
- `printf "%s_%s" ... .SourceFile` appends an underscore and the original source file base name. This can help keep names unique when metadata is duplicated.

In plain English, this sample means: "Build a file name from first-author last name and initial, optional two-digit series number, book title, and original source filename."

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

How the `title_template` works:

- `if gt (len .Series) 0` checks whether the book has at least one series entry.
- `with first .Series` switches the current dot (`.`) to the first series entry.
- `{{ .Name }}` writes that series name.
- `printf "%02d" .Number` formats the series number as a two-digit integer. For example, `1` becomes `01` and `12` stays `12`.
- The literal punctuation outside actions, such as `(`, ` - `, `)`, and spaces, is copied directly to the output.
- After the series prefix, `{{ .Title }}` writes the original book title from the metadata context.

In plain English, the title sample means: "If the book belongs to a series, prefix the title with `(Series Name - 01)`, then write the book title."

How the `creator_name_template` works:

- `.LastName`, `.FirstName`, and `.MiddleName` come from the current author. This template is executed once per author.
- `{{- .LastName -}}` writes the last name and trims surrounding whitespace.
- `if .FirstName` writes the comma and first name only when a first name is present.
- `if .MiddleName` writes the middle name only when a middle name is present.

In plain English, the author sample means: "Render each author as `LastName, FirstName MiddleName`, omitting missing name parts."

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

How this template works:

- `.BodyNumber` is the logical footnote body number. It is `0` when the book has only one footnote body, so single-body books do not need a body prefix.
- `.NoteNumber` is the sequential number of the note inside that body.
- `if gt .BodyNumber 0` checks whether a body number should be shown.
- `printf "%d.%d" .BodyNumber .NoteNumber` formats two integers separated by a dot, such as `2.17`.
- The `else` branch is used when `.BodyNumber` is `0`.
- `printf "%d" .NoteNumber` formats just the note number, such as `17`.

In plain English, the sample means: "Use `body.note` numbering when there are multiple footnote bodies; otherwise use just the note number."

PDF-aware example that uses note titles when they look meaningful:

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

How this template works:

- `if eq .Format "pdf"` creates a separate branch for PDF output.
- `default "Notes" .BodyTitle` is a slim-sprig helper. It returns `.BodyTitle` when it is non-empty; otherwise it returns `Notes`.
- `$bodyTitle := ...` stores that result in a variable so it can be reused.
- `trim .NoteTitle` removes leading and trailing whitespace from the original note title.
- `$noteNumber := printf "%d" .NoteNumber` converts the numeric note number to a string.
- `$label := ""` creates a variable that will hold the generated numeric label.
- `if gt .BodyNumber 0` checks whether the note belongs to one of multiple footnote bodies.
- `printf "%d.%s" .BodyNumber $noteNumber` builds a body-prefixed label such as `2.17`.
- If `.BodyNumber` is `0`, `$label = $noteNumber` uses only the note number, such as `17`.
- `and $noteTitle (regexMatch "[^0-9[:space:]]" $noteTitle)` checks two things: the note title is non-empty, and it contains at least one character that is not a digit or whitespace.
- `regexMatch "[^0-9[:space:]]" $noteTitle` treats titles like `17` or ` 17 ` as plain numbering, but titles like `Translator note` as meaningful text.
- If the title looks meaningful, `printf "%s: %s" $bodyTitle $noteTitle` renders something like `Notes: Translator note`.
- Otherwise, `printf "%s: %s" $bodyTitle $label` renders something like `Notes: 17` or `Comments: 2.17`.
- The final `else` branch is for non-PDF formats.
- In the non-PDF branch, `printf "%d" .BodyNumber` writes the body number only when it is greater than zero, then a literal dot is emitted outside the template action.
- `printf "%d" .NoteNumber` writes the note number.

In plain English, this sample means: "For PDF, make printed footnote title text from the footnote body title plus either a meaningful source note title or a generated numeric label. For non-PDF formats, use compact numeric labels like `17` or `2.17`."

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

How this template works:

- `and (eq .Format "pdf") .PageNumber` combines two checks. It is true only when the output format is PDF and `.PageNumber` is non-zero.
- `eq .Format "pdf"` compares the current output format with the string `pdf`.
- `.PageNumber` by itself is treated as false when it is `0` and true when it is non-zero.
- `printf "[page %d]" .PageNumber` formats a backlink label such as `[page 23]`.
- `else if .LocationNumber` is used when the PDF page condition did not match but a location number is available.
- `printf "[loc %d]" .LocationNumber` formats a label such as `[loc 183]`.
- `else if .SectionTitle` uses the nearest section title when no page or location number is available.
- `printf "[%s]" .SectionTitle` formats that string inside square brackets.
- The final `else` branch returns the compact fallback marker `[<]`.

In plain English, the sample means: "Prefer exact PDF page numbers, otherwise use a generated location, otherwise use the source section title, otherwise use a generic back marker."

Markdown note: in Markdown, `.Href` is also used as the actual Markdown backlink target, so visible template text and clickable destination correspond.

## TOC Page Author Template

`document.toc_page.authors_template` formats author text on the optional visible TOC page.

Use it when you want a visible table of contents page to display author names differently from metadata.

Example:

```yaml
document:
  toc_page:
    authors_template: |
      {{- $names := list -}}
      {{- range .Authors -}}
      {{-   $name := list -}}
      {{-   if .FirstName -}}{{ $name = append $name .FirstName -}}{{- end -}}
      {{-   if .MiddleName -}}{{ $name = append $name .MiddleName -}}{{- end -}}
      {{-   if .LastName -}}{{ $name = append $name .LastName }}{{- end -}}
      {{-   $names = append $names (join " " $name) -}}
      {{- end -}}
      {{- join ", " $names -}}
```

How this template works:

- `$names := list` creates an empty list that will hold one formatted string per author.
- `range .Authors` loops over all authors. Inside the `range` block, dot (`.`) is the current author.
- `$name := list` creates a fresh empty list for the current author's name parts.
- `if .FirstName` checks whether the current author has a first name. If yes, `append $name .FirstName` adds it to the current author's name-part list.
- `if .MiddleName` does the same for the middle name.
- `if .LastName` does the same for the last name.
- `join " " $name` joins the current author's available name parts with spaces. Missing parts are skipped, so there are no doubled spaces.
- `$names = append $names (join " " $name)` appends the completed current-author string to the outer `$names` list.
- After `end`, the loop is finished and `$names` contains all formatted authors.
- `join ", " $names` joins all formatted authors with comma-space separators.

In plain English, this sample means: "For each author, join the available first, middle, and last name parts with spaces, then join all authors with commas for display on the visible TOC page."
