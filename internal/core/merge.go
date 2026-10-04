package core

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
)

// Merges: whoever integrates work (a Lead merging a Peer's branch, a Supervisor merging a lane)
// does the merge itself, with git; `agent action=merge` records what happened so it is state and
// not only prose in a prompt: one event per record (merged, merge_conflict, merge_resolved,
// merge_aborted), and ps/top show each branch's latest record per team, an open conflict loudest.
// It wakes nobody and checks nothing in the repository.

// The statuses a merge record takes, each its event type.
const (
	MergeMerged   = "merged"
	MergeConflict = "conflict"
	MergeResolved = "resolved"
	MergeAborted  = "aborted"
)

var mergeEvents = map[string]string{MergeMerged: "merged", MergeConflict: "merge_conflict",
	MergeResolved: "merge_resolved", MergeAborted: "merge_aborted"}

// maxMergeNote caps a record's note: one line for top, not a report.
const maxMergeNote = 200

// MergeState is a branch's latest merge record in a team.
type MergeState struct {
	Branch string `json:"branch"`
	Into   string `json:"into,omitempty"`
	Status string `json:"status"` // merged | conflict | resolved | aborted
	By     string `json:"by"`     // who recorded it
	At     int64  `json:"at"`
	Note   string `json:"note,omitempty"`
	seq    int64  // its event's: what newest means
}

type mergePayload struct {
	Branch string `json:"branch"`
	Into   string `json:"into,omitempty"`
	Status string `json:"status"`
	Note   string `json:"note,omitempty"`
}

// recordMerge is `agent action=merge`: a member with the agent tool records a merge of a.Branch
// into a.Into (optional) with a.Status and an optional one-line a.Note.
func (e *Engine) recordMerge(ctx context.Context, c Caller, a AgentArgs) (AgentResult, error) {
	branch, into := strings.TrimSpace(a.Branch), strings.TrimSpace(a.Into)
	if _, ok := mergeEvents[a.Status]; !ok || branch == "" {
		return AgentResult{}, errf(CodeInvalid, "merge needs branch and status (%s, %s, %s or %s)",
			MergeMerged, MergeConflict, MergeResolved, MergeAborted)
	}
	note := firstLine(a.Note)
	if r := []rune(note); len(r) > maxMergeNote {
		note = string(r[:maxMergeNote-1]) + "…"
	}
	var res AgentResult
	err := e.inTx(ctx, func(t *txn) error {
		p, err := t.callerGranted(c, "agent.merge", "agent")
		if err != nil {
			return err
		}
		if p.team == "" {
			return errf(CodeInvalid, "merge records a team's merges: you are in no team")
		}
		res.Text = fmt.Sprintf("recorded: %s %s", branch, a.Status)
		if into != "" {
			res.Text = fmt.Sprintf("recorded: %s into %s %s", branch, into, a.Status)
		}
		return t.event(evt{typ: mergeEvents[a.Status], participant: p.id, team: p.team, run: p.run,
			payload: mergePayload{Branch: branch, Into: into, Status: a.Status, Note: note}})
	})
	return res, err
}

// teamMerges returns each (branch, into) pair's latest merge record in the team, open conflicts
// first, then newest first.
func (t *txn) teamMerges(team string) ([]MergeState, error) {
	rows, err := t.QueryContext(t.ctx, `SELECT e.seq, e.ts, e.payload, COALESCE(p.name,'') FROM events e
		LEFT JOIN participants p ON p.id=e.participant
		WHERE e.team_id=? AND e.type IN ('merged','merge_conflict','merge_resolved','merge_aborted') ORDER BY e.seq`, team)
	if err != nil {
		return nil, internal(err)
	}
	defer rows.Close()
	type pair struct{ branch, into string }
	latest := map[pair]int{}
	var out []MergeState
	for rows.Next() {
		var ms MergeState
		var payload string
		if err := rows.Scan(&ms.seq, &ms.At, &payload, &ms.By); err != nil {
			return nil, internal(err)
		}
		var mp mergePayload
		if json.Unmarshal([]byte(payload), &mp) != nil {
			continue
		}
		ms.Branch, ms.Into, ms.Status, ms.Note = mp.Branch, mp.Into, mp.Status, mp.Note
		k := pair{mp.Branch, mp.Into}
		if i, ok := latest[k]; ok {
			out[i] = ms
			continue
		}
		latest[k] = len(out)
		out = append(out, ms)
	}
	if err := rows.Err(); err != nil {
		return nil, internal(err)
	}
	sortMerges(out)
	return out, nil
}

// sortMerges puts open conflicts first, then the newest.
func sortMerges(ms []MergeState) {
	open := func(m MergeState) bool { return m.Status == MergeConflict }
	slices.SortStableFunc(ms, func(a, b MergeState) int {
		if open(a) != open(b) {
			if open(a) {
				return -1
			}
			return 1
		}
		return cmp.Compare(b.seq, a.seq)
	})
}
