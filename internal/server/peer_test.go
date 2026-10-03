package server

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"testing"

	"github.com/sting8k/piggery/internal/driver/local"
	"github.com/sting8k/piggery/internal/platform"
)

// Auth by host trusts the peer's process tree, not the host string: a caller that is not the host
// process or its descendant is refused even with the right pid and start time.
func TestHostNeedsPeerInItsTree(t *testing.T) {
	self := os.Getpid()
	host := func(pid int) string { return fmt.Sprintf("claude:%d:%d", pid, local.ProcessStartTime(pid)) }

	other := exec.Command("sleep", "30") // a live process the test is not a descendant of
	if runtime.GOOS == "windows" {
		other = exec.Command("ping", "-n", "30", "127.0.0.1")
	}
	if err := other.Start(); err != nil {
		t.Fatal(err)
	}
	defer other.Process.Kill()
	if hostMatches(self, host(other.Process.Pid), hostDepth) {
		t.Fatal("a process outside the host's tree was accepted")
	}
	if !hostMatches(self, host(os.Getppid()), hostDepth) {
		t.Fatal("a child of the host was refused")
	}
	if hostMatches(self, fmt.Sprintf("claude:%d:%d", os.Getppid(), local.ProcessStartTime(os.Getppid())+1000), hostDepth) {
		t.Fatal("a host with another start time (a reused pid) was accepted")
	}

	// The peer pid read from the socket is the connecting process.
	sock := platform.SocketPath(t.TempDir())
	ln, err := platform.Listen(sock)
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go func() {
		if c, err := platform.Dial(sock, 0); err == nil {
			defer c.Close()
			c.Read(make([]byte, 1))
		}
	}()
	c, err := ln.Accept()
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	if pid, err := platform.PeerPID(c); err != nil || pid != self {
		t.Fatalf("peer pid = %d, %v; want %d", pid, err, self)
	}
}
