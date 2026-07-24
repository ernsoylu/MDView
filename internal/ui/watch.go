package ui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/fsnotify/fsnotify"
)

type fileChangedMsg struct{ name string }

// watchCmd blocks on the fsnotify channels and emits a message when a
// watched directory entry changes. Update filters for the current file and
// re-issues the command, bubbletea-subscription style. Watching the
// directory rather than the file survives editors that write via rename.
func (m Model) watchCmd() tea.Cmd {
	if m.watcher == nil {
		return nil
	}
	w := m.watcher
	return func() tea.Msg {
		for {
			select {
			case ev, ok := <-w.Events:
				if !ok {
					return nil
				}
				if ev.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Rename) != 0 {
					return fileChangedMsg{name: ev.Name}
				}
			case _, ok := <-w.Errors:
				if !ok {
					return nil
				}
			}
		}
	}
}
