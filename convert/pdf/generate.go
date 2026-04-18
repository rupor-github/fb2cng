package pdf

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/carlos7ags/folio/document"
	"github.com/carlos7ags/folio/layout"
	"go.uber.org/zap"

	"fbc/common"
	"fbc/config"
	"fbc/content"
	"fbc/convert/margins"
	"fbc/convert/structure"
	"fbc/fb2"
)

type splitSection struct {
	section    *fb2.Section
	depth      int
	titleDepth int
	remaining  []fb2.FlowItem
	parentUnit *structure.Unit
}

type renderContext struct {
	c                *content.Content
	doc              *document.Document
	styles           *styleResolver
	fonts            *fontRegistry
	log              *zap.Logger
	bodyStyle        resolvedStyle                       // resolved CSS body style — root of the inherited property cascade
	contentHeight    float64                             // usable page content height in points (page height minus vertical margins)
	deviceDPI        float64                             // device screen resolution (pixels per inch)
	marginMeta       map[*layout.Div]*marginMeta         // margin collapsing container metadata, keyed by Div pointer
	emptyLineSignals map[layout.Element]*emptyLineSignal // empty-line margin signals, keyed by element pointer
	atPageTop        bool                                // true when the renderer is at the top of a (new) page — suppresses duplicate AreaBreaks
	anchors          *anchorTracker                      // registers FB2 ids as named destinations during layout
	tracer           *PDFTracer                          // accumulates pipeline-state events for the debug report; nil-safe no-op when disabled
}

// addElement adds an element to the document, deduplicating consecutive
// AreaBreaks and preserving first-element margins at page top.
//
// AreaBreak deduplication: the folio renderer always flushes the current
// page when it sees an AreaBreak, even when the page is empty — producing
// a blank page.  By tracking whether we are already at page-top (after a
// prior break or at document start) we silently drop the redundant break.
//
// Margin preservation: folio's stripLeadingOffset (render_plans.go) removes
// the SpaceBefore of the first element on every new page.  This is wrong
// for us because element margins (e.g. heading wrapper's 2em top margin)
// are font-size dependent and must be preserved.  We insert a zero-height
// sentinel Div as the true first element — it absorbs stripLeadingOffset
// (nothing to strip since its Y is 0), flips atPageTop to false inside
// folio's renderer, and the real content element keeps its SpaceBefore.
func (rc *renderContext) addElement(elem layout.Element) {
	if _, isBreak := elem.(*layout.AreaBreak); isBreak {
		if rc.atPageTop {
			rc.tracer.TraceAreaBreak("suppressed")
			return // suppress duplicate page break
		}
		rc.tracer.TraceAreaBreak("added")
		rc.doc.Add(elem)
		rc.atPageTop = true
		return
	}
	rc.tracer.TraceAddElement(elem, rc.atPageTop)
	if rc.atPageTop {
		// Zero-height sentinel: absorbs folio's stripLeadingOffset so the
		// real first element's SpaceBefore is preserved.
		rc.doc.Add(layout.NewDiv())
	}
	// Wrap with a page probe so that every top-level element observes
	// its Draw-time '*PageResult' pointer.  This keeps the anchor tracker's
	// pointer→index map in sync with folio's physical page order even
	// on pages that carry no anchor ids — required because the tracker
	// assigns sequential indices on first sight and relies on every
	// page being visited in order.
	rc.doc.Add(newPageProbe(elem, rc.anchors))
	rc.atPageTop = false
}

// emptyLineSignal stores empty-line margin collapsing annotations for a
// folio element.  These are consumed by buildMarginTree to set the
// corresponding fields on ContentNode (Phase 0 of the collapse algorithm).
type emptyLineSignal struct {
	StripMarginBottom     bool
	EmptyLineMarginTop    *float64
	EmptyLineMarginBottom *float64
}

type flowBuilder struct {
	ctx                    *renderContext
	elements               *[]layout.Element
	ancestors              []styleScope
	parent                 resolvedStyle
	depth                  int      // nesting depth: 0 = top-level unit elements, >0 = inside a container Div
	pendingEmptyLineMargin *float64 // margin-top from a preceding empty-line, to be applied to the next element
	pendingAnchors         []string // FB2 ids queued to attach to the next element appended to *elements
}

type textContext struct {
	ancestors []styleScope
	parent    resolvedStyle
	linkURI   string
	hyphenate bool
}

// Generate creates a PDF using the PDF-local style and rendering pipeline.
func Generate(ctx context.Context, c *content.Content, outputPath string, cfg *config.DocumentConfig, log *zap.Logger) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if c == nil || c.Book == nil {
		return fmt.Errorf("nil content or book")
	}
	if log == nil {
		log = zap.NewNop()
	}

	plan, err := structure.BuildPlan(c)
	if err != nil {
		return fmt.Errorf("build structure plan: %w", err)
	}

	parsed := buildCombinedStylesheet(c.Book.Stylesheets, log)
	styles := newStyleResolverFromParsed(parsed, log)
	detectDropcapPatterns(parsed, log)
	bodyStyle := defaultResolvedStyle()
	if styles != nil {
		bodyStyle = styles.Resolve("body", "", nil, defaultResolvedStyle())
	}
	logBodyStyle(log, bodyStyle)

	geom := GeometryFromStyles(cfg, bodyStyle)
	doc := document.NewDocument(geom.PageSize)
	doc.SetMargins(geom.Margins)
	// Disable folio's auto-bookmark generation: it derives the outline
	// tree from heading tags (H1..H6) and is therefore capped at the
	// HTML heading ceiling.  We build the outline manually from
	// plan.TOC after layout (see outlineFinalizer below), which
	// supports unlimited nesting depth — matching KFX/EPUB TOC trees
	// verbatim for deeply nested FB2 wrong-nesting structures.
	doc.SetAutoBookmarks(false)
	doc.SetTagged(true)
	applyMetadata(doc, c)

	rc := &renderContext{
		c:                c,
		doc:              doc,
		styles:           styles,
		fonts:            newFontRegistry(c.Book.Stylesheets, parsed, log),
		log:              log.Named("pdf"),
		bodyStyle:        bodyStyle,
		contentHeight:    geom.PageSize.Height - geom.Margins.Top - geom.Margins.Bottom,
		deviceDPI:        float64(cfg.Images.Screen.DPI),
		marginMeta:       make(map[*layout.Div]*marginMeta),
		emptyLineSignals: make(map[layout.Element]*emptyLineSignal),
		atPageTop:        true, // document starts at the top of the first page
		tracer:           NewPDFTracer(c.WorkDir),
	}
	rc.anchors = newAnchorTracker(doc, rc.tracer)
	defer func() {
		if path := rc.tracer.Flush(); path != "" {
			rc.log.Debug("PDF trace written", zap.String("path", path))
		}
	}()

	if err := addPlan(rc, plan); err != nil {
		return fmt.Errorf("render structure plan: %w", err)
	}

	// Install the outline finalizer as the very last element in the
	// document.  Its Draw closure fires during layout after every
	// preceding element's anchor registrations have been recorded, so
	// tracker.idPage is complete by the time the finalizer walks
	// plan.TOC and populates doc.outlines.  This runs before folio's
	// WriteTo serializes the outline tree, so manual outlines win
	// over auto-bookmarks (which are disabled in any case).
	doc.Add(newOutlineFinalizer(doc, plan.TOC, rc.anchors, rc.tracer))

	if err := ctx.Err(); err != nil {
		return err
	}

	if err := savePDF(doc, outputPath); err != nil {
		return err
	}
	log.Info("PDF generation completed", zap.String("output", outputPath), zap.Int("units", len(plan.Units)))
	return nil
}

// savePDF writes the document through a buffered writer so folio's many
// small writes are coalesced into large OS-level writes.  This avoids
// thousands of individual write(2) syscalls that are particularly expensive
// on cross-boundary filesystems (e.g. WSL2 drvfs mounts).
func savePDF(doc *document.Document, path string) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("save pdf: %w", err)
	}
	bw := bufio.NewWriterSize(f, 256*1024)
	if _, err := doc.WriteTo(bw); err != nil {
		_ = f.Close()
		return fmt.Errorf("save pdf: %w", err)
	}
	if err := bw.Flush(); err != nil {
		_ = f.Close()
		return fmt.Errorf("save pdf: %w", err)
	}
	return f.Close()
}

func applyMetadata(doc *document.Document, c *content.Content) {
	if doc == nil || c == nil || c.Book == nil {
		return
	}

	book := c.Book
	doc.Info.Title = encodePDFTextString(book.Description.TitleInfo.BookTitle.Value)

	var authors []string
	for _, a := range book.Description.TitleInfo.Authors {
		name := strings.TrimSpace(strings.Join([]string{a.FirstName, a.MiddleName, a.LastName}, " "))
		if name != "" {
			authors = append(authors, name)
		}
	}
	if len(authors) > 0 {
		doc.Info.Author = encodePDFTextString(strings.Join(authors, ", "))
	}

	if book.Description.TitleInfo.Annotation != nil {
		doc.Info.Subject = encodePDFTextString(book.Description.TitleInfo.Annotation.AsPlainText())
	}
	if c.SrcName != "" {
		doc.Info.Creator = "fbc"
	}
}

