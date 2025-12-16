package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"fbc/misc"
)

// In MHL
// params := Format('"%s" "%s"', [InpFile, ChangeFileExt(OutFile, '.epub')]);
// Result := ExecAndWait(FAppPath + 'converters\fb2epub\fb2epub.exe', params, SW_HIDE);

const usageMsg = `
	MyHomeLib wrapper for fb2 (ng) converter
	Version %s (%s) : %s

	Expected usage: %s <from fb2> <to file>

	MyHomeLib expect converters to be located in the installation directory with following structure:

	MHL installation directory
	|	MyHomeLib.exe
	|
	\---converters
		+---fb2converter
		|		fbc.exe
		|		mhl-connector.exe
		|
		+---fb2epub
		|		fb2epub.exe  (copy of mhl-connector.exe or symlink to it)
		|		fb2epub.yaml (fbc.exe configuration file if needed)
		|
		\---fb2mobi
				fb2mobi.exe  (copy of mhl-connector.exe or symlink to it)
				fb2mobi.yaml (fbc.exe configuration file if needed)

	If you are copying mhl-connector.exe you could either follow above structure or have fbc.exe in a OS PATH.
	If you are using symlinks, mhl-connector.exe should be located next to fbc.exe and they could be anywhere,
	no fb2converter directory or OS PATH modification is necessary.
`

func main() {

	log.SetPrefix("\n*** ")

	if len(os.Args) < 3 {
		fmt.Fprintf(os.Stderr, usageMsg, misc.GetVersion(), runtime.Version(), misc.GetGitHash(), os.Args[0])
		os.Exit(0)
	}

	exePath, err := os.Executable()
	if err != nil {
		log.Fatal(err)
	}

	resolvedPath, err := filepath.EvalSymlinks(exePath)
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("Started as: %s", exePath)
	log.Printf("Actual path: %s", resolvedPath)

	// let's locate actual convesion engine
	converterName := misc.GetAppName() + ".exe"

	paths := []string{
		filepath.Join(filepath.Dir(resolvedPath), converterName),            // where symlink points
		filepath.Join(filepath.Dir(exePath), converterName),                 // where I was started from
		filepath.Join(filepath.Dir(exePath), "fb2converter", converterName), // `pwd`/../fb2converter
		filepath.Join(converterName),                                        // in the system PATH
	}

	var converterPath string
	for _, p := range paths {
		if converterPath, err = exec.LookPath(p); err == nil {
			break
		}
		converterPath = ""
	}
	if len(converterPath) == 0 {
		log.Fatalf("Unable to locate conversion engine: %s", converterName)
	}

	// let's get the target name from the executable name
	target := strings.TrimSuffix(filepath.Base(exePath), filepath.Ext(exePath))
	if !strings.EqualFold(target, "fb2mobi") && !strings.EqualFold(target, "fb2epub") {
		log.Fatalf("MHL connector could be named either fb2mobi or fb2epub (or started via appropriate symlinks), current name is: %s", target)
	}

	from, err := filepath.Abs(os.Args[1])
	if err != nil {
		log.Fatal(err)
	}
	if _, err := os.Stat(from); err != nil {
		log.Printf("Source file does not exist: %s", from)
		log.Fatal(err)
	}

	to, err := filepath.Abs(os.Args[2])
	if err != nil {
		log.Fatal(err)
	}
	to = filepath.Dir(to)
	if _, err := os.Stat(to); err != nil {
		log.Printf("Destination directory does not exist: %s", to)
		log.Fatal(err)
	}

	args := make([]string, 0, 10)

	config := filepath.Join(filepath.Dir(exePath), target+".yaml")
	if _, err := os.Stat(config); err == nil {
		args = append(args, "-config", config)
	}

	// TODO: for now it will do, however we may need a separate configuration
	// for connector itself
	if flag := os.Getenv("FBC_DEBUG"); strings.EqualFold(flag, "yes") {
		args = append(args, "-debug")
	}

	args = append(args, "convert")
	args = append(args, "--ow")

	switch target {
	case "fb2mobi":
		args = append(args, "--to", "kfx")
	case "fb2epub":
		args = append(args, "--to", "epub3")
	}

	args = append(args, from)
	args = append(args, to)

	cmd := exec.Command(converterPath, args...)

	log.Printf("Starting %s with %q\n", converterPath, args)

	out, err := cmd.StdoutPipe()
	if err != nil {
		log.Fatalf("Unable to redirect conversion engine output: %v", err)
	}

	if err := cmd.Start(); err != nil {
		log.Fatalf("Unable to start conversion engine: %v", err)
	}

	// read and print converter stdout
	scanner := bufio.NewScanner(out)
	for scanner.Scan() {
		fmt.Println(scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		log.Fatalf("Conversion engine stdout pipe broken: %v", err)
	}

	if err := cmd.Wait(); err != nil {
		if ee, ok := err.(*exec.ExitError); ok && len(ee.Stderr) > 0 {
			log.Println(string(ee.Stderr))
		}
		log.Fatalf("Conversion engine returned error: %v", err)
	}
}
