package state

import (
	"fmt"
	"testing"
)

func TestRoundTrip(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	s := Open()
	if got := s.Get("/a.md"); got != 0 {
		t.Fatalf("empty store Get = %d", got)
	}
	s.Set("/a.md", 42)
	s.Set("", 7)      // ignored
	s.Set("/b.md", 0) // ignored
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}

	s2 := Open()
	if got := s2.Get("/a.md"); got != 42 {
		t.Errorf("reloaded Get = %d, want 42", got)
	}
	if got := s2.Get("/b.md"); got != 0 {
		t.Errorf("invalid Set persisted: %d", got)
	}
}

func TestPrune(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	s := Open()
	for i := 0; i < maxEntries+50; i++ {
		s.Set(fmt.Sprintf("/doc-%d.md", i), i+1)
	}
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}
	if len(s.positions) != maxEntries {
		t.Errorf("pruned to %d entries, want %d", len(s.positions), maxEntries)
	}
}

func TestNoStateDirIsQuiet(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", "")
	t.Setenv("HOME", "")
	s := Open()
	s.Set("/x.md", 3)
	if err := s.Save(); err != nil {
		t.Errorf("save without a state dir should be a no-op, got %v", err)
	}
}
