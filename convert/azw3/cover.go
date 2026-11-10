package azw3

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"slices"

	"github.com/beevik/etree"

	"fbc/common"
	"fbc/config"
	"fbc/content"
	"fbc/convert/publication"
)

// prepareCoverPublication keeps ordinary raster covers outside KF8's main text,
// as KindleGen does. The reader uses the cover resource rather than a cover
// skeleton. A retained broken SVG needs the old document fallback because it
// cannot provide raster cover metadata. Source publication data stays read-only.
func prepareCoverPublication(ctx context.Context, pub *publication.Publication, cfg *config.ImagesConfig) (*publication.Publication, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if pub == nil || cfg == nil {
		return nil, errors.New("publication and cover configuration are required")
	}
	if pub.CoverID == "" {
		return pub, nil
	}
	cover := pub.Images[pub.CoverID]
	if cover == nil {
		return nil, errors.New("cover image is missing from publication")
	}
	for chapter := range pub.Documents() {
		if chapter.Filename == "cover.xhtml" {
			return nil, errors.New("publication already contains cover.xhtml")
		}
	}
	for _, landmark := range pub.Landmarks {
		if landmark.Kind == "cover" {
			return nil, errors.New("publication already contains a cover landmark")
		}
	}
	if imageMediaType(cover.MimeType) != "image/svg+xml" {
		result := *pub
		// A virtual cover has no frozen main-text position for an APNX entry.
		// Keep the existing labels of every remaining page, without renumbering.
		result.Pages = slices.DeleteFunc(slices.Clone(pub.Pages), func(page content.PageMapEntry) bool {
			return page.Filename == "cover.xhtml" && page.SpanID == ""
		})
		return &result, nil
	}
	w, h := cover.Dim.Width, cover.Dim.Height
	if w <= 0 || h <= 0 {
		if dimensions, _, err := image.DecodeConfig(bytes.NewReader(cover.Data)); err == nil {
			w, h = dimensions.Width, dimensions.Height
		} else {
			// An original broken cover retained by use_broken still gets a wrapper.
			w, h = max(cfg.Screen.Width, 1), max(cfg.Screen.Height, 1)
		}
	}
	doc := etree.NewDocument()
	doc.CreateProcInst("xml", `version="1.0" encoding="UTF-8"`)
	doc.CreateDirective("DOCTYPE html")
	root := doc.CreateElement("html")
	root.CreateAttr("xmlns", "http://www.w3.org/1999/xhtml")
	root.CreateAttr("xmlns:epub", "http://www.idpf.org/2007/ops")
	head := root.CreateElement("head")
	head.CreateElement("title").SetText("Cover")
	style := head.CreateElement("style")
	style.CreateAttr("type", "text/css")
	style.SetText("html, body { margin: 0; padding: 0; width: 100%; height: 100%; } " +
		"svg { display: block; width: auto; height: 100%; margin: 0 auto; }")
	body := root.CreateElement("body")
	body.CreateAttr("epub:type", "cover")
	svg := body.CreateElement("svg")
	svg.CreateAttr("xmlns", "http://www.w3.org/2000/svg")
	svg.CreateAttr("xmlns:xlink", "http://www.w3.org/1999/xlink")
	svg.CreateAttr("viewBox", fmt.Sprintf("0 0 %d %d", w, h))
	svg.CreateAttr("preserveAspectRatio", "xMidYMid meet")
	if cfg.Cover.Resize == common.ImageResizeModeStretch {
		w, h = 100, 100
		svg.CreateAttr("viewBox", "0 0 100 100")
		svg.CreateAttr("preserveAspectRatio", "xMidYMid slice")
		style.SetText("html, body { margin: 0; padding: 0; width: 100%; height: 100%; } " +
			"svg { display: block; width: 100%; height: 100%; }")
	}
	img := svg.CreateElement("image")
	img.CreateAttr("x", "0")
	img.CreateAttr("y", "0")
	img.CreateAttr("width", fmt.Sprint(w))
	img.CreateAttr("height", fmt.Sprint(h))
	img.CreateAttr("xlink:href", cover.Filename)
	result := *pub
	result.Chapters = append([]publication.Chapter{{ID: "cover-page", Filename: "cover.xhtml", Title: "Cover", Doc: doc}}, pub.Chapters...)
	result.Landmarks = append(slices.Clone(pub.Landmarks), publication.Landmark{Kind: "cover", Title: "Cover", Href: "cover.xhtml"})
	return &result, nil
}
