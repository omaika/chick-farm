package core

import (
	"context"
	"strings"
)

// MailArgs is `mail` (admin, read-only): the messages a participant sent or received, or a team's,
// or every one, newest first; with Pins, a team's live board pins, oldest first, as its members
// read the board. Reading acks nothing: a participant's inbox is its own.
type MailArgs struct {
	Participant string `json:"participant,omitempty"` // its mail, sent and received (id, or name)
	Team        string `json:"team,omitempty"`        // a team (id, or name): the messages in it
	Before      int64  `json:"before,omitempty"`      // only messages older than this #seq (the next page)
	Limit       int    `json:"limit,omitempty"`       // default mailLimit, at most mailMax
	Pins        bool   `json:"pins,omitempty"`        // only the team's live pins, oldest first (needs Team; no paging)
}

const (
	mailLimit = 50
	mailMax   = 200
)

// MailRow is one message as a person reads it: names instead of ids, its #seq, and where it stands.
type MailRow struct {
	Seq       int64  `json:"seq"`
	ID        string `json:"id"`
	Team      string `json:"team,omitempty"` // the team's name
	FromID    string `json:"from_id"`
	From      string `json:"from"`
	ToID      string `json:"to_id"`
	To        string `json:"to"` // a participant's name, or "board"
	Kind      string `json:"kind,omitempty"`
	ReplyTo   int64  `json:"reply_to,omitempty"` // the #seq it answers
	Op        string `json:"op,omitempty"`       // assign, accept, drop (a member's task); replace, remove (a board pin)
	Body      string `json:"body"`
	CreatedAt int64  `json:"created_at"`
	AckedAt   int64  `json:"acked_at,omitempty"`
	Held      string `json:"held,omitempty"` // why a limit holds it (`piggery release`)
	// State is held (a limit keeps it), pending (not delivered yet), delivered (in a turn, not
	// acked), acked, or board (a pin).
	State string `json:"state"`
}

type MailResult struct {
	Messages []MailRow `json:"messages"`
	More     bool      `json:"more"` // older messages match: ask again with before = the last seq
}

// Mail lists messages for the operator (MailArgs).
func (e *Engine) Mail(ctx context.Context, a MailArgs) (MailResult, error) {
	limit := a.Limit
	if limit <= 0 {
		limit = mailLimit
	}
	limit = min(limit, mailMax)
	var where []string
	var args []any
	order := "DESC"
	if a.Pins {
		if a.Team == "" || a.Participant != "" || a.Before != 0 {
			return MailResult{}, errf(CodeInvalid, "pins needs a team, and takes no participant or before")
		}
		where, order, limit = append(where, "m.id IN (SELECT id FROM pins)"), "ASC", BoardLimit
	}
	if a.Participant != "" {
		where = append(where, "(m.from_id=? OR m.to_id=? OR pf.name=? OR pt.name=?)")
		args = append(args, a.Participant, a.Participant, a.Participant, a.Participant)
	}
	if a.Team != "" {
		where, args = append(where, "(m.team_id=? OR t.name=?)"), append(args, a.Team, a.Team)
	}
	if a.Before > 0 {
		where, args = append(where, "m.seq<?"), append(args, a.Before)
	}
	q := `SELECT m.seq, m.id, COALESCE(t.name,''), m.from_id, COALESCE(pf.name, m.from_id), m.to_id,
		COALESCE(pt.name, m.to_id), COALESCE(m.kind,''), COALESCE(r.seq,0), COALESCE(m.op,''), m.body, m.created_at,
		COALESCE(m.acked_at,0), COALESCE(m.held_reason,''), EXISTS (SELECT 1 FROM deliveries d WHERE d.message_id=m.id)
		FROM messages m
		LEFT JOIN teams t ON t.id=m.team_id
		LEFT JOIN participants pf ON pf.id=m.from_id
		LEFT JOIN participants pt ON pt.id=m.to_id
		LEFT JOIN messages r ON r.id=m.reply_to`
	if len(where) > 0 {
		q += " WHERE " + strings.Join(where, " AND ")
	}
	q += " ORDER BY m.seq " + order + " LIMIT ?"
	args = append(args, limit+1)
	out := MailResult{Messages: []MailRow{}}
	err := e.readOnly(ctx, func(t *txn) error {
		rows, err := t.QueryContext(t.ctx, q, args...)
		if err != nil {
			return internal(err)
		}
		defer rows.Close()
		for rows.Next() {
			var m MailRow
			var delivered bool
			if err := rows.Scan(&m.Seq, &m.ID, &m.Team, &m.FromID, &m.From, &m.ToID, &m.To, &m.Kind, &m.ReplyTo, &m.Op,
				&m.Body, &m.CreatedAt, &m.AckedAt, &m.Held, &delivered); err != nil {
				return internal(err)
			}
			m.State = mailState(m, delivered)
			out.Messages = append(out.Messages, m)
		}
		return internal(rows.Err())
	})
	if len(out.Messages) > limit {
		out.Messages, out.More = out.Messages[:limit], true
	}
	return out, err
}

func mailState(m MailRow, delivered bool) string {
	switch {
	case m.ToID == "board":
		return "board"
	case m.AckedAt > 0:
		return "acked"
	case delivered:
		return "delivered"
	case m.Held != "":
		return "held"
	}
	return "pending"
}
