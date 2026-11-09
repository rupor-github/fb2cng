#!/bin/bash

set -euo pipefail

usage() {
    cat <<'EOF'
Usage: testdata/run_kindlegen.sh INPUT.epub [KINDLEGEN_OPTIONS...]

Runs KindleGen to generate reference combo MOBI files, not to validate existing
MOBI/AZW3 files. OPF and HTML inputs are also supported.

Example:
  testdata/run_kindlegen.sh book.epub -c0 -o reference.mobi

Options are passed directly to KindleGen:
  -c0       no compression (useful for inspecting reference files)
  -c1       PalmDOC compression
  -c2       HUFF/CDIC compression
  -verbose  detailed compiler diagnostics
  -o NAME   output basename, written beside the input (not a destination path)

Environment:
  KINDLEGEN_PATH   executable path or directory containing kindlegen
  BOOK_TESTS_PATH  tools directory; defaults to ../../../book-tests from repo root

Exit status is preserved: 0 = success, 1 = warnings, 2 = compiler error.
EOF
}

if [ "$#" -eq 0 ]; then
    usage >&2
    exit 2
fi

if [ "$#" -eq 1 ] && { [ "$1" = "-h" ] || [ "$1" = "--help" ]; }; then
    usage
    exit 0
fi

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd -- "${SCRIPT_DIR}/.." && pwd)"
PROJECTS_ROOT="$(cd -- "${PROJECT_ROOT}/../../.." && pwd)"
BOOK_TESTS_ROOT="${BOOK_TESTS_PATH:-${PROJECTS_ROOT}/book-tests}"
KINDLEGEN="${KINDLEGEN_PATH:-${BOOK_TESTS_ROOT}/kindlegen/kindlegen}"

if [ -d "${KINDLEGEN}" ]; then
    KINDLEGEN="${KINDLEGEN}/kindlegen"
fi

if [ ! -f "${KINDLEGEN}" ] || [ ! -x "${KINDLEGEN}" ]; then
    echo "KindleGen executable not found or not executable: ${KINDLEGEN}" >&2
    echo "Place the Linux binary at ${BOOK_TESTS_ROOT}/kindlegen/kindlegen or set KINDLEGEN_PATH." >&2
    exit 1
fi

echo "Using KindleGen: ${KINDLEGEN}" >&2
exec "${KINDLEGEN}" "$@"
