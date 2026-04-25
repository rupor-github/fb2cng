// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package document

import (
	"github.com/carlos7ags/folio/core"
)

// recompressStreams walks every registered indirect object and, for
// each PdfStream payload deemed eligible, re-Flate-compresses the
// payload at zlib.BestCompression. The size-regression guard reverts
// any rewrite that fails to produce strictly smaller output.
//
// Eligibility (see WriteOptions.RecompressStreams godoc for the
// canonical contract):
//
//   - Empty /Filter: payload is plaintext, deflate it.
//   - Single /FlateDecode filter: inflate then re-deflate.
//   - Anything else: skip.
//
// The pass is encryption-agnostic at this layer; WriteToWithOptions
// refuses RecompressStreams when an encryptor is configured. The
// guard at the top of this method is defense in depth so a future
// caller that bypasses WriteToWithOptions cannot corrupt encrypted
// payloads.
//
// The pass never enlarges any stream and never invalidates any
// reference, so it composes safely with OrphanSweep, UseXRefStream,
// and UseObjectStreams.
func (w *Writer) recompressStreams() {
	if w.encryptor != nil {
		return
	}
	for _, obj := range w.objects {
		stream, ok := obj.Object.(*core.PdfStream)
		if !ok {
			continue
		}
		recompressOne(stream)
	}
}

// recompressOne attempts to recompress a single stream's payload.
// Mutates the stream in place on success; leaves it untouched on
// failure or when the guard reverts.
func recompressOne(stream *core.PdfStream) {
	if stream.WillCompress() {
		// Stream is already marked for FlateDecode at WriteTo time
		// (via NewPdfStreamCompressed / SetCompress(true)). Folio's
		// writer always uses BestCompression for those, so re-running
		// here would just duplicate work for identical output. Skip.
		return
	}
	mode, ok := classifyForRecompress(stream)
	if !ok {
		return
	}

	original := stream.Data
	if len(original) == 0 {
		// Nothing to compress.
		return
	}

	// Decode to plaintext according to the eligibility class. The
	// raw branch is a no-op (plaintext is the data); the FlateDecode
	// branch inflates. A decode error is treated as "not eligible
	// after all" — leave the stream alone rather than fail the whole
	// write.
	var plaintext []byte
	switch mode {
	case recompressRaw:
		plaintext = original
	case recompressFlate:
		decoded, err := core.InflateStreamData(original)
		if err != nil {
			return
		}
		plaintext = decoded
	}

	// Run the candidate transform under the size-regression guard.
	// The guard returns the smaller of (baseline, candidate); if it
	// returns the baseline, no commit. We compare lengths to detect
	// the commit decision because the helper does not surface a
	// boolean flag.
	candidate, err := sizeRegressionGuard(original, func() ([]byte, error) {
		return core.DeflateStreamData(plaintext)
	})
	if err != nil {
		// Candidate transform failed; guard already fell back. Leave
		// the stream untouched.
		return
	}
	if len(candidate) >= len(original) {
		// Guard preserved the original — no shrink. Skip commit.
		return
	}

	// Commit the rewrite. The new payload is FlateDecode-encoded
	// without a predictor or any other parameterized post-processing,
	// so /Filter must be /FlateDecode and /DecodeParms must be absent.
	// classifyForRecompress refuses to mark a Flate-with-/DecodeParms
	// stream eligible (§7.4.4.4), so the Remove call below is
	// defensive cleanup against a /DecodeParms set on what was a raw
	// (no-/Filter) stream — meaningless there, but stripping it is
	// harmless and keeps the dictionary tidy.
	stream.Data = candidate
	stream.SetCompress(false) // payload is already compressed; do not re-deflate on WriteTo
	stream.Dict.Set("Filter", core.NewPdfName("FlateDecode"))
	stream.Dict.Remove("DecodeParms")
	// /Length is reset by PdfStream.WriteTo from len(stream.Data).
}

// recompressMode names the eligibility class of a stream — primarily
// to distinguish "data is already plaintext" from "data needs inflate
// first" without a second filter inspection at decode time.
type recompressMode int

const (
	recompressRaw   recompressMode = iota // payload is raw bytes (no /Filter)
	recompressFlate                       // payload is FlateDecode-encoded
)

// classifyForRecompress returns the recompression class of a stream
// and whether the stream is eligible at all. Streams whose filter
// chain includes a specialized-compression filter (DCT/JPX/CCITT/JBIG2)
// or any other unsupported filter are not eligible. A malformed
// /Filter entry (e.g., an array containing a non-name) is treated as
// not eligible — better to leave the stream untouched than to assume
// "no filter" against a value that is clearly something else.
//
// FlateDecode streams that carry a /DecodeParms entry (most commonly
// a PNG or TIFF predictor per ISO 32000-1 §7.4.4.4) are also not
// eligible. The inflated payload of such a stream is predictor-
// filtered bytes, not plaintext; re-deflating those bytes and
// dropping /DecodeParms would emit a stream that decodes to garbage
// in any reader that honors the predictor contract. Skipping is the
// only safe call without re-applying the predictor inversion, which
// is out of scope for this pass.
func classifyForRecompress(stream *core.PdfStream) (recompressMode, bool) {
	filterObj := stream.Dict.Get("Filter")
	if filterObj == nil {
		return recompressRaw, true
	}
	filters, ok := filterChainNames(filterObj)
	if !ok {
		return 0, false
	}
	if len(filters) == 1 && filters[0] == "FlateDecode" {
		// /DecodeParms (§7.4.4.4) on a Flate stream signals that a
		// predictor (or other parameterized post-processing) was
		// applied to the bytes BEFORE Flate. Inflate alone does not
		// undo the predictor, so we cannot safely re-deflate.
		if stream.Dict.Get("DecodeParms") != nil {
			return 0, false
		}
		return recompressFlate, true
	}
	return 0, false
}

// filterChainNames extracts the filter names from a /Filter entry,
// which per ISO 32000-1 §7.4.2 may be either a single name or an
// array of names. Returns ok=false for any value that does not match
// one of those two shapes (or an array element that is not a name);
// callers must distinguish "no /Filter set" from "malformed /Filter"
// before invoking this helper.
func filterChainNames(obj core.PdfObject) ([]string, bool) {
	if name, ok := obj.(*core.PdfName); ok {
		return []string{name.Value}, true
	}
	arr, ok := obj.(*core.PdfArray)
	if !ok {
		return nil, false
	}
	out := make([]string, 0, arr.Len())
	for _, elem := range arr.All() {
		name, ok := elem.(*core.PdfName)
		if !ok {
			return nil, false
		}
		out = append(out, name.Value)
	}
	return out, true
}
