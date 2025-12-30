package watch

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// WatcherState represents a running watcher
type WatcherState struct {
	PID          int       `json:"pid"`
	Store        string    `json:"store"`
	Path         string    `json:"path"`
	ConnectionID string    `json:"connection_id"`
	StartedAt    time.Time `json:"started_at"`
	FileCount    int       `json:"file_count"`
	LastSync     time.Time `json:"last_sync"`
}

// StateDir returns the directory for watcher state files
func StateDir() string {
	// XDG_STATE_HOME or ~/.local/state
	if dir := os.Getenv("XDG_STATE_HOME"); dir != "" {
		return filepath.Join(dir, "rice-search", "watchers")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "state", "rice-search", "watchers")
}

// StatePath returns the path for a specific watcher's state file
func StatePath(pid int) string {
	return filepath.Join(StateDir(), fmt.Sprintf("%d.json", pid))
}

// SaveState saves watcher state to disk
func SaveState(state *WatcherState) error {
	dir := StateDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(StatePath(state.PID), data, 0644)
}

// LoadState loads watcher state from disk
func LoadState(pid int) (*WatcherState, error) {
	data, err := os.ReadFile(StatePath(pid))
	if err != nil {
		return nil, err
	}

	var state WatcherState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, err
	}

	return &state, nil
}

// ListStates returns all watcher states
func ListStates() ([]*WatcherState, error) {
	dir := StateDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var states []*WatcherState
	for _, entry := range entries {
		if filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			continue
		}

		var state WatcherState
		if err := json.Unmarshal(data, &state); err != nil {
			continue
		}

		// Check if process is still running
		if !isProcessRunning(state.PID) {
			// Clean up stale state file
			os.Remove(filepath.Join(dir, entry.Name()))
			continue
		}

		states = append(states, &state)
	}

	return states, nil
}

// RemoveState removes a watcher's state file
func RemoveState(pid int) error {
	return os.Remove(StatePath(pid))
}
