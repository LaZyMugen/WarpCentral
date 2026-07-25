package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/LaZyMugen/warpcentral/internal/daemon"
)

var (
	// Programmer Neon Palette (Pink/Purple/Cyan/Emerald on Dark Slate)
	rosePink     = lipgloss.Color("#f43f5e") // Soft Neon Pink
	purpleAccent = lipgloss.Color("#c084fc") // Lavender Purple
	cyanAccent   = lipgloss.Color("#38bdf8") // Sky Cyan
	greenAccent  = lipgloss.Color("#22c55e") // Programmer Green
	grayColor    = lipgloss.Color("#71717a") // Muted Slate Gray
	darkBorder   = lipgloss.Color("#334155") // Structural Border

	roseStyle    = lipgloss.NewStyle().Foreground(rosePink).Bold(true)
	purpleStyle  = lipgloss.NewStyle().Foreground(purpleAccent).Bold(true)
	cyanStyle    = lipgloss.NewStyle().Foreground(cyanAccent).Bold(true)
	greenStyle   = lipgloss.NewStyle().Foreground(greenAccent).Bold(true)
	grayStyle    = lipgloss.NewStyle().Foreground(grayColor)
)

// RenderBtopBox draws a btop-style box with title embedded directly into top border.
// Example: ╭────────────── Server ──────────────╮
func RenderBtopBox(leftTitle, rightTitle string, content string, width, height int, borderColor lipgloss.Color) string {
	const (
		topLeft     = "╭"
		topRight    = "╮"
		bottomLeft  = "╰"
		bottomRight = "╯"
		horizontal  = "─"
		vertical    = "│"
	)

	innerWidth := width - 2
	if innerWidth < 1 {
		innerWidth = 1
	}

	leftTitleWidth := lipgloss.Width(leftTitle)
	rightTitleWidth := lipgloss.Width(rightTitle)

	borderStyler := lipgloss.NewStyle().Foreground(borderColor)

	var topBorder string
	if leftTitle != "" && rightTitle != "" {
		rem := innerWidth - leftTitleWidth - rightTitleWidth - 2
		if rem < 0 {
			rem = 0
		}
		topBorder = borderStyler.Render(topLeft+horizontal) + leftTitle + borderStyler.Render(strings.Repeat(horizontal, rem)) + rightTitle + borderStyler.Render(horizontal+topRight)
	} else if leftTitle != "" {
		rem := innerWidth - leftTitleWidth - 1
		if rem < 0 {
			rem = 0
		}
		topBorder = borderStyler.Render(topLeft+horizontal) + leftTitle + borderStyler.Render(strings.Repeat(horizontal, rem)+topRight)
	} else if rightTitle != "" {
		rem := innerWidth - rightTitleWidth - 1
		if rem < 0 {
			rem = 0
		}
		topBorder = borderStyler.Render(topLeft+strings.Repeat(horizontal, rem)) + rightTitle + borderStyler.Render(horizontal+topRight)
	} else {
		topBorder = borderStyler.Render(topLeft + strings.Repeat(horizontal, innerWidth) + topRight)
	}

	bottomBorder := borderStyler.Render(bottomLeft + strings.Repeat(horizontal, innerWidth) + bottomRight)

	contentLines := strings.Split(content, "\n")
	innerHeight := height - 2
	if innerHeight < 1 {
		innerHeight = 1
	}

	truncStyle := lipgloss.NewStyle().MaxWidth(innerWidth)

	var wrappedLines []string
	for i := 0; i < innerHeight; i++ {
		line := ""
		if i < len(contentLines) {
			line = contentLines[i]
		}
		w := lipgloss.Width(line)
		if w < innerWidth {
			line = line + strings.Repeat(" ", innerWidth-w)
		} else if w > innerWidth {
			line = truncStyle.Render(line)
			w = lipgloss.Width(line)
			if w < innerWidth {
				line = line + strings.Repeat(" ", innerWidth-w)
			}
		}
		wrappedLines = append(wrappedLines, borderStyler.Render(vertical)+line+borderStyler.Render(vertical))
	}

	return lipgloss.JoinVertical(lipgloss.Left, topBorder, strings.Join(wrappedLines, "\n"), bottomBorder)
}

func formatBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for v := n / unit; v >= unit; v /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.2f %cB", float64(n)/float64(div), "KMGTPE"[exp])
}

func (m Model) View() string {
	if !m.ready {
		return "Initializing WarpCentral Telemetry Console..."
	}

	w := m.width
	if w < 90 {
		w = 90
	}
	h := m.height
	if h < 24 {
		h = 24
	}

	topHeight := 10
	bottomHeight := h - topHeight - 2

	serverWidth := 34
	logWidth := (w - serverWidth) / 2
	netWidth := w - serverWidth - logWidth

	serverPanel := renderServerPanel(m.daemonState, serverWidth, topHeight)
	logPanel := renderLogPanel(m.logs, logWidth, topHeight)
	netPanel := renderNetworkPanel(m.daemonState, netWidth, topHeight)

	topRow := lipgloss.JoinHorizontal(lipgloss.Top, serverPanel, logPanel, netPanel)

	leftWidth := w / 2
	rightWidth := w - leftWidth

	downloadsPanel := renderDownloadsPanel(m, leftWidth, bottomHeight)
	rightSide := renderRightSidePanel(m, rightWidth, bottomHeight)

	bottomRow := lipgloss.JoinHorizontal(lipgloss.Top, downloadsPanel, rightSide)

	footer := renderFooterBar(m, w)

	fullView := lipgloss.JoinVertical(lipgloss.Left, topRow, bottomRow, footer)

	if m.inputMode {
		return renderInputModal(m, w, h)
	}

	return fullView
}

func renderServerPanel(st DaemonState, width, height int) string {
	logoLine1 := roseStyle.Render("  __ _  _  _  ___")
	logoLine2 := roseStyle.Render(" / || || || \\| _ \\")
	logoLine3 := roseStyle.Render("/ // // // \\\\  __/")
	logoLine4 := roseStyle.Render("\\_//_//_/\\_//_|") + " " + purpleStyle.Render("CENTRAL")

	statusLight := grayStyle.Render("● Offline")
	if st.IsOnline {
		statusLight = greenStyle.Render("● Serving at " + st.ServingAt)
	}

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		logoLine1,
		logoLine2,
		logoLine3,
		logoLine4,
		"",
		statusLight,
	)

	title := lipgloss.NewStyle().Foreground(rosePink).Bold(true).Render(" Server ")
	return RenderBtopBox(title, "", content, width, height, darkBorder)
}

func renderLogPanel(logs []daemon.LogEntry, width, height int) string {
	var lines []string
	maxLines := height - 3
	if maxLines < 1 {
		maxLines = 1
	}

	start := 0
	if len(logs) > maxLines {
		start = len(logs) - maxLines
	}

	for i := start; i < len(logs); i++ {
		l := logs[i]
		msg := l.Message
		lines = append(lines, fmt.Sprintf("%s %s", grayStyle.Render("["+l.Timestamp+"]"), msg))
	}

	if len(lines) == 0 {
		lines = append(lines, grayStyle.Render("No recent activity logs."))
	}

	title := lipgloss.NewStyle().Foreground(cyanAccent).Bold(true).Render(" Activity Log ")
	return RenderBtopBox(title, "", strings.Join(lines, "\n"), width, height, darkBorder)
}

func renderNetworkPanel(st DaemonState, width, height int) string {
	curSpeedMB := st.TotalSpeedBps / (1024 * 1024)
	topSpeedMB := st.TopSpeedBps / (1024 * 1024)

	header := fmt.Sprintf("▼ %s\nTop: %.2f MB/s\nTotal: %.1f MB",
		roseStyle.Render(fmt.Sprintf("%.2f MB/s", curSpeedMB)),
		topSpeedMB,
		st.TotalSessionMB,
	)

	bars := []string{" ", "▂", "▃", "▄", "▅", "▆", "▇", "█"}
	hist := st.SpeedHistory
	if len(hist) == 0 {
		hist = make([]float64, 15)
	}

	chartWidth := width - 20
	if chartWidth < 5 {
		chartWidth = 5
	}

	if len(hist) > chartWidth {
		hist = hist[len(hist)-chartWidth:]
	}

	var graphBars []string
	maxVal := topSpeedMB
	if maxVal <= 0 {
		maxVal = 1.0
	}

	for _, v := range hist {
		vMB := v / (1024 * 1024)
		idx := int((vMB / maxVal) * float64(len(bars)-1))
		if idx < 0 {
			idx = 0
		}
		if idx >= len(bars) {
			idx = len(bars) - 1
		}
		graphBars = append(graphBars, roseStyle.Render(bars[idx]))
	}

	chartStr := strings.Join(graphBars, "")
	rightContent := fmt.Sprintf("%.1f MB/s\n%s\n0 MB/s", maxVal, chartStr)

	row := lipgloss.JoinHorizontal(lipgloss.Top,
		lipgloss.NewStyle().Width(16).Render(header),
		lipgloss.NewStyle().Render(rightContent),
	)

	title := lipgloss.NewStyle().Foreground(purpleAccent).Bold(true).Render(" Network Activity ")
	return RenderBtopBox(title, "", row, width, height, darkBorder)
}

