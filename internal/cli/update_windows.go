//go:build windows

package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// replaceExe moves the new binary into exe's place. Windows cannot replace a running .exe, but it
// can rename it: the old one moves aside (exe.old, or a unique name while an older one still runs
// from there), then the new one moves in; a failure puts the old one back. What is left aside is
// removed by a later update, once nothing runs it.
func replaceExe(tmp, exe string) error {
	olds, _ := filepath.Glob(exe + ".old*")
	for _, o := range olds {
		os.Remove(o)
	}
	old := exe + ".old"
	if _, err := os.Lstat(old); err == nil {
		old = fmt.Sprintf("%s.old-%d", exe, time.Now().UnixNano())
	}
	if err := os.Rename(exe, old); err != nil {
		return err
	}
	if err := os.Rename(tmp, exe); err != nil {
		os.Rename(old, exe)
		return err
	}
	os.Remove(old) // fails while this process runs it
	return nil
}
