package state

import (
	"time"

	"fbc/config"
)

// newLocalEnv creates a new LocalEnv instance with default values
func newLocalEnv() *LocalEnv {
	return &LocalEnv{
		start: time.Now(),
		DefaultVignettes: map[config.VignettePos][]byte{
			config.VignettePosBookTitleTop: []byte(`<svg viewBox="0 0 500 80" xmlns="http://www.w3.org/2000/svg">
  <path d="
    M20 50 H200
    C220 15, 280 15, 300 50
    H480

    M250 50
    C235 40, 235 25, 250 15
    C265 25, 265 40, 250 50

    M230 50
    C225 55, 225 65, 230 70
    M270 50
    C275 55, 275 65, 270 70
  "
  fill="none" stroke="black" stroke-width="1"/>
</svg>`),
			config.VignettePosBookTitleBottom: []byte(`<svg viewBox="0 0 500 80" xmlns="http://www.w3.org/2000/svg">
  <path d="
    M20 30 H200
    C220 65, 280 65, 300 30
    H480

    M250 30
    C235 40, 235 55, 250 65
    C265 55, 265 40, 250 30

    M230 30
    C225 25, 225 15, 230 10
    M270 30
    C275 25, 275 15, 270 10
  "
  fill="none" stroke="black" stroke-width="1"/>
</svg>`),
			config.VignettePosChapterTitleTop: []byte(`<svg viewBox="0 0 500 80" xmlns="http://www.w3.org/2000/svg">
  <path d="
    M20 50 H200
    C220 15, 280 15, 300 50
    H480

    M250 50
    C235 40, 235 25, 250 15
    C265 25, 265 40, 250 50

    M230 50
    C225 55, 225 65, 230 70
    M270 50
    C275 55, 275 65, 270 70
  "
  fill="none" stroke="black" stroke-width="1"/>
</svg>`),
			config.VignettePosChapterTitleBottom: []byte(`<svg viewBox="0 0 500 80" xmlns="http://www.w3.org/2000/svg">
  <path d="
    M20 30 H200
    C220 65, 280 65, 300 30
    H480

    M250 30
    C235 40, 235 55, 250 65
    C265 55, 265 40, 250 30

    M230 30
    C225 25, 225 15, 230 10
    M270 30
    C275 25, 275 15, 270 10
  "
  fill="none" stroke="black" stroke-width="1"/>
</svg>`),
			config.VignettePosChapterEnd: []byte(`<svg viewBox="0 0 300 30" xmlns="http://www.w3.org/2000/svg">
  <path d="
	M10 15 C40 30, 60 30, 90 15
	C120 0, 180 0, 210 15
	C240 30, 260 30, 290 15
  "
  fill="none" stroke="black" stroke-width="1"/>
</svg>`),
		},
	}
}