func renderDownloadsPanel(m Model, width, height int) string {
	tab1 := fmt.Sprintf("Queued (%d)", m.countTab(TabQueued))
	tab2 := fmt.Sprintf("Active (%d)", m.countTab(TabActive))
	tab3 := fmt.Sprintf("Done (%d)", m.countTab(TabDone))

	tabBox := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(darkBorder).
		Padding(0, 1).
		Foreground(grayColor)

	activeTabBox := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(rosePink).
		Padding(0, 1).
		Foreground(rosePink).
		Bold(true)

	t1, t2, t3 := tabBox.Render(tab1), tabBox.Render(tab2), tabBox.Render(tab3)
	switch m.activeTab {
	case TabQueued:
		t1 = activeTabBox.Render(tab1)
	case TabActive:
		t2 = activeTabBox.Render(tab2)
	case TabDone:
		t3 = activeTabBox.Render(tab3)
	}

	tabsRow := lipgloss.JoinHorizontal(lipgloss.Left, t1, " ", t2, " ", t3)

	filtered := m.filteredJobs()
	var listLines []string

	maxList := (height - 6) / 2
	if maxList < 1 {
		maxList = 1
	}

	for i, job := range filtered {
		if i >= maxList {
			break
		}
		cursor := "  "
		itemStyle := lipgloss.NewStyle()
		if i == m.cursor {
			cursor = "▌ "
			itemStyle = roseStyle
		}

		icon := "⬇"
		statusStyle := roseStyle
		if job.Status == StatusPaused {
			icon = "⏸"
			statusStyle = purpleStyle
		} else if job.Status == StatusDone {
			icon = "✔"
			statusStyle = greenStyle
		}

		percent := int(job.Progress * 100)
		name := job.Name

		line1 := itemStyle.Render(fmt.Sprintf("%s%s", cursor, name))
		line2 := fmt.Sprintf("   %s %s • %d%% • %s • %s / %s",
			icon,
			statusStyle.Render(string(job.Status)),
			percent,
			job.Speed,
			formatBytes(job.Downloaded),
			formatBytes(job.TotalSize),
		)

		listLines = append(listLines, line1, grayStyle.Render(line2))
	}

	if len(filtered) == 0 {
		listLines = append(listLines, "", grayStyle.Render("  No transfers in this queue section."))
	}

	content := lipgloss.JoinVertical(lipgloss.Left, tabsRow, "", strings.Join(listLines, "\n"))

	title := lipgloss.NewStyle().Foreground(rosePink).Bold(true).Render(" Downloads ")
	return RenderBtopBox(title, "", content, width, height, darkBorder)
}

func renderRightSidePanel(m Model, width, height int) string {
	filtered := m.filteredJobs()
	var selected *Job
	if len(filtered) > 0 && m.cursor < len(filtered) {
		selected = &filtered[m.cursor]
	}

	detailsHeight := (height / 2) + 1
	chunkHeight := height - detailsHeight

	detailsView := renderFileDetails(selected, width, detailsHeight)
	chunkView := renderChunkMap(selected, width, chunkHeight)

	return lipgloss.JoinVertical(lipgloss.Left, detailsView, chunkView)
}

