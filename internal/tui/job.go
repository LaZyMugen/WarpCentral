package tui

type JobStatus string

const (
	StatusQueued   JobStatus = "queued"
	StatusActive   JobStatus = "active"
	StatusPaused   JobStatus = "paused"
	StatusDone     JobStatus = "done"
	StatusFailed   JobStatus = "failed"
)

type Job struct {
	ID       string
	Name     string
	Status   JobStatus
	Progress float64 // 0.0 → 1.0
	Speed    string  // preformatted for now
}
