package core

import (
	"context"
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
		var n int
		if err := t.QueryRowContext(t.ctx, `SELECT COUNT(*) FROM pins WHERE id=? AND team_id=?`,
			a.Target, p.team).Scan(&n); err != nil {
			return internal(err)
		}
		if n == 0 {
			return deny(&p, "pin", "target", "board.target_not_live", "target is not a live pin of this team", nil,
				map[string]any{"target": a.Target, "op": a.Op})
		}
	default:
		return errf(CodeInvalid, "unknown board op %q", a.Op)
	}
	return nil
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
