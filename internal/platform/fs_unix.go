//go:build unix

package platform

import "os"

// SyncDir flushes dir's entries (a rename into it) to disk.
func SyncDir(dir string) error {
	d, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer d.Close()
	return d.Sync()
}

// Link makes link a symlink to target.
func Link(target, link string) error { return os.Symlink(target, link) }
