//go:build windows

package platform

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Microsoft/go-winio"
	"golang.org/x/sys/windows"
)

// SyncDir does nothing on Windows: a directory cannot be opened for a flush there, and NTFS
// journals a rename itself.
func SyncDir(dir string) error { return nil }

// Link makes link point at target: a symlink where this user may create one (Developer Mode or
// an elevated process); otherwise a directory becomes a junction and a file a hard link, neither
// of which needs a privilege. A hard link needs target on the same volume.
func Link(target, link string) error {
	err := os.Symlink(target, link)
	if err == nil || !errors.Is(err, windows.ERROR_PRIVILEGE_NOT_HELD) {
		return err
	}
	fi, serr := os.Stat(target)
	if serr != nil {
		return err
	}
	if fi.IsDir() {
		return junction(target, link)
	}
	if lerr := os.Link(target, link); lerr != nil {
		return fmt.Errorf("link %s: no symlink (turn on Developer Mode to allow them) and no hard link: %w", link, lerr)
	}
	return nil
}

// junction makes link an NTFS junction (a mount point) to the directory target.
func junction(target, link string) error {
	abs, err := filepath.Abs(target)
	if err != nil {
		return err
	}
	if err := os.Mkdir(link, 0o700); err != nil {
		return err
	}
	if err := setMountPoint(abs, link); err != nil {
		os.Remove(link)
		return fmt.Errorf("junction %s -> %s: %w", link, abs, err)
	}
	return nil
}

func setMountPoint(target, dir string) error {
	p, err := windows.UTF16PtrFromString(dir)
	if err != nil {
		return err
	}
	h, err := windows.CreateFile(p, windows.GENERIC_WRITE, 0, nil, windows.OPEN_EXISTING,
		windows.FILE_FLAG_OPEN_REPARSE_POINT|windows.FILE_FLAG_BACKUP_SEMANTICS, 0)
	if err != nil {
		return err
	}
	defer windows.CloseHandle(h)
	b := winio.EncodeReparsePoint(&winio.ReparsePoint{Target: target, IsMountPoint: true})
	var n uint32
	return windows.DeviceIoControl(h, windows.FSCTL_SET_REPARSE_POINT, &b[0], uint32(len(b)), nil, 0, &n, nil)
}
