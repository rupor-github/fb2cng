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
			config.VignettePosChapterTitleTop: []byte(`<svg viewBox="0 0 300 20" xmlns="http://www.w3.org/2000/svg">
  <path d="M10 10
           Q40 0 70 10
           M230 10
           Q260 0 290 10"
        stroke="black" fill="none" stroke-width="1"/>
  <line x1="70" y1="10" x2="230" y2="10" stroke="black" stroke-width="1"/>
</svg>`),
			config.VignettePosChapterTitleBottom: []byte(`<svg viewBox="0 0 300 20" xmlns="http://www.w3.org/2000/svg">
  <path d="M10 10
           Q40 20 70 10
           M230 10
           Q260 20 290 10"
        stroke="black" fill="none" stroke-width="1"/>
  <line x1="70" y1="10" x2="230" y2="10" stroke="black" stroke-width="1"/>
</svg>`),
			config.VignettePosChapterEnd: []byte(`<svg viewBox="0 0 240 20" xmlns="http://www.w3.org/2000/svg">
  <path d="M10 10 H90
           M150 10 H230"
        stroke="black" stroke-width="1"/>
  <path d="M120 3 A7 7 0 1 1 119.9 3" fill="none" stroke="black" stroke-width="1"/>
</svg>`),
			config.VignettePosSectionTitleTop: []byte(`<svg viewBox="0 0 240 24" xmlns="http://www.w3.org/2000/svg">
  <path d="M20 12 H100
           M140 12 H220"
        stroke="black" stroke-width="1"/>
  <path d="M100 12
           C110 2 130 2 140 12
           C130 22 110 22 100 12"
        fill="none" stroke="black" stroke-width="1.2"/>
</svg>`),
			config.VignettePosSectionTitleBottom: []byte(`<svg viewBox="0 0 240 24" xmlns="http://www.w3.org/2000/svg">
  <path d="M20 12 H100
           M140 12 H220"
        stroke="black" stroke-width="1"/>
  <path d="M100 12
           C110 2 130 2 140 12
           C130 22 110 22 100 12"
        fill="none" stroke="black" stroke-width="1.2"/>
</svg>`),
			config.VignettePosSectionEnd: []byte(`<svg viewBox="0 0 200 20" xmlns="http://www.w3.org/2000/svg">
  <path d="M50 10
           C65 0 85 0 100 10
           C85 20 65 20 50 10
           M100 10
           C115 0 135 0 150 10
           C135 20 115 20 100 10"
        stroke="black" fill="none" stroke-width="1.3"/>
</svg>`),
		},
	}
}
