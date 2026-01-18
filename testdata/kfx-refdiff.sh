#!/usr/bin/env bash
set -euo pipefail

OUT_DIR="${OUT_DIR:-/mnt/d}"
REF_DIR="${REF_DIR:-/mnt/d/test}"
FB2="${FB2:-testdata/_Test.fb2}"
CONFIG="${CONFIG:-build/test.yaml}"
WORK_DIR="$(mktemp -d /tmp/fb2cng-kfxdiff-XXXXXX)"
if [[ -z "${KEEP_WORK:-}" ]]; then
	trap 'rm -rf "${WORK_DIR}"' EXIT
else
	echo "[kfx-refdiff] KEEP_WORK set, preserving ${WORK_DIR}"
fi

echo "[kfx-refdiff] work dir: ${WORK_DIR}"
echo "[kfx-refdiff] generating KFX from ${FB2} -> ${OUT_DIR}"
go run cmd/fbc/main.go -d -c "${CONFIG}" convert -ow --to kfx "${FB2}" "${OUT_DIR}"

echo "[kfx-refdiff] dumping ${OUT_DIR}/_Test.kfx"
go run cmd/debug/kfxdump/main.go -overwrite -all "${OUT_DIR}/_Test.kfx"

echo "[kfx-refdiff] running validator"
python3 testdata/input.py "${OUT_DIR}/_Test.kfx" >"${WORK_DIR}/input.log"

diff_file() {
	local label="$1"
	local generated="$2"
	local reference="$3"

	if [[ ! -f "${generated}" ]]; then
		echo "[kfx-refdiff] missing generated ${label}: ${generated}"
		return
	fi
	if [[ ! -f "${reference}" ]]; then
		echo "[kfx-refdiff] missing reference ${label}: ${reference}"
		return
	fi

	local out="${WORK_DIR}/${label}.diff"
	echo "[kfx-refdiff] diffing ${label}"
	if ! diff -u "${generated}" "${reference}" >"${out}"; then
		echo "[kfx-refdiff] differences saved to ${out}"
	else
		echo "[kfx-refdiff] no differences for ${label}"
	fi
}

diff_file "styles" "${OUT_DIR}/_Test-styles.txt" "${REF_DIR}/_Test-kfxout-styles.txt"
diff_file "storyline" "${OUT_DIR}/_Test-storyline.txt" "${REF_DIR}/_Test-kfxout-storyline.txt"
diff_file "dump" "${OUT_DIR}/_Test-dump.txt" "${REF_DIR}/_Test-kfxout-dump.txt"

echo "[kfx-refdiff] validator log: ${WORK_DIR}/input.log"
echo "[kfx-refdiff] done"
