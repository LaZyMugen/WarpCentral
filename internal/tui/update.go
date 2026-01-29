package tui

import tea "github.com/charmbracelet/bubbletea"

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
		}

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

	return m, nil
}
