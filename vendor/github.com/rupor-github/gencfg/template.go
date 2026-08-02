package gencfg

import (
	"bytes"
	"errors"
	"net"
	"os"
	"runtime"
	"strings"
	"testing"
	"text/template"

	sprig "github.com/go-task/slim-sprig/v3"
)

var (
	lookupIP          = net.LookupIP
	networkInterfaces = net.Interfaces
	interfaceAddrs    = func(iface net.Interface) ([]net.Addr, error) { return iface.Addrs() }
)

// Values is a struct that holds variables we make available for template expansion
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

// templateContext holds pre-computed values and function map that remain constant
// across all field expansions within a single Process() call.
type templateContext struct {
	funcMap template.FuncMap
	values  Values
}

// newTemplateContext creates a templateContext by computing all environment-dependent
// values once. This avoids redundant syscalls (hostname, DNS, stat) for every field.
func newTemplateContext(opts *ProcessingOptions) (*templateContext, error) {
	// Make available functions from slim-sprig package: https://go-task.github.io/slim-sprig/
	funcMap := sprig.FuncMap()
	// Add our functions
	funcMap["joinPath"] = joinPath
	funcMap["freeLocalPort"] = freeLocalPort

	values := Values{
		ProjectDir: opts.rootDir,
		Arguments:  opts.args,
		Testing:    testing.Testing(),
		CPUs:       runtime.NumCPU(),
		ARCH:       runtime.GOARCH,
		OS:         runtime.GOOS,
	}

	var err error
	if values.Hostname, err = os.Hostname(); err != nil {
		return nil, err
	}
	if values.IPv4, err = getIPv4(values.Hostname); err != nil {
		return nil, err
	}
	if _, err = os.Stat("/.dockerenv"); err == nil {
		values.Containerized = true
	} else if _, err = os.Stat("/.containerenv"); err == nil {
		values.Containerized = true
	}

	return &templateContext{funcMap: funcMap, values: values}, nil
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
func (tc *templateContext) expandField(name, field string) (string, error) {
	tmpl, err := template.New(name).Funcs(tc.funcMap).Parse(field)
	if err != nil {
		return "", err
	}

	// Name is per-field, so set it on a copy of the cached values
	values := tc.values
	values.Name = name

	buf := new(bytes.Buffer)
	if err := tmpl.Execute(buf, values); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// getIPv4 resolves the given hostname and returns the first IPv4 address found.
func getIPv4(host string) (string, error) {
	var errs []error

	ipv4, err := lookupIPv4(host)
	if err != nil {
		errs = append(errs, err)
	} else if ipv4 != "" {
		return ipv4, nil
	}

	if !strings.HasSuffix(host, ".local") {
		ipv4, err = lookupIPv4(host + ".local")
		if err != nil {
			errs = append(errs, err)
		} else if ipv4 != "" {
			return ipv4, nil
		}
	}

	ipv4, err = localInterfaceIPv4()
	if err != nil {
		errs = append(errs, err)
	} else if ipv4 != "" {
		return ipv4, nil
	}

	return "", errors.Join(errs...)
}

func lookupIPv4(host string) (string, error) {
	addrs, err := lookupIP(host)
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

func localInterfaceIPv4() (string, error) {
	interfaces, err := networkInterfaces()
	if err != nil {
		return "", err
	}

	var errs []error
	for _, iface := range interfaces {
		if iface.Flags&net.FlagLoopback != 0 || iface.Flags&net.FlagUp == 0 {
			continue
		}

		addrs, err := interfaceAddrs(iface)
		if err != nil {
			errs = append(errs, err)
			continue
		}

		for _, addr := range addrs {
			var ip net.IP
			switch addr := addr.(type) {
			case *net.IPAddr:
				ip = addr.IP
			case *net.IPNet:
				ip = addr.IP
			}

			if ipv4 := ip.To4(); ipv4 != nil && !ipv4.IsLoopback() {
				return ipv4.String(), nil
			}
		}
	}

	return "", errors.Join(errs...)
}
