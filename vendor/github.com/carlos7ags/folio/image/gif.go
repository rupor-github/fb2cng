// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package image

import (
	"bytes"
	"fmt"
	goimage "image"
	"image/draw"
	"image/gif"
	"os"
)

// NewGIF creates an Image from raw GIF data.
// Only the first frame is used. The image is decoded and re-encoded
// as RGB(A) with FlateDecode.
func NewGIF(data []byte) (*Image, error) {
	img, err := gif.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("gif: %w", err)
	}

	bounds := img.Bounds()
	w := bounds.Dx()
	h := bounds.Dy()

	// Convert to RGBA for uniform handling.
	rgba := goimage.NewRGBA(bounds)
	draw.Draw(rgba, bounds, img, bounds.Min, draw.Src)

	if imageHasAlpha(rgba) {
		return buildRGBA(rgba, w, h)
	}
	return buildRGB(rgba, w, h)
}

// LoadGIF reads a GIF file and creates an Image.
func LoadGIF(path string) (*Image, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return NewGIF(data)
}
