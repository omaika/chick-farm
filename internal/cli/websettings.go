package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"unicode"

	"github.com/sting8k/piggery/internal/core"
	"github.com/sting8k/piggery/internal/driver/local"
	"github.com/sting8k/piggery/internal/jsonobj"
	"github.com/sting8k/piggery/manifests"
	"gopkg.in/yaml.v3"
)

// The dashboard's Settings: the model and thinking a worker starts with, where piggery reads them
// (docs/guide.md): each harness profile's (~/.piggery/harness/<h>.json, every worker of a harness)
// and each template role's spawn (~/.piggery/templates/<t>/manifest.yaml, that role's workers,
// before the profile); and each template's limits (how many workers at once, how deep). The page reads and writes those files itself, in place: the daemon reads a
// profile at each spawn and a template at each found or team up, so nothing restarts, and a team
// already up keeps the manifest it was brought up with.

type settingsProfile struct {
	Harness  string `json:"harness"`
	Path     string `json:"path"`
	Model    string `json:"model"`
	Thinking string `json:"thinking"`
	Err      string `json:"error,omitempty"` // missing or unreadable: not editable here
}

type settingsRole struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Harness     string `json:"harness"`
	Model       string `json:"model"`
	Thinking    string `json:"thinking"`
}

type settingsLimit struct {
	Name  string `json:"name"`
	Value string `json:"value"` // a number, or none
}

type settingsTemplate struct {
	Name   string          `json:"name"`
	Path   string          `json:"path"`
	Roles  []settingsRole  `json:"roles"`
	Limits []settingsLimit `json:"limits"`
	Err    string          `json:"error,omitempty"`
}

// settings is every harness profile's model and thinking, every template role's spawn and every
// template's limits, as written (inherit or none where a value is left out).
func (s *webServer) settings(r *http.Request) (any, error) {
	profiles := []settingsProfile{}
	for _, h := range harnesses {
		p := settingsProfile{Harness: h.name, Path: h.profilePath(s.e.dir)}
		if o, err := readProfileObject(p.Path); err != nil {
			p.Err = err.Error()
		} else {
			p.Model, p.Thinking = profileString(o, "model"), profileString(o, "thinking")
		}
		profiles = append(profiles, p)
	}
	listed, err := manifests.List(s.e.dir)
	if err != nil {
		return nil, err
	}
	templates := []settingsTemplate{}
	for _, l := range listed {
		t := settingsTemplate{Name: l.Name, Path: filepath.Join(l.From, manifests.ManifestFile), Roles: []settingsRole{}, Limits: []settingsLimit{}}
		if roles, limits, err := templateSettings(t.Path); err != nil {
			t.Err = err.Error()
		} else {
			t.Roles, t.Limits = roles, limits
		}
		templates = append(templates, t)
	}
	return map[string]any{"harnesses": append([]string{local.Inherit}, local.Harnesses()...), "levels": pickLevels,
		"profiles": profiles, "templates": templates}, nil
}

// templateSettings are the roles of the manifest at path in the file's order, with their spawn,
// and its limits in core.LimitNames' order.
func templateSettings(path string) ([]settingsRole, []settingsLimit, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, err
	}
	var doc struct {
		Roles  yaml.Node         `yaml:"roles"`
		Limits map[string]string `yaml:"limits"`
	}
	if err := yaml.Unmarshal(b, &doc); err != nil {
		return nil, nil, err
	}
	limits := []settingsLimit{}
	for _, n := range core.LimitNames {
		v := doc.Limits[n]
		if v == "" {
			v = "none"
		}
		limits = append(limits, settingsLimit{Name: n, Value: v})
	}
	out := []settingsRole{}
	for i := 0; i+1 < len(doc.Roles.Content); i += 2 {
		var r struct {
			Description string `yaml:"description"`
			Spawn       struct {
				Harness  string `yaml:"harness"`
				Model    string `yaml:"model"`
				Thinking string `yaml:"thinking"`
			} `yaml:"spawn"`
		}
		if err := doc.Roles.Content[i+1].Decode(&r); err != nil {
			return nil, nil, fmt.Errorf("role %s: %w", doc.Roles.Content[i].Value, err)
		}
		out = append(out, settingsRole{Name: doc.Roles.Content[i].Value, Description: r.Description,
			Harness: orInherit(r.Spawn.Harness), Model: orInherit(r.Spawn.Model), Thinking: orInherit(r.Spawn.Thinking)})
	}
	return out, limits, nil
}

func orInherit(v string) string {
	if v == "" {
		return local.Inherit
	}
	return v
}

func readProfileObject(path string) (jsonobj.Object, error) {
	b, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("no profile at %s: piggery setup writes it", path)
	}
	if err != nil {
		return nil, err
	}
	o, err := jsonobj.Parse(b)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return o, nil
}

// profileString is key's string in a profile, inherit when it is missing or not a string.
func profileString(o jsonobj.Object, key string) string {
	var v string
	if raw, ok := o.Get(key); ok {
		_ = json.Unmarshal(raw, &v)
	}
	return orInherit(v)
}

// settingValue checks a model or thinking value the page sends: one line of printable text ("" is
// inherit).
func settingValue(what, v string) (string, error) {
	if len(v) > 200 || slices.ContainsFunc([]rune(v), func(r rune) bool { return !unicode.IsPrint(r) }) {
		return "", fmt.Errorf("%w: %s: one line of printable text, at most 200 bytes", errUsage, what)
	}
	return orInherit(v), nil
}

