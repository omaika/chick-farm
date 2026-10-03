//go:build windows

package platform

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Microsoft/go-winio"
	"golang.org/x/sys/windows"
)

// SocketPath is the daemon's named pipe for dir: Node, which the extensions run in, has no unix
// sockets on Windows. The name is a hash of the lowercased dir, so each dir has its own daemon;
// the extensions compute it the same way.
func SocketPath(dir string) string {
	if abs, err := filepath.Abs(dir); err == nil {
		dir = abs
	}
	sum := sha256.Sum256([]byte(strings.ToLower(filepath.Clean(dir))))
	return `\\.\pipe\piggery-` + hex.EncodeToString(sum[:8])
}

// Listen creates the named pipe at path, open to this user only (and SYSTEM).
func Listen(path string) (net.Listener, error) {
	sid, err := userSID()
	if err != nil {
		return nil, err
	}
	return winio.ListenPipe(path, &winio.PipeConfig{
		SecurityDescriptor: fmt.Sprintf("D:P(A;;GA;;;%s)(A;;GA;;;SY)", sid),
		InputBufferSize:    64 << 10,
		OutputBufferSize:   64 << 10,
	})
}

func userSID() (string, error) {
	tok := windows.GetCurrentProcessToken()
	u, err := tok.GetTokenUser()
	if err != nil {
		return "", err
	}
	return u.User.Sid.String(), nil
}

// Dial connects to the named pipe at path (or a unix socket, for a path that is not a pipe);
// timeout 0 is none.
func Dial(path string, timeout time.Duration) (net.Conn, error) {
	if !isPipe(path) {
		return net.DialTimeout("unix", path, timeout)
	}
	ctx := context.Background()
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}
	return winio.DialPipeContext(ctx, path)
}

func isPipe(path string) bool {
	return strings.HasPrefix(path, `\\.\pipe\`) || strings.HasPrefix(path, `//./pipe/`)
}

// SocketExists reports whether the pipe (or socket file) at path exists.
func SocketExists(path string) bool {
	if !isPipe(path) {
		_, err := os.Stat(path)
		return err == nil
	}
	entries, err := os.ReadDir(`\\.\pipe\`)
	if err != nil {
		return false
	}
	name := path[len(`\\.\pipe\`):]
	for _, e := range entries {
		if strings.EqualFold(e.Name(), name) {
			return true
		}
	}
	return false
}

// NotRunning reports whether a Dial error means no daemon listens there.
func NotRunning(err error) bool {
	return errors.Is(err, os.ErrNotExist) || errors.Is(err, windows.WSAECONNREFUSED)
}
