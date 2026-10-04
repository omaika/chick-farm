package core_test

import (
	"fmt"
	"testing"

	"github.com/sting8k/piggery/internal/core"
)

// tasks: every task given in a team, newest first, with what became of it: accepted after a rework,
// replaced by a next task before anyone closed it, or still open; the same by the team's directory.
func TestTasks(t *testing.T) {
	f := newTaskWatchFixture(t, "{on: worker, notify: reports_to, silent_for: 1h}")
	s, err := f.e.State(ctx, core.StateArgs{})
	if err != nil {
		t.Fatal(err)
	}
	team := s.Teams[0]
	first := ""
	for _, m := range team.Members {
		if m.Assignment != nil {
			first = fmt.Sprintf("#%d", m.Assignment.Seq)
		}
	}
	hb := f.send(t, f.w, core.SendArgs{To: "lead", Kind: "handback", Body: "done", ReplyTo: first})
	f.send(t, f.lead, core.SendArgs{To: "w", Kind: "rework", Body: "not yet", ReplyTo: hb})
	hb = f.send(t, f.w, core.SendArgs{To: "lead", Kind: "handback", Body: "done now", ReplyTo: first})
	r, err := f.e.Tasks(ctx, core.TasksArgs{Team: team.Name})
	if err != nil || len(r.Tasks) != 1 || r.Tasks[0].State != core.TaskHandedBack || r.Tasks[0].Reworks != 1 || r.Tasks[0].Handbacks != 2 {
		t.Fatalf("after two handbacks: %+v, %v", r, err)
	}
	f.send(t, f.lead, core.SendArgs{To: "w", Op: core.OpAccept, Body: "good", ReplyTo: hb})
	f.send(t, f.lead, core.SendArgs{To: "w", Op: core.OpAssign, Kind: "task", Body: "tidy the lexer"})
	f.send(t, f.lead, core.SendArgs{To: "w", Op: core.OpAssign, Kind: "task", Body: "write the docs"})
	f.send(t, f.w, core.SendArgs{To: "lead", Kind: "ask", Body: "which docs?"})

	byDir, err := f.e.Tasks(ctx, core.TasksArgs{Dir: team.Root})
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, k := range byDir.Tasks {
		if k.Member != "w" || k.From != "lead" || k.Team != team.Name {
			t.Fatalf("task %+v", k)
		}
		got = append(got, k.Title+" "+k.State)
	}
	want := []string{"write the docs open", "tidy the lexer replaced", "fix the parser accepted"}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] || got[2] != want[2] {
		t.Fatalf("tasks = %q, want %q", got, want)
	}
	if byDir.Tasks[2].ClosedSeq == 0 {
		t.Fatalf("accepted task without its accept: %+v", byDir.Tasks[2])
	}
	if _, err := f.e.Tasks(ctx, core.TasksArgs{}); code(err) != core.CodeInvalid {
		t.Fatalf("tasks without a team or a directory: %v", err)
	}
}
