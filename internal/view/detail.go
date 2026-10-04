package view

import (
	"cmp"
	"fmt"
	"time"

	"github.com/sting8k/piggery/internal/core"
)

// DetailKind is what the selected row is.
type DetailKind int

const (
	DetailNone        DetailKind = iota // nothing in the snapshot has that id
	DetailTeam                          // a team's line, or its folded gone members' line
	DetailParticipant                   // a member or a solo
)

func (k DetailKind) MarshalText() ([]byte, error) {
	return []byte([...]string{"none", "team", "participant"}[k]), nil
}

// Detail is what the Overview says of the selected row: a title, its facts as label and value, and
// for a team top's hint of what its key does (Hint, which ps --view does not carry). The caller draws it.
type Detail struct {
	Kind    DetailKind `json:"kind"`
	ID      string     `json:"id,omitempty"`    // a participant's id
	Title   string     `json:"title"`           // drawn bold
	Sub     string     `json:"sub,omitempty"`   // after the title, muted: a member's role, "open", "closed"...
	State   string     `json:"state,omitempty"` // a participant's state: drawn as a pill of StateText, coloured by it
	Task    *Task      `json:"task,omitempty"`  // a member's current assignment
	Facts   []Fact     `json:"facts"`
	Hint    string     `json:"-"` // what top's key does here, muted, last: top's own words, not in ps --view
	HintGap bool       `json:"-"` // a blank line before Hint
}

// Fact is a labelled line. Note follows Value, muted; Muted draws Value muted; Pick marks the model
// of a worker whose model can be changed here.
type Fact struct {
	Label string `json:"label"`
	Value string `json:"value"`
	Note  string `json:"note,omitempty"`
	Muted bool   `json:"muted,omitempty"`
	Pick  bool   `json:"pick,omitempty"`
}

// Task is a member's current assignment as words: its title (with the mail's number), who gave
// it and when, how its reply chain stands, and a newer mail from the assigner outside the chain.
type Task struct {
	Title string `json:"title"`           // "#175 the first line of the body"
	From  string `json:"from"`            // "from alice · 22m ago"
	Chain string `json:"chain,omitempty"` // "handed back #183 · 3m ago", "reply #12 · 1m ago"; "" when the assignment is the newest
	Mail  *Mail  `json:"mail,omitempty"`
}

// Mail is the newer mail of a Task: "#190 " Title " · 1m ago".
type Mail struct {
	Head  string `json:"head"`
	Title string `json:"title"`
	Tail  string `json:"tail"`
}

