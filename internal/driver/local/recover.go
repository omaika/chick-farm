package local

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/sting8k/piggery/internal/core"
	"github.com/sting8k/piggery/internal/platform"
)

// Recovery. A recorded worker is identified by pid + OS start time
// (platform.Info, the same source and resolution as Start) + program name. The full argv is not
// comparable: pi rewrites process.title, so the OS shows `node …/bin/pi <args>` right after exec
// and only `pi` a moment later. The program name is the basename of the first command token, or
// of the second when an interpreter runs a script (node <script>); on Windows also of the third
// (`cmd.exe /c <npm shim>`).

// Inspect reports whether p's process is dead, still ours, or a reused pid.
func (d *Driver) Inspect(ctx context.Context, p core.Proc) (core.ProcState, error) {
	if p.PID <= 0 {
		return core.ProcDead, nil
	}
	if p.StartTime == 0 || len(p.Cmdline) == 0 {
		return "", errors.New("recorded process has no start time or cmdline: cannot verify it")
	}
	start, command, alive, err := platform.Info(ctx, p.PID)
	if err != nil {
		return "", err
	}
	if !alive {
		return core.ProcDead, nil
	}
	if start != p.StartTime || !sameProgram(command, p.Cmdline[0]) {
		return core.ProcReused, nil
	}
	return core.ProcOurs, nil
}

func sameProgram(command, recorded string) bool {
	name := programName(recorded)
	tok := strings.Fields(command)
	for i := 0; i < len(tok) && i < programTokens; i++ {
		if programName(tok[i]) == name {
			return true
		}
	}
	return false
}

// KillVerified is terminate for a worker that is not our child (after a daemon restart): SIGTERM
// to its group, then SIGKILL to the group and its tree, only while Inspect says it is still ours,
// re-checking before each signal. A dead or reused pid is never signalled.
func (d *Driver) KillVerified(ctx context.Context, p core.Proc) (core.Exit, error) {
	pgid := p.PGID
	if pgid <= 0 {
		pgid = p.PID
	}
	sent := ""
	var tree []proc
	for _, step := range []struct {
		sig  syscall.Signal
		wait time.Duration
	}{{syscall.SIGTERM, d.opts.TermWait}, {syscall.SIGKILL, 5 * time.Second}} {
		if step.sig == syscall.SIGTERM && !platform.Signals {
			continue // no SIGTERM on Windows
		}
		st, err := d.Inspect(ctx, p)
		if err != nil {
			return core.Exit{}, err
		}
		if st != core.ProcOurs {
			return core.Exit{Code: -1, Signal: sent, At: time.Now().UnixMilli()}, nil
		}
		tree = treeBelow(p.PID, true, tree)
		if step.sig == syscall.SIGTERM {
			d.kill(-pgid, step.sig)
		} else {
			d.signalTree(pgid, true, tree, step.sig)
		}
		sent = signalName(step.sig)
		// Not our child after a daemon restart: no wait status, so poll until it is gone.
		for deadline := time.Now().Add(step.wait); time.Now().Before(deadline); time.Sleep(50 * time.Millisecond) {
			if st, err := d.Inspect(ctx, p); err == nil && st != core.ProcOurs {
				// The leader is gone: kill what it left of the tree.
				d.signalTree(pgid, false, treeBelow(p.PID, false, tree), syscall.SIGKILL)
				return core.Exit{Code: -1, Signal: sent, At: time.Now().UnixMilli()}, nil
			}
		}
	}
	return core.Exit{}, fmt.Errorf("process %d still alive after SIGKILL", p.PID)
}

// StopAll stops every live worker in parallel with Stop's escalation. It returns once each
// exit has been reported through OnExit, so the caller may close the DB afterwards.
func (d *Driver) StopAll(ctx context.Context) {
	d.mu.Lock()
	var live []string
	for id, w := range d.procs {
		if !w.exited() {
			live = append(live, id)
		}
	}
	d.mu.Unlock()
	var wg sync.WaitGroup
	for _, id := range live {
		wg.Add(1)
		go func() {
			defer wg.Done()
			d.Stop(ctx, id)
		}()
	}
	wg.Wait()
}
