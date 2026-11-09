package fb2

import (
	"bytes"
	"fmt"
	"testing"

	"go.uber.org/zap"

	"fbc/common"
	"fbc/config"
)

func TestKindleRasterPolicyDoesNotRetainSVG(t *testing.T) {
	for _, force := range []bool{false, true} {
		for _, remove := range []bool{false, true} {
			for _, cover := range []bool{false, true} {
				t.Run(fmt.Sprintf("force=%t/remove=%t/cover=%t", force, remove, cover), func(t *testing.T) {
					cfg := &config.ImagesConfig{
						JPEGQuality: 90, ScaleFactor: 0.5, RasterizeSVG: force, RemoveTransparency: remove,
						Screen: config.ScreenConfig{Width: 80, Height: 100},
						Cover:  config.CoverConfig{Resize: common.ImageResizeModeStretch},
					}
					bo := BinaryObject{ID: "art", ContentType: "image/svg+xml", Data: []byte(
						`<svg viewBox="0 0 40 20"><rect width="20" height="10" fill="red"/></svg>`)}
					policy := ImagePolicy{RequireJPEG: true, KindleSVGStrokes: true}
					got := bo.PrepareImage(policy, cover, cfg, zap.NewNop())
					want := bo.RasterizeSVG(policy, cover, cfg, zap.NewNop())
					if got.MimeType != "image/jpeg" || !bytes.Equal(got.Data, want.Data) || got.Dim != want.Dim {
						t.Fatal("raster-only Kindle output must never retain SVG for native vector conversion")
					}
				})
			}
		}
	}
}
