package azw3

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"go.uber.org/zap"

	"fbc/utils/filepub"
)

type outputFixture struct {
	path string
	book *bookLayout
	apnx *apnxLayout
	data []byte
}

func newOutputFixture(t *testing.T, mode string) outputFixture {
	t.Helper()
	pub, text, metadata, cfg := apnxTestPublication(t, false)
	cfg.AZW3.APNX = mode
	path := filepath.Join(t.TempDir(), "Книга with spaces.azw3")
	book, err := compileBook(context.Background(), text, nil, metadata, "standard")
	if err != nil {
		t.Fatal(err)
	}
	apnx, err := compileAPNX(context.Background(), path, pub, text, metadata, cfg, zap.NewNop())
	if err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err := writePalmDB(context.Background(), &output, book.Database, time.Time{}, book.Records); err != nil {
		t.Fatal(err)
	}
	return outputFixture{path: path, book: book, apnx: apnx, data: output.Bytes()}
}

func writeOutputTestFile(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}
}

func assertOutputFile(t *testing.T, path string, want []byte) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(data, want) {
		t.Fatalf("output %s differs: got %d bytes, want %d, error %v", path, len(data), len(want), err)
	}
}

func assertNoBundleTemps(t *testing.T, root string) {
	t.Helper()
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if strings.HasPrefix(entry.Name(), ".fbc-azw3-") {
			t.Errorf("temporary bundle file left behind: %s", path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func outputTestAPNXPath(t *testing.T, bookPath, mode string) string {
	t.Helper()
	path, err := apnxOutputPath(bookPath, mode)
	if err != nil {
		t.Fatal(err)
	}
	return path
}

func TestOutputBundlePublish(t *testing.T) {
	for _, mode := range []string{"none", "app", "eink"} {
		for _, overwrite := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/overwrite=%t", mode, overwrite), func(t *testing.T) {
				fixture := newOutputFixture(t, mode)
				if overwrite {
					writeOutputTestFile(t, fixture.path, []byte("old book"))
					if fixture.apnx != nil {
						writeOutputTestFile(t, fixture.apnx.Path, []byte("old APNX"))
					}
				}
				unrelated := filepath.Join(filepath.Dir(fixture.path), "Книга with spaces.sdr", "annotations.dat")
				writeOutputTestFile(t, unrelated, []byte("reader annotations"))
				bundle, err := preflightOutputBundle(context.Background(), fixture.path, fixture.apnx, overwrite)
				if err != nil {
					t.Fatal(err)
				}
				if err := bundle.publish(context.Background(), fixture.book, time.Time{}); err != nil {
					t.Fatal(err)
				}
				assertOutputFile(t, fixture.path, fixture.data)
				if fixture.apnx != nil {
					assertOutputFile(t, fixture.apnx.Path, fixture.apnx.Data)
				}
				assertOutputFile(t, unrelated, []byte("reader annotations"))
				assertNoBundleTemps(t, filepath.Dir(fixture.path))
				if err := bundle.publish(context.Background(), fixture.book, time.Time{}); err == nil {
					t.Fatal("used output bundle was accepted again")
				}
			})
		}
	}
}

