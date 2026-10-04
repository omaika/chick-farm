package core_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/sting8k/piggery/internal/core"
)

// A member's assignment is the latest mail marked op assign (a spawn task is one), tracked through
// its reply chain: a handback, a rework, a note that is not a task, and a later assign that replaces it.
func TestAssignment(t *testing.T) {
	f := newAgentFixture(t)
	w := f.spawn(t, f.lead, "w1")
	send := func(from core.Caller, a core.SendArgs) core.SendResult {
		t.Helper()
		r, err := f.e.Send(ctx, from, a)
		if err != nil {
			t.Fatal(err)
		}
		return r
	}
	assignment := func() core.Assignment {
		t.Helper()
		s, err := f.e.State(ctx, core.StateArgs{})
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range s.Teams[0].Members {
			if m.Name == "w1" && m.Assignment != nil {
				return *m.Assignment
			}
		}
		t.Fatal("w1 has no assignment")
		return core.Assignment{}
	}

	a := assignment()
	if a.Title != "do w1" || a.From != "lead" || a.Latest != nil {
		t.Fatalf("spawn task = %+v; want it, from lead, no reply", a)
	}
	handback := send(w, core.SendArgs{To: "lead", Body: "done", ReplyTo: fmt.Sprintf("#%d", a.Seq)})
	if l := assignment().Latest; l == nil || l.Seq != handback.Seq || !l.ByMember {
		t.Fatalf("after handback latest = %+v; want #%d by the member", l, handback.Seq)
	}
	// The lead sees the worker's handback labelled with the worker's real role and the relation.
	if in, err := f.e.Inbox(ctx, f.lead, core.InboxArgs{}); err != nil || len(in) == 0 || !strings.HasSuffix(in[len(in)-1].FromLabel, ", reports to you)") {
		t.Fatalf("lead inbox after the handback = %+v, %v; want the sender labelled (<role>, reports to you)", in, err)
	}
	note := send(f.lead, core.SendArgs{To: "w1", Body: "I restart the daemon"})
	if a2 := assignment(); a2.Seq != a.Seq || a2.Newer == nil || a2.Newer.Seq != note.Seq {
		t.Fatalf("after a note = %+v; want the same assignment and the note as newer", a2)
	}
	rework := send(f.lead, core.SendArgs{To: "w1", Body: "not enough", ReplyTo: fmt.Sprintf("#%d", handback.Seq)})
	if a2 := assignment(); a2.Latest == nil || a2.Latest.Seq != rework.Seq || a2.Latest.ByMember || a2.Newer != nil {
		t.Fatalf("after rework = %+v; want the rework newest, by the assigner, no newer", a2)
	}
	next := send(f.lead, core.SendArgs{To: "w1", Op: core.OpAssign, Body: "## **Second** task\nmore"})
	if a2 := assignment(); a2.Seq != next.Seq || a2.Title != "Second task" || a2.Latest != nil {
		t.Fatalf("after a later assign = %+v; want #%d titled %q", a2, next.Seq, "Second task")
	}

	if _, err := f.e.Send(ctx, f.lead2, core.SendArgs{To: "w1", Op: core.OpAssign, Body: "mine"}); rule(err) != "permission/assign.not_reports_to" {
		t.Fatalf("assign by a non-reports_to: %v", err)
	}
}

// A task is closed by a mail marked op accept or drop from the member's reports_to, in reply to a
// mail of the task's chain: ps shows the outcome, an event records it, and a closed task cannot be
// closed again. A later assign is a new, open task.
func TestTaskAcceptDrop(t *testing.T) {
	f := newAgentFixture(t)
	w := f.spawn(t, f.lead, "w1")
	send := func(from core.Caller, a core.SendArgs) (core.SendResult, error) { return f.e.Send(ctx, from, a) }
	assignment := func() core.Assignment {
		t.Helper()
		s, err := f.e.State(ctx, core.StateArgs{})
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range s.Teams[0].Members {
			if m.Name == "w1" && m.Assignment != nil {
				return *m.Assignment
			}
		}
		t.Fatal("w1 has no assignment")
		return core.Assignment{}
	}
	a := assignment()
	handback, err := send(w, core.SendArgs{To: "lead", Kind: "handback", Body: "done", ReplyTo: fmt.Sprintf("#%d", a.Seq)})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := send(f.lead, core.SendArgs{To: "w1", Op: core.OpAccept, Body: "good"}); code(err) != core.CodeInvalid {
		t.Fatalf("accept without reply_to: %v", err)
	}
	if _, err := send(f.lead2, core.SendArgs{To: "w1", Op: core.OpAccept, Body: "good", ReplyTo: fmt.Sprintf("#%d", handback.Seq)}); rule(err) != "permission/assign.not_reports_to" &&
		rule(err) != "visibility/reply_to.not_in_view" {
		t.Fatalf("accept by a non-reports_to: %v", err)
	}
	note, err := send(f.lead, core.SendArgs{To: "w1", Body: "a note"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := send(f.lead, core.SendArgs{To: "w1", Op: core.OpAccept, Body: "good", ReplyTo: fmt.Sprintf("#%d", note.Seq)}); code(err) != core.CodeInvalid {
		t.Fatalf("accept outside the task's chain: %v", err)
	}
	if a.Closed != nil {
		t.Fatalf("open task closed = %+v", a.Closed)
	}
	acc, err := send(f.lead, core.SendArgs{To: "w1", Op: core.OpAccept, Body: "good", ReplyTo: fmt.Sprintf("#%d", handback.Seq)})
	if err != nil {
		t.Fatal(err)
	}
	if c := assignment().Closed; c == nil || c.Op != core.OpAccept || c.Seq != acc.Seq {
		t.Fatalf("after accept closed = %+v; want accept #%d", c, acc.Seq)
	}
	if _, err := send(f.lead, core.SendArgs{To: "w1", Op: core.OpDrop, Body: "no", ReplyTo: fmt.Sprintf("#%d", handback.Seq)}); code(err) != core.CodeInvalid ||
		!strings.Contains(err.Error(), "already closed: accepted") {
		t.Fatalf("closing a closed task: %v", err)
	}
	next, err := send(f.lead, core.SendArgs{To: "w1", Op: core.OpAssign, Body: "second"})
	if err != nil {
		t.Fatal(err)
	}
	if a2 := assignment(); a2.Seq != next.Seq || a2.Closed != nil {
		t.Fatalf("after a new assign = %+v; want it open", a2)
	}
	drop, err := send(f.lead, core.SendArgs{To: "w1", Op: core.OpDrop, Body: "not needed", ReplyTo: fmt.Sprintf("#%d", next.Seq)})
	if err != nil {
		t.Fatal(err)
	}
	if c := assignment().Closed; c == nil || c.Op != core.OpDrop || c.Seq != drop.Seq {
		t.Fatalf("after drop closed = %+v", c)
	}
	evs, err := f.e.Log(ctx, core.LogArgs{})
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, ev := range evs {
		if ev.Type == "task_accepted" || ev.Type == "task_dropped" {
			got = append(got, fmt.Sprintf("%s %s", ev.Type, ev.Payload))
		}
	}
	if len(got) != 2 || !strings.Contains(got[0], fmt.Sprintf(`"task":%d`, a.Seq)) || !strings.Contains(got[1], fmt.Sprintf(`"task":%d`, next.Seq)) {
		t.Fatalf("task events = %q", got)
	}
}
