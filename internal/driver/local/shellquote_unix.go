//go:build unix

package local

import "strings"

// ShellQuote quotes s for a POSIX shell (hook commands).
func ShellQuote(s string) string { return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'" }
