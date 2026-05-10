package pdf

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"fbc/convert/structure"
)

type pdfDebugStructurePlan struct {
	Units     []pdfDebugStructureUnit     `json:"units"`
	TOC       []pdfDebugStructureTOCEntry `json:"toc,omitempty"`
	Landmarks structure.LandmarkInfo      `json:"landmarks"`
	Generated pdfDebugStructureGenerated  `json:"generated"`
}

type pdfDebugStructureUnit struct {
	Index        int    `json:"index"`
	Kind         string `json:"kind"`
	ID           string `json:"id,omitempty"`
	Title        string `json:"title,omitempty"`
	Depth        int    `json:"depth,omitempty"`
	TitleDepth   int    `json:"title_depth,omitempty"`
	ForceNewPage bool   `json:"force_new_page,omitempty"`
	BodyName     string `json:"body_name,omitempty"`
	SectionID    string `json:"section_id,omitempty"`
	IsTopLevel   bool   `json:"is_top_level,omitempty"`
}

type pdfDebugStructureTOCEntry struct {
	ID           string                      `json:"id,omitempty"`
	Title        string                      `json:"title,omitempty"`
	IncludeInTOC bool                        `json:"include_in_toc"`
	Children     []pdfDebugStructureTOCEntry `json:"children,omitempty"`
}

type pdfDebugStructureGenerated struct {
	AnnotationPage                 bool   `json:"annotation_page"`
	AnnotationInTOC                bool   `json:"annotation_in_toc"`
	TOCPagePlacement               string `json:"toc_page_placement,omitempty"`
	TOCIncludeChaptersWithoutTitle bool   `json:"toc_include_chapters_without_title,omitempty"`
	TOCType                        string `json:"toc_type,omitempty"`
}

type pdfDebugBlock struct {
	Index        int    `json:"index"`
	Kind         string `json:"kind"`
	ID           string `json:"id,omitempty"`
	Depth        int    `json:"depth,omitempty"`
	StyleName    string `json:"style_name,omitempty"`
	StyleClasses string `json:"style_classes,omitempty"`
	ImageID      string `json:"image_id,omitempty"`
	Text         string `json:"text,omitempty"`
}

type pdfDebugPage struct {
	Number      int                  `json:"number"`
	ObjectID    int                  `json:"object_id,omitempty"`
	ContentID   int                  `json:"content_id,omitempty"`
	Anchors     []string             `json:"anchors,omitempty"`
	Lines       []pdfDebugLine       `json:"lines"`
	Images      []pdfDebugImage      `json:"images,omitempty"`
	Backgrounds []pdfDebugBackground `json:"backgrounds,omitempty"`
	Borders     []pdfDebugBorder     `json:"borders,omitempty"`
	Links       []pdfDebugLink       `json:"links,omitempty"`
}

type pdfDebugBackground struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
	Color  string  `json:"color"`
}

type pdfDebugBorder struct {
	X         float64 `json:"x"`
	Y         float64 `json:"y"`
	Width     float64 `json:"width"`
	Height    float64 `json:"height"`
	LineWidth float64 `json:"line_width"`
	Color     string  `json:"color"`
}

type pdfDebugLine struct {
	Text             string  `json:"text"`
	X                float64 `json:"x"`
	Y                float64 `json:"y"`
	FontSize         float64 `json:"font_size"`
	LetterSpacing    float64 `json:"letter_spacing,omitempty"`
	FontResource     string  `json:"font_resource,omitempty"`
	FontFamily       string  `json:"font_family,omitempty"`
	FontWeight       string  `json:"font_weight,omitempty"`
	FontStyle        string  `json:"font_style,omitempty"`
	Color            string  `json:"color,omitempty"`
	Underline        bool    `json:"underline,omitempty"`
	Strikethrough    bool    `json:"strikethrough,omitempty"`
	Width            float64 `json:"width"`
	ExtraWordSpacing float64 `json:"extra_word_spacing,omitempty"`
}

type pdfDebugImage struct {
	Page         int     `json:"page,omitempty"`
	ImageID      string  `json:"image_id"`
	ResourceName string  `json:"resource_name,omitempty"`
	X            float64 `json:"x"`
	Y            float64 `json:"y"`
	Width        float64 `json:"width"`
	Height       float64 `json:"height"`
}

