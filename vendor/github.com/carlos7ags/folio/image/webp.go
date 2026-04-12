// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package image

import (
	"bytes"
	"fmt"
	goimage "image"
	"image/draw"
	"os"

	"golang.org/x/image/webp"
)

// NewWebP creates an Image from raw WebP data.
// The image is decoded and re-encoded as RGB(A) with FlateDecode.
func NewWebP(data []byte) (*Image, error) {
	img, err := webp.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("webp: %w", err)
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

// LoadWebP reads a WebP file and creates an Image.
func LoadWebP(path string) (*Image, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return NewWebP(data)
}
