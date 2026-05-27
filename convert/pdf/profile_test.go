package pdf

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"testing"
	"time"

	"go.uber.org/zap"

	"fbc/common"
	"fbc/config"
	"fbc/content"
	"fbc/state"
)

const pdfProfileMemRate = 512 * 1024

// TestPDFProfilePath profiles one PDF generation stage. It is skipped unless
// PDF_PROFILE_BOOK points to an FB2 file. The test intentionally prepares FB2
// content before starting profiles so debug reporting and shared conversion
// setup do not pollute PDF-path measurements.
//
// Example, profile all PDF stages for a larger fixture:
//
//	rm -rf /tmp/fb2cng-pdf-profile && mkdir -p /tmp/fb2cng-pdf-profile
//	for target in collect styles layout builddoc generate; do
//	  echo "=== $target ==="
//	  PDF_PROFILE_BOOK=/mnt/d/test/1.fb2 \
//	    PDF_PROFILE_TARGET=$target \
//	    PDF_PROFILE_OUT=/tmp/fb2cng-pdf-profile \
//	    GOFLAGS=-mod=mod go test ./convert/pdf \
//	      -run '^TestPDFProfilePath$' \
//	      -count=1 \
//	      -timeout=30m \
//	      -v || exit 1
//	done
//
// For a quicker smoke profile, use PDF_PROFILE_BOOK=/mnt/d/test/_Test.fb2.
// Supported PDF_PROFILE_TARGET values are: collect, styles, layout, builddoc,
// and generate. PDF_PROFILE_CONFIG defaults to build/test.yaml when present.
func TestPDFProfilePath(t *testing.T) {
	bookPath := pdfProfileBookPath(t)
	target := os.Getenv("PDF_PROFILE_TARGET")
	if target == "" {
		target = "generate"
	}
	outDir := os.Getenv("PDF_PROFILE_OUT")
	if outDir == "" {
		outDir = filepath.Join(os.TempDir(), "fb2cng-pdf-profile")
	}
	if err := os.MkdirAll(outDir, 0755); err != nil {
		t.Fatalf("create profile dir: %v", err)
	}

	fixture := newPDFProfileFixture(t, bookPath)
	defer fixture.cleanup()

	switch target {
	case "collect":
		profilePDFTarget(t, outDir, target, func() error {
			_, err := collectPDFContent(fixture.c, fixture.cfg)
			return err
		})
	case "styles":
		profilePDFTarget(t, outDir, target, func() error {
			parsed := parsePDFStylesheets(fixture.c.Book, zap.NewNop())
			_ = newPDFStyleResolverFromParsed(parsed, zap.NewNop(), newPDFStyleTracer(""))
			_ = newPDFFontRegistryFromParsed(parsed, zap.NewNop())
			return nil
		})
	case "layout":
		doc := newPDFProfileDocument(t, fixture)
		doc.Blocks = applyPDFPseudoContentToBlocks(doc.Blocks, doc.Styles)
		doc.PrintedFootnotes = applyPDFPseudoContentToPrintedFootnotes(doc.PrintedFootnotes, doc.Styles)
		profilePDFTarget(t, outDir, target, func() error {
			_, _, _, err := layoutPDFDocumentPages(doc)
			return err
		})
	case "builddoc":
		doc := newPDFProfileDocument(t, fixture)
		profilePDFTarget(t, outDir, target, func() error {
			data, err := buildPDFDocument(doc)
			if err == nil {
				t.Logf("pdf bytes=%d", len(data))
			}
			return err
		})
	case "generate":
		outputName := filepath.Join(outDir, "profile-output.pdf")
		profilePDFTarget(t, outDir, target, func() error {
			return Generate(context.Background(), fixture.c, outputName, fixture.cfg, zap.NewNop())
		})
	default:
		t.Fatalf("unknown PDF_PROFILE_TARGET %q", target)
	}
}

func BenchmarkPDFProfileCollectContent(b *testing.B) {
	fixture := newPDFProfileFixture(b, pdfProfileBookPath(b))
	defer fixture.cleanup()
	b.ReportAllocs()

	for b.Loop() {
		if _, err := collectPDFContent(fixture.c, fixture.cfg); err != nil {
			b.Fatalf("collect PDF content: %v", err)
		}
	}
}

func BenchmarkPDFProfileStyles(b *testing.B) {
	fixture := newPDFProfileFixture(b, pdfProfileBookPath(b))
	defer fixture.cleanup()
	b.ReportAllocs()

	for b.Loop() {
		parsed := parsePDFStylesheets(fixture.c.Book, zap.NewNop())
		_ = newPDFStyleResolverFromParsed(parsed, zap.NewNop(), newPDFStyleTracer(""))
		_ = newPDFFontRegistryFromParsed(parsed, zap.NewNop())
	}
}