type pdfDebugLink struct {
	Page     int          `json:"page,omitempty"`
	ObjectID int          `json:"object_id,omitempty"`
	Href     string       `json:"href"`
	Internal bool         `json:"internal"`
	Rect     pdfDebugRect `json:"rect"`
}

type pdfDebugRect struct {
	X1 float64 `json:"x1"`
	Y1 float64 `json:"y1"`
	X2 float64 `json:"x2"`
	Y2 float64 `json:"y2"`
}

type pdfDebugFont struct {
	ResourceName   string   `json:"resource_name"`
	Family         string   `json:"family"`
	Bold           bool     `json:"bold,omitempty"`
	Italic         bool     `json:"italic,omitempty"`
	PostScriptName string   `json:"post_script_name"`
	UnitsPerEm     int      `json:"units_per_em"`
	Ascent         int      `json:"ascent"`
	Descent        int      `json:"descent"`
	CapHeight      int      `json:"cap_height"`
	BBox           [4]int   `json:"bbox"`
	Flags          int      `json:"flags"`
	ItalicAngle    int      `json:"italic_angle"`
	UsedGlyphCount int      `json:"used_glyph_count"`
	UsedGlyphIDs   []uint16 `json:"used_glyph_ids"`
}

func writePDFDebugDumps(doc skeletonDocument, pages []pdfPage, fontResources []pdfFontResource) error {
	if !doc.Debug || doc.WorkDir == "" {
		return nil
	}
	if err := writeJSONDebugDump(filepath.Join(doc.WorkDir, "pdf-structure-plan.json"), doc.DebugPlan); err != nil {
		return err
	}
	styles := doc.Styles
	if styles == nil {
		styles = newPDFStyleResolver(nil, nil)
	}
	if err := writeJSONDebugDump(filepath.Join(doc.WorkDir, "pdf-resolved-styles.json"), styles.debugStyles()); err != nil {
		return err
	}

	blocks := make([]pdfDebugBlock, 0, len(doc.Blocks))
	for i, block := range doc.Blocks {
		styleName := pdfStyleNameForBlock(block)
		styles.tracer.traceAssign(block, styleName, styles.styleForBlock(block))
		blocks = append(blocks, pdfDebugBlock{
			Index:        i,
			Kind:         block.Kind.String(),
			ID:           block.ID,
			Depth:        block.Depth,
			StyleName:    styleName,
			StyleClasses: strings.TrimSpace(block.StyleClasses),
			ImageID:      block.ImageID,
			Text:         block.Text,
		})
	}
	if err := writeJSONDebugDump(filepath.Join(doc.WorkDir, "pdf-text-blocks.json"), blocks); err != nil {
		return err
	}
	styles.tracer.flush()

	debugPages, debugImages, debugLinks := pdfDebugPages(pages)
	if err := writeJSONDebugDump(filepath.Join(doc.WorkDir, "pdf-layout-pages.json"), debugPages); err != nil {
		return err
	}
	if err := writeJSONDebugDump(filepath.Join(doc.WorkDir, "pdf-images.json"), debugImages); err != nil {
		return err
	}
	if err := writeJSONDebugDump(filepath.Join(doc.WorkDir, "pdf-links.json"), debugLinks); err != nil {
		return err
	}
	return writeJSONDebugDump(filepath.Join(doc.WorkDir, "pdf-fonts.json"), pdfDebugFonts(fontResources))
}

