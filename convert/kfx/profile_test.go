package kfx

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
	"fbc/fb2"
	"fbc/misc"
	"fbc/state"
)

const kfxProfileMemRate = 512 * 1024

// TestKFXProfilePath profiles one KFX generation stage. It is skipped unless
// KFX_PROFILE_BOOK points to an FB2 file. The test intentionally prepares FB2
// content before starting profiles so debug reporting and shared conversion
// setup do not pollute KFX-path measurements.
//
// Example, profile all KFX stages for a larger fixture:
//
//	rm -rf /tmp/fb2cng-kfx-profile && mkdir -p /tmp/fb2cng-kfx-profile
//	for mode in default float floatRenumbered; do
//	  for target in styles images storyline fragments serialize generate; do
//	    echo "=== $mode / $target ==="
//	    KFX_PROFILE_BOOK=/mnt/d/test/1.fb2 \
//	      KFX_PROFILE_FOOTNOTES_MODE=$mode \
//	      KFX_PROFILE_TARGET=$target \
//	      KFX_PROFILE_OUT=/tmp/fb2cng-kfx-profile/$mode \
//	      GOFLAGS=-mod=mod go test ./convert/kfx \
//	        -run '^TestKFXProfilePath$' \
//	        -count=1 \
//	        -timeout=30m \
//	        -v || exit 1
//	  done
//	done
//
// For a quicker smoke profile, use KFX_PROFILE_BOOK=/mnt/d/test/_Test.fb2.
// Supported KFX_PROFILE_TARGET values are: styles, images, storyline,
// fragments, serialize, and generate. KFX_PROFILE_CONFIG defaults to
// build/test.yaml when present. KFX_PROFILE_FOOTNOTES_MODE optionally overrides
// the config footnotes mode before content preparation.
func TestKFXProfilePath(t *testing.T) {
	bookPath := kfxProfileBookPath(t)
	target := os.Getenv("KFX_PROFILE_TARGET")
	if target == "" {
		target = "generate"
	}
	outDir := os.Getenv("KFX_PROFILE_OUT")
	if outDir == "" {
		outDir = filepath.Join(os.TempDir(), "fb2cng-kfx-profile")
	}
	if err := os.MkdirAll(outDir, 0755); err != nil {
		t.Fatalf("create profile dir: %v", err)
	}

	fixture := newKFXProfileFixture(t, bookPath)
	defer fixture.cleanup()

	switch target {
	case "styles":
		profileKFXTarget(t, outDir, target, func() error {
			styles, parsedCSS := buildStyleRegistry(fixture.c.Book.Stylesheets, nil, zap.NewNop())
			fontInfo := BuildFontInfo(fixture.c.Book.Stylesheets, parsedCSS, zap.NewNop())
			if fontInfo.HasBodyFont() {
				styles.SetBodyFontFamily(fontInfo.BodyFontFamily)
			}
			return nil
		})
	case "images":
		profileKFXTarget(t, outDir, target, func() error {
			_, _, _ = kfxProfileImageFragments(fixture.c)
			return nil
		})
	case "storyline":
		profileKFXTarget(t, outDir, target, func() error {
			styles := kfxProfileStyleRegistry(fixture.c)
			_, _, imageResourceInfo := kfxProfileImageFragments(fixture.c)
			_, _, _, _, _, _, _, _, err := generateStoryline(context.Background(), fixture.c, styles, imageResourceInfo, 1000)
			return err
		})
	case "fragments":
		profileKFXTarget(t, outDir, target, func() error {
			container := kfxProfileContainer(fixture.c)
			return buildFragments(context.Background(), container, fixture.c, fixture.cfg, zap.NewNop())
		})
	case "serialize":
		container := kfxProfileContainer(fixture.c)
		if err := buildFragments(context.Background(), container, fixture.c, fixture.cfg, zap.NewNop()); err != nil {
			t.Fatalf("build fragments before serialization profile: %v", err)
		}
		profileKFXTarget(t, outDir, target, func() error {
			data, err := container.WriteContainer()
			if err == nil {
				t.Logf("kfx bytes=%d fragments=%d", len(data), container.Fragments.Len())
			}
			return err
		})
	case "generate":
		outputName := filepath.Join(outDir, "profile-output.kfx")
		profileKFXTarget(t, outDir, target, func() error {
			return Generate(context.Background(), fixture.c, outputName, fixture.cfg, zap.NewNop())
		})
	default:
		t.Fatalf("unknown KFX_PROFILE_TARGET %q", target)
	}
}

type kfxProfileFixture struct {
	c       *content.Content
	cfg     *config.DocumentConfig
	cleanup func()
}

