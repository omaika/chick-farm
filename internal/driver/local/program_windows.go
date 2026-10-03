//go:build windows

package local

import (
	"path/filepath"
	"strings"
)

// programTokens is how many leading tokens of a command line may name its program: an npm shim
// (pi.cmd) runs as `cmd.exe /c <shim> …`.
const programTokens = 3

// programName is the program a command token names: no quotes, directory, extension or case.
func programName(tok string) string {
	b := filepath.Base(strings.Trim(tok, `"`))
	return strings.ToLower(strings.TrimSuffix(b, filepath.Ext(b)))
}
