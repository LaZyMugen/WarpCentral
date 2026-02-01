package tui

import tea "github.com/charmbracelet/bubbletea"

type Model struct {
	width  int
	height int
	ready  bool

	jobs   []Job
	cursor int // selected row
}

func New() Model {
	return Model{
		jobs: loadJobs(),
	}
}

func (m Model) Init() tea.Cmd {
	// Start periodic polling
	return tick()
}
