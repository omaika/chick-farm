package core

import (
	"context"
	"database/sql"
	"errors"
	"strings"
)

// Board returns the live pins of the caller's team, oldest first.
func (e *Engine) Board(ctx context.Context, c Caller) ([]Message, error) {
	var pins []Message
	err := e.inTx(ctx, func(t *txn) error {
		p, err := t.callerGranted(c, "board", "inbox")
		if err != nil {
			return err
		}
		pins, err = t.livePins(p)
		return err
	})
	return pins, err
}

// gateBoard checks a board send: can_pin, then the op's target or the board limit.
func (t *txn) gateBoard(p participant, m manifest, a SendArgs) error {
	if !m.Roles[p.role].CanPin {
		return deny(&p, "pin", "permission", "can_pin", "role "+p.role+" cannot pin", nil, nil)
	}
	switch a.Op {
	case "":
		if a.Target != "" {
			return errf(CodeInvalid, "target requires op replace or remove")
		}
		if a.Body == "" {
			return errf(CodeInvalid, "body is required")
		}
		pins, err := t.livePins(p)
		if err != nil {
			return err
		}
		if len(pins) >= BoardLimit {
			return deny(&p, "pin", "limit", "board.full", "board is full", pins, map[string]any{"live": len(pins)})
		}
	case "replace", "remove":
		if a.Op == "replace" && a.Body == "" {
			return errf(CodeInvalid, "body is required")
		}
		var author string
		err := t.QueryRowContext(t.ctx, `SELECT from_id FROM pins WHERE id=? AND team_id=?`, a.Target, p.team).Scan(&author)
		if errors.Is(err, sql.ErrNoRows) {
			return deny(&p, "pin", "target", "board.target_not_live", "target is not a live pin of this team", nil,
				map[string]any{"target": a.Target, "op": a.Op})
		}
		if err != nil {
			return internal(err)
		}
		mine, err := t.mayEditPin(p, author)
		if err != nil {
			return err
		}
		if !mine {
			return deny(&p, "pin", "permission", "board.not_yours",
				"only the pin's author, a member it reports to (directly or not) or the team's gate can "+a.Op+" it", nil,
				map[string]any{"target": a.Target, "op": a.Op})
		}
	default:
		return errf(CodeInvalid, "unknown board op %q", a.Op)
	}
	return nil
}

// mayEditPin reports whether p may replace or remove a pin by author: p is the author, is above it
// in its reports_to chain, or is the team's gate. A pin whose author is gone (or was an admin) is
// anyone's to tidy.
func (t *txn) mayEditPin(p participant, author string) (bool, error) {
	if author == p.id {
		return true, nil
	}
	a, ok, err := t.participantByID(author)
	if err != nil || !ok || a.state == "gone" {
		return err == nil, err
	}
	for hops, up := 0, a.reportsTo; up != "" && hops < 64; hops++ { // 64: a guard, not a limit
		if up == p.id {
			return true, nil
		}
		q, ok, err := t.participantByID(up)
		if err != nil || !ok {
			return false, err
		}
		up = q.reportsTo
	}
	gate, ok, err := t.teamGate(p.team)
	return ok && gate.id == p.id, err
}

// livePins returns the live pins of p's team, oldest first.
func (t *txn) livePins(p participant) ([]Message, error) {
	pins, err := t.readMessages(p, `id IN (SELECT id FROM pins WHERE team_id=?)`, p.team)
	if pins == nil && err == nil {
		pins = []Message{}
	}
	return pins, err
}

// teamPins are the team's live pins as top shows them, oldest first.
func (t *txn) teamPins(team string) ([]PinState, error) {
	rows, err := t.QueryContext(t.ctx, `SELECT m.seq, COALESCE(p.name, 'admin'), m.created_at, m.body FROM pins x
		JOIN messages m ON m.id=x.id LEFT JOIN participants p ON p.id=m.from_id WHERE x.team_id=? ORDER BY m.seq`, team)
	if err != nil {
		return nil, internal(err)
	}
	defer rows.Close()
	var out []PinState
	for rows.Next() {
		var ps PinState
		var body string
		if err := rows.Scan(&ps.Seq, &ps.By, &ps.At, &body); err != nil {
			return nil, internal(err)
		}
		ps.Title, ps.Lines = assignmentTitle(body), strings.Count(strings.TrimSpace(body), "\n")+1
		out = append(out, ps)
	}
	return out, internal(rows.Err())
}
