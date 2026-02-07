package state

import (
	"time"

	"fbc/common"
)

// newLocalEnv creates a new LocalEnv instance with default values
func newLocalEnv() *LocalEnv {
	return &LocalEnv{
		start: time.Now(),
		DefaultVignettes: map[common.VignettePos][]byte{
			common.VignettePosBookTitleTop: []byte(`<svg viewBox="0 0 500 80" xmlns="http://www.w3.org/2000/svg">
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
			common.VignettePosBookTitleBottom: []byte(`<svg viewBox="0 0 500 80" xmlns="http://www.w3.org/2000/svg">
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
			common.VignettePosChapterTitleTop: []byte(`<svg viewBox="0 0 300 20" xmlns="http://www.w3.org/2000/svg">
  <path d="M10 10
           Q40 0 70 10
           M230 10
           Q260 0 290 10"
        stroke="black" fill="none" stroke-width="1"/>
  <line x1="70" y1="10" x2="230" y2="10" stroke="black" stroke-width="1"/>
</svg>`),
			common.VignettePosChapterTitleBottom: []byte(`<svg viewBox="0 0 300 20" xmlns="http://www.w3.org/2000/svg">
  <path d="M10 10
           Q40 20 70 10
           M230 10
           Q260 20 290 10"
        stroke="black" fill="none" stroke-width="1"/>
  <line x1="70" y1="10" x2="230" y2="10" stroke="black" stroke-width="1"/>
</svg>`),
			common.VignettePosChapterEnd: []byte(`<svg viewBox="0 0 300 20" xmlns="http://www.w3.org/2000/svg">
  <line x1="10" y1="10" x2="290" y2="10"
        stroke="black" stroke-width="1"/>
  <path d="M10 10
           Q40 20 70 10"
        stroke="black" fill="none" stroke-width="1"/>
  <path d="M120 10
           Q150 0 180 10"
        stroke="black" fill="none" stroke-width="1"/>
  <path d="M230 10
           Q260 20 290 10"
        stroke="black" fill="none" stroke-width="1"/>
</svg>`),
			common.VignettePosSectionTitleTop: []byte(`<svg viewBox="0 0 240 24" xmlns="http://www.w3.org/2000/svg">
  <path d="M20 22
           C30 12 50 12 60 12"
        fill="none" stroke="black" stroke-width="1"/>
  <path d="M60 12 H100" stroke="black" stroke-width="1"/>
  <path d="M100 12
           C110 2 130 2 140 12
           C130 22 110 22 100 12"
        fill="none" stroke="black" stroke-width="1.2"/>
  <path d="M140 12 H180" stroke="black" stroke-width="1"/>
  <path d="M180 12
           C190 12 210 12 220 22"
        fill="none" stroke="black" stroke-width="1"/>
</svg>`),
			common.VignettePosSectionTitleBottom: []byte(`<svg viewBox="0 0 240 24" xmlns="http://www.w3.org/2000/svg">
  <path d="M20 2
           C30 12 50 12 60 12"
        fill="none" stroke="black" stroke-width="1"/>
  <path d="M60 12 H100" stroke="black" stroke-width="1"/>
  <path d="M100 12
           C110 22 130 22 140 12
           C130 2 110 2 100 12"
        fill="none" stroke="black" stroke-width="1.2"/>
  <path d="M140 12 H180" stroke="black" stroke-width="1"/>
  <path d="M180 12
           C190 12 210 12 220 2"
        fill="none" stroke="black" stroke-width="1"/>
</svg>`),
			common.VignettePosSectionEnd: []byte(`<svg viewBox="0 0 240 24" xmlns="http://www.w3.org/2000/svg">
  <path d="M20 2
           C30 12 50 12 60 12"
        fill="none" stroke="black" stroke-width="1"/>
  <path d="M60 12 H100" stroke="black" stroke-width="1"/>
  <path d="M100 12
           C110 22 130 22 140 12
           C130 2 110 2 100 12"
        fill="none" stroke="black" stroke-width="1.2"/>
  <path d="M140 12 H180" stroke="black" stroke-width="1"/>
  <path d="M180 12
           C190 12 210 12 220 2"
        fill="none" stroke="black" stroke-width="1"/>
  <path d="M20 2
           C70 13 170 13 220 2"
        fill="none" stroke="black" stroke-width="1"/>
</svg>`),
		},
	}
}
