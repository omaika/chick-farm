package cli

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os/exec"
	"os/signal"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/sting8k/piggery/internal/core"
	"github.com/sting8k/piggery/internal/proto"
	"github.com/sting8k/piggery/internal/server"
	"github.com/sting8k/piggery/internal/view"
)

// The web dashboard: top in a browser. One page (web.html) polls the snapshot as `ps --view` writes
// it, the selected participant's tail as `tail --view` reads it, and does what top does to a worker
// (kill, model and thinking), and edits the model and thinking workers start with (Settings,
// websettings.go). It listens on loopback only; every API call carries the token the page
// was served with, and a request whose Host is not the address it listens on is refused, so another
// site in the same browser can neither call it nor read the token (DNS rebinding).

//go:embed web.html
var webHTML string

const (
	webAddr      = "127.0.0.1:4125"
	webTailLines = 100 // the latest tail lines the page asks for at most
	webEvents    = 30  // the latest events in each snapshot
	tokenHeader  = "X-Piggery-Token"
)

// web serves the dashboard until Ctrl-C.
func (e *env) web(args []string) error {
	fs := e.flags("web")
	addr := fs.String("addr", webAddr, "where to listen (a loopback address)")
	open := fs.Bool("open", false, "open the page in the browser")
	pos, err := parse(fs, args)
	if err != nil {
		return err
	}
	if len(pos) != 0 {
		return fmt.Errorf("%w: web takes no arguments", errUsage)
	}
	host, _, err := net.SplitHostPort(*addr)
	if err != nil {
		return fmt.Errorf("%w: --addr %q: %v", errUsage, *addr, err)
	}
	if ip := net.ParseIP(host); host != "localhost" && (ip == nil || !ip.IsLoopback()) {
		return fmt.Errorf("%w: --addr %q: the dashboard acts as admin, so it listens on a loopback address only", errUsage, *addr)
	}
	// The first call starts the daemon when needed, as top does; the page's calls never start it.
	c, err := e.connect()
	if err != nil {
		return err
	}
	c.Close()
	e.noStart = true
	ln, err := net.Listen("tcp", *addr)
	if err != nil {
		return err
	}
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return err
	}
	s := &webServer{e: e, token: hex.EncodeToString(b), logs: loadLogCache(e.dir)}
	url := "http://" + ln.Addr().String() + "/"
	fmt.Fprintf(e.stdout, "piggery web on %s (Ctrl-C to stop)\n", url)
	if *open {
		openBrowser(url)
	}
	srv := &http.Server{Handler: s.handler(ln.Addr().String()), ReadHeaderTimeout: 10 * time.Second}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		sctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		srv.Shutdown(sctx)
	}()
	if err := srv.Serve(ln); !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	saveLogCache(e.dir, s.logState())
	return nil
}

// openBrowser opens url in the default browser; a failure only means the user opens it.
func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	_ = cmd.Start()
}

type webServer struct {
	e     *env
	token string
	mu    sync.Mutex
	logs  map[string]logState // worker logs and transcripts read so far, by path, as top keeps them
}

func (s *webServer) logState() map[string]logState {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.logs
}

// handler routes the page and its API behind the Host and token checks; addr is where it listens.
func (s *webServer) handler(addr string) http.Handler {
	_, port, _ := net.SplitHostPort(addr)
	hosts := []string{addr, "localhost:" + port, "127.0.0.1:" + port, "[::1]:" + port}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", s.page)
	mux.HandleFunc("GET /api/view", s.api(s.view))
	mux.HandleFunc("GET /api/tail", s.api(s.tail))
	mux.HandleFunc("GET /api/models", s.api(s.models))
	mux.HandleFunc("GET /api/mail", s.api(s.mail))
	mux.HandleFunc("GET /api/tasks", s.api(s.tasks))
	mux.HandleFunc("POST /api/kill", s.api(s.kill))
	mux.HandleFunc("POST /api/model", s.api(s.model))
	mux.HandleFunc("GET /api/settings", s.api(s.settings))
	mux.HandleFunc("POST /api/settings/profile", s.api(s.setProfile))
	mux.HandleFunc("POST /api/settings/role", s.api(s.setRole))
	mux.HandleFunc("POST /api/settings/limits", s.api(s.setLimits))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !slices.Contains(hosts, r.Host) {
			http.Error(w, "unknown host", http.StatusForbidden)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		mux.ServeHTTP(w, r)
	})
}

// page is web.html with this run's token in it.
func (s *webServer) page(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; script-src 'unsafe-inline'; style-src 'unsafe-inline'; connect-src 'self'; img-src data:")
	fmt.Fprint(w, strings.Replace(webHTML, "{{TOKEN}}", s.token, 1))
}

