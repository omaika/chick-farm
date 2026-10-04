package core

import (
	"database/sql"
	"errors"
)

// A task (a member's latest mail marked op assign) is closed by a mail marked op accept or drop
// from whoever may assign it one, in reply to a mail of that task's chain. The outcome is the
// mail itself: ps reads it from the chain, the watch rules stop waiting on the task, and an event
// (task_accepted, task_dropped) lets the log count them.

var taskEvents = map[string]string{OpAccept: "task_accepted", OpDrop: "task_dropped"}

// TaskEvent is the payload of a task_accepted or task_dropped event.
type TaskEvent struct {
	Member string `json:"member"` // the member's id
	Task   int64  `json:"task"`   // the assignment's #N
	Title  string `json:"title"`
	Seq    int64  `json:"seq"` // the closing mail's #N
}

// taskClosedBy returns the task an accept or drop to member closes: its current task, which
// a.ReplyTo must lie in the reply chain of, and which nothing has closed yet.
func (t *txn) taskClosedBy(member string, a SendArgs) (watchTask, error) {
	if a.ReplyTo == "" {
		return watchTask{}, errf(CodeInvalid, "op %s needs reply_to: the handback (or the task) it answers", a.Op)
	}
	task, ok, err := t.currentTask(participant{id: member})
	if err != nil {
		return task, err
	}
	if !ok {
		return task, errf(CodeInvalid, "%s has no task to %s (no mail marked op assign)", a.To, a.Op)
	}
	var in bool
	err = t.QueryRowContext(t.ctx, `WITH RECURSIVE up(id, parent) AS (
			SELECT id, reply_to FROM messages WHERE id=?
			UNION SELECT m.id, m.reply_to FROM messages m JOIN up ON m.id=up.parent)
		SELECT EXISTS (SELECT 1 FROM up WHERE id=?)`, a.ReplyTo, task.id).Scan(&in)
	if err != nil {
		return task, internal(err)
	}
	if !in {
		return task, errf(CodeInvalid, "reply_to is not in the chain of %s's current task %s", a.To, taskRef(task))
	}
	if c, err := t.taskClosure(task.id); err != nil {
		return task, err
	} else if c != nil {
		return task, errf(CodeInvalid, "task %s is already closed: %s #%d", taskRef(task), pastOp(c.Op), c.Seq)
	}
	return task, nil
}

// taskClosedEvent records that p closed member's task with op.
func (t *txn) taskClosedEvent(p participant, member, op string, task watchTask, res SendResult) error {
	return t.event(evt{typ: taskEvents[op], participant: p.id, team: p.team, run: p.run, ref: res.ID,
		payload: TaskEvent{Member: member, Task: task.seq, Title: task.title, Seq: res.Seq}})
}

// taskClosure returns the delivered mail marked op accept or drop in the reply chain of the task
// with id task; nil when it is open.
func (t *txn) taskClosure(task string) (*TaskClosure, error) {
	var c TaskClosure
	err := t.QueryRowContext(t.ctx, `WITH RECURSIVE chain(id) AS (
			SELECT ? UNION SELECT m.id FROM messages m JOIN chain ON m.reply_to=chain.id)
		SELECT op, seq, created_at FROM messages WHERE id IN chain AND op IN (?, ?) AND held_reason IS NULL
		ORDER BY seq DESC LIMIT 1`, task, OpAccept, OpDrop).Scan(&c.Op, &c.Seq, &c.At)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, internal(err)
	}
	return &c, nil
}

// pastOp is "accepted" or "dropped".
func pastOp(op string) string {
	if op == OpAccept {
		return "accepted"
	}
	return "dropped"
}
