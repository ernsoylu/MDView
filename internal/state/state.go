// Package state persists per-file reading positions in the XDG state
// directory, so reopening a document restores where you were.
package state

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"time"
)

const maxEntries = 500

type entry struct {
	Line int   `json:"line"` // 1-based source line at the top of the viewport
	Seen int64 `json:"seen"` // unix time of the last visit, for pruning
}

type Store struct {
	path      string
	positions map[string]entry
}

func dir() string {
	if d := os.Getenv("XDG_STATE_HOME"); d != "" {
		return filepath.Join(d, "mdv")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".local", "state", "mdv")
}

// Open loads the position store. It never fails: a missing or corrupt
// file just means an empty store, and an undeterminable state dir means a
// store that quietly persists nothing.
func Open() *Store {
	s := &Store{positions: map[string]entry{}}
	d := dir()
	if d == "" {
		return s
	}
	s.path = filepath.Join(d, "positions.json")
	data, err := os.ReadFile(s.path)
	if err != nil {
		return s
	}
	var positions map[string]entry
	if json.Unmarshal(data, &positions) == nil && positions != nil {
		s.positions = positions
	}
	return s
}

// Get returns the saved top source line for a document, 0 if none.
func (s *Store) Get(doc string) int {
	return s.positions[doc].Line
}

// Set records the top source line for a document.
func (s *Store) Set(doc string, line int) {
	if doc == "" || line < 1 {
		return
	}
	s.positions[doc] = entry{Line: line, Seen: time.Now().Unix()}
}

// Save writes the store, pruning the least recently seen entries beyond
// the cap. Best-effort: an unwritable state dir is not an error worth
// bothering the user about.
func (s *Store) Save() error {
	if s.path == "" {
		return nil
	}
	if len(s.positions) > maxEntries {
		type keyed struct {
			key string
			e   entry
		}
		all := make([]keyed, 0, len(s.positions))
		for k, e := range s.positions {
			all = append(all, keyed{k, e})
		}
		sort.Slice(all, func(i, j int) bool { return all[i].e.Seen > all[j].e.Seen })
		s.positions = make(map[string]entry, maxEntries)
		for _, ke := range all[:maxEntries] {
			s.positions[ke.key] = ke.e
		}
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	data, err := json.Marshal(s.positions)
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}