func TestOutputBundlePreflight(t *testing.T) {
	for _, name := range []string{
		"book exists", "APNX exists", "book directory", "APNX directory", "sdr file", "sdr symlink",
		"book symlink", "APNX symlink", "dangling book symlink", "empty path", "directory path", "dot path",
		"wrong APNX stem", "wrong APNX directory", "empty APNX", "APNX collides with book", "canceled",
	} {
		t.Run(name, func(t *testing.T) {
			fixture := newOutputFixture(t, "eink")
			ctx := context.Background()
			overwrite := true
			sentinel := filepath.Join(filepath.Dir(fixture.path), "untouched")
			writeOutputTestFile(t, sentinel, []byte("untouched"))
			var want error
			switch name {
			case "book exists":
				writeOutputTestFile(t, fixture.path, []byte("old book"))
				overwrite, want = false, os.ErrExist
			case "APNX exists":
				writeOutputTestFile(t, fixture.apnx.Path, []byte("old APNX"))
				overwrite, want = false, os.ErrExist
			case "book directory":
				if err := os.Mkdir(fixture.path, 0755); err != nil {
					t.Fatal(err)
				}
			case "APNX directory":
				if err := os.MkdirAll(fixture.apnx.Path, 0755); err != nil {
					t.Fatal(err)
				}
			case "sdr file":
				writeOutputTestFile(t, filepath.Dir(fixture.apnx.Path), []byte("not a directory"))
			case "book symlink", "APNX symlink", "sdr symlink", "dangling book symlink":
				target, link := sentinel, fixture.path
				if name == "APNX symlink" {
					link = fixture.apnx.Path
					if err := os.MkdirAll(filepath.Dir(link), 0755); err != nil {
						t.Fatal(err)
					}
				}
				if name == "sdr symlink" {
					link, target = filepath.Dir(fixture.apnx.Path), t.TempDir()
				}
				if name == "dangling book symlink" {
					target += "-missing"
				}
				if err := os.Symlink(target, link); err != nil {
					t.Skipf("symlinks unavailable: %v", err)
				}
			case "empty path":
				fixture.path = ""
			case "directory path":
				fixture.path = filepath.Dir(fixture.path) + string(filepath.Separator)
			case "dot path":
				fixture.path = filepath.Dir(fixture.path) + string(filepath.Separator) + "missing" + string(filepath.Separator) + "."
			case "wrong APNX stem":
				fixture.apnx.Path += "-other"
			case "wrong APNX directory":
				fixture.apnx.Path = filepath.Join(t.TempDir(), filepath.Base(fixture.apnx.Path))
			case "empty APNX":
				fixture.apnx.Data = nil
			case "APNX collides with book":
				fixture.apnx.Path = fixture.path
			case "canceled":
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
				want = context.Canceled
			}
			bundle, err := preflightOutputBundle(ctx, fixture.path, fixture.apnx, overwrite)
			if err == nil || bundle != nil || want != nil && !errors.Is(err, want) {
				t.Fatalf("invalid preflight = %+v, error = %v, want %v", bundle, err, want)
			}
			assertOutputFile(t, sentinel, []byte("untouched"))
			assertNoBundleTemps(t, filepath.Dir(sentinel))
		})
	}
	t.Run("no directory creation", func(t *testing.T) {
		fixture := newOutputFixture(t, "eink")
		root := filepath.Dir(fixture.path)
		fixture.path = filepath.Join(root, "new", "nested", filepath.Base(fixture.path))
		fixture.apnx.Path = outputTestAPNXPath(t, fixture.path, "eink")
		if _, err := preflightOutputBundle(context.Background(), fixture.path, fixture.apnx, false); err != nil {
			t.Fatal(err)
		}
		entries, err := os.ReadDir(root)
		if err != nil || len(entries) != 0 {
			t.Fatalf("preflight changed the filesystem: %v, %v", entries, err)
		}
	})
}

func TestOutputBundleStagingFailure(t *testing.T) {
	for _, name := range []string{"invalid book", "nil book", "canceled", "APNX directory blocked", "new directories", "parent is file"} {
		t.Run(name, func(t *testing.T) {
			fixture := newOutputFixture(t, "eink")
			root := filepath.Dir(fixture.path)
			if name == "new directories" {
				fixture.path = filepath.Join(root, "new", "nested", "book.azw3")
				fixture.apnx.Path = outputTestAPNXPath(t, fixture.path, "eink")
			} else {
				writeOutputTestFile(t, fixture.path, []byte("old book"))
			}
			if name != "APNX directory blocked" && name != "new directories" {
				writeOutputTestFile(t, fixture.apnx.Path, []byte("old APNX"))
			}
			bundle, err := preflightOutputBundle(context.Background(), fixture.path, fixture.apnx, true)
			if err != nil {
				t.Fatal(err)
			}
			ctx := context.Background()
			switch name {
			case "invalid book", "new directories":
				fixture.book.Records = nil
			case "nil book":
				fixture.book = nil
			case "canceled":
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			case "APNX directory blocked":
				writeOutputTestFile(t, filepath.Dir(fixture.apnx.Path), []byte("blocked"))
			case "parent is file":
				bundle.book.path = filepath.Join(root, "blocked", "book.azw3")
				writeOutputTestFile(t, filepath.Join(root, "blocked"), []byte("blocked"))
			}
			if err := bundle.publish(ctx, fixture.book, time.Time{}); err == nil {
				t.Fatal("expected staging failure")
			}
			if name == "new directories" {
				if _, err := os.Stat(filepath.Join(root, "new")); !errors.Is(err, os.ErrNotExist) {
					t.Fatal("empty staging directories were not removed")
				}
			} else {
				assertOutputFile(t, fixture.path, []byte("old book"))
				if name != "APNX directory blocked" {
					assertOutputFile(t, fixture.apnx.Path, []byte("old APNX"))
				}
			}
			assertNoBundleTemps(t, root)
		})
	}
}

