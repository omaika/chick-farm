package main

import (
	"runtime"
)

// shortTemp is where a test's piggery home goes: /tmp keeps unix socket paths short (macOS limits
// them); Windows has no /tmp, and its daemon pipe name does not depend on the path's length.
func shortTemp() string {
	if runtime.GOOS == "windows" {
		return ""
	}
	return "/tmp"
}
