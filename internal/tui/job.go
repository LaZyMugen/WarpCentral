package tui

import "github.com/LaZyMugen/warpcentral/internal/resume"

type JobStatus string

const (
	StatusQueued   JobStatus = "queued"
	StatusActive   JobStatus = "active"
	StatusPaused   JobStatus = "paused"
	StatusDone     JobStatus = "done"
	StatusFailed   JobStatus = "failed"
)

type Job struct {
	ID           string
	Name         string
	URL          string
	OutPath      string
	MetaPath     string
	Status       JobStatus
	Progress     float64 // 0.0 → 1.0
	Speed        string  // preformatted
	SpeedBps     float64
	Downloaded   int64
	TotalSize    int64
	Conns        int
	TimeElapsed  string
	ETA          string
	Error        string
	Chunks       []resume.ChunkState
}
