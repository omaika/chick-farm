package view

import (
	"encoding/json"
	"slices"
	"testing"
	"time"

	"github.com/sting8k/piggery/internal/core"
)

// A team with an open merge conflict is flagged on its line and lists its merges in its Overview;
// a merge event names its branch.
func TestMergesShown(t *testing.T) {
	now := time.UnixMilli(1_000_000_000_000)
	team := core.TeamState{ID: "t", Name: "demo", Root: "/p", CreatedAt: now.UnixMilli(),
		Members: []core.MemberState{{ID: "m", Name: "lead", State: "idle", StateSince: now.UnixMilli()}},
		Merges: []core.MergeState{{Branch: "a", Into: "main", Status: core.MergeConflict, By: "lead", At: now.UnixMilli(), Note: "x.go"},
			{Branch: "b", Into: "main", Status: core.MergeMerged, By: "lead", At: now.UnixMilli()}}}
	s := core.State{Teams: []core.TeamState{team}}
	var flags []string
	for _, d := range BuildList(ListInput{State: s, Tab: TabOpen, Now: now}).Dirs {
		for _, b := range d.Blocks {
			if b.Head != nil {
				flags = b.Head.Flags
			}
		}
	}
	if !slices.Contains(flags, "1 merge conflict") {
		t.Fatalf("flags = %q; want the open conflict", flags)
	}
	var facts []Fact
	for _, f := range Describe(s, TeamRow+"t", nil, now).Facts {
		if f.Label == "conflict" || f.Label == "merge" {
			facts = append(facts, f)
		}
	}
	if len(facts) != 2 || facts[0].Label != "conflict" || facts[0].Value != "a → main: conflict by lead, 0s ago (x.go)" {
		t.Fatalf("merge facts = %+v", facts)
	}
	payload, _ := json.Marshal(core.MergeState{Branch: "a", Into: "main", Status: core.MergeConflict})
	s.Events = []core.Event{{Type: "merge_conflict", Ts: now.UnixMilli(), Payload: payload}}
	if r := EventRows(s, 1, now); len(r) != 1 || r[0].Target != "a → main" || r[0].Tone != ToneWarning {
		t.Fatalf("event rows = %+v", r)
	}
}
