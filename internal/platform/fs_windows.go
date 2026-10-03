//go:build windows

package platform

// SyncDir does nothing on Windows: a directory cannot be opened for a flush there, and NTFS
// journals a rename itself.
func SyncDir(dir string) error { return nil }
