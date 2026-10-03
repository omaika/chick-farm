//go:build windows

package server

import (
	"context"
	"io/fs"
	"os/exec"
	"path/filepath"
	"strings"
)

// isHook reports whether a file of notify.d is one to run: Windows has no executable bit, so a
// program or a script Windows knows how to run.
func isHook(fi fs.FileInfo) bool {
	switch strings.ToLower(filepath.Ext(fi.Name())) {
	case ".exe", ".com", ".cmd", ".bat", ".ps1":
		return true
	}
	return false
}

// hookCommand runs a .ps1 hook through PowerShell; any other hook directly.
func hookCommand(ctx context.Context, path string) *exec.Cmd {
	if strings.EqualFold(filepath.Ext(path), ".ps1") {
		return exec.CommandContext(ctx, "powershell.exe", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-File", path)
	}
	return exec.CommandContext(ctx, path)
}
