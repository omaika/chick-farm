package manifests

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The built-in skills are unpacked into <home>/skills with their references and NOTICE.md; an
// edited file is kept on the next unpack; a template's own skills/<name>/ is found before the
// library's, and its description gives when to use it.
func TestSkillsUnpackAndFind(t *testing.T) {
	home := t.TempDir()
	if err := Unpack(home); err != nil {
		t.Fatal(err)
	}
	for _, rel := range []string{"NOTICE.md", "test-first/SKILL.md", "test-first/references/test-antipatterns.md"} {
		if _, err := os.Stat(filepath.Join(SkillsDir(home), rel)); err != nil {
			t.Fatalf("%s not unpacked: %v", rel, err)
		}
	}
	edited := filepath.Join(SkillsDir(home), "test-first", SkillFile)
	if err := os.WriteFile(edited, []byte("---\nname: test-first\ndescription: mine. Use when I say so.\n---\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Unpack(home); err != nil {
		t.Fatal(err)
	}
	file, desc, ok := FindSkill(home, "supervisor-executor", "test-first")
	if !ok || file != edited || WhenToUse(desc) != "Use when I say so." {
		t.Fatalf("edited skill: %s %q %v", file, desc, ok)
	}
	own := filepath.Join(Dir(home), "supervisor-executor", "skills", "test-first")
	if err := os.MkdirAll(own, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(own, SkillFile), []byte("---\ndescription: \"the template's. Use for this team.\"\n---\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if file, desc, ok := FindSkill(home, "supervisor-executor", "test-first"); !ok || !strings.HasPrefix(file, own) || WhenToUse(desc) != "Use for this team." {
		t.Fatalf("template's own skill: %s %q %v", file, desc, ok)
	}
	if _, _, ok := FindSkill(home, "slp", "../test-first"); ok {
		t.Fatal("a name with a path found a skill")
	}
	// Every skill a built-in template lists is a built-in skill.
	for _, name := range Builtins() {
		raw, err := builtin.ReadFile(name + ".yaml")
		if err != nil {
			t.Fatal(err)
		}
		for _, line := range strings.Split(string(raw), "\n") {
			v, ok := strings.CutPrefix(strings.TrimSpace(line), "skills: [")
			if !ok {
				continue
			}
			list, _, _ := strings.Cut(v, "]")
			for _, s := range strings.Split(list, ",") {
				if _, _, ok := FindSkill(home, "", strings.TrimSpace(s)); !ok {
					t.Fatalf("%s lists skill %q, which is not built in", name, strings.TrimSpace(s))
				}
			}
		}
	}
}
