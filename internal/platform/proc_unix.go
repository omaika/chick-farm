//go:build unix

package platform

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// lstartLayout is `ps -o lstart` (local time).
const lstartLayout = "Mon Jan _2 15:04:05 2006"

// Processes is every process (`ps -A`, on macOS and Linux); nil when it cannot be read. Start is
// `ps lstart` verbatim.
func Processes() []Proc {
	out, err := exec.Command("ps", "-A", "-o", "pid=,ppid=,pgid=,lstart=").Output()
	if err != nil {
		return nil
	}
	var rows []Proc
	for _, line := range strings.Split(string(out), "\n") {
		f := strings.Fields(line)
		if len(f) < 8 {
			continue
		}
		pid, e1 := strconv.Atoi(f[0])
		ppid, e2 := strconv.Atoi(f[1])
		pgid, e3 := strconv.Atoi(f[2])
		if e1 != nil || e2 != nil || e3 != nil {
			continue
		}
		rows = append(rows, Proc{pid, ppid, pgid, strings.Join(f[3:], " ")})
	}
	return rows
}

// StartTime is the OS start time of pid in unix ms, or 0 when unknown. `ps -o lstart=` exists on
// macOS and Linux (procps) and prints local time with one-second resolution.
func StartTime(pid int) int64 {
	out, err := exec.Command("ps", "-o", "lstart=", "-p", strconv.Itoa(pid)).Output()
	if err != nil {
		return 0
	}
	t, err := time.ParseInLocation(lstartLayout, strings.Join(strings.Fields(string(out)), " "), time.Local)
	if err != nil {
		return 0
	}
	return t.UnixMilli()
}

// ParentPID is pid's parent, or 0 when unknown (the process is gone).
func ParentPID(pid int) int {
	out, err := exec.Command("ps", "-o", "ppid=", "-p", strconv.Itoa(pid)).Output()
	if err != nil {
		return 0
	}
	n, _ := strconv.Atoi(strings.TrimSpace(string(out)))
	return n
}

// Parent is pid's parent and pid's program name; ok=false when pid is gone.
func Parent(pid int) (ppid int, name string, ok bool) {
	out, err := exec.Command("ps", "-o", "ppid=,comm=", "-p", strconv.Itoa(pid)).Output()
	f := strings.Fields(string(out))
	if err != nil || len(f) < 2 {
		return 0, "", false
	}
	ppid, _ = strconv.Atoi(f[0])
	return ppid, strings.Join(f[1:], " "), true
}

// Info reads the OS start time (unix ms, as StartTime) and command line of pid. alive=false when
// there is no such process; err when the process table could not be read.
func Info(ctx context.Context, pid int) (start int64, command string, alive bool, err error) {
	out, err := exec.CommandContext(ctx, "ps", "-ww", "-o", "lstart=,command=", "-p", strconv.Itoa(pid)).Output()
	var ee *exec.ExitError
	if errors.As(err, &ee) && ee.ExitCode() == 1 && len(strings.TrimSpace(string(out))) == 0 {
		return 0, "", false, nil // ps: no such process
	}
	if err != nil {
		return 0, "", false, fmt.Errorf("read process table: %w", err)
	}
	f := strings.Fields(string(out))
	if len(f) < 6 {
		return 0, "", false, fmt.Errorf("read process table: unexpected ps output %q", out)
	}
	t, err := time.ParseInLocation(lstartLayout, strings.Join(f[:5], " "), time.Local)
	if err != nil {
		return 0, "", false, fmt.Errorf("read process table: %w", err)
	}
	return t.UnixMilli(), strings.Join(f[5:], " "), true, nil
}

// Kill is kill(2): a negative pid is a process group.
func Kill(pid int, sig syscall.Signal) error { return syscall.Kill(pid, sig) }

// OwnGroup is this process's group.
func OwnGroup() int { return syscall.Getpgrp() }

// NewGroup starts cmd in a process group of its own (its pid), which KillGroup ends.
func NewGroup(cmd *exec.Cmd) { cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true} }

// KillGroup SIGKILLs the group NewGroup gave the process pid.
func KillGroup(pid int) error { return syscall.Kill(-pid, syscall.SIGKILL) }

// Detach starts cmd in a session of its own, so it outlives this process and its terminal.
func Detach(cmd *exec.Cmd) { cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true} }

// Alive reports whether pid is a process (a zombie too).
func Alive(pid int) bool { return !errors.Is(syscall.Kill(pid, 0), syscall.ESRCH) }
