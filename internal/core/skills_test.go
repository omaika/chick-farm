package core_test

import (
	"strings"
	"testing"

	"github.com/sting8k/piggery/internal/core"
	"github.com/sting8k/piggery/internal/store"
)

const skilled = `
template: sk
roles:
  lead:   {tools: [send, inbox, who], skills: [planning-lanes, code-review]}
  quiet:  {tools: [send, inbox, who], skills: []}
  any:    {tools: [send, inbox, who], skills: inherit}
`

// A role's skills are named in its card with when to use each and the file to read (a missing
// one as not installed; nothing said for none or inherit) and shown in the team's state; a bad list
// is refused at team up.
func TestRoleSkills(t *testing.T) {
	db, err := store.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	e := core.New(db, core.WithSkillFiles(func(template, name string) (string, string, bool) {
		if template != "sk" || name != "planning-lanes" {
			return "", "", false
		}
		return "/home/skills/planning-lanes/SKILL.md", "Use when a lane is high-risk.", true
	}))
	team, err := e.TeamUp(ctx, core.TeamUpArgs{Manifest: skilled, Cwd: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	card := func(role string) string {
		t.Helper()
		j, err := e.Join(ctx, core.JoinArgs{Team: team.ID, Role: role, Name: role, Cwd: team.RootCwd})
		if err != nil {
			t.Fatal(err)
		}
		c, _ := e.Authenticate(ctx, j.ID, j.Token)
		id, err := e.Identify(ctx, c, core.IdentifyArgs{RunID: c.RunID})
		if err != nil {
			t.Fatal(err)
		}
		return id.RoleCard
	}
	if c := card("lead"); !strings.Contains(c, "- planning-lanes (/home/skills/planning-lanes/SKILL.md): Use when a lane is high-risk.\n") ||
		!strings.Contains(c, "- code-review: not installed") {
		t.Fatalf("lead card = %q", c)
	}
	for _, role := range []string{"quiet", "any"} {
		if c := card(role); strings.Contains(c, "Skills") {
			t.Fatalf("%s card = %q; want nothing said", role, c)
		}
	}
	s, err := e.State(ctx, core.StateArgs{})
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]*[]string{}
	for _, m := range s.Teams[0].Members {
		got[m.Name] = m.Skills
	}
	if l := got["lead"]; l == nil || strings.Join(*l, ",") != "planning-lanes,code-review" || got["quiet"] == nil || len(*got["quiet"]) != 0 || got["any"] != nil {
		t.Fatalf("member skills = %v", got)
	}
	for _, bad := range []string{"[Code Review]", "[a, a]", "lots"} {
		man := "template: x\nroles:\n  r: {skills: " + bad + "}\n"
		if _, err := e.TeamUp(ctx, core.TeamUpArgs{Manifest: man, Cwd: t.TempDir()}); code(err) != core.CodeInvalid {
			t.Fatalf("skills %s: %v", bad, err)
		}
	}
}
