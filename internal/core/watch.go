package core

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Watch rules (manifest `timers:`): the engine notices a condition on a member from state alone
// (its presence, and the kinds and order of its mail) and tells the rule's target once per
// incident; when the incident outlasts that notice, it tells escalate_to once. Nothing is judged by
// content.
//
// Conditions, one per rule:
//   - silent_for D: working for longer than D with no turn end.
//   - idle_with_task_for D: idle for longer than D while its task (its latest mail marked op assign)
//     waits on it: the newest mail between it and the task's sender, since the task, is the sender's,
//     and no mail marked op accept or drop has closed it.
//   - unanswered_for D: a live teammate's mail to it has had no mail from it back to that teammate for
//     longer than D (the oldest such mail is the incident); an accept or drop wants no answer.
//   - max_rework N: more than N mails of kind rework to it since its task.
//
// escalate_to fires escalate_after (default: the condition's D) after the notice while the same
// incident holds; for max_rework, at the next rework after the notice.

// watchRule is one entry of the manifest's `timers:` list.
type watchRule struct {
	On              string `yaml:"on"`
	Notify          string `yaml:"notify"` // a role, reports_to or self (notify loads and is ignored)
	SilentFor       string `yaml:"silent_for"`
	IdleWithTaskFor string `yaml:"idle_with_task_for"`
	UnansweredFor   string `yaml:"unanswered_for"`
	MaxRework       int    `yaml:"max_rework"`
	EscalateTo      string `yaml:"escalate_to"` // a role, reports_to, self or notify (the Human's notices)
	EscalateAfter   string `yaml:"escalate_after"`
}

// The conditions of a watch rule (its yaml keys) and the targets beyond a role.
const (
	condSilent     = "silent_for"
	condIdleTask   = "idle_with_task_for"
	condUnanswered = "unanswered_for"
	condMaxRework  = "max_rework"

	targetReportsTo = "reports_to"
	targetSelf      = "self"
)

var watchKeys = map[string]bool{"on": true, "notify": true, condSilent: true, condIdleTask: true, condUnanswered: true,
	condMaxRework: true, "escalate_to": true, "escalate_after": true}

// condition returns the rule's one condition and its duration (0 for max_rework).
func (r watchRule) condition() (string, time.Duration, error) {
	var set []string
	for _, c := range []struct{ key, val string }{{condSilent, r.SilentFor}, {condIdleTask, r.IdleWithTaskFor},
		{condUnanswered, r.UnansweredFor}} {
		if c.val != "" {
			set = append(set, c.key)
		}
	}
	if r.MaxRework != 0 {
		set = append(set, condMaxRework)
	}
	switch {
	case len(set) == 0:
		return "", 0, fmt.Errorf("needs one of %s, %s, %s or %s", condSilent, condIdleTask, condUnanswered, condMaxRework)
	case len(set) > 1:
		return "", 0, fmt.Errorf("has %s: one condition per rule", strings.Join(set, " and "))
	}
	switch set[0] {
	case condMaxRework:
		if r.MaxRework < 0 {
			return "", 0, fmt.Errorf("max_rework must be positive")
		}
		return condMaxRework, 0, nil
	case condIdleTask:
		d, err := positiveDuration(condIdleTask, r.IdleWithTaskFor)
		return condIdleTask, d, err
	case condUnanswered:
		d, err := positiveDuration(condUnanswered, r.UnansweredFor)
		return condUnanswered, d, err
	}
	d, err := positiveDuration(condSilent, r.SilentFor)
	return condSilent, d, err
}

// describe is the rule's condition as written, e.g. "silent_for 20m".
func (r watchRule) describe() string {
	cond, _, _ := r.condition()
	switch cond {
	case condMaxRework:
		return fmt.Sprintf("%s %d", cond, r.MaxRework)
	case condIdleTask:
		return cond + " " + r.IdleWithTaskFor
	case condUnanswered:
		return cond + " " + r.UnansweredFor
	}
	return cond + " " + r.SilentFor
}

// escalation returns how long after the notice an incident that still holds goes to escalate_to
// (0 for max_rework: the next rework does). Without escalate_to it is 0.
func (r watchRule) escalation() (time.Duration, error) {
	cond, d, err := r.condition()
	switch {
	case err != nil:
		return 0, err
	case r.EscalateTo == "" && r.EscalateAfter != "":
		return 0, fmt.Errorf("escalate_after needs escalate_to")
	case r.EscalateTo == "":
		return 0, nil
	case cond == condMaxRework && r.EscalateAfter != "":
		return 0, fmt.Errorf("escalate_after does not apply to max_rework: it escalates at the next rework after the notice")
	case r.EscalateAfter == "":
		return d, nil
	}
	return positiveDuration("escalate_after", r.EscalateAfter)
}

