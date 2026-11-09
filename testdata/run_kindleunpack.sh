#!/bin/bash

set -euo pipefail

usage() {
    cat <<'EOF'
Usage: testdata/run_kindleunpack.sh [KINDLEUNPACK_OPTIONS...] INPUT.mobi [OUTPUT_DIR]

Runs KindleUnpack to inspect and reconstruct unencrypted MOBI/AZW3 books.
Successful unpacking alone does not prove rendering compatibility.

Example:
  testdata/run_kindleunpack.sh -r -s -d reference.mobi unpacked/

Options are passed directly to KindleUnpack, before the input filename:
  -r                write raw data
  -s                split combo books into standalone MOBI7 and AZW3 files
  -d                dump headers and other diagnostic information
  -p APNX_FILE      use an associated APNX page map
  --epub_version=V  reconstructed EPUB version: 2, 3, A, or F

If OUTPUT_DIR is omitted, KindleUnpack uses the input path without its extension.

Environment:
  KINDLEUNPACK_PATH  checkout directory containing lib/kindleunpack.py
  BOOK_TESTS_PATH    tools directory; defaults to ../../../book-tests from repo root

Requires python3 on PATH. Tool arguments, output, and exit status are not
modified; KindleUnpack diagnostics still need inspection even on success.
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
KINDLEUNPACK_ROOT="${KINDLEUNPACK_PATH:-${BOOK_TESTS_ROOT}/KindleUnpack}"
KINDLEUNPACK="${KINDLEUNPACK_ROOT}/lib/kindleunpack.py"

if [ ! -f "${KINDLEUNPACK}" ] || [ ! -r "${KINDLEUNPACK}" ]; then
    echo "KindleUnpack entry point not found or not readable: ${KINDLEUNPACK}" >&2
    echo "Place the full checkout at ${BOOK_TESTS_ROOT}/KindleUnpack or set KINDLEUNPACK_PATH." >&2
    exit 1
fi

if ! command -v python3 >/dev/null 2>&1; then
    echo "Python 3 not found. Install Python 3 and ensure python3 is on PATH." >&2
    exit 1
fi

echo "Using KindleUnpack: ${KINDLEUNPACK}" >&2
exec python3 "${KINDLEUNPACK}" "$@"
