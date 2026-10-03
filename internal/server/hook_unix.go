//go:build unix

package server

import (
	"context"
	"io/fs"
	"os/exec"
)

// isHook reports whether a file of notify.d is one to run: executable.
func isHook(fi fs.FileInfo) bool { return fi.Mode()&0o111 != 0 }

func hookCommand(ctx context.Context, path string) *exec.Cmd { return exec.CommandContext(ctx, path) }