func positiveDuration(key, val string) (time.Duration, error) {
	d, err := time.ParseDuration(val)
	if err == nil && d <= 0 {
		err = fmt.Errorf("%s must be positive", key)
	}
	return d, err
}

// parseTimers reads the manifest's `timers:` list.
func parseTimers(text string) ([]watchRule, error) {
	var t struct {
		Timers []watchRule `yaml:"timers"`
	}
	err := yaml.Unmarshal([]byte(text), &t)
	return t.Timers, err
}

// validateTimers checks the `timers:` list of a manifest being brought up.
func validateTimers(text string, m manifest) error {
	var raw struct {
		Timers []map[string]any `yaml:"timers"`
	}
	if err := yaml.Unmarshal([]byte(text), &raw); err != nil {
		return errf(CodeInvalid, "manifest: timers: %v", err)
	}
	for i, r := range raw.Timers {
		for k := range r {
			if !watchKeys[k] {
				return errf(CodeInvalid, "manifest: timers[%d]: unknown key %q", i, k)
			}
		}
	}
	rules, err := parseTimers(text)
	if err != nil {
		return errf(CodeInvalid, "manifest: timers: %v", err)
	}
	for i, r := range rules {
		if _, ok := m.Roles[r.On]; !ok {
			return errf(CodeInvalid, "manifest: timers[%d]: on: unknown role %q", i, r.On)
		}
		if _, ok := m.Roles[r.Notify]; !ok && r.Notify != targetReportsTo && r.Notify != targetSelf && r.Notify != AddrNotify {
			return errf(CodeInvalid, "manifest: timers[%d]: notify: %q is not a role, reports_to, self, or notify", i, r.Notify)
		}
		if _, ok := m.Roles[r.EscalateTo]; !ok && r.EscalateTo != "" && r.EscalateTo != targetReportsTo &&
			r.EscalateTo != targetSelf && r.EscalateTo != AddrNotify {
			return errf(CodeInvalid, "manifest: timers[%d]: escalate_to: %q is not a role, reports_to, self, or notify", i, r.EscalateTo)
		}
		if _, err := r.escalation(); err != nil {
			return errf(CodeInvalid, "manifest: timers[%d]: %v", i, err)
		}
	}
	return nil
}

// Watch evaluates every open team's watch rules (daemon tick) and sends one notice per new
// incident, and one escalation per incident that outlasts its notice. It returns the number of
// notices and escalations sent.
func (e *Engine) Watch(ctx context.Context) (int, error) {
	rows, err := e.db.QueryContext(ctx, `SELECT id, manifest FROM teams WHERE closed_at IS NULL`)
	if err != nil {
		return 0, internal(err)
	}
	type team struct{ id, text string }
	var teams []team
	for rows.Next() {
		var t team
		if err := rows.Scan(&t.id, &t.text); err != nil {
			rows.Close()
			return 0, internal(err)
		}
		teams = append(teams, t)
	}
	rows.Close()
	fired := 0
	for _, tm := range teams {
		rules, err := parseTimers(tm.text)
		if err != nil {
			continue
		}
		for i, r := range rules {
			if _, err := r.escalation(); err != nil {
				continue // TeamUp validated; skip anything unreadable
			}
			if r.Notify == AddrNotify {
				continue // only the engine writes to notify (notifyWarnings)
			}
			var wake, hooks []string
			err = e.inTx(ctx, func(t *txn) error {
				n, w, h, err := t.watchRule(tm.id, i, r)
				fired += n
				wake, hooks = w, h
				return err
			})
			if err != nil {
				return fired, err
			}
			for _, id := range wake {
				e.notifyAfterCommit(id)
			}
			for _, id := range hooks {
				e.notifyHookAfterCommit(id)
			}
		}
	}
	return fired, rows.Err()
}

// incident is one predicate hit: the subject, its dedupe key, and the notice text part.
type incident struct {
	subject participant
	key     string
	what    string // e.g. "has been working for 21m with no turn end"
	count   int    // max_rework: the reworks so far
}

// firedIncident is a watch_fired event: when the notice went out and the count it carried.
type firedIncident struct {
	ts    int64
	count int
}

