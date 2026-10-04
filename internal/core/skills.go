package core

import (
	"fmt"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// Skills by role (manifest roles.<r>.skills): which of its harness's skills a role's participant
// may use. For now the list is told, not enforced: the role card names it and asks the model to
// keep to it (skillsText). The harness still offers every skill of its setup. The design for
// enforcing it in the drivers is docs/design/role-skills.md.

// skillList is roles.<r>.skills: inherit (Set false: not written, or `inherit`), [] (no skill)
// or the names allowed.
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

// skillsText is the role card's line about skills; "" for inherit.
func skillsText(s skillList) string {
	if !s.Set {
		return ""
	}
	if len(s.Names) == 0 {
		return "\nSkills: your role uses none. Your harness may offer skills: do not use any of them; if one seems needed, ask whoever gave you the work.\n"
	}
	return fmt.Sprintf("\nSkills: your role uses only %s. Your harness may offer others: do not use them; if one seems needed, ask whoever gave you the work.\n",
		strings.Join(s.Names, ", "))
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
