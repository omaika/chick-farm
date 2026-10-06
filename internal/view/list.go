package view

import (
	"fmt"
	"slices"
	"time"

	"github.com/sting8k/piggery/internal/core"
)

// The ids of the rows that are not a participant, and the Closed tab's key. A team's line (a live
// one in All, or a team listed as one line) is TeamRow + the team's id; the line of a team's folded
// gone members is GoneRow + its id. Neither is a team or participant id.
const (
	TeamRow   = "\x00team:"
	GoneRow   = "\x00gone:"
	TabClosed = "\x00closed"
	TabOpen   = "\x00open"  // every open team and the solos, no closed team: what the text ps lists
	tabEvery  = "\x00every" // every team, open or closed, and the solos: the Board's directories

	closedRecent = time.Hour // All shows teams closed this recently
)

// Stats is a participant's context now and turns, read from its log (the caller reads the logs).
type Stats struct {
	Ctx    int
	HasCtx bool
	Turns  int
}

// Tab is one tab: All (key ""), a team (its id), or Closed (TabClosed); Count is its rows.
type Tab struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	Count int    `json:"count"`
}

// Tabs are All, then one per team, then Closed (teams gc has not removed) last; solos are only in
// All, under their directory. With only one group besides All, All is the only one.
func Tabs(s core.State, now time.Time) []Tab {
	all := Tab{Label: "All", Count: len(s.Solos)}
	for _, t := range s.Teams {
		if Dead(t) {
			all.Count++ // one line
		} else {
			all.Count += len(t.Members)
		}
	}
	for _, c := range s.Closed {
		if Recent(c, now) {
			all.Count++
		}
	}
	out := []Tab{all}
	for _, t := range s.Teams {
		out = append(out, Tab{Key: t.ID, Label: t.Name, Count: len(t.Members)})
	}
	if len(s.Closed) > 0 {
		out = append(out, Tab{Key: TabClosed, Label: "Closed", Count: len(s.Closed)})
	}
	if len(out) < 3 { // one group besides All: All shows it all, no tab bar
		return out[:1]
	}
	return out
}

// Recent reports whether c closed within an hour of now (All lists it).
func Recent(c core.ClosedTeam, now time.Time) bool {
	return now.Sub(time.UnixMilli(c.ClosedAt)) < closedRecent
}

// Teams is every team of the snapshot, open then closed (logs, names, lookups).
func Teams(s core.State) []core.TeamState {
	out := append([]core.TeamState(nil), s.Teams...)
	for _, c := range s.Closed {
		out = append(out, c.TeamState)
	}
	return out
}

// Groups is what tab lists, by project directory (GroupByDir): All every open team, the recent
// closed ones and the solos; a team's tab that team; Closed every closed team.
func Groups(s core.State, tab string, now time.Time) []DirGroup {
	var teams []core.TeamState
	for _, t := range s.Teams {
		if tab == "" || tab == tabEvery || tab == TabOpen || tab == t.ID {
			teams = append(teams, t)
		}
	}
	var closed []core.ClosedTeam
	for _, c := range s.Closed {
		if tab == TabClosed || tab == tabEvery || tab == "" && Recent(c, now) { // TabOpen: none
			closed = append(closed, c)
		}
	}
	var solos []core.SoloState
	if tab == "" || tab == tabEvery || tab == TabOpen {
		solos = s.Solos
	}
	var roots []string
	for _, t := range Teams(s) {
		roots = append(roots, t.Root)
	}
	return GroupByDir(teams, closed, solos, roots, now)
}

// OpenByDefault: a live team lists its members; a dead or closed one is one line until opened.
func OpenByDefault(s core.State, teamID string) bool {
	for _, t := range s.Teams {
		if t.ID == teamID {
			return !Dead(t)
		}
	}
	return false
}

// IsOpen reports whether team id lists its members: the user's choice in choices, else def.
func IsOpen(choices map[string]bool, id string, def bool) bool {
	if v, ok := choices[id]; ok {
		return v
	}
	return def
}

