package fb2

import "embed"

//go:embed placeholders/*.svg
var placeholderFiles embed.FS

func mustReadPlaceholder(name string) []byte {
	data, err := placeholderFiles.ReadFile(name)
	if err != nil {
		panic("embedded placeholder missing: " + name + ": " + err.Error())
	}
	return data
}

var (
	brokenImage   = mustReadPlaceholder("placeholders/broken.svg")
	notFoundImage = mustReadPlaceholder("placeholders/not-found.svg")
)
