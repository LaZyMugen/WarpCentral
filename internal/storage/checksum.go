package storage

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/LaZyMugen/warpcentral/internal/resume"
)

type Job struct {
	ID       string
	MetaPath string
	Status   string
}

func SHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

// LoadJobs scans the current directory for .warp.meta.json files and
// constructs a lightweight Job view for the TUI.
func LoadJobs() ([]Job, error) {
	files, err := filepath.Glob("*.warp.meta.json")
	if err != nil {
		return nil, err
	}

	jobs := make([]Job, 0, len(files))
	for _, metaPath := range files {
		m, err := resume.Load(metaPath)
		if err != nil {
			// Skip malformed or unreadable meta files
			continue
		}

		// Prefer orchestrator-owned status from meta, but fall back
		// to a heuristic for older meta files that don't have it yet.
		status := m.Status
		if status == "" {
			var totalDone int64
			for _, c := range m.Chunks {
				length := (c.End - c.Start + 1)
				done := c.DoneBytes
				if done > length {
					done = length
				}
				totalDone += done
			}

			status = "queued"
			if m.TotalSize > 0 {
				switch {
				case totalDone >= m.TotalSize:
					status = "done"
				case totalDone == 0:
					status = "queued"
				default:
					status = "paused"
				}
			}
		}

		jobs = append(jobs, Job{
			ID:       m.OutPath,
			MetaPath: metaPath,
			Status:   status,
		})
	}

	return jobs, nil
}
