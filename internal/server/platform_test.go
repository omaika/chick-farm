package server_test

import (
	"encoding/json"
	"io"
	"os"
	"runtime"
	"testing"
	"time"
)

// shortTemp is where a test's piggery home goes: /tmp keeps unix socket paths short (macOS limits
// them); Windows has no /tmp, and its daemon pipe name does not depend on the path's length.
func shortTemp() string {
	if runtime.GOOS == "windows" {
		return ""
	}
	return "/tmp"
}

// skipOnWindows skips a test that needs a POSIX shell or signals.
func skipOnWindows(t testing.TB, why string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("not on Windows: " + why)
	}
}

// unixModes: file permission bits mean something (not on Windows, where a file is 0666 or 0444).
var unixModes = runtime.GOOS != "windows"

// fakeWorker is a harness profile that runs this test binary as a worker (TestMain): "sleep"
// exits after 0.3s, "cat" when its stdin closes. No shell, so it runs on every OS.
func fakeWorker(t *testing.T, mode string) []byte {
	t.Setenv("PGTEST_WORKER", mode)
	prof, err := json.Marshal(map[string]any{"cmd": os.Args[0], "args": []string{}})
	if err != nil {
		t.Fatal(err)
	}
	return prof
}

// TestMain runs the fake worker of fakeWorker before the test flags are parsed (the driver adds
// the harness's own flags) and before anything is printed on stdout, the harness's protocol.
func TestMain(m *testing.M) {
	switch os.Getenv("PGTEST_WORKER") {
	case "sleep":
		time.Sleep(300 * time.Millisecond)
		os.Exit(0)
	case "cat":
		io.Copy(io.Discard, os.Stdin)
		os.Exit(0)
	}
	os.Exit(m.Run())
}
