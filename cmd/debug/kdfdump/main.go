// kdfdump reads KDF files produced by Kindle Previewer (KP3), strips SQLite fingerprint
// records, parses the Ion fragment payloads, and produces output similar to kfxdump.
//
// KDF files are standard SQLite databases with 1024-byte "fingerprint" records injected at
// regular intervals. The fragments table contains Ion binary payloads identical in format
// to KFX container entities.
//
// Input can be either a standalone .kdf file or a .kpf (ZIP) archive containing a .kdf entry.
package main

import (
	"archive/zip"
	"bytes"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"zombiezen.com/go/sqlite"
	"zombiezen.com/go/sqlite/sqlitex"

	"fbc/cmd/debug/internal/dumputil"
	"fbc/convert/kfx"
)

const (
	fingerprintOffset    = 1024
	fingerprintRecordLen = 1024
	dataRecordLen        = 1024
	dataRecordCount      = 1024
)

var (
	sqliteSig      = []byte("SQLite format 3\x00")
	fingerprintSig = []byte{0xfa, 0x50, 0x0a, 0x5f}
	zipSig         = []byte("PK\x03\x04")
)

func main() {
	all := flag.Bool("all", false, "enable all dump flags (-dump, -resources, -styles, -storyline, -margins, -sqlite)")
	dump := flag.Bool("dump", false, "dump all fragments into <file>-dump.txt")
	resources := flag.Bool("resources", false, "dump $417/$418 raw bytes into <file>-resources.zip")
	styles := flag.Bool("styles", false, "dump $157 (style) fragments into <file>-styles.txt")
	storyline := flag.Bool("storyline", false, "dump $259 (storyline) fragments into <file>-storyline.txt with expanded symbols and styles")
	margins := flag.Bool("margins", false, "dump vertical margin tree into <file>-margins.txt for easy comparison")
	writeSqlite := flag.Bool("sqlite", false, "write clean SQLite database to <file>.sqlite")
	overwrite := flag.Bool("overwrite", false, "overwrite existing output")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: kdfdump [-all] [-dump] [-resources] [-styles] [-storyline] [-margins] [-sqlite] [-overwrite] <file.kdf|file.kpf> [outdir]\n\n")
		fmt.Fprintf(os.Stderr, "Reads KDF/KPF files and produces output similar to kfxdump.\n")
		fmt.Fprintf(os.Stderr, "Fingerprint records are stripped automatically before parsing.\n\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	if flag.NArg() < 1 || flag.NArg() > 2 {
		flag.Usage()
		os.Exit(2)
	}

	if *all {
		*dump = true
		*resources = true
		*styles = true
		*storyline = true
		*margins = true
		*writeSqlite = true
	}

	if !*dump && !*resources && !*styles && !*storyline && !*margins && !*writeSqlite {
		flag.Usage()
		os.Exit(2)
	}

	defer func(startedAt time.Time) {
		fmt.Fprintf(os.Stderr, "\nExecution time: %s\n", time.Since(startedAt))
	}(time.Now())

	inPath := flag.Arg(0)
	outDir := ""
	if flag.NArg() == 2 {
		outDir = flag.Arg(1)
	}

	b, err := os.ReadFile(inPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read %s: %v\n", inPath, err)
		os.Exit(1)
	}

	kdfData, kdfName, resourceResolver, err := extractKDF(b, inPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "extract KDF from %s: %v\n", inPath, err)
		os.Exit(1)
	}

	cleaned, fpCount, err := removeFingerprints(kdfData)
	if err != nil {
		fmt.Fprintf(os.Stderr, "remove fingerprints: %v\n", err)
		os.Exit(1)
	}

	if fpCount == 0 {
		fmt.Fprintf(os.Stderr, "no fingerprints found in %s, file is already a clean SQLite database\n", kdfName)
	} else {
		fmt.Fprintf(os.Stderr, "removed %d fingerprint(s) from %s (%d bytes -> %d bytes)\n",
			fpCount, kdfName, len(kdfData), len(cleaned))
	}

	// Write clean SQLite if requested.
	if *writeSqlite {
		if err := dumputil.WriteOutput(inPath, outDir, ".sqlite", cleaned, *overwrite); err != nil {
			fmt.Fprintf(os.Stderr, "write sqlite: %v\n", err)
			os.Exit(1)
		}
	}

	// Parse fragments and build Container if any content dump mode is active.
	needContainer := *dump || *resources || *styles || *storyline || *margins
	if needContainer {
		container, err := readKDFContainer(cleaned, resourceResolver)
		if err != nil {
			fmt.Fprintf(os.Stderr, "parse KDF container: %v\n", err)
			os.Exit(1)
		}

		if *dump {
			if err := dumputil.DumpDumpTxt(container, inPath, outDir, *overwrite); err != nil {
				fmt.Fprintf(os.Stderr, "dump: %v\n", err)
				os.Exit(1)
			}
			fmt.Println(container.StatsString())
		}

		if *resources {
			if err := dumputil.DumpResources(container, inPath, outDir, *overwrite); err != nil {
				fmt.Fprintf(os.Stderr, "dump resources: %v\n", err)
				os.Exit(1)
			}
		}

		if *styles {
			if err := dumputil.DumpStylesTxt(container, inPath, outDir, *overwrite); err != nil {
				fmt.Fprintf(os.Stderr, "dump styles: %v\n", err)
				os.Exit(1)
			}
		}

		if *storyline {
			if err := dumputil.DumpStorylineTxt(container, inPath, outDir, *overwrite); err != nil {
				fmt.Fprintf(os.Stderr, "dump storyline: %v\n", err)
				os.Exit(1)
			}
		}

		if *margins {
			if err := dumputil.DumpMarginsTxt(container, inPath, outDir, *overwrite); err != nil {
				fmt.Fprintf(os.Stderr, "dump margins: %v\n", err)
				os.Exit(1)
			}
		}
	}
}