// logBodyStyle logs the resolved body CSS properties that will cascade
// to all content elements.  Mirrors the KFX "Detected body font rule"
// debug log but also covers non-font inherited properties.
func logBodyStyle(log *zap.Logger, style resolvedStyle) {
	if log == nil {
		return
	}
	defaults := defaultResolvedStyle()
	fields := []zap.Field{
		zap.String("font-family", style.FontFamily),
		zap.String("font-size", fmt.Sprintf("%.4gpt", style.FontSize)),
		zap.String("line-height", fmt.Sprintf("%.4g", style.LineHeight)),
	}
	if style.Bold != defaults.Bold {
		fields = append(fields, zap.Bool("font-weight-bold", style.Bold))
	}
	if style.Italic != defaults.Italic {
		fields = append(fields, zap.Bool("font-style-italic", style.Italic))
	}
	if style.HasColor && style.Color != defaults.Color {
		fields = append(fields, zap.String("color", fmt.Sprintf("#%02x%02x%02x", style.Color.R, style.Color.G, style.Color.B)))
	}
	if style.LetterSpacing != defaults.LetterSpacing {
		fields = append(fields, zap.String("letter-spacing", fmt.Sprintf("%.4gpt", style.LetterSpacing)))
	}
	if style.Hyphens != defaults.Hyphens {
		fields = append(fields, zap.String("hyphens", style.Hyphens))
	}
	if style.Align != defaults.Align {
		fields = append(fields, zap.String("text-align", alignString(style.Align)))
	}
	if style.TextIndent != defaults.TextIndent {
		fields = append(fields, zap.String("text-indent", fmt.Sprintf("%.4gpt", style.TextIndent)))
	}
	log.Debug("Resolved body style (CSS cascade root)", fields...)
}

func alignString(a layout.Align) string {
	switch a {
	case layout.AlignLeft:
		return "left"
	case layout.AlignCenter:
		return "center"
	case layout.AlignRight:
		return "right"
	case layout.AlignJustify:
		return "justify"
	default:
		return "left"
	}
}

func addPlan(rc *renderContext, plan *structure.Plan) error {
	if rc == nil || rc.doc == nil || plan == nil {
		return nil
	}

	if len(plan.Units) == 0 {
		rc.addElement(newParagraphElement(rc, "p", "", nil, defaultResolvedStyle(), "Empty document"))
		return nil
	}

	for i := range plan.Units {
		unit := &plan.Units[i]
		if i > 0 && unit.ForceNewPage {
			rc.addElement(layout.NewAreaBreak())
		}
		if err := addUnit(rc, unit); err != nil {
			return err
		}
	}

	return nil
}

func addUnit(rc *renderContext, unit *structure.Unit) error {
	if rc == nil || unit == nil {
		return nil
	}

	switch unit.Kind {
	case structure.UnitCover:
		return addCoverUnit(rc)
	case structure.UnitBodyImage:
		return addBodyImageUnit(rc, unit.Body)
	case structure.UnitBodyIntro:
		return addBodyIntroUnit(rc, unit.ID, unit.Body)
	case structure.UnitFootnotesBody:
		return addFootnotesBodyUnit(rc, unit.ID, unit.Body)
	case structure.UnitSection:
		return addSectionUnit(rc, unit)
	default:
		return nil
	}
}

func addCoverUnit(rc *renderContext) error {
	if rc == nil || rc.c == nil || rc.c.CoverID == "" {
		return nil
	}
	img := rc.c.ImagesIndex[rc.c.CoverID]
	if img == nil {
		return nil
	}
	elem, err := newWrappedImageElement(rc, img, img.Filename, "image", "image-block", nil, "", img.Filename)
	if err != nil {
		return err
	}
	rc.addElement(elem)
	return nil
}

func addBodyImageUnit(rc *renderContext, body *fb2.Body) error {
	if body == nil || body.Image == nil {
		return nil
	}
	elem, err := newBlockBookImageElement(rc, body.Image, "image", "image-block", nil)
	if err != nil {
		return err
	}
	rc.addElement(elem)
	return nil
}

func addBodyIntroUnit(rc *renderContext, unitID string, body *fb2.Body) error {
	if rc == nil || body == nil {
		return nil
	}
	var elements []layout.Element
	b := flowBuilder{ctx: rc, elements: &elements, parent: rc.bodyStyle}
	b.emitAnchor(unitID)
	b.renderBodyIntro(body)
	collapseMargins(elements, rc.marginMeta, rc.emptyLineSignals, rc.tracer)
	for _, elem := range elements {
		rc.addElement(elem)
	}
	return nil
}

func addFootnotesBodyUnit(rc *renderContext, unitID string, body *fb2.Body) error {
	if rc == nil || body == nil {
		return nil
	}
	var elements []layout.Element
	b := flowBuilder{ctx: rc, elements: &elements, parent: rc.bodyStyle}
	b.emitAnchor(unitID)
	if body.Title != nil {
		b.renderTitleBlock(body.Title, "footnote-title", 1, false)
	}
	b.renderEpigraphs(body.Epigraphs, 1)
	for i := range body.Sections {
		b.renderFootnoteSection(&body.Sections[i])
	}
	collapseMargins(elements, rc.marginMeta, rc.emptyLineSignals, rc.tracer)
	for _, elem := range elements {
		rc.addElement(elem)
	}
	return nil
}

func addSectionUnit(rc *renderContext, unit *structure.Unit) error {
	if rc == nil || unit == nil || unit.Section == nil {
		return nil
	}
	queue := []splitSection{{
		section:    unit.Section,
		depth:      unit.Depth,
		titleDepth: unit.TitleDepth,
		parentUnit: unit,
	}}
	first := true
	for len(queue) > 0 {
		work := queue[0]
		queue = queue[1:]
		if !first {
			rc.addElement(layout.NewAreaBreak())
		}
		first = false

		elements, splits, err := renderSplitSection(rc, &work)
		if err != nil {
			return err
		}
		for _, elem := range elements {
			rc.addElement(elem)
		}
		next := append([]splitSection{}, splits...)
		for _, rem := range work.remaining {
			if rem.Kind != fb2.FlowSection || rem.Section == nil {
				continue
			}
			next = append(next, splitSection{
				section:    rem.Section,
				depth:      work.depth,
				titleDepth: work.titleDepth,
				parentUnit: work.parentUnit,
			})
		}
		queue = append(next, queue...)
	}
	return nil
}

func renderSplitSection(rc *renderContext, work *splitSection) ([]layout.Element, []splitSection, error) {
	if rc == nil || work == nil || work.section == nil {
		return nil, nil, nil
	}
	var elements []layout.Element
	b := flowBuilder{ctx: rc, elements: &elements, parent: rc.bodyStyle}
	splits := b.renderSection(work.section, work.depth, work.titleDepth)
	for i := range splits {
		splits[i].parentUnit = work.parentUnit
	}
	collapseMargins(elements, rc.marginMeta, rc.emptyLineSignals, rc.tracer)
	return elements, splits, nil
}

func (b *flowBuilder) renderBodyIntro(body *fb2.Body) {
	if body == nil {
		return
	}
	if !b.ctx.c.Book.BodyTitleNeedsBreak() && body.Image != nil {
		b.renderImage(body.Image, "image image-block", nil)
	}
	if body.Title != nil {
		b.renderBodyTitle(body)
	}
	b.renderEpigraphs(body.Epigraphs, 1)
}

func (b *flowBuilder) renderBodyTitle(body *fb2.Body) {
	if body == nil || body.Title == nil {
		return
	}
	var elems []layout.Element
	child := b.descend("div", "body-title", b.resolve("div", "body-title"), &elems)
	if body.Main() {
		child.renderVignette(common.VignettePosBookTitleTop, "vignette vignette-book-title-top")
	}
	child.renderTitleHeading(body.Title, 1, "body-title-header")
	if body.Main() {
		child.renderVignette(common.VignettePosBookTitleBottom, "vignette vignette-book-title-bottom")
	}
	b.pushWrappedTagged("div", "body-title", elems, margins.ContainerTitleBlock, margins.FlagTitleBlockMode)
}

func (b *flowBuilder) renderFootnoteSection(section *fb2.Section) {
	if section == nil {
		return
	}
	var elems []layout.Element
	child := b.descend("div", "footnote", b.resolve("div", "footnote"), &elems)
	child.emitAnchor(section.ID)
	if section.Title != nil {
		child.renderTitleBlock(section.Title, "footnote-title", 1, false)
	}
	child.renderEpigraphs(section.Epigraphs, 1)
	if section.Image != nil {
		child.renderImage(section.Image, "image image-block", nil)
	}
	if section.Annotation != nil {
		child.renderAnnotation(section.Annotation, 1, 1)
	}
	child.renderFlowItems(section.Content, 1, 1, "section")
	b.pushWrappedTagged("div", "footnote", elems, margins.ContainerFootnote, 0)
}

func (b *flowBuilder) renderSection(section *fb2.Section, depth int, titleDepth int) []splitSection {
	if section == nil {
		return nil
	}
	b.emitAnchor(section.ID)
	if section.Title != nil {
		b.renderSectionTitle(section, depth, titleDepth)
	}
	b.renderEpigraphs(section.Epigraphs, depth)
	if section.Image != nil {
		b.renderImage(section.Image, "image image-block", nil)
	}

	contentTitleDepth := titleDepth
	if section.HasTitle() {
		contentTitleDepth = titleDepth + 1
	}
	if section.Annotation != nil {
		b.renderAnnotation(section.Annotation, depth, contentTitleDepth)
	}
	splits := b.renderFlowItems(section.Content, depth, contentTitleDepth, "section")
	if len(splits) == 0 && section.Title != nil {
		if titleDepth == 1 {
			b.renderVignette(common.VignettePosChapterEnd, "vignette vignette-chapter-end")
		} else {
			b.renderVignette(common.VignettePosSectionEnd, "vignette vignette-section-end")
		}
	}
	return splits
}

