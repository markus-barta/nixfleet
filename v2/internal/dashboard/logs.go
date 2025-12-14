package dashboard

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// LogStore handles file-based storage of command output logs
type LogStore struct {
	basePath string
	mu       sync.RWMutex
	files    map[string]*os.File // hostID:command -> file
}

// NewLogStore creates a new log store with the given base path
func NewLogStore(basePath string) (*LogStore, error) {
	if err := os.MkdirAll(basePath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create log directory: %w", err)
	}
	return &LogStore{
		basePath: basePath,
		files:    make(map[string]*os.File),
	}, nil
}

// StartCommand opens a new log file for a command execution
func (ls *LogStore) StartCommand(hostID, command string) error {
	ls.mu.Lock()
	defer ls.mu.Unlock()

	// Create host directory if needed
	hostDir := filepath.Join(ls.basePath, hostID)
	if err := os.MkdirAll(hostDir, 0755); err != nil {
		return fmt.Errorf("failed to create host log directory: %w", err)
	}

	// Create log file with timestamp
	timestamp := time.Now().Format("2006-01-02T15-04-05")
	filename := fmt.Sprintf("%s-%s.log", timestamp, command)
	path := filepath.Join(hostDir, filename)

	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("failed to create log file: %w", err)
	}

	// Write header
	header := fmt.Sprintf("# Command: %s\n# Host: %s\n# Started: %s\n\n",
		command, hostID, time.Now().Format(time.RFC3339))
	if _, err := f.WriteString(header); err != nil {
		_ = f.Close()
		return fmt.Errorf("failed to write log header: %w", err)
	}

	// Store file handle
	key := hostID + ":" + command
	if existing, ok := ls.files[key]; ok {
		_ = existing.Close()
	}
	ls.files[key] = f

	return nil
}

// AppendLine writes a line to the active log file for a host's command
func (ls *LogStore) AppendLine(hostID, command, line string, isError bool) error {
	ls.mu.RLock()
	key := hostID + ":" + command
	f, ok := ls.files[key]
	ls.mu.RUnlock()

	if !ok {
		// No active log file, start one
		if err := ls.StartCommand(hostID, command); err != nil {
			return err
		}
		ls.mu.RLock()
		f = ls.files[key]
		ls.mu.RUnlock()
	}

	// Format line with timestamp
	timestamp := time.Now().Format("15:04:05")
	prefix := ""
	if isError {
		prefix = "[ERR] "
	}
	formatted := fmt.Sprintf("[%s] %s%s\n", timestamp, prefix, line)

	ls.mu.Lock()
	defer ls.mu.Unlock()
	_, err := f.WriteString(formatted)
	return err
}

// CompleteCommand closes the log file for a completed command
func (ls *LogStore) CompleteCommand(hostID, command string, exitCode int) error {
	ls.mu.Lock()
	defer ls.mu.Unlock()

	key := hostID + ":" + command
	f, ok := ls.files[key]
	if !ok {
		return nil // No log file was opened
	}

	// Write footer
	footer := fmt.Sprintf("\n# Completed: %s\n# Exit code: %d\n",
		time.Now().Format(time.RFC3339), exitCode)
	_, _ = f.WriteString(footer)

	// Close and remove from map
	_ = f.Close()
	delete(ls.files, key)

	return nil
}

// GetLogPath returns the path to the logs directory for a host
func (ls *LogStore) GetLogPath(hostID string) string {
	return filepath.Join(ls.basePath, hostID)
}

// ListLogs returns a list of log files for a host
func (ls *LogStore) ListLogs(hostID string) ([]string, error) {
	hostDir := filepath.Join(ls.basePath, hostID)
	entries, err := os.ReadDir(hostDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var logs []string
	for _, e := range entries {
		if !e.IsDir() && filepath.Ext(e.Name()) == ".log" {
			logs = append(logs, e.Name())
		}
	}
	return logs, nil
}

// Close closes all open log files
func (ls *LogStore) Close() error {
	ls.mu.Lock()
	defer ls.mu.Unlock()

	for _, f := range ls.files {
		_ = f.Close()
	}
	ls.files = make(map[string]*os.File)
	return nil
}