func TestOutputBundlePublicationFailure(t *testing.T) {
	injected := errors.New("injected publication failure")
	for _, mode := range []string{"app", "eink"} {
		for _, existing := range []bool{false, true} {
			for _, failure := range []string{"APNX", "book", "cancel after APNX", "rollback"} {
				t.Run(fmt.Sprintf("%s/existing=%t/%s", mode, existing, failure), func(t *testing.T) {
					fixture := newOutputFixture(t, mode)
					writeOutputTestFile(t, fixture.path, []byte("old book"))
					if existing {
						writeOutputTestFile(t, fixture.apnx.Path, []byte("old APNX"))
						if runtime.GOOS != "windows" {
							if err := os.Chmod(fixture.apnx.Path, 0600); err != nil {
								t.Fatal(err)
							}
						}
					}
					bundle, err := preflightOutputBundle(context.Background(), fixture.path, fixture.apnx, true)
					if err != nil {
						t.Fatal(err)
					}
					ctx, cancel := context.WithCancel(context.Background())
					defer cancel()
					calls := 0
					bundle.publishFile = func(source, destination string, overwrite bool) error {
						calls++
						if calls == 1 {
							// Both complete siblings are closed/readable before the first rename.
							assertOutputFile(t, bundle.book.temp, fixture.data)
							assertOutputFile(t, source, fixture.apnx.Data)
							if destination != fixture.apnx.Path {
								t.Fatal("book was published before APNX")
							}
							if failure == "APNX" {
								return injected
							}
							if err := filepub.Publish(source, destination, overwrite); err != nil {
								return err
							}
							if failure == "cancel after APNX" {
								cancel()
							}
							return nil
						}
						if destination == fixture.path || failure == "rollback" {
							return injected
						}
						return filepub.Publish(source, destination, overwrite)
					}
					// A new APNX has no backup rename to fail; use a foreign file
					// to exercise safe refusal of rollback removal in that case.
					if failure == "rollback" && !existing {
						originalPublish := bundle.publishFile
						bundle.publishFile = func(source, destination string, overwrite bool) error {
							if destination == fixture.path {
								foreign := filepath.Join(filepath.Dir(fixture.path), "foreign.apnx")
								writeOutputTestFile(t, foreign, []byte("other writer"))
								if err := filepub.Publish(foreign, fixture.apnx.Path, true); err != nil {
									t.Fatal(err)
								}
							}
							return originalPublish(source, destination, overwrite)
						}
					}
					err = bundle.publish(ctx, fixture.book, time.Time{})
					want := injected
					if failure == "cancel after APNX" {
						want = context.Canceled
					}
					if !errors.Is(err, want) {
						t.Fatalf("error = %v, want %v", err, want)
					}
					assertOutputFile(t, fixture.path, []byte("old book"))
					if failure == "rollback" {
						if !strings.Contains(err.Error(), "rollback failed") {
							t.Fatalf("partial publication was not reported: %v", err)
						}
						if existing {
							assertOutputFile(t, fixture.apnx.Path, fixture.apnx.Data)
							assertOutputFile(t, bundle.backup.temp, []byte("old APNX"))
							if !strings.Contains(err.Error(), bundle.backup.temp) {
								t.Fatal("recovery path is missing from error")
							}
							if err := os.Remove(bundle.backup.temp); err != nil {
								t.Fatal(err)
							}
						} else {
							assertOutputFile(t, fixture.apnx.Path, []byte("other writer"))
						}
					} else if existing {
						assertOutputFile(t, fixture.apnx.Path, []byte("old APNX"))
						if runtime.GOOS != "windows" {
							info, statErr := os.Stat(fixture.apnx.Path)
							if statErr != nil || info.Mode().Perm() != 0600 {
								t.Fatal("APNX rollback did not preserve permission bits", statErr)
							}
						}
					} else if _, err := os.Stat(fixture.apnx.Path); !errors.Is(err, os.ErrNotExist) {
						t.Fatal("new APNX was not rolled back")
					}
					assertNoBundleTemps(t, filepath.Dir(fixture.path))
					if mode == "eink" && !existing && failure != "rollback" {
						if _, err := os.Stat(filepath.Dir(fixture.apnx.Path)); !errors.Is(err, os.ErrNotExist) {
							t.Fatal("empty new .sdr directory was not removed")
						}
					}
				})
			}
		}
	}
}