func BenchmarkPDFProfileLayout(b *testing.B) {
	fixture := newPDFProfileFixture(b, pdfProfileBookPath(b))
	defer fixture.cleanup()
	doc := newPDFProfileDocument(b, fixture)
	doc.Blocks = applyPDFPseudoContentToBlocks(doc.Blocks, doc.Styles)
	doc.PrintedFootnotes = applyPDFPseudoContentToPrintedFootnotes(doc.PrintedFootnotes, doc.Styles)
	b.ReportAllocs()

	for b.Loop() {
		if _, _, _, err := layoutPDFDocumentPages(doc); err != nil {
			b.Fatalf("layout PDF pages: %v", err)
		}
	}
}

func BenchmarkPDFProfileBuildDocument(b *testing.B) {
	fixture := newPDFProfileFixture(b, pdfProfileBookPath(b))
	defer fixture.cleanup()
	doc := newPDFProfileDocument(b, fixture)
	b.ReportAllocs()

	for b.Loop() {
		if _, err := buildPDFDocument(doc); err != nil {
			b.Fatalf("build PDF document: %v", err)
		}
	}
}

func BenchmarkPDFProfileGenerate(b *testing.B) {
	fixture := newPDFProfileFixture(b, pdfProfileBookPath(b))
	defer fixture.cleanup()
	outputName := filepath.Join(b.TempDir(), "profile-output.pdf")
	b.ReportAllocs()

	for b.Loop() {
		if err := Generate(context.Background(), fixture.c, outputName, fixture.cfg, zap.NewNop()); err != nil {
			b.Fatalf("generate PDF: %v", err)
		}
	}
}

type pdfProfileFixture struct {
	c       *content.Content
	cfg     *config.DocumentConfig
	cleanup func()
}

func pdfProfileBookPath(tb testing.TB) string {
	tb.Helper()
	bookPath := os.Getenv("PDF_PROFILE_BOOK")
	if bookPath == "" {
		tb.Skip("set PDF_PROFILE_BOOK to profile or benchmark PDF generation")
	}
	return bookPath
}

func newPDFProfileFixture(tb testing.TB, bookPath string) pdfProfileFixture {
	tb.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		tb.Fatalf("project root: %v", err)
	}
	oldwd, err := os.Getwd()
	if err != nil {
		tb.Fatalf("get cwd: %v", err)
	}
	if err := os.Chdir(root); err != nil {
		tb.Fatalf("chdir root: %v", err)
	}
	defer os.Chdir(oldwd)

	cfgPath := os.Getenv("PDF_PROFILE_CONFIG")
	if cfgPath == "" {
		cfgPath = filepath.Join(root, "build", "test.yaml")
	}
	cfg, err := config.LoadConfiguration(cfgPath)
	if err != nil {
		tb.Fatalf("load config %s: %v", cfgPath, err)
	}
	ctx := state.ContextWithEnv(context.Background())
	env := state.EnvFromContext(ctx)
	env.Cfg = cfg
	env.Log = zap.NewNop()
	env.DefaultStyle = readPDFProfileDefaultStyle(tb, root, cfg)

	data, err := os.ReadFile(bookPath)
	if err != nil {
		tb.Fatalf("read book %s: %v", bookPath, err)
	}
	start := time.Now()
	c, err := content.Prepare(ctx, bytes.NewReader(data), bookPath, common.OutputFmtPdf, zap.NewNop())
	if err != nil {
		tb.Fatalf("prepare content: %v", err)
	}
	c.Debug = false
	tb.Logf("prepared PDF profile content in %s: book_bytes=%d images=%d stylesheets=%d footnotes=%d workdir=%s",
		time.Since(start), len(data), len(c.ImagesIndex), len(c.Book.Stylesheets), len(c.FootnotesIndex), c.WorkDir)
	return pdfProfileFixture{
		c:   c,
		cfg: &cfg.Document,
		cleanup: func() {
			_ = os.RemoveAll(c.WorkDir)
		},
	}
}

func readPDFProfileDefaultStyle(tb testing.TB, root string, cfg *config.Config) []byte {
	tb.Helper()
	stylePath := filepath.Join(root, "convert", "default.css")
	if cfg.Document.StylesheetPath != "" {
		stylePath = cfg.Document.StylesheetPath
		if !filepath.IsAbs(stylePath) {
			stylePath = filepath.Join(root, stylePath)
		}
	}
	data, err := os.ReadFile(stylePath)
	if err != nil {
		tb.Fatalf("read stylesheet %s: %v", stylePath, err)
	}
	return data
}