// renderSectionTitle renders a section's title block.  Two distinct
// "depth" quantities participate:
//
//   - depth is the structural DOM depth of the section (root body = 0,
//     top-level <section> = 1, and +1 per <section> level regardless
//     of whether intermediate wrappers carry a title).  Currently
//     unused here — kept in the signature because callers propagate
//     it for symmetry with other render paths and for potential
//     future use (e.g. structure-tag role overrides).
//
//   - titleDepth counts only titled sections (untitled wrappers do NOT
//     increment it).  It drives the CSS wrapper class selector
//     (.chapter-title vs .section-title-hN), the visual heading font
//     size (via H1..H6 tag selection, capped at 6), and the
//     page-break decisions that hinge on those classes.
//
// The PDF outline is built separately from plan.TOC by
// outlineFinalizer (no HN depth cap).  The heading tag produced here
// is therefore a purely visual concern and is capped at H6 to match
// KFX's visual cap and to honor HTML's heading ceiling.  Heading tags
// no longer participate in bookmark generation because
// doc.SetAutoBookmarks is disabled.
func (b *flowBuilder) renderSectionTitle(section *fb2.Section, depth, titleDepth int) {
	_ = depth
	if section == nil || section.Title == nil {
		return
	}
	var wrapperClass string
	var headerClass string
	var topPos, bottomPos common.VignettePos
	if titleDepth == 1 {
		wrapperClass = "chapter-title"
		headerClass = "chapter-title-header"
		topPos = common.VignettePosChapterTitleTop
		bottomPos = common.VignettePosChapterTitleBottom
	} else {
		wrapperClass = fmt.Sprintf("section-title section-title-h%d", min(titleDepth, 6))
		headerClass = "section-title-header"
		topPos = common.VignettePosSectionTitleTop
		bottomPos = common.VignettePosSectionTitleBottom
	}

	// Visual heading level: use titleDepth capped at 6 so deeply nested
	// titled sections still get a valid HN tag.  titleDepth==1 maps to
	// H1 (chapter); deeper levels map to H2..H6.  The cap is purely
	// visual — TOC depth is unlimited and built manually from plan.TOC.
	headingLevel := min(max(titleDepth, 1), 6)

	var elems []layout.Element
	child := b.descend("div", wrapperClass, b.resolve("div", wrapperClass), &elems)
	child.renderVignette(topPos, "vignette vignette-"+topPos.String())
	child.renderTitleHeading(section.Title, headingLevel, headerClass)
	child.renderVignette(bottomPos, "vignette vignette-"+bottomPos.String())
	b.pushWrappedTagged("div", wrapperClass, elems, margins.ContainerTitleBlock, margins.FlagTitleBlockMode)
}

func (b *flowBuilder) renderTitleBlock(title *fb2.Title, classPrefix string, headingLevel int, asHeading bool) {
	if title == nil {
		return
	}
	if asHeading {
		b.renderTitleHeading(title, headingLevel, classPrefix)
		return
	}
	var elems []layout.Element
	child := b.descend("div", classPrefix, b.resolve("div", classPrefix), &elems)
	firstParagraph := true
	for _, item := range title.Items {
		if item.Paragraph != nil {
			class := classPrefix + "-next"
			if firstParagraph {
				class = classPrefix + "-first"
				firstParagraph = false
			}
			if item.Paragraph.Style != "" {
				class += " " + item.Paragraph.Style
			}
			child.renderParagraph(item.Paragraph, "p", class)
			child.consumePendingEmptyLine()
			continue
		}
		if item.EmptyLine {
			child.handleEmptyLine(classPrefix + "-emptyline")
		}
	}
	b.pushWrappedTagged("div", classPrefix, elems, margins.ContainerTitleBlock, margins.FlagTitleBlockMode)
}

// renderTitleHeading renders a FB2 <title> as a sequence of block-level
// elements: a layout.Heading for the first <p> (providing PDF bookmark
// and structure tag) and layout.Paragraphs for subsequent <p>s, all
// styled with the classPrefix + "-first"/"-next" CSS variants.
// <empty-line/> items are handled via the margin-absorption machinery
// so .classPrefix-emptyline's CSS margin becomes a visible gap.
//
// This mirrors KFX's addTitleAsParagraphs path.  The combined-heading
// path used by KFX (addTitleAsHeading) cannot be replicated faithfully
// in PDF because folio's layout.Paragraph is a single block-level unit
// with no support for inline display:block boundaries.  Using separate
// elements preserves:
//
//   - Per-line CSS styling differences between -first and -next (e.g.
//     bold title line vs italic author line).
//   - Inline images inside title paragraphs (not representable inside
//     a single folio Heading's text runs).
//   - Empty-line margins from the .classPrefix-emptyline CSS rule
//     (via handleEmptyLine / consumePendingEmptyLine).
//
// Only the first element carries the heading's bookmark entry: the
// bookmarkLabel is set to the combined plain text of all <p>s so the
// outline matches KFX's auto-bookmark behavior.  Subsequent elements
// are plain styled paragraphs — their class resolution places them in
// the heading's ancestor chain so font-size and weight inherit
// correctly from the heading scope.
func (b *flowBuilder) renderTitleHeading(title *fb2.Title, headingLevel int, classPrefix string) {
	if title == nil {
		return
	}

	// Resolve the heading tag once; it participates both in style
	// resolution (h1-h6 selectors drive font-size) and in the ancestor
	// chain we push for subsequent paragraphs so their descendant
	// selectors (e.g. "h1 strong") match correctly.
	hTag := headingTag(headingLevel)
	headingStyle := defaultResolvedStyle()
	if b.ctx != nil && b.ctx.styles != nil {
		headingStyle = b.ctx.styles.Resolve(hTag, classPrefix, b.ancestors, b.parent)
	}

	// Ancestor chain entered when rendering title-body paragraphs
	// (both the first-line Heading and subsequent -next lines).  Pushing
	// the heading scope lets descendant CSS selectors match and lets
	// paragraph font-size inherit the heading's computed size.
	headingAncestors := append(append([]styleScope{}, b.ancestors...), styleScope{Tag: hTag, Classes: splitClasses(classPrefix)})

	// Compute the combined plain-text bookmark label using the same
	// ". "-joined format KFX and EPUB use for TOC entries (see
	// fb2.Title.AsTOCText in fb2/types.go:403).  This keeps the PDF
	// outline, EPUB nav, and KFX navigation consistent for multi-
	// paragraph titles and uses image-alt text as a fallback when a
	// title contains only inline images.  The "Untitled" fallback
	// never surfaces for non-empty titles so we pass "" to suppress
	// it entirely here; renderSectionTitle's caller already guards
	// against empty titles.
	bookmarkLabel := title.AsTOCText("")

	// Render each title item as a separate element in the builder's
	// current element slice.  The first paragraph becomes a Heading so
	// it receives the PDF bookmark and H1-H6 structure tag.  Subsequent
	// paragraphs become regular styled paragraphs rendered through
	// renderParagraph — they inherit the heading's font-size via the
	// ancestors chain and pick up -next class styling.
	firstParagraph := true
	for _, item := range title.Items {
		if item.Paragraph != nil {
			class := classPrefix + "-next"
			if firstParagraph {
				class = classPrefix + "-first"
				firstParagraph = false
				b.renderTitleFirstParagraph(item.Paragraph, hTag, classPrefix, class, headingLevel, headingStyle, headingAncestors, bookmarkLabel)
				continue
			}
			if item.Paragraph.Style != "" {
				class += " " + item.Paragraph.Style
			}
			// Re-derive the title builder each iteration so pending
			// empty-line / anchor state accumulated on b since the
			// previous element (e.g. a <empty-line/> between title
			// paragraphs) is picked up by renderParagraph.
			titleBuilder := b.withAncestors(headingAncestors, headingStyle)
			titleBuilder.renderParagraph(item.Paragraph, "p", class)
			// Mirror any pending state changes renderParagraph left on
			// the derived builder back onto b so subsequent iterations
			// (and the caller) stay synchronized.
			b.pendingAnchors = titleBuilder.pendingAnchors
			b.pendingEmptyLineMargin = titleBuilder.pendingEmptyLineMargin
			continue
		}
		if item.EmptyLine {
			b.handleEmptyLine(classPrefix + "-emptyline")
		}
	}
}

// renderTitleFirstParagraph renders the first paragraph of a title as
// a folio Heading so it contributes a PDF bookmark / outline entry and
// an H1-H6 structure tag.  If the paragraph contains inline images the
// Heading would drop them (folio headings are text-only), so in that
// case we fall back to a normal styled paragraph and attach an explicit
// bookmark via a zero-height marker — the same approach we use for the
// anchor decorator.
//
// class must include both classPrefix (base header style) and the
// position variant (classPrefix + "-first") because folio's style
// resolver applies both class selectors independently and the base
// style carries font-weight / text-align, while the variant may add
// display / margin / size overrides.
func (b *flowBuilder) renderTitleFirstParagraph(p *fb2.Paragraph, hTag, classPrefix, class string, headingLevel int, headingStyle resolvedStyle, headingAncestors []styleScope, bookmarkLabel string) {
	if p == nil {
		return
	}
	hyphenate := !p.Special && b.ctx != nil && b.ctx.c != nil && b.ctx.c.Hyphen != nil
	runs := b.paragraphRuns(p, class, textContext{ancestors: headingAncestors, parent: headingStyle, hyphenate: hyphenate})

	// Decide: can we represent the first paragraph as a folio Heading?
	// A Heading only holds text runs; if the paragraph produced no runs
	// (e.g. image-only title line) we must fall back to a styled paragraph.
	if len(runs) == 0 {
		// Image-only or empty-title-paragraph.  Render as a styled
		// paragraph using the -first class so CSS margins still apply;
		// paragraph factories handle inline images via renderParagraph.
		titleBuilder := b.withAncestors(headingAncestors, headingStyle)
		titleBuilder.renderParagraph(p, "p", class)
		b.pendingAnchors = titleBuilder.pendingAnchors
		b.pendingEmptyLineMargin = titleBuilder.pendingEmptyLineMargin
		return
	}

	heading := newHeadingElement(b.ctx, hTag, class, headingStyle, headingLevel, runs, bookmarkLabel)
	b.emitAnchor(p.ID)
	*b.elements = append(*b.elements, b.attachAnchors(heading))
}

