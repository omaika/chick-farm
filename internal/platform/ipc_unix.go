//go:build unix

package platform

import (
	"errors"
	"net"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

// SocketPath is the daemon's socket in dir.
func SocketPath(dir string) string { return filepath.Join(dir, "piggery.sock") }

// Listen replaces a stale socket at path with a new one only this user may use.
func Listen(path string) (net.Listener, error) {
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	ln, err := net.Listen("unix", path)
	if err != nil {
		return nil, err
	}
	if err := os.Chmod(path, 0o600); err != nil {
		ln.Close()
		return nil, err
	}
	return ln, nil
}

// Dial connects to the socket at path; timeout 0 is none.
func Dial(path string, timeout time.Duration) (net.Conn, error) {
	return net.DialTimeout("unix", path, timeout)
}

// SocketExists reports whether there is a socket at path (a daemon may still be gone).
func SocketExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// NotRunning reports whether a Dial error means no daemon listens there.
func NotRunning(err error) bool {
	return errors.Is(err, syscall.ENOENT) || errors.Is(err, syscall.ECONNREFUSED)
}
