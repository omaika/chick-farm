package manifests

import (
	"bufio"
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// Skills: the built-in skills (skills/<name>/SKILL.md and what it references) are unpacked into
// <home>/skills like the templates, edits kept. A role's card names the skills its template lists
// (roles.<r>.skills) with when to use each and the file to read: nothing loads them into a
// harness, so any harness, and a session the Human opened, uses them the same way.

// SkillsDir is the skills library under the piggery dir home.
func SkillsDir(home string) string { return filepath.Join(home, "skills") }

// SkillFile is the file of a skill's entry point inside its directory.
const SkillFile = "SKILL.md"

// builtinSkills maps each built-in skill to its files (relative to its directory); the unit ""
// holds what sits beside them (NOTICE.md: their sources and license).
func builtinSkills(src fs.FS) (map[string]map[string][]byte, error) {
	out := map[string]map[string][]byte{}
	err := fs.WalkDir(src, "skills", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel := strings.TrimPrefix(p, "skills/")
		unit, file, ok := strings.Cut(rel, "/")
		if !ok {
			unit, file = "", rel
		}
		b, err := fs.ReadFile(src, p)
		if err != nil {
			return err
		}
		if out[unit] == nil {
			out[unit] = map[string][]byte{}
		}
		out[unit][file] = b
		return nil
	})
	return out, err
}

func unpackSkills(home string, src fs.FS) error {
	files, err := builtinSkills(src)
	if err != nil {
		return err
	}
	return unpackSet(SkillsDir(home), files)
}

// FindSkill returns the SKILL.md of skill name for a team of template: the template's own
// skills/<name>/ first, then the library's. It returns its path and its frontmatter description;
// ok is false when neither has one.
func FindSkill(home, template, name string) (file, description string, ok bool) {
	if name == "" || strings.ContainsAny(name, `/\`) || strings.HasPrefix(name, ".") {
		return "", "", false
	}
	var dirs []string
	if validName(template) == nil {
		dirs = append(dirs, filepath.Join(Dir(home), template, "skills"))
	}
	for _, d := range append(dirs, SkillsDir(home)) {
		p := filepath.Join(d, name, SkillFile)
		b, err := os.ReadFile(p)
		if err == nil {
			return p, frontmatterDescription(b), true
		}
	}
	return "", "", false
}

// frontmatterDescription is the description: line of a SKILL.md's frontmatter, unquoted.
func frontmatterDescription(b []byte) string {
	sc := bufio.NewScanner(bytes.NewReader(b))
	for i := 0; sc.Scan(); i++ {
		line := sc.Text()
		if i == 0 && line != "---" || i > 0 && line == "---" {
			break
		}
		if v, ok := strings.CutPrefix(line, "description:"); ok {
			v = strings.TrimSpace(v)
			if len(v) >= 2 && (v[0] == '"' || v[0] == '\'') && v[len(v)-1] == v[0] {
				v = v[1 : len(v)-1]
			}
			return v
		}
	}
	return ""
}

// WhenToUse is the part of a skill's description that says when to read it: from its "Use when"
// (or "Use for") sentence to the end, else the whole description.
func WhenToUse(description string) string {
	for _, k := range []string{"Use when", "Use for", "Use it when"} {
		if i := strings.Index(description, k); i >= 0 {
			return description[i:]
		}
	}
	return description
}
