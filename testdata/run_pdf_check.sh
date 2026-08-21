#!/bin/bash

set -euo pipefail

require_metadata=0
files=()

usage() {
    cat <<'EOF'
Usage: testdata/run_pdf_check.sh [--require-metadata] FILE.pdf [...]

Runs qpdf structural validation and pdfinfo metadata inspection.

Options:
  --require-metadata  fail unless pdfinfo reports an XMP metadata stream
EOF
}

print_install_hint() {
    local tool="$1"

    echo "Missing required tool: ${tool}" >&2
    echo >&2
    case "$tool" in
        qpdf)
            cat >&2 <<'EOF'
Install package:

Examples:
  Debian/Ubuntu: sudo apt install qpdf
  Fedora:        sudo dnf install qpdf
  Arch:          sudo pacman -S qpdf
  openSUSE:      sudo zypper install qpdf
EOF
            ;;
        pdfinfo)
            cat >&2 <<'EOF'
Install package that provides pdfinfo:

Examples:
  Debian/Ubuntu: sudo apt install poppler-utils
  Fedora:        sudo dnf install poppler-utils
  Arch:          sudo pacman -S poppler
  openSUSE:      sudo zypper install poppler-tools
EOF
            ;;
        *)
            echo "Install package that provides ${tool}." >&2
            ;;
    esac
}

require_tool() {
    local tool="$1"

    if ! command -v "$tool" >/dev/null 2>&1; then
        print_install_hint "$tool"
        exit 1
    fi
}

while [ "$#" -gt 0 ]; do
    case "$1" in
        --require-metadata)
            require_metadata=1
            ;;
        -h|--help)
            usage
            exit 0
            ;;
        --)
            shift
            while [ "$#" -gt 0 ]; do
                files+=("$1")
                shift
            done
            break
            ;;
        -* )
            echo "Unknown option: $1" >&2
            usage >&2
            exit 2
            ;;
        *)
            files+=("$1")
            ;;
    esac
    shift
done

if [ "${#files[@]}" -eq 0 ]; then
    usage >&2
    exit 2
fi

require_tool qpdf
require_tool pdfinfo

for file in "${files[@]}"; do
    if [ ! -f "$file" ]; then
        echo "PDF file not found: $file" >&2
        exit 1
    fi

    echo "Checking PDF: $file"
    qpdf --check "$file"

    info="$(pdfinfo "$file")"
    printf '%s\n' "$info"

    if [ "$require_metadata" -eq 1 ] && ! grep -qx 'Metadata Stream: yes' <<<"$info"; then
        echo "Missing XMP metadata stream: $file" >&2
        exit 1
    fi
done
