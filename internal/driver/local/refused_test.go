package local

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/sting8k/piggery/internal/core"
)

// A refused command is found first on the worker's PATH and fails saying why; a name taken off
// the list runs again, and no list removes the directory.
func TestRefusedCommands(t *testing.T) {
	dir := t.TempDir()
	if err := WriteRefused(dir, []string{"paseo", "gh"}); err != nil {
		t.Fatal(err)
	}
	env := refusedPath([]string{"HOME=/h", "PATH=/usr/bin" + string(os.PathListSeparator) + "/bin"}, dir, []string{"paseo", "gh"})
	if want := "PATH=" + RefusedDir(dir) + string(os.PathListSeparator) + "/usr/bin"; !strings.HasPrefix(env[1], want) {
		t.Fatalf("env = %q; want PATH to start at the refused dir", env)
	}
	if runtime.GOOS != "windows" {
		out, err := exec.Command(filepath.Join(RefusedDir(dir), "paseo"), "run").CombinedOutput()
		if err == nil || !strings.Contains(string(out), "paseo: refused for a piggery worker") {
			t.Fatalf("refused paseo: %q, %v; want it to fail saying why", out, err)
		}
	}
	if err := WriteRefused(dir, []string{"paseo"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(RefusedDir(dir), "gh")); !os.IsNotExist(err) {
		t.Fatalf("gh after it left the list: %v", err)
	}
	if err := WriteRefused(dir, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(RefusedDir(dir)); !os.IsNotExist(err) {
		t.Fatalf("refused dir with no list: %v", err)
	}
	if got := refusedPath([]string{"PATH=/bin"}, dir, nil); got[0] != "PATH=/bin" {
		t.Fatalf("PATH with no list = %q", got)
	}
}

// A Claude worker's settings deny each refused command, bare, with arguments and through a launcher.
func TestClaudeSettingsDenyRefused(t *testing.T) {
	c := &claudeCodec{dir: t.TempDir(), refused: []string{"paseo"}}
	p, err := c.writeSettings(core.Spec{ParticipantID: "p", RunID: "r"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	var set struct {
		Permissions struct{ Deny []string } `json:"permissions"`
	}
	b, _ := os.ReadFile(p)
	if err := json.Unmarshal(b, &set); err != nil {
		t.Fatal(err)
	}
	for _, rule := range []string{"Bash(paseo)", "Bash(paseo *)", "Bash(npx paseo *)", "PowerShell(paseo *)"} {
		if !slices.Contains(set.Permissions.Deny, rule) {
			t.Fatalf("deny = %q; want %s", set.Permissions.Deny, rule)
		}
	}
	c.refused = nil
	p, _ = c.writeSettings(core.Spec{ParticipantID: "p", RunID: "r2"}, nil)
	if b, _ := os.ReadFile(p); strings.Contains(string(b), "permissions") {
		t.Fatalf("settings with no refused commands = %s", b)
	}
}
