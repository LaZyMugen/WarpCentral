package tui

type PauseJobMsg struct {
	JobID string
}

type ResumeJobMsg struct {
	JobID string
}

type DeleteJobMsg struct {
	JobID string
}

type AddJobMsg struct {
	URL     string
	OutPath string
}