// watchRule runs one rule of a team: every incident with an unseen key gets notices and a
// watch_fired event; one already noticed that is due goes to escalate_to once (watch_escalated). It
// returns the notices and escalations sent, the participants to wake and the notify messages.
func (t *txn) watchRule(teamID string, idx int, r watchRule) (fired int, wake, hooks []string, err error) {
	cond, d, _ := r.condition()
	after, _ := r.escalation()
	subjects, err := t.teamRole(teamID, r.On)
	if err != nil {
		return 0, nil, nil, err
	}
	ruleID := fmt.Sprintf("timers[%d] %s", idx, cond)
	for _, p := range subjects {
		in, ok, err := t.watchIncident(cond, p, d, r.MaxRework)
		if err != nil {
			return 0, nil, nil, err
		}
		if !ok {
			continue
		}
		in.key = teamID + "/" + ruleID + "/" + in.key
		prev, seen, err := t.firedIncident(p.id, "watch_fired", in.key)
		if err != nil {
			return 0, nil, nil, err
		}
		if !seen {
			w, h, err := t.watchNotice(teamID, r.Notify, in, fmt.Sprintf("(rule %s)", r.describe()))
			if err != nil {
				return 0, nil, nil, err
			}
			wake, hooks = append(wake, w...), append(hooks, h...)
			if err := t.event(evt{typ: "watch_fired", participant: p.id, team: teamID, run: p.run,
				payload: map[string]any{"rule": ruleID, "participant": p.id, "key": in.key,
					"notified": len(w) + len(h), "count": in.count}}); err != nil {
				return 0, nil, nil, err
			}
			fired++
			continue
		}
		if r.EscalateTo == "" {
			continue
		}
		if _, done, err := t.firedIncident(p.id, "watch_escalated", in.key); err != nil || done {
			if err != nil {
				return 0, nil, nil, err
			}
			continue
		}
		if cond == condMaxRework && in.count <= prev.count || cond != condMaxRework && t.now-prev.ts < after.Milliseconds() {
			continue
		}
		tail := fmt.Sprintf("(rule %s, escalated: told %s ago)", r.describe(), age(t.now-prev.ts))
		w, h, err := t.watchNotice(teamID, r.EscalateTo, in, tail)
		if err != nil {
			return 0, nil, nil, err
		}
		wake, hooks = append(wake, w...), append(hooks, h...)
		if err := t.event(evt{typ: "watch_escalated", participant: p.id, team: teamID, run: p.run,
			payload: map[string]any{"rule": ruleID, "participant": p.id, "key": in.key, "to": r.EscalateTo,
				"notified": len(w) + len(h)}}); err != nil {
			return 0, nil, nil, err
		}
		fired++
	}
	return fired, wake, hooks, nil
}

// firedIncident is the event typ (watch_fired, watch_escalated) of incident key on p, if any.
func (t *txn) firedIncident(p, typ, key string) (firedIncident, bool, error) {
	var f firedIncident
	err := t.QueryRowContext(t.ctx, `SELECT ts, COALESCE(json_extract(payload,'$.count'),0) FROM events
		WHERE type=? AND participant=? AND json_extract(payload,'$.key')=? ORDER BY seq LIMIT 1`, typ, p, key).Scan(&f.ts, &f.count)
	if errors.Is(err, sql.ErrNoRows) {
		return f, false, nil
	}
	return f, err == nil, internal(err)
}

// watchNotice tells target (a role, reports_to, self, or notify: the Human) about in, the sentence
// ending with tail. It returns the participants to wake and the notify messages written.
func (t *txn) watchNotice(teamID, target string, in incident, tail string) (wake, hooks []string, err error) {
	if target == AddrNotify {
		gate, ok, err := t.teamGate(teamID)
		if err != nil {
			return nil, nil, err
		}
		body := fmt.Sprintf("%s (%s) %s %s.", in.subject.name, in.subject.role, in.what, tail)
		msg := newID(t.now)
		if _, err := t.insertMessage(msg, "", teamID, AddrEngine, AddrNotify, noticeWatch, "", "", "", body); err != nil {
			return nil, nil, err
		}
		ev := evt{typ: "notice", team: teamID, ref: msg, payload: map[string]any{"kind": noticeWatch, "to": AddrNotify}}
		if ok {
			ev.participant, ev.run = gate.id, gate.run
		}
		return nil, []string{msg}, t.event(ev)
	}
	targets, err := t.watchTargets(teamID, target, in.subject)
	if err != nil {
		return nil, nil, err
	}
	for _, to := range targets {
		who := label(to, in.subject)
		if to.id == in.subject.id {
			who = "You"
		}
		body := fmt.Sprintf("%s %s %s.", who, youVerb(in.what, to.id == in.subject.id), tail)
		msg := newID(t.now)
		if _, err := t.insertMessage(msg, "", teamID, AddrEngine, to.id, "", "", "", "", body); err != nil {
			return nil, nil, err
		}
		wake = append(wake, to.id)
	}
	return wake, nil, nil
}

