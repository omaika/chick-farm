//go:build unix

package local

import "path/filepath"

// programTokens is how many leading tokens of a command line may name its program.
const programTokens = 2

// programName is the program a command token names.
func programName(tok string) string { return filepath.Base(tok) }
