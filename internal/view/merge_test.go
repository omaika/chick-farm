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

// A team's Overview lists its live pins: #N, first line, who, when, and how many lines.
func TestPinsShown(t *testing.T) {
	now := time.UnixMilli(1_000_000_000_000)
	team := core.TeamState{ID: "t", Name: "demo", Root: "/p", CreatedAt: now.UnixMilli(),
		Members: []core.MemberState{{ID: "m", Name: "lead", State: "idle", StateSince: now.UnixMilli()}},
		Pins:    []core.PinState{{Seq: 7, By: "lead", At: now.UnixMilli(), Title: "Plan v2", Lines: 4}}}
	var got []string
	for _, f := range Describe(core.State{Teams: []core.TeamState{team}}, TeamRow+"t", nil, now).Facts {
		if f.Label == "pin" {
			got = append(got, f.Value)
		}
	}
	if len(got) != 1 || got[0] != "#7 Plan v2 · lead, 0s ago · 4 lines" {
		t.Fatalf("pin facts = %q", got)
	}
}

// A closed task reads as its outcome; a task_accepted event names the member and the task.
func TestClosedTask(t *testing.T) {
	now := time.UnixMilli(10 * 60_000)
	a := &core.Assignment{Seq: 5, Title: "fix", From: "lead", At: 60_000,
		Latest: &core.ChainMail{Seq: 9, At: 60_000}, Closed: &core.TaskClosure{Op: core.OpAccept, Seq: 9, At: 60_000}}
	if got := taskOf(a, now).Chain; got != "accepted #9 · 9m ago" {
		t.Fatalf("chain = %q", got)
	}
	a.Closed.Op = core.OpDrop
	if got := taskOf(a, now).Chain; got != "dropped #9 · 9m ago" {
		t.Fatalf("chain = %q", got)
	}
	payload, _ := json.Marshal(core.TaskEvent{Member: "p-w1-id", Task: 5, Seq: 9})
	s := core.State{Teams: []core.TeamState{{ID: "t", Members: []core.MemberState{{ID: "p-w1-id", Name: "w1"}}}},
		Events: []core.Event{{Type: "task_accepted", Participant: "lead", Payload: payload}}}
	if rows := EventRows(s, 5, now); len(rows) != 1 || rows[0].Target != "w1 #5" {
		t.Fatalf("rows = %+v", rows)
	}
}
