package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnsureSeedsAndLoads(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	c, err := Ensure()
	if err != nil {
		t.Fatal(err)
	}
	if c != (Config{}) {
		t.Errorf("fresh config = %+v, want zero", c)
	}
	for _, name := range []string{"config.yaml", "theme.yaml"} {
		data, err := os.ReadFile(filepath.Join(home, ".MDView", name))
		if err != nil {
			t.Fatalf("%s not seeded: %v", name, err)
		}
		if !strings.HasPrefix(string(data), "# mdv") {
			t.Errorf("%s template missing header", name)
		}
	}

	// User settings load; templates are not overwritten.
	if err := os.WriteFile(filepath.Join(home, ".MDView", "config.yaml"),
		[]byte("theme: plain\nwidth: 90\neditor: nano\nimages: off\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	c, err = Ensure()
	if err != nil {
		t.Fatal(err)
	}
	want := Config{Theme: "plain", Width: 90, Editor: "nano", Images: "off"}
	if c != want {
		t.Errorf("loaded = %+v, want %+v", c, want)
	}
}

func TestEnsureRejectsBadConfig(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if _, err := Ensure(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, ".MDView", "config.yaml"),
		[]byte("wdith: 90\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Ensure(); err == nil {
		t.Error("typo'd key should be an error")
	}
}

func TestNoHomeIsQuiet(t *testing.T) {
	t.Setenv("HOME", "")
	c, err := Ensure()
	if err != nil || c != (Config{}) {
		t.Errorf("no home: got %+v, %v", c, err)
	}
}
