package gencfg

import (
	"errors"
	"fmt"
	"math"
	"net"
	"sync"
)

// various tests may run in parallel - we should provide unique port number every time
var guard sync.Mutex

// keep track of ports we already allocated
var portMap = make(map[int]struct{})

// getFreePort returns a free port number on the local machine.
func getFreePort() (int, error) {
	addr, err := net.ResolveTCPAddr("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, fmt.Errorf("unable to resolve TCPAddr: %w", err)
	}
	l, err := net.ListenTCP("tcp", addr)
	if err != nil {
		return 0, fmt.Errorf("unable to ListenTCP: %w", err)
	}
	defer l.Close()

	return l.Addr().(*net.TCPAddr).Port, nil
}

// reservePort reserves a port number we are going to use.
func reservePort(port int) bool {
	guard.Lock()
	defer guard.Unlock()
	if _, ok := portMap[port]; ok {
		return false
	}
	portMap[port] = struct{}{}
	return true
}

// freeLocalPort returns a free port number on the local machine for template expansion.
func freeLocalPort() (int, error) {
	for range math.MaxUint16 {
		port, err := getFreePort()
		if err != nil {
			return 0, err
		}
		if !reservePort(port) {
			continue
		}
		return port, nil
	}
	return 0, errors.New("unable to find free port")
}