// youVerb turns an incident's "has been…"/"has not…" into "have…" for the subject itself.
func youVerb(what string, self bool) string {
	if rest, ok := strings.CutPrefix(what, "has "); ok && self {
		return "have " + rest
	}
	return what
}

// watchIncident evaluates condition cond (duration d, or n for max_rework) on p.
func (t *txn) watchIncident(cond string, p participant, d time.Duration, n int) (incident, bool, error) {
	switch cond {
	case condIdleTask:
		return t.idleTaskIncident(p, d)
	case condUnanswered:
		return t.unansweredIncident(p, d)
	case condMaxRework:
		return t.reworkIncident(p, n)
	}
	return t.silentIncident(p, d)
}

// silentIncident reports p working with no turn end for longer than d. The key (relative to
// the rule) is the working stretch; last_activity is never used.
func (t *txn) silentIncident(p participant, d time.Duration) (incident, bool, error) {
	if p.state != "working" {
		return incident{}, false, nil
	}
	var lastTurnEnd sql.NullInt64
	if err := t.QueryRowContext(t.ctx, `SELECT last_turn_end FROM participants WHERE id=?`, p.id).Scan(&lastTurnEnd); err != nil {
		return incident{}, false, internal(err)
	}
	since := p.stateSince
	if lastTurnEnd.Valid && lastTurnEnd.Int64 > since {
		since = lastTurnEnd.Int64
	}
	if t.now-since <= d.Milliseconds() {
		return incident{}, false, nil
	}
	return incident{subject: p, key: fmt.Sprintf("%s/%d", p.id, p.stateSince),
		what: fmt.Sprintf("has been working for %s with no turn end", age(t.now-since))}, true, nil
}

// watchTask is p's task as the watch rules see it: its latest mail marked op assign.
type watchTask struct {
	id, from string
	seq      int64
	title    string
}

// currentTask returns p's latest delivered mail marked op assign (a spawn or resume task is one).
func (t *txn) currentTask(p participant) (watchTask, bool, error) {
	var w watchTask
	var body string
	err := t.QueryRowContext(t.ctx, `SELECT id, from_id, seq, body FROM messages WHERE op=? AND to_id=?
		AND held_reason IS NULL ORDER BY seq DESC LIMIT 1`, OpAssign, p.id).Scan(&w.id, &w.from, &w.seq, &body)
	if errors.Is(err, sql.ErrNoRows) {
		return w, false, nil
	}
	if err != nil {
		return w, false, internal(err)
	}
	w.title = assignmentTitle(body)
	return w, true, nil
}

// idleTaskIncident reports p idle for longer than d while its task waits on it: after the task,
// p has sent nothing to the task's sender since the sender last wrote (p has not handed back or
// asked). The key is the idle stretch.
func (t *txn) idleTaskIncident(p participant, d time.Duration) (incident, bool, error) {
	if p.state != "idle" || t.now-p.stateSince <= d.Milliseconds() {
		return incident{}, false, nil
	}
	task, ok, err := t.currentTask(p)
	if err != nil || !ok {
		return incident{}, false, err
	}
	if c, err := t.taskClosure(task.id); err != nil || c != nil {
		return incident{}, false, err // accepted or dropped: nothing waits on it
	}
	// An admin's task (resume) has no sender to hand back to: p hands it back to its reports_to.
	if task.from == "" {
		if task.from = p.reportsTo; task.from == "" {
			return incident{}, false, nil
		}
	}
	var last string
	err = t.QueryRowContext(t.ctx, `SELECT from_id FROM messages WHERE seq>? AND cc_of IS NULL
		AND held_reason IS NULL AND ((from_id=? AND to_id=?) OR (from_id=? AND to_id=?)) ORDER BY seq DESC LIMIT 1`,
		task.seq, task.from, p.id, p.id, task.from).Scan(&last)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return incident{}, false, internal(err)
	}
	if last == p.id {
		return incident{}, false, nil
	}
	return incident{subject: p, key: fmt.Sprintf("%s/%d", p.id, p.stateSince),
		what: fmt.Sprintf("has been idle for %s with task %s not handed back", age(t.now-p.stateSince),
			taskRef(task))}, true, nil
}

