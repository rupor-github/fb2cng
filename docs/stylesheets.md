## path resoultion

- Default stylesheet resources resolve from current working directory
- FB2 embedded stylesheet resources resolve from FB2 file location
- if you use @import to get other stylesheet files, those files imported
  resources would not be processed recursively!

## URL Resolution

| URL Type | Example | EPUB Location |
|----------|---------|---------------|
| Fragment ID | `#font-id` | `fonts/font-id.woff2` or `other/font-id` |
| Relative path | `fonts/x.woff2` | `fonts/x.woff2` (basename only) |
| Nested path | `../../shared/fonts/x.ttf` | **Rejected — path traversal** |
| Absolute path | `/usr/share/fonts/x.ttf` | **Rejected — absolute path** |
| data: URL | `data:font/...` | Kept as-is (already embedded) |
| HTTP(S) URL | `https://...` | **Warning, skipped** |

## Directory Organization in EPUB

Resources are placed based on MIME type:

### Fonts → `OEBPS/fonts/`
- `font/*` (woff, woff2, ttf, otf)
- `application/font-*`
- `application/x-font-*`
- `application/vnd.ms-fontobject` (EOT)

### Other Resources → `OEBPS/other/`
- `image/*` (including SVG)
- Any other MIME type


## Example:

### config

    document:
      stylesheet_path: "fonts.css"

### fonts.css

    @font-face {
        font-family: "dropcaps";
        src: url("Lombardina-Initial-One.ttf");
    }

    @font-face {
        font-family: "paragraph";
        src: url("PTSerifTTF6.ttf");
    }
    @font-face {
        font-family: "paragraph";
        src: url("PTSerifTTF6-it.ttf");
        font-style: italic;
    }
    @font-face {
        font-family: "paragraph";
        src: url("PTSerifTTF6-bo.ttf");
        font-weight: bold;
    }
    @font-face {
        font-family: "paragraph";
        src: url("PTSerifTTF6-boit.ttf");
        font-style: italic;
        font-weight: bold;
    }
    body {
        font-family:"paragraph";
    }
    p.has-dropcap {
        text-indent: 0;
        margin: 0 0 0.4em 0;
    }

    p.has-dropcap .dropcap {
        font-family: "dropcaps";
        float: left;
        font-size: 3.2em;
        line-height: 1;
        font-weight: bold;
        padding-right: 0.1em;
    }

### directory

     fonts.css
     Lombardina-Initial-One.ttf
     PTSerifTTF6-bo.ttf
     PTSerifTTF6-boit.ttf
     PTSerifTTF6-it.ttf
     PTSerifTTF6.ttf
     config.yaml

