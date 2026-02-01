package tui

import "fmt"

func (m Model) View() string {
	if !m.ready {
		return "Initializing WarpCentral..."
	}

	if len(m.jobs) == 0 {
		return "WarpCentral\n\nNo downloads found.\n\nPress q to quit."
	}

	out := "WarpCentral\n\n"

	for i, job := range m.jobs {
		cursor := " "
		if i == m.cursor {
			cursor = ">"
		}

		status := string(job.Status)
		percent := int(job.Progress * 100)

		line := fmt.Sprintf(
			"%s %-20s [%3d%%] %-8s %s\n",
			cursor,
			job.Name,
			percent,
			status,
			job.Speed,
		)

		out += line
	}

	out += "\n↑/↓ select   q quit"

	return out
}
