package common

import "testing"

func TestNormalizeISBN(t *testing.T) {
	tests := []struct {
		name     string
		in       string
		want     string
		wantKind ISBNKind
		wantOK   bool
	}{
		{name: "isbn10", in: "0-306-40615-2", want: "0306406152", wantKind: ISBNKind10, wantOK: true},
		{name: "isbn10 x", in: "0 8044 2957 X", want: "080442957X", wantKind: ISBNKind10, wantOK: true},
		{name: "isbn13", in: "978-0-306-40615-7", want: "9780306406157", wantKind: ISBNKind13, wantOK: true},
		{name: "invalid checksum", in: "9780306406158", wantOK: false},
		{name: "invalid chars", in: "978030640615X", wantOK: false},
		{name: "empty", in: "", wantOK: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, gotKind, err := NormalizeISBN(tt.in)
			gotOK := err == nil && got != ""
			if gotOK != tt.wantOK {
				t.Fatalf("NormalizeISBN() ok = %v, want %v: %v", gotOK, tt.wantOK, err)
			}
			if got != tt.want || gotKind != tt.wantKind {
				t.Fatalf("NormalizeISBN() = (%q, %q), want (%q, %q)", got, gotKind, tt.want, tt.wantKind)
			}
		})
	}
}