// resourceResolverFunc resolves a resource path (e.g., "res/rsrc7") to its binary data.
// Returns nil if the resource cannot be found.
type resourceResolverFunc func(path string) []byte

// readKDFContainer opens the cleaned SQLite data in memory, reads all fragments,
// and builds a kfx.Container. The optional resolver is used to read external resource
// data from a KPF ZIP archive when fragments have payload_type="path".
func readKDFContainer(data []byte, resolver resourceResolverFunc) (*kfx.Container, error) {
	conn, err := sqlite.OpenConn(":memory:", sqlite.OpenReadWrite, sqlite.OpenMemory)
	if err != nil {
		return nil, fmt.Errorf("open in-memory db: %w", err)
	}
	defer conn.Close()

	if err := conn.Deserialize("main", data); err != nil {
		return nil, fmt.Errorf("deserialize: %w", err)
	}

	c := kfx.NewContainer()
	c.ContainerFormat = "KDF"

	// Step 1: Read $ion_symbol_table fragment to establish the doc symbol table.
	// If not present, fall back to the "max_id" fragment which provides the total
	// symbol count (used by some KDF files as a lighter alternative).
	// This must be done first so we can decode other fragments with proper symbols.
	var docSymData []byte
	err = sqlitex.Execute(conn, `SELECT payload_value FROM fragments WHERE id='$ion_symbol_table'`,
		&sqlitex.ExecOptions{ResultFunc: func(stmt *sqlite.Stmt) error {
			r := stmt.ColumnReader(0)
			docSymData, err = io.ReadAll(r)
			return err
		}})
	if err != nil {
		return nil, fmt.Errorf("read $ion_symbol_table: %w", err)
	}

	// Determine the Ion prolog for decoding payloads.
	var lstProlog []byte
	if len(docSymData) > 0 {
		docSymTab, err := kfx.DecodeSymbolTable(docSymData)
		if err != nil {
			return nil, fmt.Errorf("decode doc symbol table: %w", err)
		}
		c.DocSymbolTable = docSymTab
		c.LocalSymbols = docSymTab.Symbols()
		lstProlog = docSymData
	} else {
		// No $ion_symbol_table — try max_id fragment as fallback.
		var maxIDData []byte
		err = sqlitex.Execute(conn, `SELECT payload_value FROM fragments WHERE id='max_id'`,
			&sqlitex.ExecOptions{ResultFunc: func(stmt *sqlite.Stmt) error {
				r := stmt.ColumnReader(0)
				maxIDData, err = io.ReadAll(r)
				return err
			}})
		if err != nil {
			return nil, fmt.Errorf("read max_id: %w", err)
		}

		if len(maxIDData) > 0 {
			// max_id payload is an Ion integer (with BVM prefix) containing the total
			// symbol count including Ion system symbols.
			maxID, err := readIonInt(maxIDData)
			if err != nil {
				return nil, fmt.Errorf("decode max_id: %w", err)
			}
			fmt.Fprintf(os.Stderr, "using max_id=%d for symbol table (no $ion_symbol_table fragment)\n", maxID)
			lstProlog = kfx.CreatePrologForMaxID(maxID)
		} else {
			lstProlog = kfx.GetIonProlog()
		}
	}

	// Step 2: Read format_capabilities from the capabilities table.
	var capabilities []any
	err = sqlitex.Execute(conn, `SELECT key, version FROM capabilities`,
		&sqlitex.ExecOptions{ResultFunc: func(stmt *sqlite.Stmt) error {
			key := stmt.ColumnText(0)
			version := stmt.ColumnInt64(1)
			entry := map[string]any{
				"$492":    kfx.ReadSymbolValue(key),
				"version": version,
			}
			capabilities = append(capabilities, entry)
			return nil
		}})
	if err != nil {
		return nil, fmt.Errorf("read capabilities: %w", err)
	}
	if len(capabilities) > 0 {
		c.FormatCapabilities = capabilities
	}

	// Step 3: Build element_type map from fragment_properties.
	elementTypes := make(map[string]string)
	err = sqlitex.Execute(conn, `SELECT id, value FROM fragment_properties WHERE key='element_type'`,
		&sqlitex.ExecOptions{ResultFunc: func(stmt *sqlite.Stmt) error {
			elementTypes[stmt.ColumnText(0)] = stmt.ColumnText(1)
			return nil
		}})
	if err != nil {
		return nil, fmt.Errorf("read fragment_properties: %w", err)
	}

	// Step 4: Read all fragments.
	// Process $ion_symbol_table and max_id first (already handled above), then everything else.
	err = sqlitex.Execute(conn, `SELECT id, payload_type, payload_value FROM fragments`,
		&sqlitex.ExecOptions{ResultFunc: func(stmt *sqlite.Stmt) error {
			id := stmt.ColumnText(0)
			payloadType := stmt.ColumnText(1)

			// Skip $ion_symbol_table and max_id — already processed above.
			// Skip max_eid_in_sections — dictionary-specific, not needed for dump.
			if id == "$ion_symbol_table" || id == "max_id" || id == "max_eid_in_sections" {
				return nil
			}

			r := stmt.ColumnReader(2)
			payload, err := io.ReadAll(r)
			if err != nil {
				return fmt.Errorf("read payload for %s: %w", id, err)
			}

			return processFragment(c, id, payloadType, payload, lstProlog, elementTypes, resolver)
		}})
	if err != nil {
		return nil, fmt.Errorf("read fragments: %w", err)
	}

	// Synthesize $270 (container) fragment for tooling/debug.
	if c.Fragments.GetRoot(kfx.SymContainer) == nil {
		_ = c.Fragments.Add(kfx.BuildContainerFragment(c))
	}

	return c, nil
}