func pdfDebugPages(pages []pdfPage) ([]pdfDebugPage, []pdfDebugImage, []pdfDebugLink) {
	debugPages := make([]pdfDebugPage, 0, len(pages))
	debugImages := make([]pdfDebugImage, 0)
	debugLinks := make([]pdfDebugLink, 0)
	for i, page := range pages {
		debugPage := pdfDebugPage{
			Number:      i + 1,
			ObjectID:    page.ObjectID,
			ContentID:   page.ContentID,
			Anchors:     slices.Clone(page.Anchors),
			Lines:       make([]pdfDebugLine, 0, len(page.Lines)),
			Images:      make([]pdfDebugImage, 0, len(page.Images)),
			Backgrounds: make([]pdfDebugBackground, 0, len(page.Backgrounds)),
			Borders:     make([]pdfDebugBorder, 0, len(page.Borders)),
			Links:       make([]pdfDebugLink, 0, len(page.Annotations)),
		}
		for _, line := range page.Lines {
			debugPage.Lines = append(debugPage.Lines, pdfDebugLine{
				Text:             shapedRunes(line.Text),
				X:                line.X,
				Y:                line.Y,
				FontSize:         line.FontSize,
				LetterSpacing:    line.LetterSpacing,
				FontResource:     line.FontName,
				FontFamily:       line.FontKey.Family,
				FontWeight:       pdfCSSFontWeightString(line.FontKey.Bold),
				FontStyle:        pdfCSSFontStyleString(line.FontKey.Italic),
				Color:            line.Color.String(),
				Underline:        line.Underline,
				Strikethrough:    line.Strikethrough,
				Width:            shapedWidthPointsWithSpacing(line.Text, line.FontSize, line.LetterSpacing),
				ExtraWordSpacing: line.ExtraWordSpacing,
			})
		}
		for _, background := range page.Backgrounds {
			debugPage.Backgrounds = append(debugPage.Backgrounds, pdfDebugBackground{
				X:      background.X,
				Y:      background.Y,
				Width:  background.Width,
				Height: background.Height,
				Color:  background.Color.String(),
			})
		}
		for _, border := range page.Borders {
			debugPage.Borders = append(debugPage.Borders, pdfDebugBorder{
				X:         border.X,
				Y:         border.Y,
				Width:     border.Width,
				Height:    border.Height,
				LineWidth: border.LineWidth,
				Color:     border.Color.String(),
			})
		}
		for _, image := range page.Images {
			debugImage := pdfDebugImage{
				Page:         i + 1,
				ImageID:      image.ImageID,
				ResourceName: image.Name,
				X:            image.X,
				Y:            image.Y,
				Width:        image.Width,
				Height:       image.Height,
			}
			debugPage.Images = append(debugPage.Images, debugImage)
			debugImages = append(debugImages, debugImage)
		}
		for _, link := range page.Annotations {
			debugLink := pdfDebugLink{
				Page:     i + 1,
				ObjectID: link.ObjectID,
				Href:     link.Href,
				Internal: strings.HasPrefix(link.Href, "#"),
				Rect: pdfDebugRect{
					X1: link.Rect.X1,
					Y1: link.Rect.Y1,
					X2: link.Rect.X2,
					Y2: link.Rect.Y2,
				},
			}
			debugPage.Links = append(debugPage.Links, debugLink)
			debugLinks = append(debugLinks, debugLink)
		}
		debugPages = append(debugPages, debugPage)
	}
	return debugPages, debugImages, debugLinks
}

func pdfDebugFonts(resources []pdfFontResource) []pdfDebugFont {
	out := make([]pdfDebugFont, 0, len(resources))
	for _, resource := range resources {
		if resource.Face == nil {
			continue
		}
		usedGlyphIDs := make([]uint16, 0, len(resource.Used))
		for glyphID := range resource.Used {
			usedGlyphIDs = append(usedGlyphIDs, glyphID)
		}
		slices.Sort(usedGlyphIDs)
		out = append(out, pdfDebugFont{
			ResourceName:   resource.Name,
			Family:         resource.Key.Family,
			Bold:           resource.Key.Bold,
			Italic:         resource.Key.Italic,
			PostScriptName: resource.Face.PostScriptName,
			UnitsPerEm:     resource.Face.UnitsPerEm,
			Ascent:         resource.Face.Ascent,
			Descent:        resource.Face.Descent,
			CapHeight:      resource.Face.CapHeight,
			BBox:           resource.Face.BBox,
			Flags:          resource.Face.Flags,
			ItalicAngle:    resource.Face.ItalicAngle,
			UsedGlyphCount: len(usedGlyphIDs),
			UsedGlyphIDs:   usedGlyphIDs,
		})
	}
	return out
}

func writeJSONDebugDump(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal %s: %w", filepath.Base(path), err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write %s: %w", filepath.Base(path), err)
	}
	return nil
}

func shapedRunes(text shapedText) string {
	runes := make([]rune, 0, len(text.Glyphs))
	for _, glyph := range text.Glyphs {
		runes = append(runes, glyph.Rune)
	}
	return string(runes)
}
