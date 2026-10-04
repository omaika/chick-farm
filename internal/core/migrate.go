package core

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
)

// TeamMigrateArgs is `team migrate`: an open team takes another manifest. Map moves a role the new
// manifest lacks to one it has (old role -> new role).
type TeamMigrateArgs struct {
	Team     string            `json:"team"`     // team id or open-team name
	Manifest string            `json:"manifest"` // YAML text, self-contained, as team up takes it
	Map      map[string]string `json:"map,omitempty"`
}

type TeamMigrateResult struct {
	TeamID   string     `json:"team_id"`
	Name     string     `json:"name"`
	From     string     `json:"from"` // the template it ran
	To       string     `json:"to"`
	Moves    []RoleMove `json:"moves"` // every member, in the order they entered
	Warnings []string   `json:"warnings,omitempty"`
}

// RoleMove is a member's role before and after a migrate.
type RoleMove struct {
	Name string `json:"name"`
	From string `json:"from"`
	To   string `json:"to"`
}

// TeamMigrate replaces an open team's manifest, keeping its members, mail, board and workers. Each
// member that has not left gets a role in the new manifest: the gate takes the new template's
// session role (auto_join_role, or its only role), as found gives the founder; any other member
// the one Map names for its role, else a role of the same name. A member left without one refuses the whole migrate. Every live member is told by
// mail from engine, with its new role card (a session's system prompt keeps the old one); a worker
// started again later gets the new card at its start. What the new routing does not allow between a
// member and the one it reports to is a warning.
func (e *Engine) TeamMigrate(ctx context.Context, a TeamMigrateArgs) (TeamMigrateResult, error) {
	m, warnings, err := loadManifest(a.Manifest)
	if err != nil {
		return TeamMigrateResult{}, err
	}
	for _, old := range slices.Sorted(maps.Keys(a.Map)) {
		if _, ok := m.Roles[a.Map[old]]; !ok {
			return TeamMigrateResult{}, errf(CodeInvalid, "map %s=%s: template %s has no role %q (its roles: %s)",
				old, a.Map[old], m.Template, a.Map[old], strings.Join(slices.Sorted(maps.Keys(m.Roles)), ", "))
		}
	}
	res := TeamMigrateResult{To: m.Template, Moves: []RoleMove{}}
	var wake []string
	err = e.inTx(ctx, func(t *txn) error {
		err := t.QueryRowContext(t.ctx, `SELECT id, name, template_name FROM teams WHERE (id=? OR name=?) AND closed_at IS NULL
			ORDER BY created_at DESC LIMIT 1`, a.Team, a.Team).Scan(&res.TeamID, &res.Name, &res.From)
		if errors.Is(err, sql.ErrNoRows) {
			return errf(CodeNotFound, "no open team %q", a.Team)
		}
		if err != nil {
			return internal(err)
		}
		gate, _, err := t.teamGate(res.TeamID)
		if err != nil {
			return err
		}
		members, err := t.teamMembersIn(res.TeamID)
		if err != nil {
			return err
		}
		to := map[string]string{} // member id -> new role
		var unmapped []string
		for _, p := range members {
			r, ok := a.Map[p.role]
			switch {
			case p.id == gate.id && m.sessionRole() != "":
				r = m.sessionRole()
			case ok:
			default:
				if _, same := m.Roles[p.role]; !same {
					if !slices.Contains(unmapped, p.role) {
						unmapped = append(unmapped, p.role)
					}
					continue
				}
				r = p.role
			}
			to[p.id] = r
		}
		if len(unmapped) > 0 {
			slices.Sort(unmapped)
			return &Error{Code: CodeInvalid, RuleID: "migrate.unmapped",
				Message: fmt.Sprintf("template %s has no role %s: map each to one of its roles (%s) with old=new",
					m.Template, strings.Join(unmapped, ", "), strings.Join(slices.Sorted(maps.Keys(m.Roles)), ", ")),
				Details: map[string]any{"unmapped": unmapped}}
		}
		if _, err := t.ExecContext(t.ctx, `UPDATE teams SET manifest=?, template_name=? WHERE id=?`,
			a.Manifest, m.Template, res.TeamID); err != nil {
			return internal(err)
		}
		byID, was := map[string]participant{}, map[string]string{}
		for _, p := range members {
			was[p.id] = p.role
			res.Moves = append(res.Moves, RoleMove{Name: p.name, From: p.role, To: to[p.id]})
			if _, err := t.ExecContext(t.ctx, `UPDATE participants SET role=? WHERE id=?`, to[p.id], p.id); err != nil {
				return internal(err)
			}
			p.role = to[p.id]
			byID[p.id] = p
		}
		for _, p := range members {
			p = byID[p.id]
			if boss, ok := byID[p.reportsTo]; ok {
				for _, d := range [][2]participant{{p, boss}, {boss, p}} {
					if _, allow := m.route(d[0].role, d[1].role); !allow {
						warnings = append(warnings, fmt.Sprintf("%s (%s) cannot send to %s (%s): the new routing has no rule allowing %s -> %s",
							d[0].name, d[0].role, d[1].name, d[1].role, d[0].role, d[1].role))
					}
				}
			}
			if p.state == "gone" {
				continue
			}
			card, err := t.roleCard(p, m)
			if err != nil {
				return err
			}
			// The card goes in this mail, so a harness without a system prompt is not given it again.
			if _, err := t.ExecContext(t.ctx, `UPDATE participants SET card_hash=? WHERE id=?`, cardKey(p), p.id); err != nil {
				return internal(err)
			}
			moved := ""
			if was[p.id] != p.role {
				moved = " (was " + was[p.id] + ")"
			}
			body := fmt.Sprintf("Your team %s now runs template %s (was %s); your role is %s%s. Your role card from now on:\n\n%s",
				res.Name, m.Template, res.From, p.role, moved, card)
			if _, err := t.insertMessage(newID(t.now), "", res.TeamID, AddrEngine, p.id, "", "", "", "", body); err != nil {
				return err
			}
			wake = append(wake, p.id)
		}
		return t.event(evt{typ: "team_migrated", team: res.TeamID, ref: res.TeamID,
			payload: map[string]any{"from": res.From, "to": res.To, "moves": res.Moves}})
	})
	if err != nil {
		return TeamMigrateResult{}, err
	}
	for _, id := range wake {
		e.notifyAfterCommit(id)
	}
	res.Warnings = warnings
	return res, nil
}

// teamMembersIn are the team's participants that have not left it, in the order they entered.
func (t *txn) teamMembersIn(team string) ([]participant, error) {
	rows, err := t.QueryContext(t.ctx, `SELECT `+participantCols+` FROM participants WHERE team_id=? AND left_at IS NULL
		ORDER BY created_at, rowid`, team)
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
