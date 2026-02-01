package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// tick sends a TickMsg every second
func tick() tea.Cmd {
	return tea.Tick(time.Second, func(time.Time) tea.Msg {
		return TickMsg{}
	})
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit

		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
			return m, nil

		case "down", "j":
			if m.cursor < len(m.jobs)-1 {
				m.cursor++
			}
			return m, nil
		}

	case TickMsg:
		// Reload jobs + meta safely (read-only)
		m.jobs = loadJobs()

		// Clamp cursor in case job list changed
		if m.cursor >= len(m.jobs) {
			m.cursor = max(0, len(m.jobs)-1)
		}

		return m, tick()
	}

	return m, nil
}

// small helper
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