// withAncestors returns a derived flowBuilder that shares the receiver's
// element slice, context, and pending state, but uses the supplied
// ancestor chain and parent style for subsequent resolve() / render*
// calls.  This lets renderTitleHeading delegate paragraph rendering to
// renderParagraph while keeping the heading scope in the ancestor chain
// so descendant selectors (e.g. "h1 strong", ".body-title-header-next")
// match correctly.
//
// Crucially the returned builder writes to the same elements slice as
// the receiver, so ordering of emitted paragraphs is preserved and the
// pending-empty-line / anchor queues remain synchronized.
func (b *flowBuilder) withAncestors(ancestors []styleScope, parent resolvedStyle) flowBuilder {
	return flowBuilder{
		ctx:                    b.ctx,
		elements:               b.elements,
		ancestors:              ancestors,
		parent:                 parent,
		pendingAnchors:         b.pendingAnchors,
		pendingEmptyLineMargin: b.pendingEmptyLineMargin,
	}
}

func headingTag(level int) string {
	switch min(max(level, 1), 6) {
	case 1:
		return "h1"
	case 2:
		return "h2"
	case 3:
		return "h3"
	case 4:
		return "h4"
	case 5:
		return "h5"
	default:
		return "h6"
	}
}

func (b *flowBuilder) renderEpigraphs(epigraphs []fb2.Epigraph, depth int) {
	for i := range epigraphs {
		epigraph := &epigraphs[i]
		var elems []layout.Element
		child := b.descend("div", "epigraph", b.resolve("div", "epigraph"), &elems)
		child.renderFlowItems(epigraph.Flow.Items, depth, depth, "epigraph")
		for j := range epigraph.TextAuthors {
			child.renderParagraph(&epigraph.TextAuthors[j], "p", "text-author")
		}
		b.pushWrappedTagged("div", "epigraph", elems, margins.ContainerEpigraph, margins.FlagTransferMBToLastChild)
	}
}

func (b *flowBuilder) renderAnnotation(flow *fb2.Flow, depth int, titleDepth int) {
	if flow == nil {
		return
	}
	var elems []layout.Element
	child := b.descend("div", "annotation", b.resolve("div", "annotation"), &elems)
	child.renderFlowItems(flow.Items, depth, titleDepth, "annotation")
	b.pushWrappedTagged("div", "annotation", elems, margins.ContainerAnnotation, margins.FlagForceTransferMBToLastChild)
}

func (b *flowBuilder) renderFlowItems(items []fb2.FlowItem, depth int, titleDepth int, context string) []splitSection {
	var splits []splitSection
	for i := range items {
		item := &items[i]
		switch item.Kind {
		case fb2.FlowParagraph:
			if item.Paragraph != nil {
				b.renderParagraph(item.Paragraph, "p", item.Paragraph.Style)
				b.consumePendingEmptyLine()
			}
		case fb2.FlowImage:
			if item.Image != nil {
				b.renderImage(item.Image, "image image-block", nil)
				b.consumePendingEmptyLine()
			}
		case fb2.FlowEmptyLine:
			b.handleEmptyLine("emptyline")
		case fb2.FlowSubtitle:
			if item.Subtitle != nil {
				class := context + "-subtitle"
				if item.Subtitle.Style != "" {
					class += " " + item.Subtitle.Style
				}
				b.renderParagraph(item.Subtitle, "p", class)
				b.consumePendingEmptyLine()
			}
		case fb2.FlowPoem:
			if item.Poem != nil {
				b.renderPoem(item.Poem, depth)
				b.consumePendingEmptyLine()
			}
		case fb2.FlowCite:
			if item.Cite != nil {
				b.renderCite(item.Cite, depth)
				b.consumePendingEmptyLine()
			}
		case fb2.FlowTable:
			if item.Table != nil {
				b.renderTable(item.Table)
				b.consumePendingEmptyLine()
			}
		case fb2.FlowSection:
			if item.Section == nil {
				continue
			}
			sectionDepth := depth + 1
			childTitleDepth := titleDepth
			if item.Section.HasTitle() && b.ctx.c.Book.SectionNeedsBreak(sectionDepth) {
				// Titled sections whose heading depth has page-break-before
				// in the CSS are promoted to their own structural Unit by
				// the structure builder (convert/structure/builder.go,
				// collectSectionChildren -> addSectionUnit path).  Those
				// Units are rendered independently by addUnit later in the
				// plan loop, so we must NOT also render them inline here
				// — doing so would produce duplicate bookmarks and
				// duplicate content (see KFX's mirror: inline-vs-split
				// routing in convert/kfx/frag_storyline_content.go:270).
				continue
			}
			var elems []layout.Element
			child := b.descend("div", "section", b.resolve("div", "section"), &elems)
			child.renderSection(item.Section, sectionDepth, childTitleDepth)
			b.pushWrapped("div", "section", elems)
			b.consumePendingEmptyLine()
		}
	}
	return splits
}

func (b *flowBuilder) renderPoem(poem *fb2.Poem, depth int) {
	if poem == nil {
		return
	}
	var elems []layout.Element
	child := b.descend("div", "poem", b.resolve("div", "poem"), &elems)
	child.emitAnchor(poem.ID)
	if poem.Title != nil {
		child.renderTitleBlock(poem.Title, "poem-title", depth, false)
	}
	child.renderEpigraphs(poem.Epigraphs, depth)
	for i := range poem.Subtitles {
		child.renderParagraph(&poem.Subtitles[i], "p", "poem-subtitle")
	}
	for i := range poem.Stanzas {
		child.renderStanza(&poem.Stanzas[i], depth)
	}
	for i := range poem.TextAuthors {
		child.renderParagraph(&poem.TextAuthors[i], "p", "text-author")
	}
	if poem.Date != nil {
		dateText := poem.Date.Display
		if dateText == "" && !poem.Date.Value.IsZero() {
			dateText = poem.Date.Value.Format("2006-01-02")
		}
		if dateText != "" {
			child.renderPlainParagraph("p", "date", dateText)
		}
	}
	b.pushWrappedTagged("div", "poem", elems, margins.ContainerPoem, 0)
}

func (b *flowBuilder) renderStanza(stanza *fb2.Stanza, depth int) {
	if stanza == nil {
		return
	}
	var elems []layout.Element
	child := b.descend("div", "stanza", b.resolve("div", "stanza"), &elems)
	if stanza.Title != nil {
		child.renderTitleBlock(stanza.Title, "stanza-title", depth, false)
	}
	if stanza.Subtitle != nil {
		child.renderParagraph(stanza.Subtitle, "p", "stanza-subtitle")
	}
	for i := range stanza.Verses {
		child.renderParagraph(&stanza.Verses[i], "p", "verse")
	}
	b.pushWrappedTagged("div", "stanza", elems, margins.ContainerStanza, margins.FlagStripMiddleMarginBottom|margins.FlagTransferMBToLastChild)
}

func (b *flowBuilder) renderCite(cite *fb2.Cite, depth int) {
	if cite == nil {
		return
	}
	var elems []layout.Element
	child := b.descend("blockquote", "cite", b.resolve("blockquote", "cite"), &elems)
	child.emitAnchor(cite.ID)
	child.renderFlowItems(cite.Items, depth, depth, "cite")
	for i := range cite.TextAuthors {
		child.renderParagraph(&cite.TextAuthors[i], "p", "text-author")
	}
	b.pushWrappedTagged("blockquote", "cite", elems, margins.ContainerCite, margins.FlagTransferMBToLastChild)
}

func (b *flowBuilder) renderTable(table *fb2.Table) {
	if table == nil {
		return
	}
	style := b.resolve("table", table.Style)
	tbl := layout.NewTable().SetAutoColumnWidths().SetBorderCollapse(true)
	if style.WidthPercent > 0 {
		tbl.SetMinWidthUnit(layout.Pct(style.WidthPercent))
	} else {
		tbl.SetMinWidthUnit(layout.Pct(100))
	}
	for i := range table.Rows {
		rowData := &table.Rows[i]
		row := tbl.AddRow()
		for j := range rowData.Cells {
			cellData := &rowData.Cells[j]
			cellElem := b.newTableCellElement(cellData)
			cell := row.AddCellElement(cellElem)
			if cellData.ColSpan > 1 {
				cell.SetColspan(cellData.ColSpan)
			}
			if cellData.RowSpan > 1 {
				cell.SetRowspan(cellData.RowSpan)
			}
			cellStyle := b.resolve(tableCellTag(cellData), cellData.Style)
			applyCellStyle(cell, cellStyle)
		}
	}
	*b.elements = append(*b.elements, b.attachAnchors(wrapBlockElement(style, tbl)))
}

func tableCellTag(cell *fb2.TableCell) string {
	if cell != nil && cell.Header {
		return "th"
	}
	return "td"
}

func applyCellStyle(cell *layout.Cell, style resolvedStyle) {
	if cell == nil {
		return
	}
	if style.HasBorder {
		cell.SetBorders(layout.AllBorders(style.Border))
	}
	if style.Background != nil {
		cell.SetBackground(*style.Background)
	}
	if style.PaddingTop != 0 || style.PaddingRight != 0 || style.PaddingBottom != 0 || style.PaddingLeft != 0 {
		cell.SetPaddingSides(layout.Padding{
			Top:    max(style.PaddingTop, 0),
			Right:  max(style.PaddingRight, 0),
			Bottom: max(style.PaddingBottom, 0),
			Left:   max(style.PaddingLeft, 0),
		})
	}
	cell.SetAlign(style.Align)
}

