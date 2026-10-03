//go:build unix

package cli

import "os"

// replaceExe moves the new binary over exe: a running piggery keeps its own copy.
func replaceExe(tmp, exe string) error { return os.Rename(tmp, exe) }
