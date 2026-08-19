package metainfo

import (
	"testing"
	"time"

	"fbc/fb2"
)

func TestKindleIssueDate(t *testing.T) {
	parsed, err := time.Parse(time.DateOnly, "2001-02-03")
	if err != nil {
		t.Fatalf("parse date: %v", err)
	}

	tests := []struct {
		name    string
		desc    fb2.Description
		want    string
		wantErr bool
	}{
		{name: "machine date", desc: fb2.Description{TitleInfo: fb2.TitleInfo{Date: &fb2.Date{Value: parsed}}}, want: "2001-02-03"},
		{name: "display year month", desc: fb2.Description{TitleInfo: fb2.TitleInfo{Date: &fb2.Date{Display: "2001-02"}}}, want: "2001-02"},
		{name: "publish year fallback", desc: fb2.Description{PublishInfo: &fb2.PublishInfo{Year: "1999"}}, want: "1999"},
		{name: "invalid display blocks fallback", desc: fb2.Description{TitleInfo: fb2.TitleInfo{Date: &fb2.Date{Display: "spring 2001"}}}, wantErr: true},
		{name: "invalid publish year", desc: fb2.Description{PublishInfo: &fb2.PublishInfo{Year: "19xx"}}, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := KindleIssueDate(&tt.desc)
			if (err != nil) != tt.wantErr {
				t.Fatalf("KindleIssueDate() error = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Fatalf("KindleIssueDate() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestOPFDate(t *testing.T) {
	parsed, err := time.Parse(time.DateOnly, "2001-02-03")
	if err != nil {
		t.Fatalf("parse date: %v", err)
	}

	tests := []struct {
		name    string
		desc    fb2.Description
		want    string
		wantErr bool
	}{
		{name: "machine date", desc: fb2.Description{TitleInfo: fb2.TitleInfo{Date: &fb2.Date{Value: parsed}}}, want: "2001-02-03"},
		{name: "display date", desc: fb2.Description{TitleInfo: fb2.TitleInfo{Date: &fb2.Date{Display: "2001-02-03"}}}, want: "2001-02-03"},
		{name: "display russian date", desc: fb2.Description{TitleInfo: fb2.TitleInfo{Date: &fb2.Date{Display: "26.10.2015"}}}, want: "2015-10-26"},
		{name: "publish year fallback", desc: fb2.Description{PublishInfo: &fb2.PublishInfo{Year: "1999"}}, want: "1999"},
		{name: "invalid title date blocks fallback", desc: fb2.Description{
			TitleInfo:   fb2.TitleInfo{Date: &fb2.Date{Display: "spring 2001"}},
			PublishInfo: &fb2.PublishInfo{Year: "1999"},
		}, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := OPFDate(&tt.desc)
			if (err != nil) != tt.wantErr {
				t.Fatalf("OPFDate() error = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Fatalf("OPFDate() = %q, want %q", got, tt.want)
			}
		})
	}
}