func TestOutputBundleNoOverwriteRaces(t *testing.T) {
	for _, target := range []string{"book before staging", "APNX before staging", "APNX at rename", "book after APNX"} {
		t.Run(target, func(t *testing.T) {
			fixture := newOutputFixture(t, "eink")
			bundle, err := preflightOutputBundle(context.Background(), fixture.path, fixture.apnx, false)
			if err != nil {
				t.Fatal(err)
			}
			collision := fixture.path
			switch target {
			case "book before staging":
				writeOutputTestFile(t, collision, []byte("other writer"))
			case "APNX before staging":
				collision = fixture.apnx.Path
				writeOutputTestFile(t, collision, []byte("other writer"))
			default:
				bundle.publishFile = func(source, destination string, overwrite bool) error {
					if target == "APNX at rename" {
						collision = fixture.apnx.Path
						writeOutputTestFile(t, collision, []byte("other writer"))
					}
					if err := filepub.Publish(source, destination, overwrite); err != nil {
						return err
					}
					if target == "book after APNX" {
						writeOutputTestFile(t, collision, []byte("other writer"))
					}
					return nil
				}
			}
			if err := bundle.publish(context.Background(), fixture.book, time.Time{}); !errors.Is(err, os.ErrExist) {
				t.Fatalf("expected no-overwrite collision, got %v", err)
			}
			assertOutputFile(t, collision, []byte("other writer"))
			other := fixture.path
			if collision == fixture.path {
				other = fixture.apnx.Path
			}
			if _, err := os.Stat(other); !errors.Is(err, os.ErrNotExist) {
				t.Fatal("failed bundle left the other output published")
			}
			assertNoBundleTemps(t, filepath.Dir(fixture.path))
		})
	}
}

func TestStageBundleFileFailure(t *testing.T) {
	injected := errors.New("injected staging I/O failure")
	for _, failure := range []string{"write", "closed file", "cancel"} {
		t.Run(failure, func(t *testing.T) {
			output := stagedOutput{path: filepath.Join(t.TempDir(), "book.azw3")}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			err := stageBundleFile(ctx, &output, 0644, func(w io.Writer) error {
				if _, err := w.Write([]byte("partial data")); err != nil {
					return err
				}
				switch failure {
				case "write":
					return injected
				case "closed file":
					// The concrete staged file is deliberately closed to force
					// filesystem and deferred-close errors without global hooks.
					return w.(*os.File).Close()
				case "cancel":
					cancel()
				}
				return nil
			})
			if err == nil || failure == "write" && !errors.Is(err, injected) || failure == "cancel" && !errors.Is(err, context.Canceled) {
				t.Fatalf("staging failure = %v", err)
			}
			bundle := outputBundle{book: output}
			if err := bundle.cleanup(false, false); err != nil {
				t.Fatal(err)
			}
			assertNoBundleTemps(t, filepath.Dir(output.path))
		})
	}
}

