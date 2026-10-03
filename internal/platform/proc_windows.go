//go:build windows

package platform

import (
	"context"
	"errors"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Windows has no process groups and no signals to send another process: a "signal" ends the
// process (TerminateProcess), and a group is its leader. The worker's tree is still walked from
// the table (parent pids), so its children go too. A parent pid outlives its parent and may be
// reused, so a process that started before its recorded parent is no child of it.

const stillActive = 259 // STILL_ACTIVE

// snapshot is every process as pid -> parent pid and program file name.
func snapshot() (map[int]windows.ProcessEntry32, error) {
	h, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return nil, err
	}
	defer windows.CloseHandle(h)
	out := map[int]windows.ProcessEntry32{}
	e := windows.ProcessEntry32{Size: uint32(unsafe.Sizeof(windows.ProcessEntry32{}))}
	for err = windows.Process32First(h, &e); err == nil; err = windows.Process32Next(h, &e) {
		out[int(e.ProcessID)] = e
	}
	if !errors.Is(err, windows.ERROR_NO_MORE_FILES) {
		return nil, err
	}
	return out, nil
}

// creation is pid's creation time as a FILETIME in 100ns units, 0 when unknown.
func creation(pid int) int64 {
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		return 0
	}
	defer windows.CloseHandle(h)
	return creationOf(h)
}

func creationOf(h windows.Handle) int64 {
	var c, x, k, u windows.Filetime
	if windows.GetProcessTimes(h, &c, &x, &k, &u) != nil {
		return 0
	}
	return int64(c.HighDateTime)<<32 | int64(c.LowDateTime)
}

func unixMilli(ft int64) int64 {
	if ft == 0 {
		return 0
	}
	f := windows.Filetime{HighDateTime: uint32(ft >> 32), LowDateTime: uint32(ft)}
	return f.Nanoseconds() / 1e6
}

// Processes is every process; nil when the table cannot be read. Start is the creation time.
func Processes() []Proc {
	snap, err := snapshot()
	if err != nil {
		return nil
	}
	starts := make(map[int]int64, len(snap))
	for pid := range snap {
		starts[pid] = creation(pid)
	}
	rows := make([]Proc, 0, len(snap))
	for pid, e := range snap {
		ppid := int(e.ParentProcessID)
		if ps, ok := starts[ppid]; !ok || ps == 0 || starts[pid] < ps {
			ppid = 0 // the recorded parent is gone, or is a later process with its pid
		}
		start := ""
		if starts[pid] != 0 {
			start = strconv.FormatInt(starts[pid], 10)
		}
		rows = append(rows, Proc{PID: pid, PPID: ppid, Start: start})
	}
	return rows
}

// StartTime is the OS start time of pid in unix ms, or 0 when unknown.
func StartTime(pid int) int64 { return unixMilli(creation(pid)) }

// ParentPID is pid's parent, or 0 when unknown (the process or its parent is gone).
func ParentPID(pid int) int {
	ppid, _, _ := Parent(pid)
	return ppid
}

// Parent is pid's parent and pid's program name (without .exe); ok=false when pid is gone.
func Parent(pid int) (ppid int, name string, ok bool) {
	snap, err := snapshot()
	if err != nil {
		return 0, "", false
	}
	e, ok := snap[pid]
	if !ok {
		return 0, "", false
	}
	name = windows.UTF16ToString(e.ExeFile[:])
	name = strings.TrimSuffix(name, filepath.Ext(name))
	ppid = int(e.ParentProcessID)
	if _, live := snap[ppid]; !live || creation(ppid) > creation(pid) {
		ppid = 0
	}
	return ppid, name, true
}

// Info reads the OS start time (unix ms, as StartTime) and command line of pid. alive=false when
// there is no such process.
func Info(_ context.Context, pid int) (start int64, command string, alive bool, err error) {
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if errors.Is(err, windows.ERROR_INVALID_PARAMETER) {
		return 0, "", false, nil // no such process
	}
	if err != nil {
		return 0, "", false, err
	}
	defer windows.CloseHandle(h)
	var code uint32
	if err := windows.GetExitCodeProcess(h, &code); err != nil {
		return 0, "", false, err
	}
	if code != stillActive {
		return 0, "", false, nil
	}
	command, err = commandLine(h)
	if err != nil {
		return 0, "", false, err
	}
	return unixMilli(creationOf(h)), command, true, nil
}

// commandLine is the process's command line (ProcessCommandLineInformation, Windows 8.1+).
func commandLine(h windows.Handle) (string, error) {
	n := uint32(4096)
	for range 4 {
		buf := make([]byte, n)
		var ret uint32
		err := windows.NtQueryInformationProcess(h, windows.ProcessCommandLineInformation, unsafe.Pointer(&buf[0]), n, &ret)
		if err == nil {
			return (*windows.NTUnicodeString)(unsafe.Pointer(&buf[0])).String(), nil
		}
		if !errors.Is(err, windows.STATUS_INFO_LENGTH_MISMATCH) || ret <= n {
			return "", err
		}
		n = ret
	}
	return "", errors.New("process command line keeps growing")
}

// Kill ends pid (-pid alike: a "group" is its leader). sig 0 only checks that pid can be ended.
func Kill(pid int, sig syscall.Signal) error {
	if pid < 0 {
		pid = -pid
	}
	h, err := windows.OpenProcess(windows.PROCESS_TERMINATE|windows.SYNCHRONIZE, false, uint32(pid))
	if err != nil {
		return err
	}
	defer windows.CloseHandle(h)
	if sig == 0 {
		return nil
	}
	return windows.TerminateProcess(h, 1)
}

// Signals is false: Windows has no SIGTERM, ending a process there is always by force.
const Signals = false

// OwnGroup is 0: no process groups.
func OwnGroup() int { return 0 }

const createNoWindow = 0x08000000 // CREATE_NO_WINDOW

// NewGroup starts cmd without a console window of its own (a detached daemon's console child
// would open one), as a new console process group.
func NewGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true,
		CreationFlags: windows.CREATE_NEW_PROCESS_GROUP | createNoWindow}
}

// KillGroup ends the process pid and every process below it.
func KillGroup(pid int) error {
	byParent := map[int][]int{}
	for _, p := range Processes() {
		byParent[p.PPID] = append(byParent[p.PPID], p.PID)
	}
	var walk func(int)
	walk = func(p int) {
		for _, k := range byParent[p] {
			walk(k)
			Kill(k, syscall.SIGKILL)
		}
	}
	walk(pid)
	return Kill(pid, syscall.SIGKILL)
}

// Detach starts cmd with no console window and outside this console's process group, so it
// outlives this process and its terminal.
func Detach(cmd *exec.Cmd) { NewGroup(cmd) }

// Alive reports whether pid is a running process.
func Alive(pid int) bool {
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		return !errors.Is(err, windows.ERROR_INVALID_PARAMETER)
	}
	defer windows.CloseHandle(h)
	var code uint32
	return windows.GetExitCodeProcess(h, &code) != nil || code == stillActive
}