// processFragment parses a single KDF fragment row and adds it to the Container.
func processFragment(c *kfx.Container, id, payloadType string, payload []byte, lstProlog []byte, elementTypes map[string]string, resolver resourceResolverFunc) error {
	switch payloadType {
	case "blob":
		return processBlobFragment(c, id, payload, lstProlog, elementTypes)
	case "path":
		return processPathFragment(c, id, payload, elementTypes, resolver)
	default:
		fmt.Fprintf(os.Stderr, "warning: unknown payload_type=%q for id=%s, skipping\n", payloadType, id)
		return nil
	}
}

// processBlobFragment handles Ion binary payloads.
func processBlobFragment(c *kfx.Container, id string, payload []byte, lstProlog []byte, elementTypes map[string]string) error {
	if len(payload) == 0 {
		return nil
	}

	// Check if payload starts with Ion BVM.
	if !kfx.HasIonBVM(payload) {
		// Not Ion — treat as raw media ($417).
		frag := &kfx.Fragment{
			FType:   kfx.SymRawMedia,
			FID:     kfx.SymRawMedia,
			FIDName: id,
			Value:   kfx.RawValue(payload),
		}
		return c.Fragments.Add(frag)
	}

	// Empty Ion payload (just BVM, 4 bytes).
	if len(payload) <= 4 {
		return nil
	}

	// Parse the Ion payload to get the annotation (fragment type) and value.
	reader := kfx.NewIonReaderWithLocalSymbols(lstProlog, payload, c.LocalSymbols)
	if !reader.Next() {
		if err := reader.Err(); err != nil {
			return fmt.Errorf("read ion for %s: %w", id, err)
		}
		return nil // No value
	}

	// Read annotation to determine fragment type.
	annotations, err := reader.Annotations()
	if err != nil {
		return fmt.Errorf("read annotations for %s: %w", id, err)
	}

	var ftypeName string
	if len(annotations) > 0 {
		ftypeName = annotations[0]
	} else {
		// No annotation — use element_type from fragment_properties as fallback.
		if et, ok := elementTypes[id]; ok {
			ftypeName = et
		} else {
			fmt.Fprintf(os.Stderr, "warning: fragment %s has no annotation and no element_type, skipping\n", id)
			return nil
		}
	}

	ftype := kfx.SymbolID(ftypeName)
	if ftype < 0 {
		fmt.Fprintf(os.Stderr, "warning: unknown fragment type %q for id=%s, skipping\n", ftypeName, id)
		return nil
	}

	// Determine if this is a root fragment.
	isRoot := kfx.ROOT_FRAGMENT_TYPES[ftype]

	// Read the value.
	var value any
	if kfx.RAW_FRAGMENT_TYPES[ftype] {
		value = kfx.RawValue(payload)
	} else {
		value, err = reader.ReadValue()
		if err != nil {
			return fmt.Errorf("read value for %s (%s): %w", id, ftypeName, err)
		}
	}

	frag := &kfx.Fragment{
		FType: ftype,
		FID:   ftype, // Will be overridden below for non-root.
		Value: value,
	}

	if !isRoot {
		frag.FIDName = id
		frag.FID = 0
	}

	return c.Fragments.Add(frag)
}

