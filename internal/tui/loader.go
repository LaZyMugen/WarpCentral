package tui

import (
	"path/filepath"
	"strings"

	"github.com/LaZyMugen/warpcentral/internal/resume"
	"github.com/LaZyMugen/warpcentral/internal/storage"
)

func loadJobs() []Job {
	jobs, err := storage.LoadJobs()
	if err != nil {
		return nil
	}

	out := make([]Job, 0, len(jobs))
	for _, j := range jobs {
		meta, err := resume.Load(j.MetaPath)
		if err != nil {
			continue
		}

		var doneBytes int64
		for _, c := range meta.Chunks {
			doneBytes += c.DoneBytes
		}

		progress := 0.0
		if meta.TotalSize > 0 {
			progress = float64(doneBytes) / float64(meta.TotalSize)
		}

		if progress > 1 {
			progress = 1
		}
		if progress < 0 {
			progress = 0
		}

		out = append(out, Job{
			ID:       j.ID,
			Name:     trimMeta(filepath.Base(j.MetaPath)),
			Status:   JobStatus(j.Status),
			Progress: progress,
			Speed:    "",
		})
	}

	return out
}

func trimMeta(name string) string {
	const suffix = ".warp.meta.json"
	if strings.HasSuffix(name, suffix) {
		return strings.TrimSuffix(name, suffix)
	}
	return name
}
