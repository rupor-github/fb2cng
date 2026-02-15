package state

import (
	"embed"
	"time"

	"fbc/common"
)

//go:embed vignettes/*.svg
var vignetteFiles embed.FS

// newLocalEnv creates a new LocalEnv instance with default values
func newLocalEnv() *LocalEnv {
	return &LocalEnv{
		start: time.Now(),
		DefaultVignettes: map[common.VignettePos][]byte{
			common.VignettePosBookTitleTop:       mustReadVignette("vignettes/book-title-top.svg"),
			common.VignettePosBookTitleBottom:    mustReadVignette("vignettes/book-title-bottom.svg"),
			common.VignettePosChapterTitleTop:    mustReadVignette("vignettes/chapter-title-top.svg"),
			common.VignettePosChapterTitleBottom: mustReadVignette("vignettes/chapter-title-bottom.svg"),
			common.VignettePosChapterEnd:         mustReadVignette("vignettes/chapter-end.svg"),
			common.VignettePosSectionTitleTop:    mustReadVignette("vignettes/section-title-top.svg"),
			common.VignettePosSectionTitleBottom: mustReadVignette("vignettes/section-title-bottom.svg"),
			common.VignettePosSectionEnd:         mustReadVignette("vignettes/section-end.svg"),
		},
	}
}

func mustReadVignette(name string) []byte {
	data, err := vignetteFiles.ReadFile(name)
	if err != nil {
		panic("embedded vignette missing: " + name + ": " + err.Error())
	}
	return data
}
