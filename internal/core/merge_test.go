package core_test

import (
	"testing"

	"github.com/sting8k/piggery/internal/core"
)

// A merge record is an event per status; the team's state keeps each branch's latest, an open
// conflict first, until it is recorded resolved.
func TestMergeRecords(t *testing.T) {
	f := newAgentFixture(t)
	merge := func(branch, status, note string) {
		t.Helper()
		r, err := f.e.Agent(ctx, f.lead, core.AgentArgs{Action: core.AgentMerge, Branch: branch, Into: "lane", Status: status, Note: note})
		if err != nil || r.Text != "recorded: "+branch+" into lane "+status {
			t.Fatalf("merge %s %s = %+v, %v", branch, status, r, err)
		}
	}
	merges := func() []core.MergeState {
		t.Helper()
		s, err := f.e.State(ctx, core.StateArgs{Events: 20})
		if err != nil {
			t.Fatal(err)
		}
		return s.Teams[0].Merges
	}
	merge("peer-a", core.MergeMerged, "")
	merge("peer-b", core.MergeConflict, "parser.go\nmore")
	if m := merges(); len(m) != 2 || m[0].Branch != "peer-b" || m[0].Status != core.MergeConflict || m[0].Note != "parser.go" ||
		m[0].By != "lead" || m[1].Branch != "peer-a" {
		t.Fatalf("merges = %+v; want the conflict first, its note one line", m)
	}
	merge("peer-b", core.MergeResolved, "")
	if m := merges(); len(m) != 2 || m[0].Branch != "peer-b" || m[0].Status != core.MergeResolved {
		t.Fatalf("merges after resolve = %+v", m)
	}
	var n int
	if err := f.db.QueryRow(`SELECT COUNT(*) FROM events WHERE type IN ('merged','merge_conflict','merge_resolved')`).Scan(&n); err != nil || n != 3 {
		t.Fatalf("merge events = %d, %v", n, err)
	}
	for _, a := range []core.AgentArgs{{Branch: "x", Status: "done"}, {Status: core.MergeMerged}} {
		a.Action = core.AgentMerge
		if _, err := f.e.Agent(ctx, f.lead, a); code(err) != core.CodeInvalid {
			t.Fatalf("merge %+v: %v", a, err)
		}
	}
}
