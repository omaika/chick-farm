package core_test

import (
	"strings"
	"testing"

	"github.com/sting8k/piggery/internal/core"
)

const supExec = `
template: sx
auto_join_role: supervisor
roles:
  supervisor: {can_spawn: [executor], tools: [send, inbox, who, agent]}
  executor:   {tools: [send, inbox, who]}
routing:
  - {from: supervisor, to: executor, allow: true}
limits: {depth: 2, concurrency: 2}
`

// A migrate moves every member to a role of the new template (the gate to its session role, the
// rest by map or same name), refuses when one is left without, and tells each live member by mail.
func TestTeamMigrate(t *testing.T) {
	f := newAgentFixture(t)
	f.spawn(t, f.lead, "w1")
	_, err := f.e.TeamMigrate(ctx, core.TeamMigrateArgs{Team: f.teamID, Manifest: supExec})
	if rule(err) != "/migrate.unmapped" || !strings.Contains(err.Error(), "no role lead, worker") {
		t.Fatalf("migrate with unmapped roles: %v", err)
	}
	if _, err := f.e.TeamMigrate(ctx, core.TeamMigrateArgs{Team: f.teamID, Manifest: supExec, Map: map[string]string{"worker": "boss"}}); code(err) != core.CodeInvalid {
		t.Fatalf("map to a role the template lacks: %v", err)
	}
	r, err := f.e.TeamMigrate(ctx, core.TeamMigrateArgs{Team: f.teamID, Manifest: supExec,
		Map: map[string]string{"lead": "executor", "worker": "executor"}})
	if err != nil {
		t.Fatal(err)
	}
	moves := []core.RoleMove{{Name: "lead", From: "lead", To: "supervisor"}, {Name: "lead2", From: "lead", To: "executor"},
		{Name: "w1", From: "worker", To: "executor"}}
	if r.From != "lw" || r.To != "sx" || len(r.Moves) != 3 || r.Moves[0] != moves[0] || r.Moves[1] != moves[1] || r.Moves[2] != moves[2] {
		t.Fatalf("migrate = %+v; want the map applied, but the gate to the session role", r)
	}
	if len(r.Warnings) != 1 || !strings.HasPrefix(r.Warnings[0], "w1 (executor) cannot send to lead (supervisor)") {
		t.Fatalf("warnings = %q; want the missing executor -> supervisor route", r.Warnings)
	}
	in, err := f.e.Inbox(ctx, f.lead2, core.InboxArgs{})
	if err != nil || len(in) != 1 || in[0].From != core.AddrEngine ||
		!strings.HasPrefix(in[0].Body, "Your team lw now runs template sx (was lw); your role is executor (was lead).") ||
		!strings.Contains(in[0].Body, `role "executor" in piggery team "lw"`) {
		t.Fatalf("lead2's mail = %+v, %v; want the move with its new card", in, err)
	}
	// The new manifest holds: the gate spawns as a supervisor.
	if _, err := f.e.Agent(ctx, f.lead, core.AgentArgs{Action: core.AgentSpawn, Role: "executor", Name: "x1", Task: "t"}); err != nil {
		t.Fatalf("spawn under the new template: %v", err)
	}
	if _, err := f.e.TeamMigrate(ctx, core.TeamMigrateArgs{Team: "nope", Manifest: supExec}); code(err) != core.CodeNotFound {
		t.Fatalf("migrate of no team: %v", err)
	}
}
