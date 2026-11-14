package images

import (
	"fmt"
	"image"
	"image/color"
	"testing"
)

func TestGrayscaleLuma(t *testing.T) {
	for _, tt := range []struct {
		name string
		c    color.NRGBA
		want uint8
	}{
		{"red", color.NRGBA{R: 255, A: 255}, 76},
		{"green", color.NRGBA{G: 255, A: 255}, 150},
		{"blue", color.NRGBA{B: 255, A: 255}, 29},
		{"white", color.NRGBA{R: 255, G: 255, B: 255, A: 255}, 255},
		{"black", color.NRGBA{A: 255}, 0},
		{"transparent red", color.NRGBA{R: 255, A: 128}, 76},
		{"almost transparent green", color.NRGBA{G: 255, A: 1}, 150},
		{"invisible blue", color.NRGBA{B: 255, A: 0}, 29},
	} {
		t.Run(tt.name, func(t *testing.T) {
			src := image.NewNRGBA(image.Rect(3, 4, 4, 5))
			src.SetNRGBA(3, 4, tt.c)
			got := GrayscaleLuma(src)
			if got.Bounds() != src.Bounds() {
				t.Fatal("conversion changed image bounds")
			}
			c := color.NRGBAModel.Convert(got.At(3, 4)).(color.NRGBA)
			if c.R != tt.want || c.G != tt.want || c.B != tt.want || c.A != tt.c.A {
				t.Fatalf("got %+v, want gray %d with alpha %d", c, tt.want, tt.c.A)
			}
			if tt.c.A == 255 {
				if _, ok := got.(*image.Gray); !ok {
					t.Fatal("opaque conversion should use single-channel grayscale")
				}
			}
		})
	}
}

func TestGrayscaleLuma16(t *testing.T) {
	for _, alpha := range []uint16{0, 1, 0x1234, 0x8000, 0xffff} {
		t.Run(fmt.Sprint(alpha), func(t *testing.T) {
			src := image.NewNRGBA64(image.Rect(0, 0, 1, 1))
			c := color.NRGBA64{R: 0xf123, G: 0x5123, B: 0x2123, A: alpha}
			src.SetNRGBA64(0, 0, c)
			got := color.NRGBA64Model.Convert(GrayscaleLuma(src).At(0, 0)).(color.NRGBA64)
			want := color.Gray16Model.Convert(color.RGBA64{R: c.R, G: c.G, B: c.B, A: 0xffff}).(color.Gray16)
			if got.R != want.Y || got.G != want.Y || got.B != want.Y || got.A != alpha {
				t.Fatalf("got %+v, want gray %d with alpha %d", got, want.Y, alpha)
			}
		})
	}
}

func TestGrayscaleLumaAlreadyGray(t *testing.T) {
	for _, src := range []image.Image{
		image.NewGray(image.Rect(0, 0, 1, 1)), image.NewGray16(image.Rect(0, 0, 1, 1)),
	} {
		t.Run(fmt.Sprintf("%T", src), func(t *testing.T) {
			if GrayscaleLuma(src) != src {
				t.Fatal("already grayscale images should not be copied")
			}
		})
	}
}
