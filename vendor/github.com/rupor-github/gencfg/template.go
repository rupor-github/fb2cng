package gencfg

import (
	"bytes"
	"net"
	"os"
	"runtime"
	"testing"
	"text/template"

	sprig "github.com/go-task/slim-sprig/v3"
)

// Values is a struct that holds variables we make available for template expantion
type Values struct {
	Name          string
	ProjectDir    string
	Arguments     map[string]string
	Hostname      string
	IPv4          string
	Containerized bool
	Testing       bool
	CPUs          int
	ARCH          string
	OS            string
}

// expandField expands a field using the given name and field string, for example
// configuration template may have something like this defined:
//
// server:
//
//	admin_service:
//	  http:
//	    sources: "{{ .Name }}-http"
//
// In this case name will be "sources" and result will be "sources-http"
func expandField(name, field string, opts *ProcessingOptions) (string, error) {

	// Make avalable functions from slim-sprig package: https://go-task.github.io/slim-sprig/
	funcMap := sprig.FuncMap()
	// Add our functions
	funcMap["joinPath"] = joinPath
	funcMap["freeLocalPort"] = freeLocalPort

	tmpl, err := template.New(name).Funcs(funcMap).Parse(field)
	if err != nil {
		return "", err
	}

	values := Values{
		Name:       name,
		ProjectDir: opts.rootDir,
		Arguments:  opts.args,
		Testing:    testing.Testing(),
		CPUs:       runtime.NumCPU(),
		ARCH:       runtime.GOARCH,
		OS:         runtime.GOOS,
	}
	if values.Hostname, err = os.Hostname(); err != nil {
		return "", err
	}
	if values.IPv4, err = getIPv4(values.Hostname); err != nil {
		return "", err
	}
	if _, err = os.Stat("/.dockerenv"); err == nil {
		values.Containerized = true
	} else if _, err = os.Stat("/.containerenv"); err == nil {
		values.Containerized = true
	}

	buf := new(bytes.Buffer)
	if err := tmpl.Execute(buf, values); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func getIPv4(host string) (string, error) {
	addrs, err := net.LookupIP(host)
	if err != nil {
		return "", err
	}
	for _, addr := range addrs {
		if ipv4 := addr.To4(); ipv4 != nil {
			return ipv4.String(), nil
		}
	}
	return "", nil
}
