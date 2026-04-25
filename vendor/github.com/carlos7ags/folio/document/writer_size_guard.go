// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package document

import "errors"

// sizeRegressionGuard runs an optimization pass and only commits its
// output when the result is strictly smaller than the baseline. The
// guard is the cross-cutting primitive that lets callers opt into
// transformations that *might* enlarge the file (notably Flate
// recompression of payloads that were already well-compressed, and
// future lossy passes that change pixel data) without risking a
// regression on inputs the pass does not help.
//
// The semantics are:
//
//   - candidate is invoked once. Its byte output and any error are
//     captured.
//   - If candidate returns an error, baseline is returned with the
//     candidate's error wrapped via errors.Join so the caller can
//     observe both the fallback and the underlying failure.
//   - If candidate succeeds and produces fewer bytes than baseline,
//     the candidate output is returned.
//   - Otherwise baseline is returned. The "otherwise" branch covers
//     equal-size outputs deliberately: zero-byte savings do not justify
//     the perturbation of a stable byte-identical artifact.
//
// The guard does not invoke the baseline producer — the caller passes
// in already-serialized baseline bytes. This keeps the contract small
// (no double-running expensive transforms when the candidate succeeds)
// and makes the helper trivially testable.
func sizeRegressionGuard(baseline []byte, candidate func() ([]byte, error)) ([]byte, error) {
	candBytes, candErr := candidate()
	if candErr != nil {
		return baseline, errors.Join(errSizeGuardCandidateFailed, candErr)
	}
	if len(candBytes) < len(baseline) {
		return candBytes, nil
	}
	return baseline, nil
}

// errSizeGuardCandidateFailed is the sentinel wrapped into the error
// returned by sizeRegressionGuard when the candidate transform itself
// errored. Callers can errors.Is against it to distinguish a true
// candidate failure from a no-op fallback (which returns nil error).
var errSizeGuardCandidateFailed = errors.New("document: size-regression guard fell back to baseline")