func (b *flowBuilder) newTableCellElement(cell *fb2.TableCell) layout.Element {
	class := ""
	if cell != nil {
		class = cell.Style
	}
	style := b.resolve(tableCellTag(cell), class)
	ancestors := append(append([]styleScope{}, b.ancestors...), styleScope{Tag: tableCellTag(cell), Classes: splitClasses(class)})
	var runs []layout.TextRun
	if cell != nil {
		for i := range cell.Content {
			runs = append(runs, b.inlineRuns(&cell.Content[i], textContext{ancestors: ancestors, parent: style, hyphenate: false})...)
		}
	}
	if len(runs) == 0 {
		para := newParagraphElement(b.ctx, tableCellTag(cell), class, b.ancestors, b.parent, strings.TrimSpace(cell.AsPlainText()))
		applyParagraphStyle(para, style)
		return para
	}
	para := layout.NewStyledParagraph(runs...)
	applyParagraphStyle(para, style)
	return para
}

func (b *flowBuilder) renderParagraph(p *fb2.Paragraph, tag string, class string) {
	if p == nil {
		return
	}
	// Float-based dropcap: extract the first character and emit it as a
	// floating element before the paragraph body.  This replaces the old
	// inline-only approach so the body text wraps around the enlarged
	// initial character.
	//
	// At the top level (depth == 0) the Float element is placed directly
	// in the unit's element list where folio's render loop tracks it and
	// narrows subsequent elements.  Inside container Divs (depth > 0)
	// Div.PlanLayout does NOT track floats, so the body Paragraph would
	// get full width and overlap the dropcap.  In that case we simulate
	// the effect with a Div wrapper: padding-left reserves space for the
	// dropcap, and an AddOverlay places the enlarged character at the
	// Div's left edge.
	if hasStyle("has-dropcap", p.Style) {
		if dropChar, restSegs := extractDropcapChar(p.Text); dropChar != "" {
			if b.depth == 0 {
				// Top-level: true Float dropcap with text wrapping.
				// If an empty-line preceded this paragraph, the pending
				// margin must push both the Float AND the body Paragraph
				// down together.  We emit a zero-content spacer Paragraph
				// with SpaceBefore so the render loop advances curY before
				// placing the Float.  A spacer Paragraph (not Div) is used
				// because empty Divs are self-collapsed by the margin
				// collapsing algorithm, losing the gap.  Text-typed nodes
				// survive collapsing intact.
				b.consumeEmptyLineForDropcap()
				b.emitDropcapFloat(p, tag, class, dropChar)
				// Continue with the remaining segments — strip
				// "has-dropcap" so paragraphRuns doesn't re-inject.
				restP := *p
				restP.Text = restSegs
				restP.ID = "" // anchor already registered on the float
				restP.Style = removeStyle("has-dropcap", p.Style)
				p = &restP
			} else {
				// Nested inside a container Div: overlay-based dropcap.
				b.emitDropcapOverlay(p, tag, class, dropChar, restSegs)
				return
			}
		}
	}
	runs := b.paragraphRuns(p, class, textContext{ancestors: b.ancestors, parent: b.parent, hyphenate: !p.Special && b.ctx != nil && b.ctx.c != nil && b.ctx.c.Hyphen != nil})
	if len(runs) == 0 {
		plain := strings.TrimSpace(p.AsPlainText())
		if plain == "" {
			style := b.resolve(tag, class)
			spacer := layout.NewDiv().SetSpaceBefore(style.MarginTop).SetSpaceAfter(style.MarginBottom)
			b.emitAnchor(p.ID)
			*b.elements = append(*b.elements, b.attachAnchors(spacer))
			return
		}
		runs = []layout.TextRun{b.textRunForStyle(plain, b.resolve(tag, class))}
	}
	para := newStyledParagraphElement(b.ctx, tag, class, b.ancestors, b.parent, runs)
	b.emitAnchor(p.ID)
	*b.elements = append(*b.elements, b.attachAnchors(para))
}

// emitDropcapFloat builds a floating dropcap element for the given character
// and appends it to the element list.  The float is styled according to the
// CSS ".dropcap" class and placed to the left so subsequent paragraph text
// wraps around it.
//
// The element structure is Float → Paragraph (no Div wrapper, because a Div
// defaults to the full available width, which would make the float claim the
// entire page).  The Paragraph naturally shrinks to the character's glyph
// width.  CSS padding-right on the .dropcap class becomes the Float margin
// (the gap between the dropcap box and the wrapped text).
func (b *flowBuilder) emitDropcapFloat(p *fb2.Paragraph, tag, class, dropChar string) {
	// Resolve the dropcap style: CSS selector "p.has-dropcap .dropcap"
	// maps to resolving "span" with class "dropcap" inside the paragraph's
	// ancestor scope.
	paraStyle := b.resolve(tag, class)
	paraAncestors := append(append([]styleScope{}, b.ancestors...), styleScope{Tag: tag, Classes: splitClasses(class)})
	dropcapStyle := defaultResolvedStyle()
	if b.ctx != nil && b.ctx.styles != nil {
		dropcapStyle = b.ctx.styles.Resolve("span", "dropcap", paraAncestors, paraStyle)
	}

	// Build a single-character paragraph with the dropcap style.
	run := b.textRunForStyle(dropChar, dropcapStyle)
	dropcapPara := layout.NewStyledParagraph(run)
	dropcapPara.SetLeading(dropcapStyle.LineHeight)
	dropcapPara.SetAlign(dropcapStyle.Align)
	if dropcapStyle.Background != nil {
		dropcapPara.SetBackground(*dropcapStyle.Background)
	}

	// CSS padding-right becomes the float margin — the gap between the
	// enlarged character and the wrapped text.  This matches CSS semantics
	// where padding on a floated inline element widens the float box.
	margin := dropcapStyle.PaddingRight
	if margin <= 0 {
		margin = 2 // minimal fallback in points
	}
	floatElem := layout.NewFloat(layout.FloatLeft, dropcapPara).SetMargin(margin)

	b.emitAnchor(p.ID)
	*b.elements = append(*b.elements, b.attachAnchors(floatElem))
}

// emitDropcapOverlay renders a dropcap paragraph inside a container Div
// where folio's Float element cannot work (Div.PlanLayout does not track
// floats, so the body text would get full width and overlap the dropcap).
//
// The approach: create a wrapper Div with padding-left equal to the
// dropcap character width + gap.  The body Paragraph is a normal-flow
// child and wraps at the reduced width.  The dropcap character is placed
// via AddOverlay at a negative X offset so it appears at the Div's left
// edge.  This gives an indented-text effect: the dropcap is at the left,
// body text is to its right.  All body lines are indented (not just the
// lines adjacent to the dropcap), but this is an acceptable trade-off
// compared to the overlapping text that occurs without it.
func (b *flowBuilder) emitDropcapOverlay(p *fb2.Paragraph, tag, class, dropChar string, restSegs []fb2.InlineSegment) {
	// Resolve the dropcap style (same logic as emitDropcapFloat).
	paraStyle := b.resolve(tag, class)
	paraAncestors := append(append([]styleScope{}, b.ancestors...), styleScope{Tag: tag, Classes: splitClasses(class)})
	dropcapStyle := defaultResolvedStyle()
	if b.ctx != nil && b.ctx.styles != nil {
		dropcapStyle = b.ctx.styles.Resolve("span", "dropcap", paraAncestors, paraStyle)
	}

	// Build the dropcap Paragraph.
	dcRun := b.textRunForStyle(dropChar, dropcapStyle)
	dropcapPara := layout.NewStyledParagraph(dcRun)
	dropcapPara.SetLeading(dropcapStyle.LineHeight)
	if dropcapStyle.Background != nil {
		dropcapPara.SetBackground(*dropcapStyle.Background)
	}

	// Compute gap between dropcap box and body text.
	gap := dropcapStyle.PaddingRight
	if gap <= 0 {
		gap = 2
	}

	// Measure the dropcap's rendered width via a trial layout.
	dcPlan := dropcapPara.PlanLayout(layout.LayoutArea{Width: 1e9, Height: 1e9})
	dcWidth := 0.0
	for _, blk := range dcPlan.Blocks {
		if w := blk.X + blk.Width; w > dcWidth {
			dcWidth = w
		}
	}
	indent := dcWidth + gap

	// Build the body Paragraph from the remaining segments.
	restP := *p
	restP.Text = restSegs
	restP.ID = "" // anchor handled below on the wrapper
	restP.Style = removeStyle("has-dropcap", p.Style)
	bodyClass := restP.Style
	bodyRuns := b.paragraphRuns(&restP, bodyClass, textContext{
		ancestors: b.ancestors, parent: b.parent,
		hyphenate: !p.Special && b.ctx != nil && b.ctx.c != nil && b.ctx.c.Hyphen != nil,
	})
	var bodyElem layout.Element
	if len(bodyRuns) == 0 {
		plain := strings.TrimSpace(restP.AsPlainText())
		if plain != "" {
			bodyRuns = []layout.TextRun{b.textRunForStyle(plain, b.resolve(tag, bodyClass))}
		}
	}
	if len(bodyRuns) > 0 {
		bodyElem = newStyledParagraphElement(b.ctx, tag, bodyClass, b.ancestors, b.parent, bodyRuns)
	}

	// Assemble the wrapper Div: padding-left reserves space for the
	// dropcap; the body Paragraph wraps at the reduced width; the
	// dropcap character is placed as an overlay at X = -indent.
	wrapper := layout.NewDiv()
	wrapper.SetPaddingAll(layout.Padding{Left: indent})
	if bodyElem != nil {
		wrapper.Add(bodyElem)
	}
	wrapper.AddOverlay(dropcapPara, -indent, 0, dcWidth, false, 0)

	b.emitAnchor(p.ID)
	*b.elements = append(*b.elements, b.attachAnchors(wrapper))
}

