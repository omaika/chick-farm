//go:build unix

package platform

import (
	"errors"
	"os"
	"syscall"
)

// TryLock takes an exclusive lock on f without waiting; held reports that another process has it.
// The lock goes with f's close.
func TryLock(f *os.File) (held bool, err error) {
	err = syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	if errors.Is(err, syscall.EWOULDBLOCK) {
		return true, nil
	}
	return false, err
}
