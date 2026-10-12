package envelope

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
)

func TestAdaptivePageSizeThresholds(t *testing.T) {
	for _, test := range []struct{ size, want int }{
		{1, PageSize}, {1 << 20, PageSize},
		{1<<20 + 1, 16 * 1024}, {4 << 20, 16 * 1024},
		{4<<20 + 1, 32 * 1024}, {8 << 20, 32 * 1024},
		{8<<20 + 1, MaxPageSize}, {16 << 20, MaxPageSize}, {1 << 30, MaxPageSize},
	} {
		t.Run(fmt.Sprint(test.size), func(t *testing.T) {
			if got := adaptivePageSize(test.size); got != test.want {
				t.Fatalf("got %d, want %d", got, test.want)
			}
		})
	}
}

func TestAdaptivePageRoundTrip(t *testing.T) {
	for _, test := range []struct{ size, pageSize int }{
		{1<<20 + 1, 16 * 1024}, {4<<20 + 1, 32 * 1024}, {8<<20 + 1, MaxPageSize},
	} {
		for _, mode := range []Mode{Plaintext, LZMA} {
			t.Run(fmt.Sprintf("%s/%d", mode, test.pageSize), func(t *testing.T) {
				pattern := []byte("UTF-8 世界 and binary\x00\xff\n")
				inner := bytes.Repeat(pattern, (test.size+len(pattern)-1)/len(pattern))[:test.size]
				data, info, err := Encode(t.Context(), inner, mode)
				if err != nil {
					t.Fatal(err)
				}
				if info.PageSize != uint64(test.pageSize) || len(info.Pages) != (test.size+test.pageSize-1)/test.pageSize {
					t.Fatalf("page layout: size=%d count=%d", info.PageSize, len(info.Pages))
				}
				if last := info.Pages[len(info.Pages)-1]; last.DecodedSize != 1 {
					t.Fatalf("short final page: %+v", last)
				}
				if !strings.Contains(info.Summary(), fmt.Sprintf("page_size=%d", test.pageSize)) {
					t.Fatal("summary used wrong page size")
				}
				e, err := Inspect(t.Context(), data)
				if err != nil {
					t.Fatal(err)
				}
				got, err := e.Decode(t.Context())
				if err != nil {
					t.Fatal(err)
				}
				if !bytes.Equal(got, inner) {
					t.Fatal("adaptive page round-trip changed bytes")
				}
			})
		}
	}
}