// processPathFragment handles resource path payloads.
// These point to external files (e.g., "res/rsrc7") within the KPF ZIP archive.
// If a resolver is available (KPF input), the resource data is read and stored as
// a raw media ($417) fragment. Otherwise, the fragment is stored with empty content.
func processPathFragment(c *kfx.Container, id string, payload []byte, elementTypes map[string]string, resolver resourceResolverFunc) error {
	if len(payload) == 0 {
		return nil
	}

	path := strings.TrimSpace(string(payload))

	var resourceData []byte
	if resolver != nil {
		resourceData = resolver(path)
		if resourceData != nil {
			fmt.Fprintf(os.Stderr, "resolved resource %s from %q (%d bytes)\n", id, path, len(resourceData))
		} else {
			fmt.Fprintf(os.Stderr, "warning: could not resolve resource %s from %q\n", id, path)
		}
	} else {
		fmt.Fprintf(os.Stderr, "note: fragment %s references external resource %q (not embedded in KDF)\n", id, path)
	}

	frag := &kfx.Fragment{
		FType:   kfx.SymRawMedia,
		FIDName: id,
		Value:   kfx.RawValue(resourceData),
	}
	return c.Fragments.Add(frag)
}

// --- Ion helpers ---

// readIonInt reads a single Ion integer from data (with BVM prefix).
func readIonInt(data []byte) (int, error) {
	reader := kfx.NewIonReaderBytes(data)
	if !reader.Next() {
		if err := reader.Err(); err != nil {
			return 0, err
		}
		return 0, fmt.Errorf("no value in max_id payload")
	}
	v, err := reader.ReadValue()
	if err != nil {
		return 0, err
	}
	switch n := v.(type) {
	case int64:
		return int(n), nil
	case int:
		return n, nil
	default:
		return 0, fmt.Errorf("max_id value is %T, expected int", v)
	}
}

// --- KDF extraction and fingerprint removal ---