func TestOutputBundleHardLinkFallbackAndCleanup(t *testing.T) {
	for _, cleanupFailure := range []bool{false, true} {
		t.Run(fmt.Sprintf("cleanup failure=%t", cleanupFailure), func(t *testing.T) {
			fixture := newOutputFixture(t, "app")
			bundle, err := preflightOutputBundle(context.Background(), fixture.path, fixture.apnx, false)
			if err != nil {
				t.Fatal(err)
			}
			bundle.publishFile = func(source, destination string, _ bool) error {
				if err := os.Link(source, destination); err != nil {
					return err
				}
				if cleanupFailure && destination == fixture.path {
					if err := os.Remove(source); err != nil {
						t.Fatal(err)
					}
					writeOutputTestFile(t, filepath.Join(source, "block-cleanup"), []byte("unrelated"))
				}
				return nil
			}
			err = bundle.publish(context.Background(), fixture.book, time.Time{})
			if cleanupFailure {
				if err == nil || !strings.Contains(err.Error(), "was published, but cleanup failed") {
					t.Fatalf("committed output with cleanup failure was not identified: %v", err)
				}
				if err := os.Remove(filepath.Join(bundle.book.temp, "block-cleanup")); err != nil {
					t.Fatal(err)
				}
				if err := os.Remove(bundle.book.temp); err != nil {
					t.Fatal(err)
				}
			} else if err != nil {
				t.Fatal(err)
			}
			assertOutputFile(t, fixture.path, fixture.data)
			assertOutputFile(t, fixture.apnx.Path, fixture.apnx.Data)
			assertNoBundleTemps(t, filepath.Dir(fixture.path))
		})
	}
}

func TestOutputBundleCancellationAfterCommit(t *testing.T) {
	fixture := newOutputFixture(t, "app")
	bundle, err := preflightOutputBundle(context.Background(), fixture.path, fixture.apnx, false)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	bundle.publishFile = func(source, destination string, overwrite bool) error {
		if err := filepub.Publish(source, destination, overwrite); err != nil {
			return err
		}
		if destination == fixture.path {
			cancel()
		}
		return nil
	}
	if err := bundle.publish(ctx, fixture.book, time.Time{}); err != nil {
		t.Fatalf("cancellation after commit incorrectly reported failure: %v", err)
	}
	assertOutputFile(t, fixture.path, fixture.data)
	assertOutputFile(t, fixture.apnx.Path, fixture.apnx.Data)
}

func TestOutputBundleNoAPNXLeavesSidecarsAlone(t *testing.T) {
	fixture := newOutputFixture(t, "none")
	for _, mode := range []string{"app", "eink"} {
		path, err := apnxOutputPath(fixture.path, mode)
		if err != nil {
			t.Fatal(err)
		}
		writeOutputTestFile(t, path, []byte("unselected sidecar"))
	}
	bundle, err := preflightOutputBundle(context.Background(), fixture.path, nil, true)
	if err != nil {
		t.Fatal(err)
	}
	if err := bundle.publish(context.Background(), fixture.book, time.Time{}); err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"app", "eink"} {
		path := outputTestAPNXPath(t, fixture.path, mode)
		assertOutputFile(t, path, []byte("unselected sidecar"))
	}
}

func TestOutputBundleLongFinalName(t *testing.T) {
	fixture := newOutputFixture(t, "eink")
	fixture.path = filepath.Join(filepath.Dir(fixture.path), strings.Repeat("b", 250)+".azw3")
	fixture.apnx.Path = outputTestAPNXPath(t, fixture.path, "eink")
	bundle, err := preflightOutputBundle(context.Background(), fixture.path, fixture.apnx, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := bundle.publish(context.Background(), fixture.book, time.Time{}); err != nil {
		t.Fatal(err)
	}
	assertOutputFile(t, fixture.path, fixture.data)
	assertOutputFile(t, fixture.apnx.Path, fixture.apnx.Data)
	assertNoBundleTemps(t, filepath.Dir(fixture.path))
}

func TestOutputBundleFilesystemPermissions(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("requires Unix permission checks as an unprivileged user")
	}
	for _, failure := range []string{"APNX staging", "APNX backup"} {
		t.Run(failure, func(t *testing.T) {
			fixture := newOutputFixture(t, "eink")
			writeOutputTestFile(t, fixture.path, []byte("old book"))
			writeOutputTestFile(t, fixture.apnx.Path, []byte("old APNX"))
			protected := fixture.apnx.Path
			mode, restore := os.FileMode(0), os.FileMode(0644)
			if failure == "APNX staging" {
				protected = filepath.Dir(fixture.apnx.Path)
				mode, restore = 0555, 0755
			}
			if err := os.Chmod(protected, mode); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := os.Chmod(protected, restore); err != nil {
					t.Error(err)
				}
			})
			bundle, err := preflightOutputBundle(context.Background(), fixture.path, fixture.apnx, true)
			if err != nil {
				t.Fatal(err)
			}
			if err := bundle.publish(context.Background(), fixture.book, time.Time{}); !errors.Is(err, os.ErrPermission) {
				t.Fatalf("expected filesystem permission failure, got %v", err)
			}
			if err := os.Chmod(protected, restore); err != nil {
				t.Fatal(err)
			}
			assertOutputFile(t, fixture.path, []byte("old book"))
			assertOutputFile(t, fixture.apnx.Path, []byte("old APNX"))
			assertNoBundleTemps(t, filepath.Dir(fixture.path))
		})
	}
}