func kfxProfileBookPath(tb testing.TB) string {
	tb.Helper()
	bookPath := os.Getenv("KFX_PROFILE_BOOK")
	if bookPath == "" {
		tb.Skip("set KFX_PROFILE_BOOK to profile KFX generation")
	}
	return bookPath
}

func newKFXProfileFixture(tb testing.TB, bookPath string) kfxProfileFixture {
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

	cfgPath := os.Getenv("KFX_PROFILE_CONFIG")
	if cfgPath == "" {
		cfgPath = filepath.Join(root, "build", "test.yaml")
	}
	cfg, err := config.LoadConfiguration(cfgPath)
	if err != nil {
		tb.Fatalf("load config %s: %v", cfgPath, err)
	}
	if modeName := os.Getenv("KFX_PROFILE_FOOTNOTES_MODE"); modeName != "" {
		mode, err := common.ParseFootnotesMode(modeName)
		if err != nil {
			tb.Fatalf("parse KFX_PROFILE_FOOTNOTES_MODE %q: %v", modeName, err)
		}
		cfg.Document.Footnotes.Mode = mode
	}

	ctx := state.ContextWithEnv(context.Background())
	env := state.EnvFromContext(ctx)
	env.Cfg = cfg
	env.Log = zap.NewNop()
	env.DefaultStyle = readKFXProfileDefaultStyle(tb, root, cfg)

	data, err := os.ReadFile(bookPath)
	if err != nil {
		tb.Fatalf("read book %s: %v", bookPath, err)
	}
	start := time.Now()
	c, err := content.Prepare(ctx, bytes.NewReader(data), bookPath, common.OutputFmtKfx, zap.NewNop())
	if err != nil {
		tb.Fatalf("prepare content: %v", err)
	}
	c.Debug = false
	tb.Logf("prepared KFX profile content in %s: book_bytes=%d images=%d stylesheets=%d footnotes=%d footnotes_mode=%s workdir=%s",
		time.Since(start), len(data), len(c.ImagesIndex), len(c.Book.Stylesheets), len(c.FootnotesIndex), c.FootnotesMode, c.WorkDir)
	return kfxProfileFixture{
		c:   c,
		cfg: &cfg.Document,
		cleanup: func() {
			_ = os.RemoveAll(c.WorkDir)
		},
	}
}

func readKFXProfileDefaultStyle(tb testing.TB, root string, cfg *config.Config) []byte {
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

func kfxProfileStyleRegistry(c *content.Content) *StyleRegistry {
	styles, parsedCSS := buildStyleRegistry(c.Book.Stylesheets, nil, zap.NewNop())
	fontInfo := BuildFontInfo(c.Book.Stylesheets, parsedCSS, zap.NewNop())
	if fontInfo.HasBodyFont() {
		styles.SetBodyFontFamily(fontInfo.BodyFontFamily)
	}
	return styles
}

func kfxProfileImageFragments(c *content.Content) ([]*Fragment, []*Fragment, imageResourceInfoByID) {
	usedIDs := collectUsedImageIDs(c.Book)
	usedImages := make(fb2.BookImages, len(usedIDs))
	for id, img := range c.ImagesIndex {
		if usedIDs[id] {
			usedImages[id] = img
		}
	}
	return buildImageResourceFragments(usedImages)
}

func kfxProfileContainer(c *content.Content) *Container {
	container := NewContainer()
	container.ContainerID = "CR!" + hashToAlphanumeric(c.Book.Description.DocumentInfo.ID, 28)
	container.GeneratorApp = misc.GetAppName()
	container.GeneratorPkg = misc.GetVersion()
	return container
}

func profileKFXTarget(t *testing.T, outDir, name string, fn func() error) {
	t.Helper()
	runtime.GC()
	runtime.MemProfileRate = kfxProfileMemRate
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
	writeKFXRuntimeProfile(t, allocPath, "allocs")
	heapPath := filepath.Join(outDir, name+".heap.pprof")
	writeKFXRuntimeProfile(t, heapPath, "heap")
	t.Logf("profile target=%s elapsed=%s total_alloc_delta=%s mallocs_delta=%d heap_alloc_delta=%s cpu=%s allocs=%s heap=%s",
		name,
		elapsed,
		formatKFXProfileBytes(after.TotalAlloc-before.TotalAlloc),
		after.Mallocs-before.Mallocs,
		formatKFXProfileBytes(after.HeapAlloc-before.HeapAlloc),
		cpuPath,
		allocPath,
		heapPath)
}

func writeKFXRuntimeProfile(t *testing.T, path, name string) {
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

func formatKFXProfileBytes(n uint64) string {
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
