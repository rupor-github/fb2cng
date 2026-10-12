#!/bin/bash

set -eo pipefail

SDKMAN_INIT="${SDKMAN_DIR:-${HOME}/.sdkman}/bin/sdkman-init.sh"
if [ -r "${SDKMAN_INIT}" ]; then
    source "${SDKMAN_INIT}"
fi

if ! command -v java >/dev/null 2>&1; then
    echo "Java not found. Install a Java runtime or configure one with SDKMAN, and ensure java is on PATH." >&2
    exit 1
fi

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd -- "${SCRIPT_DIR}/.." && pwd)"
PROJECTS_ROOT="$(cd -- "${PROJECT_ROOT}/../../.." && pwd)"
BOOK_TESTS_ROOT="${BOOK_TESTS_PATH:-${PROJECTS_ROOT}/book-tests}"

EPUBCHECK_DIR=""
for candidate in \
    "${EPUBCHECK_PATH:-}" \
    "${BOOK_TESTS_ROOT}/epubcheck" \
    "${BOOK_TESTS_ROOT}/EPUBCheck" \
    "${PROJECTS_ROOT}/epubcheck" \
    "${PROJECTS_ROOT}/EPUBCheck" \
    "${HOME}/projects/epubcheck" \
    "${HOME}/projects/EPUBCheck" \
    "${HOME}/epubcheck"; do
    if [ -f "${candidate}/epubcheck.jar" ]; then
        EPUBCHECK_DIR="${candidate}"
        break
    fi
done

if [ -z "${EPUBCHECK_DIR}" ]; then
    echo "Unpack latest epubcheck into ${BOOK_TESTS_ROOT}/epubcheck or set EPUBCHECK_PATH" >&2
    exit 1
fi

java -jar "${EPUBCHECK_DIR}/epubcheck.jar" -p default "$@"
