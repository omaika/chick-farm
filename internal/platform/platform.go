// Package platform is what piggery does differently on each OS: the daemon's socket (a unix
// socket on macOS and Linux, a named pipe on Windows), the singleton lock, the pid of a socket's
// peer, the process table, and how a detached or grouped child is started and killed.
package platform

// Proc is one row of the process table. Start is opaque: a pid is the same process only while
// its Start is too. PGID is 0 where the OS has no process groups (Windows).
type Proc struct {
	PID, PPID, PGID int
	Start           string
}