// unansweredIncident reports the oldest mail to p from a live teammate that p has sent nothing
// back to, for longer than d. The key is that mail.
func (t *txn) unansweredIncident(p participant, d time.Duration) (incident, bool, error) {
	if p.state == "gone" {
		return incident{}, false, nil
	}
	var seq, at int64
	var id, from, kind string
	err := t.QueryRowContext(t.ctx, `SELECT m.id, m.seq, m.created_at, f.name, COALESCE(m.kind,'')
		FROM messages m JOIN participants f ON f.id=m.from_id
		WHERE m.to_id=? AND m.cc_of IS NULL AND m.held_reason IS NULL AND m.created_at<? AND COALESCE(m.op,'') NOT IN (?, ?)
		AND f.team_id=? AND f.id<>? AND f.state<>'gone'
		AND NOT EXISTS (SELECT 1 FROM messages r WHERE r.from_id=m.to_id AND r.to_id=m.from_id AND r.seq>m.seq
			AND r.cc_of IS NULL)
		ORDER BY m.seq LIMIT 1`, p.id, t.now-d.Milliseconds(), OpAccept, OpDrop, p.team, p.id).Scan(&id, &seq, &at, &from, &kind)
	if errors.Is(err, sql.ErrNoRows) {
		return incident{}, false, nil
	}
	if err != nil {
		return incident{}, false, internal(err)
	}
	if kind != "" {
		kind = " (" + kind + ")"
	}
	return incident{subject: p, key: p.id + "/" + id,
		what: fmt.Sprintf("has not answered #%d%s from %s for %s", seq, kind, from, age(t.now-at))}, true, nil
}

// reworkIncident reports more than n mails of kind rework to p since its task (since it joined,
// with none). The key is the task, so a new task starts the count again.
func (t *txn) reworkIncident(p participant, n int) (incident, bool, error) {
	if p.state == "gone" {
		return incident{}, false, nil
	}
	task, ok, err := t.currentTask(p)
	if err != nil {
		return incident{}, false, err
	}
	var count int
	if err := t.QueryRowContext(t.ctx, `SELECT COUNT(*) FROM messages WHERE to_id=? AND kind='rework'
		AND cc_of IS NULL AND held_reason IS NULL AND seq>?`, p.id, task.seq).Scan(&count); err != nil {
		return incident{}, false, internal(err)
	}
	if count <= n {
		return incident{}, false, nil
	}
	key, on := p.id+"/-", ""
	if ok {
		key, on = p.id+"/"+task.id, " on task "+taskRef(task)
	}
	return incident{subject: p, key: key, count: count,
		what: fmt.Sprintf("has been sent %d reworks%s", count, on)}, true, nil
}

func taskRef(w watchTask) string {
	if w.title == "" {
		return fmt.Sprintf("#%d", w.seq)
	}
	return fmt.Sprintf("#%d %q", w.seq, w.title)
}

// watchTargets resolves a rule's notify or escalate_to target for subject p.
func (t *txn) watchTargets(teamID, notify string, p participant) ([]participant, error) {
	switch notify {
	case targetSelf:
		return []participant{p}, nil
	case targetReportsTo:
		if p.reportsTo == "" {
			return nil, nil
		}
		q, ok, err := t.participantByID(p.reportsTo)
		if err != nil || !ok {
			return nil, err
		}
		return []participant{q}, nil
	}
	all, err := t.teamRole(teamID, notify)
	if err != nil {
		return nil, err
	}
	out := all[:0]
	for _, q := range all {
		if q.id != p.id {
			out = append(out, q)
		}
	}
	return out, nil
}

func (t *txn) teamRole(teamID, role string) ([]participant, error) {
	rows, err := t.QueryContext(t.ctx, `SELECT `+participantCols+` FROM participants WHERE team_id=? AND role=? ORDER BY name`,
		teamID, role)
	if err != nil {
		return nil, internal(err)
	}
	defer rows.Close()
	var out []participant
	for rows.Next() {
		p, err := scanParticipant(rows)
		if err != nil {
			return nil, internal(err)
		}
		out = append(out, p)
	}
	return out, internal(rows.Err())
}

func age(ms int64) string {
	return (time.Duration(ms) * time.Millisecond).Round(time.Second).String()
}