func TestOutputBundleRollbackWithExternalChanges(t *testing.T) {
	injected := errors.New("injected book publication failure")
	for _, change := range []string{"APNX missing", "foreign APNX", "annotations", "book symlink"} {
		t.Run(change, func(t *testing.T) {
			fixture := newOutputFixture(t, "eink")
			writeOutputTestFile(t, fixture.apnx.Path, []byte("old APNX"))
			unrelated := filepath.Join(filepath.Dir(fixture.apnx.Path), "notes.dat")
			sentinel := filepath.Join(filepath.Dir(fixture.path), "symlink-target")
			if change == "book symlink" {
				writeOutputTestFile(t, sentinel, []byte("do not overwrite"))
				// Check capability before entering a publication callback.
				link := filepath.Join(filepath.Dir(fixture.path), "test-link")
				if err := os.Symlink(sentinel, link); err != nil {
					t.Skipf("symlinks unavailable: %v", err)
				}
				if err := os.Remove(link); err != nil {
					t.Fatal(err)
				}
			}
			bundle, err := preflightOutputBundle(context.Background(), fixture.path, fixture.apnx, true)
			if err != nil {
				t.Fatal(err)
			}
			bundle.publishFile = func(source, destination string, overwrite bool) error {
				if destination == fixture.path {
					return injected
				}
				if source == bundle.backup.temp {
					return filepub.Publish(source, destination, overwrite)
				}
				if err := filepub.Publish(source, destination, overwrite); err != nil {
					return err
				}
				switch change {
				case "APNX missing":
					if err := os.Remove(fixture.apnx.Path); err != nil {
						t.Fatal(err)
					}
				case "foreign APNX":
					foreign := filepath.Join(filepath.Dir(fixture.path), "foreign.apnx")
					writeOutputTestFile(t, foreign, []byte("other writer"))
					if err := filepub.Publish(foreign, fixture.apnx.Path, true); err != nil {
						t.Fatal(err)
					}
				case "annotations":
					writeOutputTestFile(t, unrelated, []byte("new annotations"))
				case "book symlink":
					if err := os.Symlink(sentinel, fixture.path); err != nil {
						t.Fatal(err)
					}
				}
				return nil
			}
			err = bundle.publish(context.Background(), fixture.book, time.Time{})
			if err == nil {
				t.Fatal("expected publication failure")
			}
			if change != "book symlink" && !errors.Is(err, injected) {
				t.Fatalf("original failure was lost: %v", err)
			}
			if change == "foreign APNX" {
				assertOutputFile(t, fixture.apnx.Path, []byte("other writer"))
				assertOutputFile(t, bundle.backup.temp, []byte("old APNX"))
				if !strings.Contains(err.Error(), bundle.backup.temp) {
					t.Fatal("retained recovery path was not reported")
				}
				if err := os.Remove(bundle.backup.temp); err != nil {
					t.Fatal(err)
				}
			} else {
				assertOutputFile(t, fixture.apnx.Path, []byte("old APNX"))
			}
			if change == "annotations" {
				assertOutputFile(t, unrelated, []byte("new annotations"))
			}
			if change == "book symlink" {
				assertOutputFile(t, sentinel, []byte("do not overwrite"))
			}
			assertNoBundleTemps(t, filepath.Dir(fixture.path))
		})
	}
}
