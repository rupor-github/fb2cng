//go:build !windows

package convert

import "os"

func replaceOutputFile(tmpOutputName, outputName string) error {
	return os.Rename(tmpOutputName, outputName)
}
