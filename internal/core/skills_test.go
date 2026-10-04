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

// A role's skills are told in its card (only those, none, or nothing said for inherit) and shown
// in the team's state; a bad list is refused at team up.
func TestRoleSkills(t *testing.T) {
	db, err := store.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	e := core.New(db)
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
	if c := card("lead"); !strings.Contains(c, "Skills: your role uses only planning-lanes, code-review.") {
		t.Fatalf("lead card = %q", c)
	}
	if c := card("quiet"); !strings.Contains(c, "Skills: your role uses none.") {
		t.Fatalf("quiet card = %q", c)
	}
	if c := card("any"); strings.Contains(c, "Skills:") {
		t.Fatalf("inherit card = %q; want nothing said", c)
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
