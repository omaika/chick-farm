package core

import (
	"fmt"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// Skills by role (manifest roles.<r>.skills): piggery's skills a role's participant uses. Nothing
// loads them into a harness: the role card names each with when to use it and the file to read
// (skillsText), so every harness, and a session the Human opened, uses them the same way, and
// pays for a line per skill until one is read. The harness's own skills are left as they are.

// skillList is roles.<r>.skills: inherit (Set false: not written, or `inherit`) or [] (none of
// piggery's skills), or the names given.
type skillList struct {
	Set   bool
	Names []string
}

func (s *skillList) UnmarshalYAML(n *yaml.Node) error {
	if n.Kind == yaml.ScalarNode && (n.Value == inherit || n.Tag == "!!null") {
		*s = skillList{}
		return nil
	}
	var names []string
	if err := n.Decode(&names); err != nil {
		return fmt.Errorf("skills: want inherit or a list of skill names")
	}
	*s = skillList{Set: true, Names: names}
	if s.Names == nil {
		s.Names = []string{}
	}
	return nil
}

// skillName is a skill's name as the harnesses name their skill directories.
var skillName = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]*$`)

// validateSkills checks role name's skills list.
func validateSkills(name string, s skillList) error {
	seen := map[string]bool{}
	for _, n := range s.Names {
		if !skillName.MatchString(n) {
			return errf(CodeInvalid, "manifest: roles.%s.skills: %q is not a skill name (lowercase letters, digits, '.', '_', '-')", name, n)
		}
		if seen[n] {
			return errf(CodeInvalid, "manifest: roles.%s.skills: %q is listed twice", name, n)
		}
		seen[n] = true
	}
	return nil
}

// skillsText is the role card's part about skills: each listed one, when to use it and its file
// (read when that case comes up: it is not loaded for you); "" when none is listed. A skill with no
// file found is named as missing, so the gap shows instead of a silent omission.
func (t *txn) skillsText(template string, s skillList) string {
	if len(s.Names) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("\nSkills of your role: when a case below comes up, read that skill's file first and follow it (it is not loaded for you).\n")
	for _, n := range s.Names {
		var file, when string
		ok := false
		if t.skill != nil {
			file, when, ok = t.skill(template, n)
		}
		if !ok {
			fmt.Fprintf(&b, "- %s: not installed (no %s/SKILL.md found); tell whoever gave you the work if you need it\n", n, n)
			continue
		}
		fmt.Fprintf(&b, "- %s (%s): %s\n", n, file, when)
	}
	return b.String()
}

// skillsOf is s as MemberState.Skills: nil for inherit.
func skillsOf(s skillList) *[]string {
	if !s.Set {
		return nil
	}
	names := append([]string{}, s.Names...)
	return &names
}

// skillsNote is a role's skills after its description in the templates list; "" for inherit.
func skillsNote(s skillList) string {
	switch {
	case !s.Set:
		return ""
	case len(s.Names) == 0:
		return " (skills: none)"
	}
	return " (skills: " + strings.Join(s.Names, ", ") + ")"
}