// extractKDF extracts KDF data from the input. If the input is a ZIP (KPF) archive, it looks for
// a .kdf entry inside and returns a resource resolver for reading external resources from the ZIP.
// For standalone KDF files, the resolver is nil.
func extractKDF(data []byte, name string) (kdfData []byte, kdfName string, resolver resourceResolverFunc, err error) {
	if len(data) < len(zipSig) {
		return nil, "", nil, fmt.Errorf("file too small")
	}

	// Check if input is a KPF (ZIP) archive.
	if bytes.HasPrefix(data, zipSig) {
		r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
		if err != nil {
			return nil, "", nil, fmt.Errorf("open ZIP: %w", err)
		}

		// Build a map of all ZIP entries for fast lookup.
		zipEntries := make(map[string]*zip.File, len(r.File))
		for _, f := range r.File {
			zipEntries[f.Name] = f
		}

		// Find the .kdf entry.
		var kdfEntry *zip.File
		for _, f := range r.File {
			if strings.EqualFold(filepath.Ext(f.Name), ".kdf") {
				kdfEntry = f
				break
			}
		}
		if kdfEntry == nil {
			return nil, "", nil, fmt.Errorf("no .kdf entry found in ZIP archive %s", name)
		}

		rc, err := kdfEntry.Open()
		if err != nil {
			return nil, "", nil, fmt.Errorf("open %s in ZIP: %w", kdfEntry.Name, err)
		}
		defer rc.Close()

		buf, err := io.ReadAll(rc)
		if err != nil {
			return nil, "", nil, fmt.Errorf("read %s from ZIP: %w", kdfEntry.Name, err)
		}

		// Create a resource resolver that resolves paths relative to the KDF entry's
		// directory within the ZIP (e.g., "res/rsrc7" -> "content/res/rsrc7" if KDF
		// is at "content/book.kdf").
		kdfDir := filepath.ToSlash(filepath.Dir(kdfEntry.Name))
		resolver = func(path string) []byte {
			// Resolve relative to the KDF's directory in the ZIP.
			resolvedPath := path
			if kdfDir != "" && kdfDir != "." {
				resolvedPath = kdfDir + "/" + path
			}

			f, ok := zipEntries[resolvedPath]
			if !ok {
				return nil
			}
			rc, err := f.Open()
			if err != nil {
				return nil
			}
			defer rc.Close()
			data, err := io.ReadAll(rc)
			if err != nil {
				return nil
			}
			return data
		}

		return buf, kdfEntry.Name, resolver, nil
	}

	// Standalone KDF file — must start with SQLite signature.
	if !bytes.HasPrefix(data, sqliteSig) {
		return nil, "", nil, fmt.Errorf("file does not start with SQLite or ZIP signature")
	}

	return data, filepath.Base(name), nil, nil
}

// removeFingerprints strips KDF fingerprint records from SQLite data.
// Returns the cleaned data, the number of fingerprints removed, and any error.
func removeFingerprints(data []byte) ([]byte, int, error) {
	if len(data) < fingerprintOffset+fingerprintRecordLen {
		return data, 0, nil
	}

	// Check if the first fingerprint is present.
	if !bytes.Equal(data[fingerprintOffset:fingerprintOffset+len(fingerprintSig)], fingerprintSig) {
		return data, 0, nil
	}

	// Work on a copy so we don't mutate the caller's slice.
	out := make([]byte, 0, len(data))
	out = append(out, data[:fingerprintOffset]...)
	data = data[fingerprintOffset:]

	count := 0
	dataStride := dataRecordLen * dataRecordCount // 1 MB of actual SQLite data between fingerprints

	for len(data) >= fingerprintRecordLen {
		// Verify fingerprint signature.
		if !bytes.Equal(data[:len(fingerprintSig)], fingerprintSig) {
			return nil, 0, fmt.Errorf("unexpected data at fingerprint %d position: got %02x, want %02x",
				count, data[:len(fingerprintSig)], fingerprintSig)
		}

		// Skip the fingerprint record, copy the next dataStride bytes of real data.
		data = data[fingerprintRecordLen:]
		count++

		// Copy up to dataStride bytes of actual data.
		copyLen := dataStride
		if copyLen > len(data) {
			copyLen = len(data)
		}
		out = append(out, data[:copyLen]...)
		data = data[copyLen:]
	}

	// Append any remaining data after the last fingerprint/data cycle.
	out = append(out, data...)

	return out, count, nil
}
