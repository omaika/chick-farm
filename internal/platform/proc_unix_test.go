//go:build unix

package platform

import (
	"context"
	"os"
	"testing"
	"time"
)

// The start time is read whatever the user's locale: in pt_BR, ps writes lstart as
// "dom  4 out 06:40:36 2026", and a session's host was not found (its MCP server had no tools).
// A system without the locale writes C: the test then passes either way.
func TestStartTimeAnyLocale(t *testing.T) {
	t.Setenv("LANG", "pt_BR.UTF-8")
	t.Setenv("LC_TIME", "pt_BR.UTF-8")
	os.Unsetenv("LC_ALL")
	pid := os.Getpid()
	st := StartTime(pid)
	if st == 0 {
		t.Fatal("StartTime: 0")
	}
	if age := time.Since(time.UnixMilli(st)); age < -2*time.Second || age > time.Hour {
		t.Fatalf("StartTime: %v ago", age)
	}
	start, _, alive, err := Info(context.Background(), pid)
	if err != nil || !alive || start != st {
		t.Fatalf("Info: start %d (StartTime %d) alive %v err %v", start, st, alive, err)
	}
	if ppid, name, ok := Parent(pid); !ok || ppid != os.Getppid() || name == "" {
		t.Fatalf("Parent: %d %q %v", ppid, name, ok)
	}
}