// Describe is the Overview of the row sel: a team's line (live, dead, closed or its gone members'
// line), or a member or a solo; none when no row of the snapshot has that id. stats are the logs
// read so far, by participant id.
func Describe(s core.State, sel string, stats map[string]Stats, now time.Time) Detail {
	for i := range s.Teams {
		t := &s.Teams[i]
		switch {
		case sel == GoneRow+t.ID: // the line of its folded gone members
			_, gone := FoldGone(t.Members)
			d := Detail{Kind: DetailTeam, Title: fmt.Sprint(len(gone), " gone"), Sub: t.Name, Hint: "enter lists them in the tree", HintGap: true}
			for _, g := range gone {
				d.Facts = append(d.Facts, Fact{Label: g.Role, Value: g.Name, Note: Ago(g.StateSince, now) + " ago"})
			}
			return d
		case sel == TeamRow+t.ID && !Dead(*t): // a live team's line: the team's facts
			var working, idle, gone int
			for _, mem := range t.Members {
				switch mem.State {
				case "working":
					working++
				case "idle":
					idle++
				case "gone":
					gone++
				}
			}
			d := Detail{Kind: DetailTeam, Title: t.Name, Sub: "open", Hint: "enter folds or opens its members",
				Facts: []Fact{{Label: "members", Value: fmt.Sprint(len(t.Members))}}}
			for _, c := range []struct {
				what string
				n    int
			}{{"working", working}, {"idle", idle}, {"gone", gone}} {
				if c.n > 0 {
					d.Facts = append(d.Facts, Fact{Label: c.what, Value: fmt.Sprint(c.n)})
				}
			}
			d.Facts = append(d.Facts, Fact{Label: "gate", Value: OrDash(t.Gate)}, Fact{Label: "held", Value: fmt.Sprint(t.Held)},
				Fact{Label: "unacked", Value: fmt.Sprint(t.Unacked)}, Fact{Label: "root", Value: Home(t.Root)})
			d.Facts = append(d.Facts, mergeFacts(*t, now)...)
			return d
		case sel == TeamRow+t.ID: // a dead team's line: the team's facts
			return Detail{Kind: DetailTeam, Title: t.Name, Sub: "open, all gone", Hint: "enter shows its members", Facts: []Fact{
				{Label: "active", Value: Ago(Unit{Team: t}.Active(), now) + " ago"}, {Label: "gate", Value: OrDash(t.Gate)},
				{Label: "members", Value: fmt.Sprint(len(t.Members))}, {Label: "root", Value: Home(t.Root)}}}
		}
	}
	teams := make([]*core.TeamState, 0, len(s.Teams)+len(s.Closed))
	for i := range s.Teams {
		teams = append(teams, &s.Teams[i])
	}
	for i := range s.Closed {
		c := &s.Closed[i]
		if sel == TeamRow+c.ID { // a closed team's line: the team's facts
			return Detail{Kind: DetailTeam, Title: c.Name, Sub: "closed", Hint: "enter shows its members", Facts: []Fact{
				{Label: "closed", Value: Ago(c.ClosedAt, now) + " ago"}, {Label: "by", Value: cmp.Or(c.ClosedBy, "admin")}, {Label: "gate", Value: OrDash(c.Gate)},
				{Label: "members", Value: fmt.Sprint(len(c.Members))}, {Label: "root", Value: Home(c.Root)}}}
		}
		teams = append(teams, &c.TeamState)
	}
	var team *core.TeamState
	var mem *core.MemberState
	var solo *core.SoloState
	for _, t := range teams {
		for j := range t.Members {
			if t.Members[j].ID == sel {
				team, mem = t, &t.Members[j]
			}
		}
	}
	for i := range s.Solos {
		if s.Solos[i].ID == sel {
			solo = &s.Solos[i]
		}
	}
	if mem == nil && solo == nil {
		return Detail{}
	}

	names := Names(s)
	var name, role, state, ref string
	var since, lastTurn, created int64
	var unacked int
	if mem != nil {
		name, role, state, since, unacked, ref = mem.Name, mem.Role, mem.State, mem.StateSince, mem.Unacked, mem.ID
		created, lastTurn = mem.CreatedAt, mem.LastTurnEnd
	} else {
		name, role, state, since, unacked, ref = solo.Name, "solo", solo.State, solo.StateSince, solo.Unacked, solo.ID
		created, lastTurn = solo.CreatedAt, solo.LastTurnEnd
	}
	cameKey, came := "joined", time.UnixMilli(created).Format("15:04") // a session opened by the Human
	if mem != nil && mem.Headless && mem.SpawnedBy != "" {
		cameKey, came = "spawned", came+" by "+OrDash(names[mem.SpawnedBy])
	}
	d := Detail{Kind: DetailParticipant, ID: ref, Title: name, Sub: role, State: state}
	if mem != nil {
		d.Task = taskOf(mem.Assignment, now)
		kind := "session"
		if mem.Headless {
			kind = "headless worker"
		}
		if mem.Harness != "" {
			kind = mem.Harness + " " + kind
		}
		if mem.Gate {
			kind += " · gate"
		}
		d.Facts = append(d.Facts, Fact{Label: "team", Value: team.Name}, Fact{Label: "kind", Value: kind},
			Fact{Label: "model", Value: ModelLabel(mem.Model, mem.Thinking), Pick: Pickable(*mem, team.ID, s.Closed)})
	}
	if ws, ok := stats[ref]; ok {
		ctx := "-"
		if ws.HasCtx {
			ctx = Tokens(ws.Ctx)
		}
		d.Facts = append(d.Facts, Fact{Label: "ctx", Value: ctx}, Fact{Label: "turns", Value: fmt.Sprint(ws.Turns)})
	}
	d.Facts = append(d.Facts, Fact{Label: cameKey, Value: came}, Fact{Label: "since", Value: Ago(SinceMs(state, since, lastTurn), now)}, Fact{Label: "unacked", Value: fmt.Sprint(unacked)})
	if mem != nil {
		if mem.ReportsTo != "" {
			d.Facts = append(d.Facts, Fact{Label: "reports", Value: OrDash(names[mem.ReportsTo])})
		}
		if mem.LastTurnEnd > 0 {
			d.Facts = append(d.Facts, Fact{Label: "last turn", Value: Ago(mem.LastTurnEnd, now) + " ago"})
		}
		d.Facts = append(d.Facts, Fact{Label: "root", Value: Home(team.Root)})
	} else {
		d.Facts = append(d.Facts, Fact{Label: "model", Value: ModelLabel(solo.Model, "")}, Fact{Label: "cwd", Value: Home(solo.Cwd)})
	}
	d.Facts = append(d.Facts, Fact{Label: "id", Value: ref, Muted: true})
	return d
}

// taskOf is a member's assignment as words; nil when it has none.
func taskOf(a *core.Assignment, now time.Time) *Task {
	if a == nil {
		return nil
	}
	t := &Task{Title: fmt.Sprintf("#%d %s", a.Seq, a.Title), From: "from " + a.From + " · " + Ago(a.At, now) + " ago"}
	if l := a.Latest; l != nil {
		what := "reply"
		if l.ByMember {
			what = "handed back"
		}
		t.Chain = fmt.Sprintf("%s #%d · %s ago", what, l.Seq, Ago(l.At, now))
	}
	if x := a.Newer; x != nil {
		t.Mail = &Mail{Head: fmt.Sprintf("#%d ", x.Seq), Title: x.Title, Tail: " · " + Ago(x.At, now) + " ago"}
	}
	return t
}

// mergeFacts are a team's merge records (agent action=merge): each branch's latest, open
// conflicts first, as "branch → into: status, by, when".
func mergeFacts(t core.TeamState, now time.Time) []Fact {
	var out []Fact
	for _, m := range t.Merges {
		what := m.Branch
		if m.Into != "" {
			what += " → " + m.Into
		}
		v := fmt.Sprintf("%s: %s by %s, %s ago", what, m.Status, OrDash(m.By), Ago(m.At, now))
		if m.Note != "" {
			v += " (" + m.Note + ")"
		}
		label := "merge"
		if m.Status == core.MergeConflict {
			label = "conflict"
		}
		out = append(out, Fact{Label: label, Value: v})
	}
	return out
}
