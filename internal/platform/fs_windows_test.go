//go:build windows

package platform

import (
	"os"
	"path/filepath"
	"testing"
)

// Without the symlink privilege a directory is linked as a junction: its files read through it,
// and removing the junction (as removing a worker's agent dir does) leaves the target whole.
func TestJunction(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	if err := os.MkdirAll(filepath.Join(target, "sub"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(target, "sub", "f"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	parent := filepath.Join(dir, "agent")
	if err := os.Mkdir(parent, 0o700); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(parent, "j")
	if err := junction(target, link); err != nil {
		t.Fatal(err)
	}
	if b, err := os.ReadFile(filepath.Join(link, "sub", "f")); err != nil || string(b) != "x" {
		t.Fatalf("read through the junction: %q, %v", b, err)
	}
	if err := os.RemoveAll(parent); err != nil {
		t.Fatal(err)
	}
	if b, err := os.ReadFile(filepath.Join(target, "sub", "f")); err != nil || string(b) != "x" {
		t.Fatalf("target after removing the junction: %q, %v; want it untouched", b, err)
	}
}