// AnyCwd reports whether any member or solo of the snapshot, folded, gone or closed ones included and
// whatever the tab, works outside its group's directory: the rows a CWD column would show something for.
func AnyCwd(s core.State, now time.Time) bool {
	for _, g := range GroupByDir(s.Teams, s.Closed, s.Solos, rootsOf(s), now) {
		for _, u := range g.Units {
			if u.Solo != nil && RelCwd(g.Dir, u.Solo.Cwd) != "" {
				return true
			}
			if u.Team != nil {
				for _, mem := range u.Team.Members {
					if RelCwd(g.Dir, mem.Cwd) != "" {
						return true
					}
				}
			}
		}
	}
	return false
}

func rootsOf(s core.State) []string {
	var roots []string
	for _, t := range Teams(s) {
		roots = append(roots, t.Root)
	}
	return roots
}

// RowKind is what a row of the list's table stands for.
type RowKind int

const (
	KindMember RowKind = iota // a team member, in its reports_to tree
	KindSolo                  // a solo session
	KindTeam                  // a team listed as one line: all its members gone, or closed
	KindGone                  // a team's folded gone members, one line
)

func (k RowKind) MarshalText() ([]byte, error) {
	return []byte([...]string{"member", "solo", "team", "gone"}[k]), nil
}

// Actions are what can be done to a member or a solo row.
type Actions struct {
	Pick bool `json:"pick,omitempty"` // its model and thinking level can be changed
	Kill bool `json:"kill,omitempty"` // it is a headless worker that is not gone: it can be killed
	Tail bool `json:"tail,omitempty"` // piggery has a log of it to read
}

// Row is one row of the list's table: what to show of a member, a solo, a team listed as one line
// or a team's folded gone members. Empty strings are cells to leave blank ("-" or nothing: the
// caller's table decides); the state's words are in StateText, its colour is the caller's.
type Row struct {
	Kind        RowKind  `json:"kind"`
	ID          string   `json:"id"`               // opaque: unique in the snapshot; a team row's and a gone row's start with TeamRow / GoneRow
	Team        string   `json:"team,omitempty"`   // the team's id (a member, a team row, a gone row)
	Depth       int      `json:"depth"`            // a member's place in the reports_to tree, 0 at the root
	Name        string   `json:"name"`             // a team row: the team's name; a gone row: "2 members"
	Prefix      string   `json:"prefix,omitempty"` // a member: the reports_to tree's drawing (├─ └─ │)
	Role        string   `json:"role,omitempty"`
	Gate        bool     `json:"gate,omitempty"`   // a member: it is its team's gate
	Closed      string   `json:"closed,omitempty"` // a team row: "closed" for a closed team, "" for an open one that is all gone
	State       string   `json:"state"`            // as ps says it; a team or gone row: "gone"
	StateText   string   `json:"state_text"`       // icon and word for State
	Status      Status   `json:"status"`
	Dim         bool     `json:"dim,omitempty"`
	Open        bool     `json:"open,omitempty"`   // a team or gone row: its members are listed (in ps --view: by default)
	Folded      bool     `json:"folded,omitempty"` // a member of a live team that is all gone below: listed only when the team's gone line is open
	Harness     string   `json:"harness,omitempty"`
	Model       string   `json:"model,omitempty"`
	Thinking    string   `json:"thinking,omitempty"` // a member's thinking level as its session reports it, else as stored; "" unknown
	Ctx         string   `json:"ctx,omitempty"`
	Turns       string   `json:"turns,omitempty"`
	Unacked     int      `json:"unacked"`
	Age         string   `json:"age,omitempty"`
	CreatedAt   int64    `json:"created_at,omitempty"`    // ms: when it joined or was spawned
	StateSince  int64    `json:"state_since,omitempty"`   // ms: since when it is in State
	LastTurnEnd int64    `json:"last_turn_end,omitempty"` // ms: when its latest turn ended; 0 none
	Since       string   `json:"since,omitempty"`         // a member or solo: how long in State (SinceMs); a team row: when it was last active (or closed); a gone row: when the latest member went
	Cwd         string   `json:"cwd,omitempty"`           // "" when it is the directory's own
	Actions     *Actions `json:"actions,omitempty"`
	SinceAt     int64    `json:"-"`              // ms Since counts from (SinceMs); a gone row: when the latest member went; for a caller that words it itself
	Tabs        []string `json:"tabs,omitempty"` // the tabs that list it (ps --view)
}

