package core

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
)

// TasksArgs is `tasks` (admin, read-only): the tasks given in a team, or in every team rooted at a
// project directory, newest first. A task is a mail marked op assign; what became of it is read from
// the mail after it, as the watch rules read it.
type TasksArgs struct {
	Team string `json:"team,omitempty"` // a team (id, or name: every team of that name)
	Dir  string `json:"dir,omitempty"`  // a project directory: the teams rooted there, open and closed
}

// tasksMax caps a listing; the rest is older.
const tasksMax = 500

// What became of a task.
const (
	TaskOpen       = "open"        // the member's move, or a question waits on the assigner
	TaskHandedBack = "handed_back" // the member's last mail to the assigner is a handback: the assigner's move
	TaskAccepted   = "accepted"
	TaskDropped    = "dropped"
	TaskReplaced   = "replaced" // the member was given another task before this one was accepted or dropped
)

// TaskRow is one task as a person reads it.
type TaskRow struct {
	Seq        int64  `json:"seq"`
	ID         string `json:"id"`
	TeamID     string `json:"team_id"`
	Team       string `json:"team"`
	TeamClosed bool   `json:"team_closed,omitempty"`
	MemberID   string `json:"member_id"`
	Member     string `json:"member"`
	From       string `json:"from"` // the assigner's name; "admin" for the admin's resume
	Title      string `json:"title"`
	At         int64  `json:"at"`
	State      string `json:"state"`
	Reworks    int    `json:"reworks"`              // mails of kind rework to the member while it was its task
	Handbacks  int    `json:"handbacks"`            // mails of kind handback from the member while it was its task
	ClosedSeq  int64  `json:"closed_seq,omitempty"` // the accept or drop
	ClosedAt   int64  `json:"closed_at,omitempty"`
}

type TasksResult struct {
	Tasks []TaskRow `json:"tasks"`
	More  bool      `json:"more"` // older tasks were left out
}

// Tasks lists tasks for the operator (TasksArgs).
func (e *Engine) Tasks(ctx context.Context, a TasksArgs) (TasksResult, error) {
	if (a.Team == "") == (a.Dir == "") {
		return TasksResult{}, errf(CodeInvalid, "tasks needs a team or a directory, not both")
	}
	where, arg := "(t.id=? OR t.name=?)", []any{a.Team, a.Team}
	if a.Dir != "" {
		dir := filepath.Clean(a.Dir)
		if real, err := filepath.EvalSymlinks(dir); err == nil {
			dir = real
		}
		where, arg = "t.root_cwd=?", []any{dir}
	}
	out := TasksResult{Tasks: []TaskRow{}}
	err := e.readOnly(ctx, func(t *txn) error {
		rows, err := t.QueryContext(t.ctx, `SELECT m.id, m.seq, m.created_at, m.body, m.to_id, COALESCE(pt.name, m.to_id),
			m.from_id, COALESCE(pf.name, 'admin'), t.id, t.name, t.closed_at IS NOT NULL
			FROM messages m JOIN teams t ON t.id=m.team_id
			LEFT JOIN participants pt ON pt.id=m.to_id LEFT JOIN participants pf ON pf.id=m.from_id
			WHERE m.op=? AND m.cc_of IS NULL AND m.held_reason IS NULL AND `+where+`
			ORDER BY m.seq DESC LIMIT ?`, append(append([]any{OpAssign}, arg...), tasksMax+1)...)
		if err != nil {
			return internal(err)
		}
		var body []string
		var from []string
		for rows.Next() {
			var r TaskRow
			var b, f string
			if err := rows.Scan(&r.ID, &r.Seq, &r.At, &b, &r.MemberID, &r.Member, &f, &r.From, &r.TeamID, &r.Team,
				&r.TeamClosed); err != nil {
				rows.Close()
				return internal(err)
			}
			r.Title = assignmentTitle(b)
			out.Tasks, body, from = append(out.Tasks, r), append(body, b), append(from, f)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return internal(err)
		}
		if len(out.Tasks) > tasksMax {
			out.Tasks, out.More = out.Tasks[:tasksMax], true
		}
		for i := range out.Tasks {
			if err := t.taskOutcome(&out.Tasks[i], from[i]); err != nil {
				return err
			}
		}
		return nil
	})
	return out, err
}

// taskOutcome fills r's state and counts from the mail after it, up to the member's next task.
func (t *txn) taskOutcome(r *TaskRow, from string) error {
	var next sql.NullInt64
	if err := t.QueryRowContext(t.ctx, `SELECT MIN(seq) FROM messages WHERE op=? AND to_id=? AND seq>?
		AND cc_of IS NULL AND held_reason IS NULL`, OpAssign, r.MemberID, r.Seq).Scan(&next); err != nil {
		return internal(err)
	}
	end := int64(1<<62 - 1)
	if next.Valid {
		end = next.Int64
	}
	if err := t.QueryRowContext(t.ctx, `SELECT
		COUNT(CASE WHEN kind='rework' AND to_id=? THEN 1 END), COUNT(CASE WHEN kind='handback' AND from_id=? THEN 1 END)
		FROM messages WHERE seq>? AND seq<? AND cc_of IS NULL AND held_reason IS NULL AND (to_id=? OR from_id=?)`,
		r.MemberID, r.MemberID, r.Seq, end, r.MemberID, r.MemberID).Scan(&r.Reworks, &r.Handbacks); err != nil {
		return internal(err)
	}
	c, err := t.taskClosure(r.ID)
	if err != nil {
		return err
	}
	switch {
	case c != nil:
		r.State, r.ClosedSeq, r.ClosedAt = pastOp(c.Op), c.Seq, c.At
		return nil
	case next.Valid:
		r.State = TaskReplaced
		return nil
	}
	// The newest mail between the member and the assigner since the task: the member's handback
	// leaves the move to the assigner.
	r.State = TaskOpen
	var sender, kind string
	err = t.QueryRowContext(t.ctx, `SELECT from_id, COALESCE(kind,'') FROM messages WHERE seq>? AND cc_of IS NULL
		AND held_reason IS NULL AND ((from_id=? AND to_id=?) OR (from_id=? AND to_id=?)) ORDER BY seq DESC LIMIT 1`,
		r.Seq, from, r.MemberID, r.MemberID, from).Scan(&sender, &kind)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return internal(err)
	}
	if sender == r.MemberID && kind == "handback" {
		r.State = TaskHandedBack
	}
	return nil
}