// emitAnchor queues an FB2 id to be attached to the next element
// appended to *b.elements.  A nil/empty id is a no-op.  Anchors are
// attached via an anchoredElement decorator that piggy-backs on the
// target element's first placed block, so they contribute no extra
// layout blocks and always register on the page where the target's
// first visible content lands — even when pagination splits the
// container before the target is placed.
//
// This is CRITICAL: emitting a free-standing zero-height marker
// element causes folio's flushPage to treat it as page content,
// producing blank pages whenever the marker lands on an otherwise
// empty area (e.g. immediately after an AreaBreak).  Piggy-backing
// avoids that entirely.
//
// If no element is subsequently appended to the slice (empty container),
// the queued anchor is discarded on the builder's next-level exit.
// This matches the semantics of an id on an element with no rendered
// content: there's nothing visible to navigate to anyway.
func (b *flowBuilder) emitAnchor(id string) {
	if id == "" || b.ctx == nil || b.ctx.anchors == nil {
		return
	}
	b.pendingAnchors = append(b.pendingAnchors, id)
}

// attachAnchors wraps elem with any queued pending anchors and returns
// the wrapped element.  If there are no pending anchors, elem is
// returned unchanged.  The pending queue is drained on every call.
//
// AreaBreaks are skipped: anchors should never attach to a page-break
// sentinel since the break has no PlacedBlocks to hook into.  In that
// case the anchors remain queued for the next real element.
func (b *flowBuilder) attachAnchors(elem layout.Element) layout.Element {
	if elem == nil || len(b.pendingAnchors) == 0 {
		return elem
	}
	if _, isBreak := elem.(*layout.AreaBreak); isBreak {
		return elem
	}
	ids := b.pendingAnchors
	b.pendingAnchors = nil
	if b.ctx == nil || b.ctx.anchors == nil {
		return elem
	}
	return newAnchoredElement(elem, ids, b.ctx.anchors)
}

func (b *flowBuilder) renderPlainParagraph(tag string, class string, text string) {
	if strings.TrimSpace(text) == "" {
		return
	}
	para := newParagraphElement(b.ctx, tag, class, b.ancestors, b.parent, text)
	*b.elements = append(*b.elements, b.attachAnchors(para))
}

// handleEmptyLine implements the KFX-style margin-absorption model for
// empty-line elements.  Instead of emitting a spacer Div it:
//   - Marks the previous element for margin-bottom stripping
//   - Stores the empty-line margin (margin-top from CSS) as pending
//
// The pending margin is applied to the next element via consumePendingEmptyLine.
func (b *flowBuilder) handleEmptyLine(class string) {
	// Compute empty-line margin from CSS (use only margin-top, matching KFX).
	style := b.resolve("div", class)
	margin := style.MarginTop

	// Mark the previous element for margin-bottom stripping.
	if elems := *b.elements; len(elems) > 0 {
		prev := elems[len(elems)-1]
		sig := b.getOrCreateSignal(prev)
		sig.StripMarginBottom = true
	}

	// Store pending margin for the next element.
	if margin > 0 {
		b.pendingEmptyLineMargin = &margin
	}
}

// consumePendingEmptyLine applies any pending empty-line margin to the
// element just appended to b.elements.  Must be called after every
// element append in contexts where empty-lines can occur.
func (b *flowBuilder) consumePendingEmptyLine() {
	if b.pendingEmptyLineMargin == nil {
		return
	}
	elems := *b.elements
	if len(elems) == 0 {
		return
	}
	last := elems[len(elems)-1]
	sig := b.getOrCreateSignal(last)
	sig.EmptyLineMarginTop = b.pendingEmptyLineMargin
	b.pendingEmptyLineMargin = nil
}

// consumeEmptyLineForDropcap handles the pending empty-line margin for
// dropcap paragraphs.  Unlike consumePendingEmptyLine (which tags the
// margin on the last element for margin-tree processing), this method
// emits a zero-content spacer Paragraph whose SpaceBefore advances
// curY in the render loop.  Both the Float and the body Paragraph are
// placed after the spacer, so they share the same Y and align properly.
//
// A Paragraph is used instead of a Div because empty Divs (Index == -1,
// no children) are self-collapsed by collapseEmptyNodes, which moves
// MarginTop into MarginBottom.  The collapsed value then migrates to the
// Float via sibling collapsing, but the Float has no SpaceBefore API, so
// the gap is lost.  Text-typed leaf nodes (IsEmpty() == false) keep
// their MarginTop through all collapsing phases.
func (b *flowBuilder) consumeEmptyLineForDropcap() {
	if b.pendingEmptyLineMargin == nil {
		return
	}
	margin := *b.pendingEmptyLineMargin
	b.pendingEmptyLineMargin = nil

	spacer := layout.NewStyledParagraph()
	spacer.SetSpaceBefore(margin)
	*b.elements = append(*b.elements, spacer)
}

// getOrCreateSignal returns the empty-line signal for elem, creating one if needed.
func (b *flowBuilder) getOrCreateSignal(elem layout.Element) *emptyLineSignal {
	sig := b.ctx.emptyLineSignals[elem]
	if sig == nil {
		sig = &emptyLineSignal{}
		b.ctx.emptyLineSignals[elem] = sig
	}
	return sig
}

func (b *flowBuilder) renderImage(img *fb2.Image, class string, extraAncestors []styleScope) {
	if img == nil {
		return
	}
	elem, err := newBlockBookImageElement(b.ctx, img, "image", class, extraAncestors)
	if err != nil {
		b.ctx.log.Warn("Skipping image", zap.String("href", img.Href), zap.Error(err))
		return
	}
	b.emitAnchor(img.ID)
	*b.elements = append(*b.elements, b.attachAnchors(elem))
}

func (b *flowBuilder) renderVignette(pos common.VignettePos, class string) {
	if b.ctx == nil || b.ctx.c == nil || b.ctx.c.Book == nil || !b.ctx.c.Book.IsVignetteEnabled(pos) {
		return
	}
	id := b.ctx.c.Book.VignetteIDs[pos]
	img := b.ctx.c.ImagesIndex[id]
	if img == nil {
		return
	}
	elem, err := newWrappedImageElement(b.ctx, img, id, class, "image-vignette", nil, "", img.Filename)
	if err != nil {
		b.ctx.log.Warn("Skipping vignette", zap.String("id", id), zap.Error(err))
		return
	}
	*b.elements = append(*b.elements, b.attachAnchors(elem))
}

func (b *flowBuilder) resolve(tag, classes string) resolvedStyle {
	if b.ctx == nil || b.ctx.styles == nil {
		return b.parent
	}
	return b.ctx.styles.Resolve(tag, classes, b.ancestors, b.parent)
}

func (b *flowBuilder) descend(tag, classes string, style resolvedStyle, out *[]layout.Element) flowBuilder {
	ancestors := append([]styleScope{}, b.ancestors...)
	ancestors = append(ancestors, styleScope{Tag: tag, Classes: splitClasses(classes)})
	return flowBuilder{
		ctx:       b.ctx,
		elements:  out,
		ancestors: ancestors,
		parent:    style,
		depth:     b.depth + 1,
	}
}

func (b *flowBuilder) pushWrapped(tag, classes string, elems []layout.Element) {
	b.pushWrappedTagged(tag, classes, elems, margins.ContainerSection, 0)
}

// pushWrappedTagged wraps elements in a Div with the given tag/classes and
// records margin collapsing metadata (container kind + flags) for the Div.
// This is the tagged variant of pushWrapped used when the container has
// specific margin collapsing semantics (title-block, epigraph, poem, etc.).
func (b *flowBuilder) pushWrappedTagged(tag, classes string, elems []layout.Element, kind margins.ContainerKind, flags margins.ContainerFlags) {
	if len(elems) == 0 {
		return
	}
	style := b.resolve(tag, classes)
	container := layout.NewDiv()
	for _, elem := range elems {
		container.Add(elem)
	}
	applyDivStyle(container, style)
	tagContainer(b.ctx.marginMeta, container, kind, flags, style)
	if tag == "blockquote" {
		container.SetTag("BlockQuote")
	} else if tag == "div" && strings.Contains(classes, "section") {
		container.SetTag("Sect")
	}
	if style.BreakBefore == "always" {
		*b.elements = append(*b.elements, layout.NewAreaBreak())
	}
	*b.elements = append(*b.elements, b.attachAnchors(container))
	if style.BreakAfter == "always" {
		*b.elements = append(*b.elements, layout.NewAreaBreak())
	}
}

func wrapIfNeeded(tag, classes string, style resolvedStyle, elem layout.Element) layout.Element {
	needsWrap := style.PaddingTop != 0 || style.PaddingRight != 0 || style.PaddingBottom != 0 || style.PaddingLeft != 0 ||
		style.Background != nil || style.HasBorder || style.KeepTogether || style.WidthPercent != 0

	// Elements without a margin API (Heading, ImageElement) need a wrapper
	// Div to carry CSS/UA margins.  Paragraphs handle margins directly via
	// SetSpaceBefore/SetSpaceAfter so they don't need this.
	if !needsWrap {
		if _, ok := elem.(*layout.Heading); ok {
			needsWrap = style.MarginTop != 0 || style.MarginBottom != 0
		}
	}

	if !needsWrap {
		return elem
	}
	div := layout.NewDiv().Add(elem)
	applyDivStyle(div, style)
	if tag == "blockquote" {
		div.SetTag("BlockQuote")
	}
	return div
}

