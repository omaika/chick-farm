package local

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
)

// Refused commands (config.yaml spawn.refused_commands): commands a worker's shell must not
// start, such as another agent orchestrator (paseo) that would start agents outside piggery's
// limits, routing and view. A worker's PATH starts at RefusedDir, where each refused name is a
// command that says why and fails; Claude workers also get a permissions.deny rule for each, so
// its Bash tool refuses them even where the user's shell profile puts the real one first again.
// A guard against mistakes, not a wall: an absolute path or a copy under another name still runs.

// RefusedDir is the directory every worker's PATH starts at.
func RefusedDir(dir string) string { return filepath.Join(dir, "bin") }

// refusedLaunchers run a package's command by name; a refused name is refused through them too.
var refusedLaunchers = []string{"npx", "bunx"}

// refusedSay is what a refused command prints before it fails.
func refusedSay(name string) string {
	return name + ": refused for a piggery worker (spawn.refused_commands in ~/.piggery/config.yaml). " +
		"Ask whoever gave you the work for what you need."
}

// WriteRefused makes RefusedDir(dir) hold one failing command for each name, and nothing else: a
// name taken off the list runs again. With no names the directory is removed.
func WriteRefused(dir string, names []string) error {
	bin := RefusedDir(dir)
	if len(names) == 0 {
		if err := os.RemoveAll(bin); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
		return nil
	}
	want := map[string]string{}
	for _, name := range names {
		say := refusedSay(name)
		want[name] = "#!/bin/sh\necho '" + strings.ReplaceAll(say, "'", `'\''`) + "' >&2\nexit 1\n"
		if runtime.GOOS == "windows" { // cmd finds the batch file; a Git Bash finds the script
			want[name+".cmd"] = "@echo off\r\n>&2 echo " + batchEcho(say) + "\r\nexit /b 1\r\n"
		}
	}
	if err := os.MkdirAll(bin, 0o700); err != nil {
		return err
	}
	ents, err := os.ReadDir(bin)
	if err != nil {
		return err
	}
	for _, e := range ents {
		if _, ok := want[e.Name()]; !ok {
			if err := os.Remove(filepath.Join(bin, e.Name())); err != nil {
				return err
			}
		}
	}
	for name, text := range want {
		p := filepath.Join(bin, name)
		if cur, err := os.ReadFile(p); err == nil && string(cur) == text {
			continue
		}
		tmp := p + ".tmp"
		if err := os.WriteFile(tmp, []byte(text), 0o755); err != nil {
			return err
		}
		if err := os.Rename(tmp, p); err != nil {
			return err
		}
	}
	return nil
}

// batchEcho escapes text for a batch file's echo.
func batchEcho(text string) string {
	var b strings.Builder
	for _, r := range text {
		switch r {
		case '^', '&', '|', '<', '>', '(', ')':
			b.WriteRune('^')
		case '%':
			b.WriteRune('%')
		}
		b.WriteRune(r)
	}
	return b.String()
}

// refusedPath returns env with RefusedDir(dir) first on its PATH (the last PATH entry, the one a
// process reads; on Windows the key is matched in any case). Without names, env as it is.
func refusedPath(env []string, dir string, names []string) []string {
	if len(names) == 0 {
		return env
	}
	isPath := func(k string) bool {
		if runtime.GOOS == "windows" {
			return strings.EqualFold(k, "PATH")
		}
		return k == "PATH"
	}
	bin := RefusedDir(dir)
	for i := len(env) - 1; i >= 0; i-- {
		k, v, _ := strings.Cut(env[i], "=")
		if isPath(k) {
			out := slices.Clone(env)
			out[i] = k + "=" + bin + string(os.PathListSeparator) + v
			return out
		}
	}
	return append(env, "PATH="+bin)
}

// claudeRefusals are the permissions.deny rules of the refused names: each bare, with arguments,
// and through each launcher, in Bash and PowerShell.
func claudeRefusals(names []string) []string {
	var out []string
	for _, name := range names {
		words := []string{name}
		for _, l := range refusedLaunchers {
			words = append(words, l+" "+name)
		}
		for _, w := range words {
			for _, tool := range []string{"Bash", "PowerShell"} {
				out = append(out, tool+"("+w+")", tool+"("+w+" *)")
			}
		}
	}
	return out
}
