package queue

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Status represents the state of a queued download.
type Status string

const (
	StatusPending    Status = "pending"
	StatusInProgress Status = "in_progress"
	StatusDone       Status = "done"
	StatusFailed     Status = "failed"
)

// Item is a single queued download.
type Item struct {
	ID      string `json:"id"`
	URL     string `json:"url"`
	OutPath string `json:"outPath"`
	AddedAt string `json:"addedAt"`
	Status  Status `json:"status"`
	Error   string `json:"error,omitempty"`
}

// FileQueue is the on-disk representation of the queue.
type FileQueue struct {
	Items []Item `json:"items"`
}

// DefaultPath returns the default location of the queue file
// (current directory by default).
func DefaultPath() string {
	return "warpcentral.queue.json"
}

// Load reads a queue file. If it does not exist, an empty queue is returned.
func Load(path string) (FileQueue, error) {
	if path == "" {
		path = DefaultPath()
	}
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return FileQueue{}, nil
		}
		return FileQueue{}, err
	}
	var q FileQueue
	if err := json.Unmarshal(b, &q); err != nil {
		return FileQueue{}, err
	}
	return q, nil
}

// Save writes the queue atomically to disk.
func Save(path string, q FileQueue) error {
	if path == "" {
		path = DefaultPath()
	}

	tmp := path + ".tmp"
	b, err := json.MarshalIndent(q, "", "  ")
	if err != nil {
		return err
	}

	// Ensure directory exists ("." is fine).
	_ = os.MkdirAll(filepath.Dir(path), 0o755)

	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// Add appends a new pending item to the queue and persists it.
func Add(path, url, outPath string) (Item, error) {
	q, err := Load(path)
	if err != nil {
		return Item{}, err
	}

	id := fmt.Sprintf("%d", time.Now().UnixNano())
	item := Item{
		ID:      id,
		URL:     url,
		OutPath: outPath,
		AddedAt: time.Now().Format(time.RFC3339),
		Status:  StatusPending,
	}

	q.Items = append(q.Items, item)
	if err := Save(path, q); err != nil {
		return Item{}, err
	}
	return item, nil
}
