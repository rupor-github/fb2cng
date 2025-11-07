package fb2

import (
	"os"
	"testing"

	"github.com/beevik/etree"
)

const sampleFB2 = "../testdata/_Test.fb2"

func loadSampleDocument(t *testing.T) *etree.Document {
	t.Helper()

	file, err := os.Open(sampleFB2)
	if err != nil {
		t.Fatalf("open sample file: %v", err)
	}
	t.Cleanup(func() { _ = file.Close() })

	doc := etree.NewDocument()
	doc.ReadSettings = etree.ReadSettings{
		ValidateInput: false,
		Permissive:    true,
	}

	if _, err := doc.ReadFrom(file); err != nil {
		t.Fatalf("parse sample file: %v", err)
	}
	return doc
}
