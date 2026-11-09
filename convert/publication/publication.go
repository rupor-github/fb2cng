// Package publication builds rendered book content independently of its output container.
package publication

import (
	"context"
	"errors"
	"fmt"
	"iter"
	"slices"
	"strings"

	"go.uber.org/zap"

	"fbc/common"
	"fbc/config"
	"fbc/content"
	"fbc/convert/tocnav"
	"fbc/fb2"
)

// Publication contains rendered content using publication-relative resource paths.
// Chapters includes navigation-only entries; Documents yields unique reading-order
// documents. Container-specific cover wrappers, metadata and navigation serialization
// belong to the output adapter.
//
// Resource bytes are shared with the prepared Content. Adapters must treat the
// publication as read-only or copy it before performing container-specific rewrites.
type Publication struct {
	Chapters   []Chapter
	IDToFile   map[string]string
	Navigation []tocnav.Item
	Pages      []content.PageMapEntry
	Stylesheet Stylesheet
	Images     fb2.BookImages
	CoverID    string
}

// Documents yields each rendered document once, in reading order, excluding
// navigation-only entries that point into an already rendered document.
func (p *Publication) Documents() iter.Seq[Chapter] {
	return func(yield func(Chapter) bool) {
		seen := make(map[string]bool)
		for _, chapter := range p.Chapters {
			filename, _, _ := strings.Cut(chapter.Filename, "#")
			if chapter.Doc == nil || seen[filename] {
				continue
			}
			seen[filename] = true
			if !yield(chapter) {
				return
			}
		}
	}
}

// Build renders prepared content once, including generated pages, link resolution,
// navigation, pagination and resource selection. It mutates Content's rendering state;
// reuse the result for multiple adapters rather than rendering the same Content again.
// The actual OutputFormat is retained for templates; markup selects content syntax.
func Build(
	ctx context.Context,
	prepared *content.Content,
	cfg *config.DocumentConfig,
	markup Markup,
	log *zap.Logger,
) (*Publication, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if prepared == nil || prepared.Book == nil {
		return nil, errors.New("prepared content is required")
	}
	if cfg == nil {
		return nil, errors.New("document config is required")
	}
	if markup != XHTML11 && markup != HTML5 {
		return nil, fmt.Errorf("unsupported publication markup: %d", markup)
	}
	c := &renderState{Content: prepared, markup: markup}

	annotationEnabled := cfg.Annotation.Enable && c.Book.Description.TitleInfo.Annotation != nil
	tocEnabled := cfg.TOCPage.Placement != common.TOCPagePlacementNone
	var tocFilename string
	if c.PageSize > 0 {
		// Generated pages count once each, without in-document page markers.
		if tocEnabled {
			_, tocFilename = generateUniqueID("toc-page", c.IDsIndex)
		}
		var annFilename string
		if annotationEnabled {
			_, annFilename = generateUniqueID("annotation-page", c.IDsIndex)
		}
		c.ResetPageMap()
		if c.CoverID != "" {
			c.ForceNewPage("cover.xhtml")
		}
		if tocEnabled && cfg.TOCPage.Placement == common.TOCPagePlacementBefore {
			c.ForceNewPage(tocFilename)
		}
		if annotationEnabled {
			c.ForceNewPage(annFilename)
		}
	}

	chapters, idToFile, err := convertToXHTML(ctx, c, cfg.TOCPage.ChaptersWithoutTitle, log)
	if err != nil {
		return nil, fmt.Errorf("unable to convert content: %w", err)
	}
	if c.PageSize > 0 && tocEnabled && cfg.TOCPage.Placement != common.TOCPagePlacementBefore {
		c.ForceNewPage(tocFilename)
	}

	// Rendering generated pages must not add markers or change logical page counts.
	oldPageTracking := c.PageTrackingEnabled
	c.PageTrackingEnabled = false
	defer func() { c.PageTrackingEnabled = oldPageTracking }()
	if annotationEnabled {
		chapters = append([]Chapter{generateAnnotation(c, &cfg.Annotation, log)}, chapters...)
	}
	if tocEnabled {
		tocChapter := generateTOCPage(c, chapters, &cfg.TOCPage, idToFile, log)
		if cfg.TOCPage.Placement == common.TOCPagePlacementBefore {
			chapters = append([]Chapter{tocChapter}, chapters...)
		} else {
			chapters = append(chapters, tocChapter)
		}
	}

	// Generated pages can add IDs and links of their own.
	idToFile = collectRenderedIDs(chapters, log)
	fixInternalLinks(chapters, idToFile, log)
	if c.CoverID != "" {
		c.TrackImageUsage(c.CoverID)
	}
	filterUnusedImages(c.Content)

	return &Publication{
		Chapters:   chapters,
		IDToFile:   idToFile,
		Navigation: CollectNavigation(chapters, idToFile),
		Pages:      slices.Collect(c.GetAllPagesSeq()),
		Stylesheet: AssembleStylesheet(c.Book.Stylesheets),
		Images:     c.ImagesIndex,
		CoverID:    c.CoverID,
	}, nil
}