func renderFileDetails(j *Job, width, height int) string {
	if j == nil {
		title := lipgloss.NewStyle().Foreground(purpleAccent).Bold(true).Render(" File Details ")
		return RenderBtopBox(title, "", grayStyle.Render("\n Select a download to inspect details."), width, height, darkBorder)
	}

	statusHeader := greenStyle.Render("⬇ Downloading")
	if j.Status == StatusPaused {
		statusHeader = purpleStyle.Render("⏸ Paused")
	} else if j.Status == StatusDone {
		statusHeader = greenStyle.Render("✔ Completed")
	}

	barWidth := width - 18
	if barWidth < 8 {
		barWidth = 8
	}
	filled := int(j.Progress * float64(barWidth))
	if filled > barWidth {
		filled = barWidth
	}
	empty := barWidth - filled
	if empty < 0 {
		empty = 0
	}

	bar := roseStyle.Render(strings.Repeat("█", filled)) + grayStyle.Render(strings.Repeat("░", empty))
	progressLine := fmt.Sprintf("Progress: [%s] %d%%", bar, int(j.Progress*100))

	metaText := fmt.Sprintf(
		"URL:  %s\nFile: %s\nPath: %s\nID:   %s",
		truncate(j.URL, width-8),
		truncate(j.Name, width-8),
		truncate(j.OutPath, width-8),
		truncate(j.ID, width-8),
	)

	gridText := fmt.Sprintf(
		"Size:  %s / %s    Time: %s\nSpeed: %s           ETA:  %s    Conns: %d",
		formatBytes(j.Downloaded), formatBytes(j.TotalSize), j.TimeElapsed,
		j.Speed, j.ETA, j.Conns,
	)

	content := lipgloss.JoinVertical(lipgloss.Left,
		statusHeader,
		metaText,
		progressLine,
		gridText,
	)

	title := lipgloss.NewStyle().Foreground(purpleAccent).Bold(true).Render(" File Details ")
	return RenderBtopBox(title, "", content, width, height, darkBorder)
}

func renderChunkMap(j *Job, width, height int) string {
	title := lipgloss.NewStyle().Foreground(cyanAccent).Bold(true).Render(" Chunk Map ")
	if j == nil || len(j.Chunks) == 0 {
		return RenderBtopBox(title, "", grayStyle.Render("No chunk telemetry."), width, height, darkBorder)
	}

	var blockStrs []string
	for _, c := range j.Chunks {
		switch c.Status {
		case "completed":
			blockStrs = append(blockStrs, greenStyle.Render("█"))
		case "downloading":
			blockStrs = append(blockStrs, roseStyle.Render("■"))
		default: // pending
			if c.DoneBytes > 0 {
				blockStrs = append(blockStrs, purpleStyle.Render("▄"))
			} else {
				blockStrs = append(blockStrs, grayStyle.Render("░"))
			}
		}
	}

	maxPerLine := (width - 4) / 2
	if maxPerLine < 4 {
		maxPerLine = 4
	}

	var lines []string
	for i := 0; i < len(blockStrs); i += maxPerLine {
		end := i + maxPerLine
		if end > len(blockStrs) {
			end = len(blockStrs)
		}
		lines = append(lines, strings.Join(blockStrs[i:end], " "))
	}

	return RenderBtopBox(title, "", strings.Join(lines, "\n"), width, height, darkBorder)
}

func renderFooterBar(m Model, width int) string {
	keys := " [a] Add URL   [p] Pause   [r] Resume   [d] Delete   [1-3/Tab] Switch Tabs   [q] Quit"
	return lipgloss.NewStyle().
		Width(width).
		Background(darkBorder).
		Foreground(rosePink).
		Bold(true).
		Render(keys)
}

func renderInputModal(m Model, width, height int) string {
	content := fmt.Sprintf("\nEnter Download URL:\n\n> %s_\n\n(Press Enter to submit, Esc to cancel)", m.urlInput)
	title := lipgloss.NewStyle().Foreground(rosePink).Bold(true).Render(" Add Download URL ")
	box := RenderBtopBox(title, "", content, 60, 9, rosePink)
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, box)
}

func truncate(s string, max int) string {
	if max <= 3 {
		return s
	}
	if len(s) > max {
		return s[:max-3] + "..."
	}
	return s
}
