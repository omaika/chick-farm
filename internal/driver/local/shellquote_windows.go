//go:build windows

package local

import (
	"path/filepath"
	"strings"
)

// ShellQuote writes the path s for a hook command, which a harness may run in Git Bash, cmd or
// PowerShell: forward slashes (bash eats unquoted backslashes; all three take /), and double
// quotes only when needed (PowerShell takes a quoted string as a value, not a command).
func ShellQuote(s string) string {
	s = filepath.ToSlash(s)
	if strings.ContainsAny(s, " \t'\"&|<>()^%!;$`") {
		return `"` + s + `"`
	}
	return s
}
