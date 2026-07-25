package resume

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

type ChunkState struct {
	ID        int    `json:"id"`
	Start     int64  `json:"start"`
	End       int64  `json:"end"`
	DoneBytes int64  `json:"doneBytes"`
	Status    string `json:"status,omitempty"` // "pending", "downloading", "completed"
}


type Meta struct {
	Version   int          `json:"version"`
	URL       string       `json:"url"`
	OutPath   string       `json:"outPath"`
	TotalSize int64        `json:"totalSize"`
	Parts     int          `json:"parts"`
	Chunks    []ChunkState `json:"chunks"`
	Checksum  string       `json:"checksum"`
	UpdatedAt string       `json:"updatedAt"`
	// Status is the orchestrator-owned lifecycle state for this job.
	// It is intentionally a simple string to avoid coupling with the TUI.
	// Expected values: "queued", "active", "paused", "done", "failed".
	Status string `json:"status,omitempty"`
	// Error holds an optional human-readable error message when Status
	// is set to "failed".
	Error string `json:"error,omitempty"`
}

func MetaPath(outPath string) string {
	return outPath + ".warp.meta.json"
}

func Save(metaPath string, m Meta) error {
	m.UpdatedAt = time.Now().Format(time.RFC3339)

	tmp := metaPath + ".tmp"
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(metaPath), 0755); err != nil {
		// If metaPath is in current dir, Dir(metaPath) might be "."
		// This is fine.
	}

	if err := os.WriteFile(tmp, b, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, metaPath)
}

func Load(metaPath string) (Meta, error) {
	b, err := os.ReadFile(metaPath)
	if err != nil {
		return Meta{}, err
	}
	var m Meta
	if err := json.Unmarshal(b, &m); err != nil {
		return Meta{}, err
	}
	return m, nil
}