func newPDFProfileDocument(tb testing.TB, fixture pdfProfileFixture) pdfDocumentSpec {
	tb.Helper()
	pageWidth, pageHeight, err := pageSizePoints(fixture.cfg.Images.Screen)
	if err != nil {
		tb.Fatalf("page size: %v", err)
	}
	contentPlan, err := collectPDFContent(fixture.c, fixture.cfg)
	if err != nil {
		tb.Fatalf("collect content: %v", err)
	}
	parsedStylesheets := parsePDFStylesheets(fixture.c.Book, zap.NewNop())
	return pdfDocumentSpec{
		PageWidth:        pageWidth,
		PageHeight:       pageHeight,
		ScreenWidthPx:    fixture.cfg.Images.Screen.Width,
		ScreenHeightPx:   fixture.cfg.Images.Screen.Height,
		ScreenDPI:        fixture.cfg.Images.Screen.DPI,
		Title:            bookTitle(fixture.c, fixture.cfg, zap.NewNop()),
		Author:           bookAuthors(fixture.c, fixture.cfg, zap.NewNop()),
		Subject:          bookSubject(fixture.c),
		Keywords:         bookKeywords(fixture.c),
		Blocks:           contentPlan.Blocks,
		TOC:              contentPlan.TOC,
		PrintedFootnotes: contentPlan.PrintedFootnotes,
		DebugPlan:        contentPlan.DebugPlan,
		Content:          fixture.c,
		Styles:           newPDFStyleResolverFromParsed(parsedStylesheets, zap.NewNop(), newPDFStyleTracer("")),
		Images:           fixture.c.ImagesIndex,
		CoverID:          fixture.c.CoverID,
		Hyphenator:       pdfHyphenator(fixture.c, zap.NewNop()),
		Fonts:            newPDFFontRegistryFromParsed(parsedStylesheets, zap.NewNop()),
		Debug:            false,
		WorkDir:          fixture.c.WorkDir,
	}
}

func profilePDFTarget(t *testing.T, outDir, name string, fn func() error) {
	t.Helper()
	runtime.GC()
	runtime.MemProfileRate = pdfProfileMemRate
	var before runtime.MemStats
	runtime.ReadMemStats(&before)

	cpuPath := filepath.Join(outDir, name+".cpu.pprof")
	cpuFile, err := os.Create(cpuPath)
	if err != nil {
		t.Fatalf("create CPU profile: %v", err)
	}
	if err := pprof.StartCPUProfile(cpuFile); err != nil {
		_ = cpuFile.Close()
		t.Fatalf("start CPU profile: %v", err)
	}
	start := time.Now()
	err = fn()
	elapsed := time.Since(start)
	pprof.StopCPUProfile()
	if closeErr := cpuFile.Close(); closeErr != nil && err == nil {
		err = closeErr
	}
	if err != nil {
		t.Fatalf("profile target %s: %v", name, err)
	}

	var after runtime.MemStats
	runtime.ReadMemStats(&after)
	allocPath := filepath.Join(outDir, name+".allocs.pprof")
	writeRuntimeProfile(t, allocPath, "allocs")
	heapPath := filepath.Join(outDir, name+".heap.pprof")
	writeRuntimeProfile(t, heapPath, "heap")
	t.Logf("profile target=%s elapsed=%s total_alloc_delta=%s mallocs_delta=%d heap_alloc_delta=%s cpu=%s allocs=%s heap=%s",
		name,
		elapsed,
		formatPDFProfileBytes(after.TotalAlloc-before.TotalAlloc),
		after.Mallocs-before.Mallocs,
		formatPDFProfileBytes(after.HeapAlloc-before.HeapAlloc),
		cpuPath,
		allocPath,
		heapPath)
}

func writeRuntimeProfile(t *testing.T, path, name string) {
	t.Helper()
	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("create %s profile: %v", name, err)
	}
	defer file.Close()
	profile := pprof.Lookup(name)
	if profile == nil {
		t.Fatalf("runtime profile %s not found", name)
	}
	if err := profile.WriteTo(file, 0); err != nil {
		t.Fatalf("write %s profile: %v", name, err)
	}
}

func formatPDFProfileBytes(n uint64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%dB", n)
	}
	div, exp := uint64(unit), 0
	for n/div >= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f%ciB", float64(n)/float64(div), "KMGTPE"[exp])
}