type profileSetting struct {
	Harness  string `json:"harness"`
	Model    string `json:"model"`
	Thinking string `json:"thinking"`
}

// setProfile writes a harness profile's model and thinking, keeping the rest of the file.
func (s *webServer) setProfile(r *http.Request) (any, error) {
	var a profileSetting
	if err := json.NewDecoder(http.MaxBytesReader(nil, r.Body, 1<<16)).Decode(&a); err != nil {
		return nil, fmt.Errorf("%w: %v", errUsage, err)
	}
	h, ok := harnessNamed(a.Harness)
	if !ok {
		return nil, fmt.Errorf("%w: no harness %q", errUsage, a.Harness)
	}
	path := h.profilePath(s.e.dir)
	o, err := readProfileObject(path)
	if err != nil {
		return nil, err
	}
	for _, f := range []struct{ key, val string }{{"model", a.Model}, {"thinking", a.Thinking}} {
		v, err := settingValue(f.key, f.val)
		if err != nil {
			return nil, err
		}
		raw, _ := json.Marshal(v)
		o = o.Set(f.key, raw)
	}
	if err := writeKeepingMode(path, o.Bytes(0)); err != nil {
		return nil, err
	}
	return map[string]any{"path": path}, nil
}

type roleSetting struct {
	Template string `json:"template"`
	Role     string `json:"role"`
	Harness  string `json:"harness"`
	Model    string `json:"model"`
	Thinking string `json:"thinking"`
}

// setRole writes a template role's spawn harness, model and thinking in place. The result must
// load as team up loads it; its warnings (a model pinned with harness inherit) come back.
func (s *webServer) setRole(r *http.Request) (any, error) {
	var a roleSetting
	if err := json.NewDecoder(http.MaxBytesReader(nil, r.Body, 1<<16)).Decode(&a); err != nil {
		return nil, fmt.Errorf("%w: %v", errUsage, err)
	}
	harness := orInherit(a.Harness)
	if harness != local.Inherit && !slices.Contains(local.Harnesses(), harness) {
		return nil, fmt.Errorf("%w: harness %q: not inherit or one of %v", errUsage, harness, local.Harnesses())
	}
	values := map[string]string{"harness": harness}
	for _, f := range []struct{ key, val string }{{"model", a.Model}, {"thinking", a.Thinking}} {
		var err error
		if values[f.key], err = settingValue(f.key, f.val); err != nil {
			return nil, err
		}
	}
	return s.editTemplate(a.Template, func(src []byte) ([]byte, error) { return core.SetRoleSpawn(src, a.Role, values) })
}

type limitsSetting struct {
	Template string            `json:"template"`
	Limits   map[string]string `json:"limits"` // a limit -> a positive number or none
}

// setLimits writes a template's limits in place; a result team up would refuse (a role that can
// spawn with depth or concurrency none) is refused here.
func (s *webServer) setLimits(r *http.Request) (any, error) {
	var a limitsSetting
	if err := json.NewDecoder(http.MaxBytesReader(nil, r.Body, 1<<16)).Decode(&a); err != nil {
		return nil, fmt.Errorf("%w: %v", errUsage, err)
	}
	if len(a.Limits) == 0 {
		return nil, fmt.Errorf("%w: no limit to set", errUsage)
	}
	return s.editTemplate(a.Template, func(src []byte) ([]byte, error) { return core.SetLimits(src, a.Limits) })
}

// editTemplate rewrites the manifest of the template called name with edit. The result must load as
// team up loads it before it is written; its warnings come back.
func (s *webServer) editTemplate(name string, edit func(src []byte) ([]byte, error)) (any, error) {
	listed, err := manifests.List(s.e.dir)
	if err != nil {
		return nil, err
	}
	i := slices.IndexFunc(listed, func(l manifests.Listed) bool { return l.Name == name })
	if i < 0 {
		return nil, fmt.Errorf("%w: no template %q in %s", errUsage, name, manifests.Dir(s.e.dir))
	}
	path := filepath.Join(listed[i].From, manifests.ManifestFile)
	src, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	out, err := edit(src)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", errUsage, err)
	}
	// checked as team up reads it (instructions_file inlined from the template's directory)
	tmp := filepath.Join(filepath.Dir(path), ".manifest.check.tmp")
	if err := os.WriteFile(tmp, out, 0o600); err != nil {
		return nil, err
	}
	text, err := manifests.Inline(tmp)
	os.Remove(tmp)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", errUsage, err)
	}
	warnings, err := core.CheckManifest(text)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", errUsage, err)
	}
	if err := writeKeepingMode(path, out); err != nil {
		return nil, err
	}
	if warnings == nil {
		warnings = []string{}
	}
	return map[string]any{"path": path, "warnings": warnings}, nil
}

// writeKeepingMode replaces the file at path with b through a rename, keeping its permissions.
func writeKeepingMode(path string, b []byte) error {
	mode := os.FileMode(0o600)
	if st, err := os.Stat(path); err == nil {
		mode = st.Mode().Perm()
	}
	tmp := filepath.Join(filepath.Dir(path), "."+filepath.Base(path)+".tmp")
	if err := os.WriteFile(tmp, b, mode); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
