package core

import (
	"fmt"

	"github.com/sting8k/piggery/internal/yamlfill"
)

// SpawnFields are the spawn keys SetRoleSpawn writes: how piggery starts a role's workers.
var SpawnFields = []string{"harness", "model", "thinking"}

// SetRoleSpawn returns the template src with role's spawn keys set from values (a key of
// SpawnFields -> its value; "" writes inherit). The keys the template lacks are added first
// (FillManifestFile's way), and every other byte stays. A result that would change anything but
// those keys, or that the parser refuses, is an error.
func SetRoleSpawn(src []byte, role string, values map[string]string) ([]byte, error) {
	before, err := parseManifest(string(src))
	if err != nil {
		return nil, err
	}
	r, ok := before.Roles[role]
	if !ok {
		return nil, errf(CodeNotFound, "no role %q in template %s", role, before.Template)
	}
	out, _, err := yamlfill.Fill(src, manifestKeys)
	if err != nil {
		return nil, errf(CodeInvalid, "manifest: %v", err)
	}
	for k, v := range values {
		var field *string
		switch k {
		case "harness":
			field = &r.Spawn.Harness
		case "model":
			field = &r.Spawn.Model
		case "thinking":
			field = &r.Spawn.Thinking
		default:
			return nil, errf(CodeInvalid, "spawn.%s: not one of %v", k, SpawnFields)
		}
		if v == "" {
			v = inherit
		}
		if out, err = yamlfill.Set(out, []string{"roles", role, "spawn", k}, v); err != nil {
			return nil, errf(CodeInvalid, "manifest: %v", err)
		}
		*field = v
		if v == inherit {
			*field = ""
		}
	}
	before.Roles[role] = r
	after, err := parseManifest(string(out))
	tb, _ := parseTimers(string(src))
	ta, _ := parseTimers(string(out))
	if err != nil || fmt.Sprintf("%+v %+v", before, tb) != fmt.Sprintf("%+v %+v", after, ta) {
		return nil, errf(CodeInvalid, "setting role %s's spawn would change more of template %s; left as it is", role, before.Template)
	}
	return out, nil
}