func newBlockBookImageElement(rc *renderContext, img *fb2.Image, wrapperClass string, imageClass string, extraAncestors []styleScope) (layout.Element, error) {
	if rc == nil || rc.c == nil || img == nil {
		return nil, fmt.Errorf("nil image")
	}
	bookImg, id := imageByID(rc.c, img.Href)
	if bookImg == nil {
		return nil, fmt.Errorf("image %q not found", img.Href)
	}
	return newWrappedImageElement(rc, bookImg, id, wrapperClass, imageClass, extraAncestors, img.Alt, img.Title)
}

func newWrappedImageElement(rc *renderContext, img *fb2.BookImage, imageID string, wrapperClass string, imageClass string, extraAncestors []styleScope, alt string, title string) (layout.Element, error) {
	if rc == nil || img == nil {
		return nil, fmt.Errorf("nil image")
	}
	var wrappedAncestors []styleScope
	style := rc.bodyStyle
	if rc.styles != nil {
		style = rc.styles.Resolve("div", wrapperClass, extraAncestors, rc.bodyStyle)
		wrappedAncestors = append(append([]styleScope{}, extraAncestors...), styleScope{Tag: "div", Classes: splitClasses(wrapperClass)})
	} else {
		wrappedAncestors = append([]styleScope{}, extraAncestors...)
	}
	imageElem, err := newImageElement(rc, img, imageID, imageClass, wrappedAncestors, alt, title)
	if err != nil {
		return nil, err
	}
	return wrapBlockElement(style, imageElem), nil
}

func newImageElement(rc *renderContext, img *fb2.BookImage, imageID string, class string, extraAncestors []styleScope, alt string, title string) (layout.Element, error) {
	if rc == nil || img == nil {
		return nil, fmt.Errorf("nil image")
	}
	pdfImg, err := newPDFImage(img)
	if err != nil {
		return nil, err
	}
	style := rc.bodyStyle
	if rc.styles != nil {
		style = rc.styles.Resolve("img", class, extraAncestors, rc.bodyStyle)
	}
	imageElem := layout.NewImageElement(pdfImg)
	applyImageStyle(imageElem, style, img, rc.contentHeight, rc.deviceDPI)
	altText := strings.TrimSpace(alt)
	if altText == "" {
		altText = strings.TrimSpace(title)
	}
	if altText == "" {
		altText = imageID
	}
	imageElem.SetAltText(altText)
	rc.tracer.Label(imageElem, "img:"+altText)
	return wrapIfNeeded("img", class, style, imageElem), nil
}

func wrapBlockElement(style resolvedStyle, elem layout.Element) layout.Element {
	if elem == nil {
		return nil
	}
	div := layout.NewDiv().Add(elem)
	applyDivStyle(div, style)
	return div
}

func newParagraphElement(rc *renderContext, tag, classes string, ancestors []styleScope, parent resolvedStyle, text string) *layout.Paragraph {
	style := defaultResolvedStyle()
	if rc != nil && rc.styles != nil {
		style = rc.styles.Resolve(tag, classes, ancestors, parent)
	}
	para := newParagraphWithStyle(rc, style, text)
	applyParagraphStyle(para, style)
	rc.tracer.Label(para, text)
	return para
}

func newStyledParagraphElement(rc *renderContext, tag, classes string, ancestors []styleScope, parent resolvedStyle, runs []layout.TextRun) layout.Element {
	style := defaultResolvedStyle()
	if rc != nil && rc.styles != nil {
		style = rc.styles.Resolve(tag, classes, ancestors, parent)
	}
	para := layout.NewStyledParagraph(runs...)
	applyParagraphStyle(para, style)
	rc.tracer.Label(para, plainTextRuns(runs))
	wrapped := wrapIfNeeded(tag, classes, style, para)
	// Post-wrap with internalLinkRewriter when any inline run carries a
	// "#"-prefixed LinkURI.  The rewriter walks the block tree recursively
	// so both bare paragraphs and Div-wrapped paragraphs work correctly.
	if runsContainInternalLink(runs) {
		wrapped = newInternalLinkRewriter(wrapped, rc.tracer)
	}
	return wrapped
}

// runsContainInternalLink reports whether any run in runs has a LinkURI
// beginning with "#" (an intra-document fragment reference).
func runsContainInternalLink(runs []layout.TextRun) bool {
	for i := range runs {
		if strings.HasPrefix(runs[i].LinkURI, "#") {
			return true
		}
	}
	return false
}

func newHeadingElement(rc *renderContext, tag, classes string, style resolvedStyle, level int, runs []layout.TextRun, bookmarkLabel string) layout.Element {
	headingLevel := layout.H1
	switch min(max(level, 1), 6) {
	case 1:
		headingLevel = layout.H1
	case 2:
		headingLevel = layout.H2
	case 3:
		headingLevel = layout.H3
	case 4:
		headingLevel = layout.H4
	case 5:
		headingLevel = layout.H5
	case 6:
		headingLevel = layout.H6
	}
	std, emb := rc.fonts.resolve(style, plainTextRuns(runs))
	var heading *layout.Heading
	if emb != nil {
		heading = layout.NewHeadingEmbedded(plainTextRuns(runs), headingLevel, emb)
	} else {
		heading = layout.NewHeadingWithFont(plainTextRuns(runs), headingLevel, std, style.FontSize)
	}
	heading.SetRuns(runs).SetAlign(style.Align)
	// bookmarkLabel overrides the auto-derived label when non-empty —
	// used for multi-paragraph titles so the outline entry reflects
	// the combined title text, not just the first visible line.
	label := bookmarkLabel
	if label == "" {
		label = plainTextRuns(runs)
	}
	heading.SetBookmarkLabel(encodePDFTextString(label))
	rc.tracer.Label(heading, plainTextRuns(runs))
	wrapped := wrapIfNeeded(tag, classes, style, heading)
	// Post-wrap with internalLinkRewriter so inline links in headings
	// (even when nested inside the margin wrapper Div) resolve to named
	// destinations.  The rewriter walks the block tree recursively.
	if runsContainInternalLink(runs) {
		wrapped = newInternalLinkRewriter(wrapped, rc.tracer)
	}
	return wrapped
}

func newParagraphWithStyle(rc *renderContext, style resolvedStyle, text string) *layout.Paragraph {
	std, emb := rc.fonts.resolve(style, text)
	if emb != nil {
		return layout.NewParagraphEmbedded(text, emb, style.FontSize)
	}
	return layout.NewParagraph(text, std, style.FontSize)
}

func applyParagraphStyle(para *layout.Paragraph, style resolvedStyle) {
	if para == nil {
		return
	}
	para.SetAlign(style.Align)
	para.SetLeading(style.LineHeight)
	para.SetSpaceBefore(style.MarginTop)
	para.SetSpaceAfter(style.MarginBottom)
	para.SetFirstLineIndent(style.TextIndent)
	if style.Background != nil {
		para.SetBackground(*style.Background)
	}
	// CSS hyphens property controls folio's built-in hyphenation engine.
	// pre-wrap mode forces manual (soft-hyphen only) breaking.
	switch {
	case style.WhiteSpace == "pre-wrap":
		para.SetHyphens("manual")
	case style.Hyphens == "none":
		para.SetHyphens("none")
	case style.Hyphens == "manual":
		para.SetHyphens("manual")
	}
	// CSS orphans/widows control page-break line thresholds.
	if style.Orphans > 0 {
		para.SetOrphans(style.Orphans)
	}
	if style.Widows > 0 {
		para.SetWidows(style.Widows)
	}
}

func applyDivStyle(div *layout.Div, style resolvedStyle) {
	if div == nil {
		return
	}
	div.SetSpaceBefore(style.MarginTop)
	div.SetSpaceAfter(style.MarginBottom)
	if style.PaddingTop != 0 || style.PaddingRight != 0 || style.PaddingBottom != 0 || style.PaddingLeft != 0 {
		div.SetPaddingAll(layout.Padding{
			Top:    max(style.PaddingTop, 0),
			Right:  max(style.PaddingRight, 0),
			Bottom: max(style.PaddingBottom, 0),
			Left:   max(style.PaddingLeft, 0),
		})
	}
	if style.Background != nil {
		div.SetBackground(*style.Background)
	}
	if style.HasBorder {
		div.SetBorder(style.Border)
	}
	if style.KeepTogether {
		div.SetKeepTogether(true)
	}
	if style.WidthPercent > 0 {
		div.SetWidthPercent(style.WidthPercent / 100.0)
	}
	applyDivDimension(div, style.MinWidth, (*layout.Div).SetMinWidth, (*layout.Div).SetMinWidthUnit)
	applyDivDimension(div, style.MaxWidth, (*layout.Div).SetMaxWidth, (*layout.Div).SetMaxWidthUnit)
	applyDivDimension(div, style.MinHeight, (*layout.Div).SetMinHeight, (*layout.Div).SetMinHeightUnit)
	applyDivDimension(div, style.MaxHeight, (*layout.Div).SetMaxHeight, (*layout.Div).SetMaxHeightUnit)
}

// applyDivDimension applies a cssDimension to a Div using the appropriate
// setter: percentage values use the UnitValue setter, absolute values use the
// point setter.
func applyDivDimension(div *layout.Div, dim cssDimension, setPt func(*layout.Div, float64) *layout.Div, setUnit func(*layout.Div, layout.UnitValue) *layout.Div) {
	if dim.Percent > 0 {
		setUnit(div, layout.Pct(dim.Percent))
	} else if dim.Pt > 0 {
		setPt(div, dim.Pt)
	}
}

