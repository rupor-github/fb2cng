// Package filepub atomically publishes completed output files.
package filepub

// Publish publishes a completed sibling temporary file. When overwrite is false,
// any existing destination prevents publication. The caller must clean up source,
// including when publication fails or a hard-link fallback leaves it in place.
func Publish(source, destination string, overwrite bool) error {
	if overwrite {
		return replaceOutput(source, destination)
	}
	return publishOutput(source, destination)
}
