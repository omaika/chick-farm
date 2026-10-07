package yamlfill

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// Set replaces one value and keeps every other byte: comments after it, flow mappings, quoted
// values, an empty value; a value YAML would read otherwise is quoted.
func TestSet(t *testing.T) {
	src := `# my team
roles:
  lead:
    spawn:                 # how
      harness: inherit       # inherit: the main session's
      model: "gpt-5"
      thinking:
  w: {spawn: {harness: pi, model: 'a b'}}
`
	cases := []struct {
		path  []string
		value string
		want  string // the line that changes, as it reads after
	}{
		{[]string{"roles", "lead", "spawn", "harness"}, "claude", "      harness: claude       # inherit: the main session's"},
		{[]string{"roles", "lead", "spawn", "model"}, "claude-sonnet-5-5", "      model: claude-sonnet-5-5"},
		{[]string{"roles", "lead", "spawn", "thinking"}, "high", "      thinking: high"},
		{[]string{"roles", "w", "spawn", "harness"}, "codex", "  w: {spawn: {harness: codex, model: 'a b'}}"},
		{[]string{"roles", "w", "spawn", "model"}, "opus[1m]", `  w: {spawn: {harness: pi, model: "opus[1m]"}}`},
		{[]string{"roles", "lead", "spawn", "model"}, "true", `      model: "true"`},
	}
	for _, c := range cases {
		out, err := Set([]byte(src), c.path, c.value)
		if err != nil {
			t.Fatalf("%v: %v", c.path, err)
		}
		var changed []string
		a, b := strings.Split(src, "\n"), strings.Split(string(out), "\n")
		if len(a) != len(b) {
			t.Fatalf("%v: lines %d -> %d:\n%s", c.path, len(a), len(b), out)
		}
		for i := range a {
			if a[i] != b[i] {
				changed = append(changed, b[i])
			}
		}
		if len(changed) != 1 || changed[0] != c.want {
			t.Fatalf("%v: changed %q, want %q", c.path, changed, c.want)
		}
		var doc map[string]any
		if err := yaml.Unmarshal(out, &doc); err != nil {
			t.Fatal(err)
		}
		got := doc
		for _, k := range c.path[:len(c.path)-1] {
			got = got[k].(map[string]any)
		}
		if got[c.path[len(c.path)-1]] != c.value {
			t.Fatalf("%v reads back %#v", c.path, got[c.path[len(c.path)-1]])
		}
	}
	for _, bad := range [][]string{{"roles", "nobody", "spawn", "model"}, {"roles", "lead", "spawn"}, {"roles", "lead", "spawn", "model", "x"}} {
		if _, err := Set([]byte(src), bad, "x"); err == nil {
			t.Fatalf("%v: no error", bad)
		}
	}
	if _, err := Set([]byte(src), []string{"roles", "lead", "spawn", "model"}, "a\nb"); err == nil {
		t.Fatal("two lines: no error")
	}
}

// SetPlain writes a number or a keyword as it is, not quoted, and refuses anything else.
func TestSetPlain(t *testing.T) {
	src := "limits:                  # none: no limit\n  depth: 2   # how deep\n  concurrency: none\n"
	out, err := SetPlain([]byte(src), []string{"limits", "concurrency"}, "20")
	if err != nil {
		t.Fatal(err)
	}
	if out, err = SetPlain(out, []string{"limits", "depth"}, "none"); err != nil {
		t.Fatal(err)
	}
	if want := "limits:                  # none: no limit\n  depth: none   # how deep\n  concurrency: 20\n"; string(out) != want {
		t.Fatalf("got\n%s", out)
	}
	for _, bad := range []string{"", "1 2", "a: b", `"x"`, "[1]"} {
		if _, err := SetPlain([]byte(src), []string{"limits", "depth"}, bad); err == nil {
			t.Fatalf("%q: no error", bad)
		}
	}
}
