package tui

import tea "github.com/charmbracelet/bubbletea"

type Model struct {
	width   int
	height  int
	ready   bool
	jobs    []Job
	cursor  int // selected row
}


func New() Model {
	return Model{
		jobs: []Job{
			// TEMP fake data (next step we load real jobs)
			{ID: "1", Name: "sample-30s.mp4", Status: StatusActive, Progress: 0.42, Speed: "3.2 MB/s"},
			{ID: "2", Name: "100Mb.dat", Status: StatusDone, Progress: 1.0},
			{ID: "3", Name: "ubuntu.iso", Status: StatusPaused, Progress: 0.73},
		},
	}
}


func (m Model) Init() tea.Cmd {
	return nil
}
