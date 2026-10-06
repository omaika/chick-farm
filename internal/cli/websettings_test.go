package cli

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sting8k/piggery/internal/driver/local"
	"github.com/sting8k/piggery/manifests"
)

// Settings reads every profile and template role, and writes a profile's model and a role's spawn
// in place, without the daemon: the rest of each file stays, a bad value is refused before any
// write, and a model pinned with harness inherit comes back as a warning.
func TestWebSettings(t *testing.T) {
	dir := t.TempDir()
	if err := manifests.Unpack(dir); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "harness"), 0o700); err != nil {
		t.Fatal(err)
	}
	profile := local.ClaudeProfilePath(dir)
	if err := os.WriteFile(profile, []byte("{\n  \"cmd\": \"claude\",\n  \"model\": \"inherit\",\n  \"blacklist\": []\n}\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	s := &webServer{e: &env{dir: dir, noStart: true, admin: true}, token: "t0k"}
	h := s.handler("127.0.0.1:4125")
	call := func(method, path string, body any) (int, map[string]any) {
		var r *http.Request
		if body == nil {
			r = httptest.NewRequest(method, path, nil)
		} else {
			b, _ := json.Marshal(body)
			r = httptest.NewRequest(method, path, strings.NewReader(string(b)))
		}
		r.Host = "127.0.0.1:4125"
		r.Header.Set(tokenHeader, "t0k")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		var out map[string]any
		json.Unmarshal(w.Body.Bytes(), &out)
		return w.Code, out
	}

	code, out := call("GET", "/api/settings", nil)
	if code != http.StatusOK {
		t.Fatalf("settings: %d %v", code, out)
	}
	var got struct {
		Profiles  []settingsProfile  `json:"profiles"`
		Templates []settingsTemplate `json:"templates"`
	}
	b, _ := json.Marshal(out)
	json.Unmarshal(b, &got)
	for _, p := range got.Profiles {
		if p.Harness == "claude" && (p.Err != "" || p.Model != "inherit" || p.Thinking != "inherit") {
			t.Fatalf("claude profile: %+v", p)
		}
		if p.Harness == "pi" && p.Err == "" {
			t.Fatalf("pi has no profile here: %+v", p)
		}
	}
	var slp *settingsTemplate
	for i := range got.Templates {
		if got.Templates[i].Name == "slp" {
			slp = &got.Templates[i]
		}
	}
	if slp == nil || len(slp.Roles) != 3 || slp.Roles[0].Name != "supervisor" || slp.Roles[2].Name != "peer" || slp.Roles[2].Model != "inherit" {
		t.Fatalf("slp: %+v", slp)
	}
	if len(slp.Limits) != 4 || slp.Limits[1] != (settingsLimit{"concurrency", "10"}) || slp.Limits[3] != (settingsLimit{"max_respawn_per_hour", "none"}) {
		t.Fatalf("slp limits: %+v", slp.Limits)
	}

	if code, out := call("POST", "/api/settings/profile", profileSetting{Harness: "claude", Model: "sonnet", Thinking: "high"}); code != http.StatusOK {
		t.Fatalf("set profile: %d %v", code, out)
	}
	pb, _ := os.ReadFile(profile)
	if want := "{\n  \"cmd\": \"claude\",\n  \"model\": \"sonnet\",\n  \"blacklist\": [],\n  \"thinking\": \"high\"\n}\n"; string(pb) != want {
		t.Fatalf("profile:\n%s", pb)
	}
	if st, _ := os.Stat(profile); st.Mode().Perm() != 0o640 {
		t.Fatalf("profile mode %v", st.Mode().Perm())
	}

	path := filepath.Join(manifests.Dir(dir), "slp", manifests.ManifestFile)
	before, _ := os.ReadFile(path)
	if code, out := call("POST", "/api/settings/role", roleSetting{Template: "slp", Role: "peer", Harness: "claude", Model: "claude-sonnet-5-5"}); code != http.StatusOK || len(out["warnings"].([]any)) != 0 {
		t.Fatalf("set role: %d %v", code, out)
	}
	after, _ := os.ReadFile(path)
	if strings.Count(string(before), "\n") != strings.Count(string(after), "\n") || !strings.Contains(string(after), "model: claude-sonnet-5-5") {
		t.Fatalf("slp after:\n%s", after)
	}
	code, out = call("POST", "/api/settings/role", roleSetting{Template: "slp", Role: "lead", Model: "opus"})
	if code != http.StatusOK || len(out["warnings"].([]any)) != 1 {
		t.Fatalf("model with harness inherit: %d %v", code, out)
	}

	if code, out := call("POST", "/api/settings/limits", limitsSetting{Template: "slp", Limits: map[string]string{"concurrency": "20", "max_respawn_per_hour": "6"}}); code != http.StatusOK {
		t.Fatalf("set limits: %d %v", code, out)
	}
	if b, _ := os.ReadFile(path); !strings.Contains(string(b), "  concurrency: 20           # live workers at once") || !strings.Contains(string(b), "max_respawn_per_hour: 6") {
		t.Fatalf("slp limits after:\n%s", b)
	}
	// slp's roles spawn, so team up refuses concurrency none: refused here, before the write
	for _, bad := range []map[string]string{{"concurrency": "none"}, {"concurrency": "0"}, {"depth": "x"}, {"hops": "1"}, {}} {
		if code, out := call("POST", "/api/settings/limits", limitsSetting{Template: "slp", Limits: bad}); code != http.StatusBadRequest {
			t.Fatalf("limits %v: %d %v", bad, code, out)
		}
	}
	if b, _ := os.ReadFile(path); !strings.Contains(string(b), "concurrency: 20") {
		t.Fatal("a refused limits call changed the template")
	}
	for _, bad := range []any{
		roleSetting{Template: "../slp", Role: "peer"},
		roleSetting{Template: "slp", Role: "nobody"},
		roleSetting{Template: "slp", Role: "peer", Harness: "vim"},
		roleSetting{Template: "slp", Role: "peer", Model: "a\nb"},
	} {
		if code, out := call("POST", "/api/settings/role", bad); code != http.StatusBadRequest {
			t.Fatalf("%+v: %d %v", bad, code, out)
		}
	}
	if code, _ := call("POST", "/api/settings/profile", profileSetting{Harness: "pi", Model: "x"}); code == http.StatusOK {
		t.Fatal("pi without a profile: written")
	}
	if code, _ := call("POST", "/api/settings/profile", profileSetting{Harness: "vim"}); code != http.StatusBadRequest {
		t.Fatal("unknown harness: not a bad request")
	}
	if b, _ := os.ReadFile(path); !strings.Contains(string(b), "model: claude-sonnet-5-5") {
		t.Fatal("a refused call changed the template")
	}
}
