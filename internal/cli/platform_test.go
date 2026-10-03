package cli

import (
	"runtime"
	"testing"
)

// exeSuffix is what a program's file name needs for PATH to find it.
var exeSuffix = map[bool]string{true: ".exe"}[runtime.GOOS == "windows"]

// shortTemp is where a test's piggery home goes: /tmp keeps unix socket paths short (macOS limits
// them); Windows has no /tmp, and its daemon pipe name does not depend on the path's length.
func shortTemp() string {
	if runtime.GOOS == "windows" {
		return ""
	}
	return "/tmp"
}

// skipOnWindows skips a test that needs a POSIX shell or signals.
func skipOnWindows(t testing.TB, why string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("not on Windows: " + why)
	}
}

// unixModes: file permission bits mean something (not on Windows, where a file is 0666 or 0444).
var unixModes = runtime.GOOS != "windows"