func applyImageStyle(elem *layout.ImageElement, style resolvedStyle, img *fb2.BookImage, maxContentHeight, deviceDPI float64) {
	if elem == nil {
		return
	}
	elem.SetAlign(style.Align)
	if style.WidthPercent > 0 {
		return
	}
	if img == nil || img.Dim.Width <= 0 || img.Dim.Height <= 0 {
		return
	}
	w := PxToPt(img.Dim.Width, deviceDPI)
	h := PxToPt(img.Dim.Height, deviceDPI)
	// Scale down proportionally so the image fits within the usable page
	// content height. Without this, images taller than the page cause the
	// folio layout engine to loop infinitely (Div returns LayoutPartial
	// with zero progress when its child image returns LayoutNothing).
	if maxContentHeight > 0 && h > maxContentHeight {
		scale := maxContentHeight / h
		w *= scale
		h = maxContentHeight
	}
	if w > 0 || h > 0 {
		elem.SetSize(w, h)
	}
}

func (b *flowBuilder) paragraphRuns(p *fb2.Paragraph, class string, tc textContext) []layout.TextRun {
	if p == nil {
		return nil
	}
	tag := "p"
	style := defaultResolvedStyle()
	if b.ctx != nil && b.ctx.styles != nil {
		style = b.ctx.styles.Resolve(tag, class, tc.ancestors, tc.parent)
	}
	// CSS hyphens property overrides the content-level hyphenation setting.
	hyphenate := tc.hyphenate
	switch style.Hyphens {
	case "none":
		hyphenate = false
	case "auto":
		hyphenate = b.ctx != nil && b.ctx.c != nil && b.ctx.c.Hyphen != nil
	}
	segments := p.Text
	var runs []layout.TextRun
	for i := range segments {
		runs = append(runs, b.inlineRuns(&segments[i], textContext{
			ancestors: append(append([]styleScope{}, tc.ancestors...), styleScope{Tag: tag, Classes: splitClasses(class)}),
			parent:    style,
			linkURI:   tc.linkURI,
			hyphenate: hyphenate,
		})...)
	}
	return runs
}

func (b *flowBuilder) inlineRuns(seg *fb2.InlineSegment, tc textContext) []layout.TextRun {
	if seg == nil {
		return nil
	}
	classes := inlineSegmentClass(seg, b.ctx.c)
	tag := inlineSegmentTag(seg)
	style := defaultResolvedStyle()
	if b.ctx != nil && b.ctx.styles != nil {
		style = b.ctx.styles.Resolve(tag, classes, tc.ancestors, tc.parent)
	}
	pseudo := pseudoContent{}
	if b.ctx != nil && b.ctx.styles != nil {
		pseudo = b.ctx.styles.ResolvePseudo(tag, classes, tc.ancestors, tc.parent)
	}
	linkURI := tc.linkURI
	if seg.Kind == fb2.InlineLink && seg.Href != "" {
		linkURI = seg.Href
	}

	var runs []layout.TextRun
	appendText := func(text string) {
		if text == "" {
			return
		}
		run := b.textRunForStyle(text, style)
		if linkURI != "" {
			run.LinkURI = linkURI
		}
		runs = append(runs, run)
	}
	appendText(pseudo.Before)
	if seg.Text != "" {
		text := seg.Text
		if tc.hyphenate && !isCodeSegment(seg) && b.ctx != nil && b.ctx.c != nil && b.ctx.c.Hyphen != nil {
			text = b.ctx.c.Hyphen.Hyphenate(text)
		}
		appendText(text)
	}
	if seg.Kind == fb2.InlineImageSegment && seg.Image != nil {
		if run := b.inlineImageRun(seg.Image, tc); run != nil {
			runs = append(runs, *run)
		}
	}
	childHyphenate := tc.hyphenate && !isCodeSegment(seg)
	childAncestors := append(append([]styleScope{}, tc.ancestors...), styleScope{Tag: tag, Classes: splitClasses(classes)})
	for i := range seg.Children {
		runs = append(runs, b.inlineRuns(&seg.Children[i], textContext{ancestors: childAncestors, parent: style, linkURI: linkURI, hyphenate: childHyphenate})...)
	}
	appendText(pseudo.After)
	return runs
}

func isCodeSegment(seg *fb2.InlineSegment) bool {
	return seg != nil && seg.Kind == fb2.InlineCode
}

func (b *flowBuilder) inlineImageRun(img *fb2.InlineImage, tc textContext) *layout.TextRun {
	if img == nil {
		return nil
	}
	bookImg, id := imageByID(b.ctx.c, img.Href)
	if bookImg == nil {
		if img.Alt == "" {
			return nil
		}
		run := b.textRunForStyle(img.Alt, tc.parent)
		return &run
	}
	elem, err := newImageElement(b.ctx, bookImg, id, "image-inline", tc.ancestors, img.Alt, "")
	if err != nil {
		if img.Alt == "" {
			return nil
		}
		run := b.textRunForStyle(img.Alt, tc.parent)
		return &run
	}
	run := layout.RunInline(elem)
	return &run
}

func (b *flowBuilder) textRunForStyle(text string, style resolvedStyle) layout.TextRun {
	std, emb := b.ctx.fonts.resolve(style, text)
	var run layout.TextRun
	if emb != nil {
		run = layout.NewRunEmbedded(text, emb, style.FontSize)
	} else {
		run = layout.NewRun(text, std, style.FontSize)
	}
	if style.HasColor {
		run.Color = style.Color
	}
	if style.Underline {
		run.Decoration |= layout.DecorationUnderline
	}
	if style.Strike {
		run.Decoration |= layout.DecorationStrikethrough
	}
	run.BaselineShift = style.BaselineShift
	run.LetterSpacing = style.LetterSpacing
	if style.Background != nil {
		run.BackgroundColor = style.Background
	}
	return run
}

func inlineSegmentTag(seg *fb2.InlineSegment) string {
	if seg == nil {
		return "span"
	}
	switch seg.Kind {
	case fb2.InlineStrong:
		return "strong"
	case fb2.InlineEmphasis:
		return "em"
	case fb2.InlineStrikethrough:
		return "del"
	case fb2.InlineSub:
		return "sub"
	case fb2.InlineSup:
		return "sup"
	case fb2.InlineCode:
		return "code"
	case fb2.InlineLink:
		return "a"
	default:
		return "span"
	}
}

func inlineSegmentClass(seg *fb2.InlineSegment, c *content.Content) string {
	if seg == nil {
		return ""
	}
	switch seg.Kind {
	case fb2.InlineStrong:
		return "strong"
	case fb2.InlineEmphasis:
		return "emphasis"
	case fb2.InlineStrikethrough:
		return "strikethrough"
	case fb2.InlineSub:
		return "sub"
	case fb2.InlineSup:
		return "sup"
	case fb2.InlineCode:
		return "code"
	case fb2.InlineNamedStyle:
		return seg.Style
	case fb2.InlineLink:
		if after, ok := strings.CutPrefix(seg.Href, "#"); ok {
			if c != nil {
				if _, isFootnote := c.FootnotesIndex[after]; isFootnote {
					return "link-footnote"
				}
			}
			return "link-internal"
		}
		return "link-external"
	default:
		return ""
	}
}

// encodePDFTextString encodes s as a PDF "text string" (ISO 32000 §7.9.2.2).
// If every character fits in PDFDocEncoding (approximately Latin-1, U+0000..U+00FF),
// the string is returned unchanged.  Otherwise it is returned as UTF-16BE with a
// leading BOM (U+FEFF) so that PDF consumers interpret the bytes as Unicode.
//
// This is needed because folio serialises outline titles and document-info
// values with core.NewPdfLiteralString, which writes raw bytes into a PDF
// literal string.  Without a BOM, PDF viewers decode such bytes as
// PDFDocEncoding and multi-byte UTF-8 sequences (e.g. Cyrillic) appear as
// mojibake.
func encodePDFTextString(s string) string {
	for _, r := range s {
		if r > 0xFF {
			codes := utf16.Encode([]rune(s))
			buf := make([]byte, 2+len(codes)*2) // BOM + code units
			buf[0] = 0xFE
			buf[1] = 0xFF
			for i, c := range codes {
				buf[2+i*2] = byte(c >> 8)
				buf[2+i*2+1] = byte(c)
			}
			return string(buf)
		}
	}
	return s
}

func plainTextRuns(runs []layout.TextRun) string {
	var sb strings.Builder
	for _, run := range runs {
		sb.WriteString(run.Text)
	}
	return strings.TrimSpace(sb.String())
}

// extractDropcapChar finds the first displayable character in segments and
// returns it as a string alongside a copy of the segments with that character
// removed.  When no character is found the first return value is empty and
// the second is the original slice unchanged.
func extractDropcapChar(segments []fb2.InlineSegment) (string, []fb2.InlineSegment) {
	clone := append([]fb2.InlineSegment(nil), segments...)
	for i := range clone {
		seg := &clone[i]
		if seg.Kind != fb2.InlineText || seg.Text == "" {
			continue
		}
		r, size := utf8.DecodeRuneInString(seg.Text)
		if r == utf8.RuneError && size == 0 {
			continue
		}
		clone[i].Text = seg.Text[size:]
		if clone[i].Text == "" && len(clone[i].Children) == 0 {
			clone = append(clone[:i], clone[i+1:]...)
		}
		return string(r), clone
	}
	return "", segments
}

// removeStyle removes the named class from a space-separated style string.
func removeStyle(style, styles string) string {
	if styles == "" || style == "" {
		return styles
	}
	var parts []string
	for part := range strings.FieldsSeq(styles) {
		if part != style {
			parts = append(parts, part)
		}
	}
	return strings.Join(parts, " ")
}

func hasStyle(style, styles string) bool {
	if styles == "" {
		return false
	}
	if style == "" {
		return true
	}
	for part := range strings.FieldsSeq(styles) {
		if part == style {
			return true
		}
	}
	return false
}
