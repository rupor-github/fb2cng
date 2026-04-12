package pdf

import (
	"context"
	"fmt"
	"strings"

	"github.com/carlos7ags/folio/document"
	"github.com/carlos7ags/folio/font"
	"github.com/carlos7ags/folio/layout"
	"go.uber.org/zap"

	"fbc/config"
	"fbc/content"
	"fbc/convert/structure"
)

// Generate creates a minimal PDF from the renderer-neutral structural plan.
// This is an internal skeleton used to validate page geometry and Folio integration.
func Generate(ctx context.Context, c *content.Content, outputPath string, cfg *config.DocumentConfig, log *zap.Logger) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	plan, err := structure.BuildPlan(c)
	if err != nil {
		return fmt.Errorf("build structure plan: %w", err)
	}

	geom := GeometryFromConfig(cfg)
	doc := document.NewDocument(geom.PageSize)
	doc.SetMargins(geom.Margins)
	doc.SetAutoBookmarks(true)
	applyMetadata(doc, c)

	if err := addPlan(doc, plan); err != nil {
		return fmt.Errorf("render structure plan: %w", err)
	}

	if err := ctx.Err(); err != nil {
		return err
	}

	if err := doc.Save(outputPath); err != nil {
		return fmt.Errorf("save pdf: %w", err)
	}
	log.Info("PDF generation completed", zap.String("output", outputPath), zap.Int("units", len(plan.Units)))
	return nil
}

func applyMetadata(doc *document.Document, c *content.Content) {
	if doc == nil || c == nil || c.Book == nil {
		return
	}

	book := c.Book
	doc.Info.Title = book.Description.TitleInfo.BookTitle.Value

	var authors []string
	for _, a := range book.Description.TitleInfo.Authors {
		name := strings.TrimSpace(strings.Join([]string{a.FirstName, a.MiddleName, a.LastName}, " "))
		if name != "" {
			authors = append(authors, name)
		}
	}
	if len(authors) > 0 {
		doc.Info.Author = strings.Join(authors, ", ")
	}

	if book.Description.TitleInfo.Annotation != nil {
		doc.Info.Subject = book.Description.TitleInfo.Annotation.AsPlainText()
	}
	if c.SrcName != "" {
		doc.Info.Creator = "fbc"
	}
}

func addPlan(doc *document.Document, plan *structure.Plan) error {
	if doc == nil || plan == nil {
		return nil
	}

	for i, unit := range plan.Units {
		if i > 0 {
			doc.Add(layout.NewAreaBreak())
		}

		title := strings.TrimSpace(unit.Title)
		if title == "" {
			title = fallbackUnitTitle(unit)
		}

		headingLevel := layout.H1
		if unit.TitleDepth > 1 {
			switch unit.TitleDepth {
			case 2:
				headingLevel = layout.H2
			case 3:
				headingLevel = layout.H3
			case 4:
				headingLevel = layout.H4
			case 5:
				headingLevel = layout.H5
			default:
				headingLevel = layout.H6
			}
		}

		doc.Add(layout.NewHeading(title, headingLevel))

		body := layout.NewParagraph(unitSummary(unit), font.Helvetica, 12)
		body.SetSpaceBefore(6)
		body.SetSpaceAfter(12)
		doc.Add(body)
	}

	if len(plan.Units) == 0 {
		doc.Add(layout.NewParagraph("Empty document", font.Helvetica, 12))
	}

	return nil
}

func fallbackUnitTitle(unit structure.Unit) string {
	switch unit.Kind {
	case structure.UnitCover:
		return "Cover"
	case structure.UnitBodyImage:
		return "Body image"
	case structure.UnitBodyIntro:
		return "Body intro"
	case structure.UnitFootnotesBody:
		return "Notes"
	case structure.UnitSection:
		return "Section"
	default:
		return "Document"
	}
}

func unitSummary(unit structure.Unit) string {
	switch unit.Kind {
	case structure.UnitCover:
		return "Cover page"
	case structure.UnitBodyImage:
		return "Body image split due to page-break-before on body title."
	case structure.UnitBodyIntro:
		return "Body introduction unit."
	case structure.UnitFootnotesBody:
		return "Footnotes body unit."
	case structure.UnitSection:
		if unit.IsTopLevel {
			return fmt.Sprintf("Top-level section at depth %d.", unit.Depth)
		}
		return fmt.Sprintf("Nested section at depth %d and title depth %d.", unit.Depth, unit.TitleDepth)
	default:
		return "Structural unit."
	}
}