// TeamHead is a live team's line (in All) or title (in its own tab): its name, what is wrong with it
// (Flags), and when it is folded the counts of its members by state.
type TeamHead struct {
	ID     string   `json:"id"`
	Detail string   `json:"detail"` // its key in Doc.Details
	Name   string   `json:"name"`
	Line   bool     `json:"line"` // the line of All: selectable (TeamRow + ID), and it folds the team
	Open   bool     `json:"open"`
	Flags  []string `json:"flags,omitempty"`  // for the title, amber: "no gate", "3 held"
	Counts []string `json:"counts,omitempty"` // folded: "2 working", "1 idle", "1 waiting", "3 gone", "unacked 4"; "no members"
	Tabs   []string `json:"tabs,omitempty"`   // the tabs that list it (ps --view)
}

// Block is one unit of a directory: a solo (one row), a team listed as one line (its row, then its
// members when open), or a live team (its head, then its members and its gone line).
type Block struct {
	Head      *TeamHead `json:"head,omitempty"`
	Rows      []Row     `json:"rows"`                 // as listed now, in order
	NoMembers string    `json:"no_members,omitempty"` // an open team without members: what to do
	// Sizing are the rows of every member of the team, those a fold hides included: the columns'
	// widths come from them, so opening or closing a fold never moves a column.
	Sizing []Row `json:"-"`
}

// Dir is a project directory and its blocks.
type Dir struct {
	Path   string  `json:"path"`
	Label  string  `json:"label"`  // the short path, ending in "/"
	Bucket string  `json:"bucket"` // where it sorts: live, sleeping, gone (every unit all gone) or closed (only closed teams)
	Folded bool    `json:"folded"` // a client that folds starts it folded: gone and closed directories
	Blocks []Block `json:"blocks"`
}

// List is what a tab lists.
type List struct {
	Dirs   []Dir    `json:"dirs"`
	Items  []string `json:"-"`               // the selectable ids, in display order
	AnyCwd bool     `json:"any_cwd"`         // some row of the snapshot, whatever the tab, has a cwd of its own: show the column
	Empty  string   `json:"empty,omitempty"` // what to say when there is nothing to list
}

// ListInput is what List needs: the snapshot and the tab; the folds the user chose (absent: the
// default) and the stats of the participants whose logs were read.
type ListInput struct {
	State core.State
	Tab   string
	Now   time.Time
	Open  map[string]bool // a team id -> its members are listed (true) or it is one line (false)
	Gone  map[string]bool // a team id -> its gone members are listed, not folded into one line
	Stats map[string]Stats
	// Readable says whether piggery can read a session's transcript (a tail of it); nil: none can.
	Readable func(*core.Transcript) bool
}

