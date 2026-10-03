package platform

import (
	"errors"
	"net"

	"golang.org/x/sys/windows"
)

// PeerPID is the pid of the client at the other end of the daemon's named pipe
// (GetNamedPipeClientProcessId).
func PeerPID(nc net.Conn) (int, error) {
	f, ok := nc.(interface{ Fd() uintptr })
	if !ok {
		return 0, errors.New("not a named pipe")
	}
	var pid uint32
	if err := windows.GetNamedPipeClientProcessId(windows.Handle(f.Fd()), &pid); err != nil {
		return 0, err
	}
	return int(pid), nil
}
