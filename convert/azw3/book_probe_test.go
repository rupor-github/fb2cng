package azw3

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"go.uber.org/zap"

	"fbc/common"
	"fbc/config"
	"fbc/content"
	"fbc/convert/publication"
	"fbc/state"
)

func TestCompleteFB2Book(t *testing.T) {
	input, err := os.ReadFile(filepath.Join("..", "..", "testdata", "_Test.fb2"))
	if err != nil {
		t.Fatal(err)
	}
	stylesheet, err := os.ReadFile(filepath.Join("..", "default.css"))
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		mode      common.FootnotesMode
		shape     common.TOCType
		placement common.TOCPagePlacement
	}{
		{common.FootnotesModeDefault, common.TOCTypeNormal, common.TOCPagePlacementBefore},
		{common.FootnotesModeFloat, common.TOCTypeOldKindle, common.TOCPagePlacementAfter},
		{common.FootnotesModeFloatRenumbered, common.TOCTypeFlat, common.TOCPagePlacementNone},
	} {
		t.Run(tt.mode.String(), func(t *testing.T) {
			cfg, err := config.LoadConfiguration("")
			if err != nil {
				t.Fatal(err)
			}
			cfg.Document.Footnotes.Mode, cfg.Document.TOCType = tt.mode, tt.shape
			cfg.Document.TOCPage.Placement = tt.placement
			cfg.Document.Annotation.Enable, cfg.Document.Annotation.InTOC = true, true
			cfg.Document.PageMap.Enable, cfg.Document.PageMap.Size = true, 500
			cfg.Document.AZW3.APNX = "app"
			cfg.Document.Images.Cover.Resize = common.ImageResizeModeKeepAR
			ctx := state.ContextWithEnv(context.Background())
			env := state.EnvFromContext(ctx)
			env.Cfg, env.Log, env.DefaultStyle = cfg, zap.NewNop(), stylesheet
			// Build mutable content once using its real AZW3 identity.
			c, err := content.Prepare(ctx, bytes.NewReader(input), "_Test.fb2", common.OutputFmtAzw3, env.Log)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := os.RemoveAll(c.WorkDir); err != nil {
					t.Errorf("remove prepared-content directory: %v", err)
				}
			})
			pub, err := publication.Build(ctx, c, &cfg.Document, publication.HTML5, env.Log)
			if err != nil {
				t.Fatal(err)
			}
			pub, err = preparePublicationImages(ctx, pub, &cfg.Document.Images, env.Log)
			if err != nil {
				t.Fatal(err)
			}
			pub, err = prepareCoverDocument(ctx, pub, &cfg.Document.Images)
			if err != nil {
				t.Fatal(err)
			}
			text, err := compileText(ctx, pub)
			if err != nil {
				t.Fatal(err)
			}
			navigation, err := compileNavigation(ctx, pub, text, tt.shape)
			if err != nil {
				t.Fatal(err)
			}
			metadata, err := compileMetadata(ctx, c, &cfg.Document, pub, text, navigation, env.Log)
			if err != nil {
				t.Fatal(err)
			}
			var expectedAPNX []byte
			for _, compression := range []string{"none", "standard", "huffdic"} {
				t.Run(compression, func(t *testing.T) {
					cfg.Document.AZW3.Compression = compression
					book, err := compileBook(ctx, text, navigation, metadata, cfg.Document.AZW3.Compression)
					if err != nil {
						t.Fatal(err)
					}
					assertBookRecords(t, book, text, navigation, metadata)
					for _, page := range pub.Pages {
						if _, found := text.Positions[linkTarget{Filename: page.Filename, ID: page.SpanID}]; !found {
							t.Errorf("page %d lost its position during assembly", page.PageNum)
						}
					}
					var output bytes.Buffer
					created := time.Date(2026, 11, 9, 12, 0, 0, 0, time.UTC)
					if err := writePalmDB(ctx, &output, book.Database, created, book.Records); err != nil {
						t.Fatal(err)
					}
					name := "assembled-fb2-" + tt.mode.String() + "-" + compression
					apnx, err := compileAPNX(ctx, filepath.Join(c.WorkDir, name+".azw3"), pub, text, metadata, &cfg.Document, env.Log)
					if err != nil || apnx == nil {
						t.Fatalf("prepare APNX: %v", err)
					}
					assertAPNXBookIdentity(t, apnx.Data, book)
					if expectedAPNX == nil {
						expectedAPNX = bytes.Clone(apnx.Data)
					} else if !bytes.Equal(expectedAPNX, apnx.Data) {
						t.Error("APNX bytes changed with text compression")
					}
					chunks, err := splitText(text.Raw)
					if err != nil {
						t.Fatal(err)
					}
					compressed := 0
					for _, record := range book.Records[1 : 1+len(chunks)] {
						compressed += len(record)
					}
					if compression != "none" && compressed >= len(text.Raw) {
						t.Error("selected compression did not compress the real book's text")
					}
					dictionaryBytes := 0
					first := binary.BigEndian.Uint32(book.Records[0][0x70:])
					count := binary.BigEndian.Uint32(book.Records[0][0x74:])
					for _, record := range book.Records[first : first+count] {
						dictionaryBytes += len(record)
					}
					summary := fmt.Sprintf("mode=%s\nmain_bytes=%d\nall_flow_bytes=%d\nencoded_bytes=%d\ndictionary_bytes=%d\nrecords=%d\npages=%d\n",
						compression, text.MainLength, len(text.Raw), compressed, dictionaryBytes, len(book.Records), len(pub.Pages))
					writeProbeArtifacts(t, map[string][]byte{
						name + ".azw3": output.Bytes(), name + ".txt": []byte(summary), name + ".rawml": text.Raw, name + ".apnx": apnx.Data,
					})
					t.Log(strings.TrimSpace(summary))
					for _, mode := range []string{"app", "eink"} {
						t.Run("publish-"+mode, func(t *testing.T) {
							cfg.Document.AZW3.APNX = mode
							finalPath := filepath.Join(t.TempDir(), "new", "nested", name+".azw3")
							sidecar, err := compileAPNX(ctx, finalPath, pub, text, metadata, &cfg.Document, env.Log)
							if err != nil || sidecar == nil {
								t.Fatalf("prepare published APNX: %v", err)
							}
							bundle, err := preflightOutputBundle(ctx, finalPath, sidecar, false)
							if err != nil {
								t.Fatal(err)
							}
							if err := bundle.publish(ctx, book, created); err != nil {
								t.Fatal(err)
							}
							assertOutputFile(t, finalPath, output.Bytes())
							assertOutputFile(t, sidecar.Path, apnx.Data)
							assertNoBundleTemps(t, filepath.Dir(finalPath))
							// Independently decode the files actually published on disk,
							// not the compiler's in-memory buffers or temporary files.
							bookData, err := os.ReadFile(finalPath)
							if err != nil {
								t.Fatal(err)
							}
							pageData, err := os.ReadFile(sidecar.Path)
							if err != nil {
								t.Fatal(err)
							}
							publishedName := name + "-published-" + mode
							writeProbeArtifacts(t, map[string][]byte{publishedName + ".azw3": bookData, publishedName + ".apnx": pageData})
							if os.Getenv("AZW3_KINDLEUNPACK_TEST") == "1" {
								work := unpackProbe(t, publishedName, bookData, pageData)
								assertUnpackedRaw(t, work, text.Raw)
								assertUnpackedNavigation(t, work, pub, text, tt.shape)
								assertUnpackedPages(t, work, pub, text)
							}
						})
					}
				})
			}
		})
	}
}