// api checks the token, then answers with f's result as JSON, or {"error": ...} (502: the daemon
// refused or is down; 400: a bad request).
func (s *webServer) api(f func(r *http.Request) (any, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if subtle.ConstantTimeCompare([]byte(r.Header.Get(tokenHeader)), []byte(s.token)) != 1 {
			w.WriteHeader(http.StatusForbidden)
			json.NewEncoder(w).Encode(map[string]string{"error": "bad token: reload the page"})
			return
		}
		out, err := f(r)
		if err != nil {
			code := http.StatusBadGateway
			if errors.Is(err, errUsage) {
				code = http.StatusBadRequest
			}
			w.WriteHeader(code)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		json.NewEncoder(w).Encode(out)
	}
}

// call makes one call to the daemon on a connection of its own (requests run at once), never
// starting it.
func (s *webServer) call(verb string, args, out any) error {
	c, err := s.e.connect()
	if err != nil {
		return err
	}
	defer c.Close()
	_, err = c.CallInto(verb, args, out)
	return err
}

// webView is what `ps --view` prints, with the columns top shows after the name.
type webView struct {
	psViewDoc
	Columns []string `json:"columns"`
}

func (s *webServer) view(r *http.Request) (any, error) {
	var ps proto.PsResult
	if err := s.call(proto.VerbPs, core.StateArgs{Events: webEvents}, &ps); err != nil {
		return nil, err
	}
	s.mu.Lock()
	s.logs = readLogs(s.e.dir, ps, s.logs)
	stats := (&topModel{dir: s.e.dir, ps: ps, logs: s.logs}).stats()
	s.mu.Unlock()
	cols := server.DisplayColumns
	if set, err := server.LoadSettings(s.e.dir); err == nil {
		cols, _ = server.ColumnsOf(set, s.e.dir)
	}
	// top has no unacked column: its header, team lines and details give it
	cols = slices.DeleteFunc(slices.Clone(cols), func(c string) bool { return c == "unacked" })
	return webView{psViewDoc: viewDoc(ps, stats, time.Now()), Columns: cols}, nil
}

// tail is the last lines of a participant's log, read back from its end on every call (a poll
// reads at most a window of it).
func (s *webServer) tail(r *http.Request) (any, error) {
	id := r.URL.Query().Get("id")
	if id == "" {
		return nil, fmt.Errorf("%w: no id", errUsage)
	}
	n, err := strconv.Atoi(r.URL.Query().Get("n"))
	if err != nil || n < 1 || n > webTailLines {
		n = webTailLines
	}
	var tr proto.TailResult
	if err := s.call(proto.VerbTail, core.WorkerLogArgs{Worker: id}, &tr); err != nil {
		return nil, err
	}
	path, newReader, err := tailSource(tr)
	if err != nil {
		return nil, err
	}
	lines, _, _, err := lastLines(path, newReader, n, false)
	if err != nil {
		return nil, err
	}
	out := []view.TailLine{}
	for _, l := range lines {
		out = append(out, view.ParseTail(l))
	}
	return map[string]any{"lines": out}, nil
}

// models is what the worker's harness lists for the picker, with the thinking levels it offers.
func (s *webServer) models(r *http.Request) (any, error) {
	id := r.URL.Query().Get("id")
	if id == "" {
		return nil, fmt.Errorf("%w: no id", errUsage)
	}
	var mr core.ModelsResult
	if err := s.call(proto.VerbModels, core.AdminTarget{Target: id}, &mr); err != nil {
		return nil, err
	}
	if mr.Models == nil {
		mr.Models = []string{}
	}
	return map[string]any{"models": mr.Models, "levels": pickLevels}, nil
}

// mail is the messages of a participant or a team (verb `mail`, read-only: it acks nothing),
// newest first; before pages back. pins=1 is a team's live board pins instead, oldest first.
func (s *webServer) mail(r *http.Request) (any, error) {
	q := r.URL.Query()
	a := core.MailArgs{Participant: q.Get("participant"), Team: q.Get("team")}
	if a.Participant == "" && a.Team == "" {
		return nil, fmt.Errorf("%w: a participant or a team", errUsage)
	}
	a.Before, _ = strconv.ParseInt(q.Get("before"), 10, 64)
	a.Limit, _ = strconv.Atoi(q.Get("limit"))
	if a.Pins = q.Get("pins") == "1"; a.Pins && a.Team == "" {
		return nil, fmt.Errorf("%w: pins needs a team", errUsage)
	}
	var out core.MailResult
	if err := s.call(proto.VerbMail, a, &out); err != nil {
		return nil, err
	}
	if a.Pins {
		return out, pinsOnly(out)
	}
	return out, nil
}

// pinsOnly refuses a pins answer with mail in it: a daemon older than pins ignores it and answers
// with the team's mail.
func pinsOnly(r core.MailResult) error {
	for _, m := range r.Messages {
		if m.ToID != core.AddrBoard {
			return fmt.Errorf("the daemon is older than this binary and cannot list pins: piggery restart")
		}
	}
	return nil
}

// tasks is the tasks of a team or of a project directory's teams (verb `tasks`, read-only).
func (s *webServer) tasks(r *http.Request) (any, error) {
	q := r.URL.Query()
	a := core.TasksArgs{Team: q.Get("team"), Dir: q.Get("dir")}
	if (a.Team == "") == (a.Dir == "") {
		return nil, fmt.Errorf("%w: a team or a directory", errUsage)
	}
	var out core.TasksResult
	if err := s.call(proto.VerbTasks, a, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// kill is top's x: the verb `kill` on a worker.
func (s *webServer) kill(r *http.Request) (any, error) {
	var a core.AdminTarget
	if err := decodeBody(r, &a, &a); err != nil {
		return nil, err
	}
	var out core.AgentResult
	if err := s.call(proto.VerbKill, a, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// model is the picker's Enter: the verb `model` with a model, a thinking level, or both.
func (s *webServer) model(r *http.Request) (any, error) {
	var a core.ModelArgs
	if err := decodeBody(r, &a, &a.AdminTarget); err != nil {
		return nil, err
	}
	if a.Model == "" && a.Thinking == "" {
		return nil, fmt.Errorf("%w: nothing to change", errUsage)
	}
	var out core.ModelResult
	if err := s.call(proto.VerbModel, a, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// decodeBody reads a JSON body into into; t is its target, which it must name.
func decodeBody(r *http.Request, into any, t *core.AdminTarget) error {
	if err := json.NewDecoder(http.MaxBytesReader(nil, r.Body, 1<<16)).Decode(into); err != nil {
		return fmt.Errorf("%w: %v", errUsage, err)
	}
	if t.Target == "" {
		return fmt.Errorf("%w: no target", errUsage)
	}
	return nil
}
