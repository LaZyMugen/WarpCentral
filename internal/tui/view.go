package tui

func (m Model) View() string {
	if !m.ready {
		return "Initializing WarpCentral..."
	}

	return "WarpCentral\n\nPress q to quit."
}