// BuildList is the rows of tab, by project directory (Groups): the directory, then its units oldest
// first (a team's head and its members' tree; a closed team's row, its members when expanded; a
// solo's row), and the selectable ids in that order.
func BuildList(in ListInput) List {
	s, now := in.State, in.Now
	groups := Groups(s, in.Tab, now)
	all := in.Tab == "" || in.Tab == tabEvery // the All tab's rules
	dirs := make([]string, len(groups))
	for i, g := range groups {
		dirs[i] = g.Dir
	}
	short := ShortPaths(dirs)
	out := List{AnyCwd: AnyCwd(s, now)}
	// usage is id's ctx and turns, "-" when no log of it was read.
	usage := func(id string) (ctx, turns string) {
		ctx, turns = "-", "-"
		if ws, ok := in.Stats[id]; ok {
			if ws.HasCtx {
				ctx = Tokens(ws.Ctx)
			}
			turns = fmt.Sprint(ws.Turns)
		}
		return ctx, turns
	}
	memberRow := func(g DirGroup, t *core.TeamState, tr TreeRow, closed bool) Row {
		mem := tr.M
		ctx, turns := usage(mem.ID)
		at := SinceMs(mem.State, mem.StateSince, mem.LastTurnEnd)
		return Row{Kind: KindMember, ID: mem.ID, Team: t.ID, Depth: tr.Depth, Name: mem.Name, Prefix: tr.Prefix, Role: mem.Role, Gate: mem.Gate,
			State: mem.State, StateText: StateText(mem.State), Status: StatusOf(mem.State), Dim: closed || mem.State == "gone" || tr.ParentGone,
			Harness: HarnessLabel(mem.Harness, mem.Headless), Model: ModelID(mem.Model), Thinking: mem.Thinking, Ctx: ctx, Turns: turns, Unacked: mem.Unacked,
			Age: Ago(mem.CreatedAt, now), Since: Ago(at, now), SinceAt: at, Cwd: RelCwd(g.Dir, mem.Cwd),
			CreatedAt: mem.CreatedAt, StateSince: mem.StateSince, LastTurnEnd: mem.LastTurnEnd,
			Actions: &Actions{Pick: Pickable(mem, t.ID, s.Closed), Kill: KillNote(mem.Name, mem.Headless, mem.State) == "", Tail: MemberTailable(mem, in.Readable)}}
	}
	goneRow := func(t *core.TeamState, gone []core.MemberState, open bool) Row {
		var last int64
		for _, g := range gone {
			last = max(last, g.StateSince)
		}
		return Row{Kind: KindGone, ID: GoneRow + t.ID, Team: t.ID, Name: Plural(len(gone), "member"), State: "gone", StateText: StateText("gone"), Status: StatusOf("gone"),
			Dim: true, Open: open, Since: Ago(last, now), SinceAt: last}
	}
	for gi, g := range groups {
		d := Dir{Path: g.Dir, Label: DirLabel(short[gi]), Bucket: bucketNames[g.Bucket], Folded: g.Bucket >= bucketGone}
		for _, u := range g.Units {
			var b Block
			switch {
			case u.Solo != nil:
				sl := u.Solo
				ctx, turns := usage(sl.ID)
				at := SinceMs(sl.State, sl.StateSince, sl.LastTurnEnd)
				b.Rows = []Row{{Kind: KindSolo, ID: sl.ID, Name: sl.Name, State: sl.State, StateText: StateText(sl.State), Status: StatusOf(sl.State),
					Actions: &Actions{Tail: SoloTailable(*sl, in.Readable)},
					Harness: HarnessLabel(sl.Harness, false), Model: ModelID(sl.Model), Ctx: ctx, Turns: turns, Unacked: sl.Unacked,
					Age: Ago(sl.CreatedAt, now), Since: Ago(at, now), SinceAt: at, Cwd: RelCwd(g.Dir, sl.Cwd),
					CreatedAt: sl.CreatedAt, StateSince: sl.StateSince, LastTurnEnd: sl.LastTurnEnd}}
			case u.Closed != nil || all && Dead(*u.Team): // one line: a closed team, or in All a dead open one
				t := u.Team
				open := IsOpen(in.Open, t.ID, false)
				at, word := u.Active(), ""
				if c := u.Closed; c != nil {
					at, word = c.ClosedAt, "closed"
				}
				b.Rows = []Row{{Kind: KindTeam, ID: TeamRow + t.ID, Team: t.ID, Name: t.Name, Closed: word, State: "gone", StateText: StateText("gone"), Status: StatusOf("gone"),
					Dim: true, Open: open, Since: Ago(at, now)}}
				if open {
					for _, tr := range MemberTree(t.Members) {
						b.Rows = append(b.Rows, memberRow(g, t, tr, u.Closed != nil))
					}
				}
			default:
				t := u.Team
				open := !all || IsOpen(in.Open, t.ID, true)
				b.Head = &TeamHead{ID: t.ID, Detail: TeamRow + t.ID, Name: t.Name, Line: all, Open: open}
				if t.Gate == "" {
					b.Head.Flags = append(b.Head.Flags, "no gate")
				}
				if t.Held > 0 {
					b.Head.Flags = append(b.Head.Flags, fmt.Sprintf("%d held", t.Held))
				}
				if n := len(OpenConflicts(*t)); n == 1 {
					b.Head.Flags = append(b.Head.Flags, "1 merge conflict")
				} else if n > 1 {
					b.Head.Flags = append(b.Head.Flags, fmt.Sprintf("%d merge conflicts", n))
				}
				if !open {
					b.Head.Counts = teamCounts(*t)
				}
				if open && len(t.Members) == 0 {
					b.NoMembers = "No members. Open an agent session in " + Home(t.Root) + " and ask the gate to admit it."
				}
				if open {
					kept, gone := FoldGone(t.Members)
					switch {
					case len(gone) == 0 || Dead(*t) && in.Tab != TabOpen: // a dead team lists every member: its line is the fold (the text ps lists it by its gone line)
						for _, tr := range MemberTree(t.Members) {
							b.Rows = append(b.Rows, memberRow(g, t, tr, false))
						}
					case in.Gone[t.ID]:
						for _, tr := range MemberTree(t.Members) {
							b.Rows = append(b.Rows, memberRow(g, t, tr, false))
						}
						b.Rows = append(b.Rows, goneRow(t, gone, true))
					default:
						for _, tr := range MemberTree(kept) {
							b.Rows = append(b.Rows, memberRow(g, t, tr, false))
						}
						b.Rows = append(b.Rows, goneRow(t, gone, false))
					}
					if !Dead(*t) { // the gone ones with nobody live below them: listed only under the open gone line
						for i, r := range b.Rows {
							if r.Kind == KindMember && slices.ContainsFunc(gone, func(m core.MemberState) bool { return m.ID == r.ID }) {
								b.Rows[i].Folded = true
							}
						}
					}
				}
			}
			if u.Team != nil {
				for _, tr := range MemberTree(u.Team.Members) {
					b.Sizing = append(b.Sizing, memberRow(g, u.Team, tr, false))
				}
				if _, gone := FoldGone(u.Team.Members); len(gone) > 0 && !Dead(*u.Team) {
					b.Sizing = append(b.Sizing, goneRow(u.Team, gone, false))
				}
			}
			if b.Head != nil && b.Head.Line {
				out.Items = append(out.Items, TeamRow+b.Head.ID)
			}
			for _, r := range b.Rows {
				out.Items = append(out.Items, r.ID)
			}
			d.Blocks = append(d.Blocks, b)
		}
		out.Dirs = append(out.Dirs, d)
	}
	if len(groups) == 0 {
		switch in.Tab {
		case "":
			out.Empty = "No teams. In pi: “found a team here”"
		case TabClosed:
			out.Empty = "No closed teams."
		}
	}
	return out
}

// teamCounts are a team's members by state and its unacked mail, for its folded line: only the
// counts above zero (held is in the title), or "no members".
func teamCounts(t core.TeamState) []string {
	var working, idle, other, gone int
	for _, mem := range t.Members {
		switch mem.State {
		case "working":
			working++
		case "idle":
			idle++
		case "gone":
			gone++
		default:
			other++
		}
	}
	var parts []string
	for _, c := range []struct {
		n    int
		what string
	}{{working, "%d working"}, {idle, "%d idle"}, {other, "%d waiting"}, {gone, "%d gone"}, {t.Unacked, "unacked %d"}} {
		if c.n > 0 {
			parts = append(parts, fmt.Sprintf(c.what, c.n))
		}
	}
	if len(parts) == 0 {
		parts = []string{"no members"}
	}
	return parts
}
