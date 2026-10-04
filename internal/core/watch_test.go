package core_test

import (
	"database/sql"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/sting8k/piggery/internal/core"
	"github.com/sting8k/piggery/internal/store"
)

type watchFixture struct {
	e       *core.Engine
	db      *sql.DB
	lead, w core.Caller
	now     *time.Time
}

// newWatchFixture: a lead and a worker "w" (both joined) under one watch rule.
func newWatchFixture(t *testing.T, rule string, opts ...core.Option) watchFixture {
	t.Helper()
	db, err := store.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	now := time.Unix(1_800_000_000, 0)
	e := core.New(db, append(opts, core.WithClock(func() time.Time { return now }))...)
	man := "template: wr\nroles: {lead: {tools: [send, inbox, who, agent]}, worker: {tools: [send, inbox, who, agent]}}\nrouting:\n" +
		"  - {from: lead, to: worker, allow: true}\n  - {from: worker, to: lead, allow: true}\n" +
		"timers:\n  - " + rule + "\n"
	team, err := e.TeamUp(ctx, core.TeamUpArgs{Manifest: man, Cwd: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	join := func(name, role string) core.Caller {
		j, err := e.Join(ctx, core.JoinArgs{Team: team.ID, Role: role, Name: name, Cwd: team.RootCwd})
		if err != nil {
			t.Fatal(err)
		}
		c, _ := e.Authenticate(ctx, j.ID, j.Token)
		return c
	}
	return watchFixture{e: e, db: db, lead: join("lead", "lead"), w: join("w", "worker"), now: &now}
}

// newTaskWatchFixture: a joined lead that spawned worker "w" (task "fix the parser", reports to the
// lead) under the watch rules given; w is idle.
func newTaskWatchFixture(t *testing.T, rules ...string) watchFixture {
	t.Helper()
	db, err := store.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	now := time.Unix(1_800_000_000, 0)
	rt := &fakeRuntime{}
	e := core.New(db, core.WithRuntime(rt), core.WithClock(func() time.Time { return now }))
	man := "template: wr\nroles: {lead: {can_spawn: [worker], tools: [send, inbox, who, agent]}, worker: {tools: [send, inbox, who]}}\n" +
		"routing:\n  - {from: lead, to: worker, allow: true}\n  - {from: worker, to: lead, allow: true}\n" +
		"limits: {depth: 2, concurrency: 2}\ntimers:\n  - " + strings.Join(rules, "\n  - ") + "\n"
	team, err := e.TeamUp(ctx, core.TeamUpArgs{Manifest: man, Cwd: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	j, err := e.Join(ctx, core.JoinArgs{Team: team.ID, Role: "lead", Name: "lead", Cwd: team.RootCwd})
	if err != nil {
		t.Fatal(err)
	}
	lead, _ := e.Authenticate(ctx, j.ID, j.Token)
	if _, err := e.Agent(ctx, lead, core.AgentArgs{Action: core.AgentSpawn, Role: "worker", Name: "w", Task: "fix the parser"}); err != nil {
		t.Fatal(err)
	}
	s := rt.starts[len(rt.starts)-1]
	w, err := e.Authenticate(ctx, s.ParticipantID, s.Token)
	if err != nil {
		t.Fatal(err)
	}
	f := watchFixture{e: e, db: db, lead: lead, w: w, now: &now}
	f.presence(t, core.PresenceAgentSettled)
	return f
}

// send sends a from b, returning its #N.
func (f watchFixture) send(t *testing.T, from core.Caller, a core.SendArgs) string {
	t.Helper()
	r, err := f.e.Send(ctx, from, a)
	if err != nil {
		t.Fatal(err)
	}
	return fmt.Sprintf("#%d", r.Seq)
}

// engineMail returns the engine notices in c's inbox.
func (f watchFixture) engineMail(t *testing.T, c core.Caller) []string {
	t.Helper()
	in, err := f.e.Inbox(ctx, c, core.InboxArgs{})
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, m := range in {
		if m.From == core.AddrEngine {
			out = append(out, m.Body)
		}
	}
	return out
}

func (f watchFixture) presence(t *testing.T, event string) {
	t.Helper()
	if err := f.e.Presence(ctx, f.w, core.PresenceArgs{Event: event}); err != nil {
		t.Fatal(err)
	}
}

func (f watchFixture) advance(d time.Duration) { *f.now = f.now.Add(d) }

// fires runs one watch tick and checks how many incidents fired.
func (f watchFixture) fires(t *testing.T, want int) {
	t.Helper()
	n, err := f.e.Watch(ctx)
	if err != nil || n != want {
		t.Fatalf("Watch fired %d (%v), want %d", n, err, want)
	}
}

// leadNotices returns the engine notices in the lead's inbox.
func (f watchFixture) leadNotices(t *testing.T) []string { return f.engineMail(t, f.lead) }

func TestWatchSilentFor(t *testing.T) {
	f := newWatchFixture(t, "{on: worker, notify: lead, silent_for: 10m}")
	f.presence(t, core.PresenceAgentStart)
	f.advance(9 * time.Minute)
	f.fires(t, 0)
	f.presence(t, core.PresenceTurnEnd) // a turn end restarts the silence
	f.advance(9 * time.Minute)
	f.fires(t, 0)
	f.advance(2 * time.Minute)
	f.fires(t, 1)
	f.fires(t, 0) // same incident
	if n := f.leadNotices(t); len(n) != 1 || !strings.HasPrefix(n[0], "w (worker) has been working for 11m") {
		t.Fatalf("notices = %q", n)
	}
	f.presence(t, core.PresenceAgentSettled) // a new working stretch is a new incident
	f.presence(t, core.PresenceAgentStart)
	f.advance(11 * time.Minute)
	f.fires(t, 1)
}

func TestWatchRulesValidated(t *testing.T) {
	db, err := store.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	e := core.New(db)
	for _, rule := range []string{
		"{on: worker, notify: lead, silent_for: 1m, colour: red}",                            // unknown key
		"{on: worker, notify: lead}",                                                         // no condition
		"{on: ghost, notify: lead, silent_for: 1m}",                                          // unknown role
		"{on: worker, notify: nobody, silent_for: 1m}",                                       // unknown target
		"{on: worker, notify: lead, silent_for: 1m, max_rework: 2}",                          // two conditions
		"{on: worker, notify: lead, max_rework: -1}",                                         // not positive
		"{on: worker, notify: lead, unanswered_for: 0s}",                                     // not positive
		"{on: worker, notify: lead, silent_for: 1m, escalate_after: 5m}",                     // escalate_after alone
		"{on: worker, notify: lead, silent_for: 1m, escalate_to: nobody}",                    // unknown escalation target
		"{on: worker, notify: lead, max_rework: 2, escalate_to: notify, escalate_after: 1m}", // max_rework escalates by count
	} {
		man := "template: wr\nroles: {lead: {tools: [send, inbox, who, agent]}, worker: {tools: [send, inbox, who, agent]}}\ntimers:\n  - " + rule + "\n"
		if _, err := e.TeamUp(ctx, core.TeamUpArgs{Manifest: man, Cwd: t.TempDir()}); code(err) != core.CodeInvalid {
			t.Fatalf("rule %s: %v", rule, err)
		}
	}
}

// idle_with_task_for: the spawn task waits on an idle worker until it hands back; a question to the
// lead counts as its move. The worker itself is reminded, and the lead gets the escalation.
func TestWatchIdleWithTask(t *testing.T) {
	f := newTaskWatchFixture(t, "{on: worker, notify: self, idle_with_task_for: 10m, escalate_to: reports_to, escalate_after: 5m}")
	f.advance(11 * time.Minute)
	f.fires(t, 1)
	f.fires(t, 0)
	if n := f.engineMail(t, f.w); len(n) != 1 || !strings.HasPrefix(n[0], "You have been idle for 11m0s with task #") ||
		!strings.Contains(n[0], `"fix the parser"`) {
		t.Fatalf("self notice = %q", n)
	}
	f.advance(5 * time.Minute)
	f.fires(t, 1) // escalated once
	f.fires(t, 0)
	if n := f.leadNotices(t); len(n) != 1 || !strings.Contains(n[0], "escalated: told 5m0s ago") {
		t.Fatalf("escalation = %q", n)
	}
	f.presence(t, core.PresenceAgentStart) // a new idle stretch after a question: not waiting on w
	f.send(t, f.w, core.SendArgs{To: "lead", Kind: "ask", Body: "which parser?"})
	f.presence(t, core.PresenceAgentSettled)
	f.advance(11 * time.Minute)
	f.fires(t, 0)
}

// unanswered_for: the oldest mail from a live teammate with nothing sent back to its sender.
func TestWatchUnanswered(t *testing.T) {
	f := newTaskWatchFixture(t, "{on: lead, notify: self, unanswered_for: 10m, escalate_to: notify}")
	n := f.send(t, f.w, core.SendArgs{To: "lead", Kind: "handback", Body: "done"})
	f.advance(9 * time.Minute)
	f.fires(t, 0)
	f.advance(2 * time.Minute)
	f.fires(t, 1)
	if got := f.leadNotices(t); len(got) != 1 || got[0] != "You have not answered "+n+" (handback) from w for 11m0s (rule unanswered_for 10m)." {
		t.Fatalf("notice = %q", got)
	}
	f.advance(10 * time.Minute)
	f.fires(t, 1) // escalated to the Human's notices
	ns, err := f.e.Notices(ctx, 5)
	if err != nil || len(ns) != 1 || ns[0].Kind != "watch" || !strings.HasPrefix(ns[0].Body, "lead (lead) has not answered "+n) {
		t.Fatalf("notices = %+v, %v", ns, err)
	}
	f.send(t, f.lead, core.SendArgs{To: "w", Kind: "rework", Body: "again"})
	f.advance(30 * time.Minute)
	f.fires(t, 0)
}

// max_rework: more reworks than allowed on one task tell the rule's target; one more after that
// escalates; a new task starts the count again.
func TestWatchMaxRework(t *testing.T) {
	f := newTaskWatchFixture(t, "{on: worker, notify: self, max_rework: 2, escalate_to: reports_to}")
	rework := func() {
		f.send(t, f.w, core.SendArgs{To: "lead", Kind: "handback", Body: "done"})
		f.send(t, f.lead, core.SendArgs{To: "w", Kind: "rework", Body: "not yet"})
	}
	rework()
	rework()
	f.fires(t, 0)
	rework()
	f.fires(t, 1)
	f.fires(t, 0)
	if n := f.engineMail(t, f.w); len(n) != 1 || !strings.HasPrefix(n[0], `You have been sent 3 reworks on task #`) {
		t.Fatalf("notice = %q", n)
	}
	rework()
	f.fires(t, 1)
	f.fires(t, 0)
	if n := f.leadNotices(t); len(n) != 1 || !strings.Contains(n[0], "been sent 4 reworks") {
		t.Fatalf("escalation = %q", n)
	}
	f.send(t, f.lead, core.SendArgs{To: "w", Op: core.OpAssign, Kind: "task", Body: "next"})
	rework()
	rework()
	rework()
	f.fires(t, 1) // the new task's own incident
}
