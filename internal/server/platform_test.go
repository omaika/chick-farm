package server_test

import (
	"runtime"
	"testing"
)

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
