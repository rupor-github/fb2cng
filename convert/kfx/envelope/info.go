package envelope

import (
	"fmt"
	"strings"
)

// MaxDecodedSize bounds whole-stream buffering for both writer and reader (1 GiB).
const MaxDecodedSize uint64 = 1 << 30

// PageInfo describes a stored page and its range in the decoded stream.
type PageInfo struct {
	Number        int
	DecodedOffset uint64
	DecodedSize   uint64
	StoredSize    uint64
	Filter        byte
	Properties    byte
	Dictionary    uint32
}

// Info describes outer envelope layout; stored offsets exclude the eight-byte magic.
type Info struct {
	Mode        Mode
	EncodedSize uint64
	DecodedSize uint64
	PageSize    uint64
	FrameOffset uint64
	FrameLength uint64
	IndexOffset uint64
	IndexLength uint64
	Pages       []PageInfo
}

// Summary returns a concise file-level summary distinct from inner CONT compression.
func (i Info) Summary() string {
	savings := 100 * (1 - float64(i.EncodedSize)/float64(i.DecodedSize))
	compression := string(i.Mode)
	if i.Mode == Plaintext {
		compression = "none"
	}
	return fmt.Sprintf(
		"KFX envelope: ProtectedData@2.0, unencrypted, compression=%s, pages=%d, page_size=%d, inner=%d, outer=%d, savings=%.2f%%",
		compression, len(i.Pages), i.PageSize, i.DecodedSize, i.EncodedSize, savings,
	)
}

// String returns detailed layout and codec information without binary page payloads.
func (i Info) String() string {
	var b strings.Builder
	fmt.Fprintln(&b, i.Summary())
	fmt.Fprintln(&b, "Shared symbols: ProtectedData v1, max_id=64 (absolute SIDs 10-73)")
	fmt.Fprintln(&b, "Records: Envelope@2.0, EnvelopeMetadata@2.0, PlainText@2.0, EnvelopeIndexTable@1.0")
	if i.Mode == LZMA {
		fmt.Fprintln(&b, "Page codec: Compressed@1.0, classic LZMA1, explicit size, EOS marker required")
	}
	fmt.Fprintln(&b, "Offsets: stored positions exclude the 8-byte DRMION magic; physical positions include it")
	fmt.Fprintf(&b, "Magic: physical=0 length=8\nPrefix: physical=8 length=%d\n", i.FrameOffset)
	fmt.Fprintf(&b, "Frame: stored=%d physical=%d length=%d (independent Ion prolog)\n", i.FrameOffset, 8+i.FrameOffset, i.FrameLength)
	endOffset := i.FrameOffset + i.FrameLength
	fmt.Fprintf(&b, "Enddoc: stored=%d physical=%d length=%d\n", endOffset, 8+endOffset, i.IndexOffset-endOffset)
	fmt.Fprintf(&b, "Index: stored=%d physical=%d length=%d (independent Ion prolog)\n", i.IndexOffset, 8+i.IndexOffset, i.IndexLength)
	fmt.Fprintf(&b, "Index row: [%d, %d, 0, %d]\n", i.FrameOffset, i.FrameLength, i.DecodedSize)
	fmt.Fprintf(&b, "Trailer: physical=%d length=9, 0x28 + BE64(%d)\n", i.EncodedSize-9, i.IndexOffset)
	for _, p := range i.Pages {
		fmt.Fprintf(&b, "Page %d: decoded=[%d,%d) declared=%d stored_blob=%d",
			p.Number, p.DecodedOffset, p.DecodedOffset+p.DecodedSize, p.DecodedSize, p.StoredSize)
		if i.Mode == LZMA {
			fmt.Fprintf(&b, " filter=%d properties=0x%02X (LC=3 LP=0 PB=2) dictionary=%d",
				p.Filter, p.Properties, p.Dictionary)
		}
		b.WriteByte('\n')
	}
	return b.String()
}
