package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/LaZyMugen/warpcentral/internal/daemon"
)

var (
	// Programmer Soft Neon Palette
	rosePink     = lipgloss.Color("#f43f5e") // Soft Neon Pink
	purpleAccent = lipgloss.Color("#c084fc") // Lavender Purple
	cyanAccent   = lipgloss.Color("#38bdf8") // Sky Cyan
	greenAccent  = lipgloss.Color("#22c55e") // Programmer Emerald Green
	grayColor    = lipgloss.Color("#64748b") // Slate Gray
	darkBorder   = lipgloss.Color("#334155") // Structural Border

	roseStyle   = lipgloss.NewStyle().Foreground(rosePink).Bold(true)
	purpleStyle = lipgloss.NewStyle().Foreground(purpleAccent).Bold(true)
	cyanStyle   = lipgloss.NewStyle().Foreground(cyanAccent).Bold(true)
	greenStyle  = lipgloss.NewStyle().Foreground(greenAccent).Bold(true)
	grayStyle   = lipgloss.NewStyle().Foreground(grayColor)
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

// renderEnergyProgressBar draws green slanted energy bars (▰▰▰▰▱▱▱) for completion progress
func renderEnergyProgressBar(progress float64, barWidth int) string {
	if barWidth < 4 {
		barWidth = 4
	}
	filled := int(progress * float64(barWidth))
	if filled > barWidth {
		filled = barWidth
	}
	empty := barWidth - filled
	if empty < 0 {
		empty = 0
	}

	filledBars := greenStyle.Render(strings.Repeat("▰", filled))
	emptyBars := grayStyle.Render(strings.Repeat("▱", empty))
	return filledBars + emptyBars
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
	if w < 95 {
		w = 95
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
	curMbps := (st.TotalSpeedBps * 8) / (1024 * 1024)
	topMbps := (st.TopSpeedBps * 8) / (1024 * 1024)

	// Inner Stats Card (Left Side)
	cardWidth := 19
	cardHeight := height - 2
	cardBoxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(darkBorder).
		Padding(0, 0)

	cardContent := fmt.Sprintf(
		" %s\n %s\n\n Top: %.2f\n %s\n\n Total: %.1f MB",
		roseStyle.Render(fmt.Sprintf("▼ %.2f MB/s", curSpeedMB)),
		grayStyle.Render(fmt.Sprintf("(%.0f Mbps)", curMbps)),
		topSpeedMB,
		grayStyle.Render(fmt.Sprintf("(%.0f Mbps)", topMbps)),
		st.TotalSessionMB,
	)
	statsCard := cardBoxStyle.Width(cardWidth).Height(cardHeight).Render(cardContent)

	// Multi-Line Graph Canvas (Right Side)
	graphWidth := width - cardWidth - 14
	if graphWidth < 6 {
		graphWidth = 6
	}
	graphHeight := height - 2
	if graphHeight < 4 {
		graphHeight = 4
	}

	maxVal := topSpeedMB
	if maxVal <= 0 {
		maxVal = 1.0
	}

	graphStr := renderMultiLineGraph(st.SpeedHistory, graphWidth, graphHeight, maxVal)

	// Y-Axis Scale Labels (Far Right Column)
	var scaleLines []string
	step := maxVal / float64(graphHeight-1)
	for i := 0; i < graphHeight; i++ {
		val := maxVal - (float64(i) * step)
		if val < 0 {
			val = 0
		}
		if i == graphHeight-1 {
			scaleLines = append(scaleLines, grayStyle.Render("    0 MB/s"))
		} else {
			scaleLines = append(scaleLines, grayStyle.Render(fmt.Sprintf("%5.1f MB/s", val)))
		}
	}
	scaleCol := strings.Join(scaleLines, "\n")

	graphRow := lipgloss.JoinHorizontal(lipgloss.Top,
		statsCard,
		" ",
		graphStr,
		" ",
		scaleCol,
	)

	title := lipgloss.NewStyle().Foreground(purpleAccent).Bold(true).Render(" Network Activity ")
	return RenderBtopBox(title, "", graphRow, width, height, darkBorder)
}

func renderMultiLineGraph(data []float64, width, height int, maxVal float64) string {
	if width < 1 || height < 1 {
		return ""
	}

	grid := make([][]string, height)
	for y := 0; y < height; y++ {
		grid[y] = make([]string, width)
		for x := 0; x < width; x++ {
			if y == height-1 {
				grid[y][x] = grayStyle.Render("─")
			} else if y%2 == 0 {
				grid[y][x] = grayStyle.Render("┄")
			} else {
				grid[y][x] = " "
			}
		}
	}

	blocks := []string{" ", " ", "▂", "▃", "▄", "▅", "▆", "▇", "█"}
	totalSubBlocks := float64(height * 8)

	colors := []lipgloss.Color{
		lipgloss.Color("#8b5cf6"), // Deep Purple
		lipgloss.Color("#c084fc"), // Lavender
		lipgloss.Color("#f43f5e"), // Neon Rose/Pink
	}

	// Align history data to fill graph width
	hist := data
	if len(hist) > width {
		hist = hist[len(hist)-width:]
	}

	for x := 0; x < width && x < len(hist); x++ {
		val := hist[x]
		if val <= 0 {
			continue
		}
		valMB := val / (1024 * 1024)
		pct := valMB / maxVal
		if pct > 1.0 {
			pct = 1.0
		}

		subBlocksToDraw := int(pct * totalSubBlocks)

		for y := height - 1; y >= 0; y-- {
			rowFromBottom := (height - 1) - y
			rowSubBlocks := subBlocksToDraw - (rowFromBottom * 8)
			if rowSubBlocks <= 0 {
				break
			}
			if rowSubBlocks > 8 {
				rowSubBlocks = 8
			}

			colorIdx := (rowFromBottom * len(colors)) / height
			if colorIdx >= len(colors) {
				colorIdx = len(colors) - 1
			}
			blockStyle := lipgloss.NewStyle().Foreground(colors[colorIdx])
			grid[y][x] = blockStyle.Render(blocks[rowSubBlocks])
		}
	}

	var lines []string
	for y := 0; y < height; y++ {
		lines = append(lines, strings.Join(grid[y], ""))
	}

	return strings.Join(lines, "\n")
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
		energyBar := renderEnergyProgressBar(job.Progress, 12)
		line2 := fmt.Sprintf("   %s %s • [%s] %d%% • %s • %s / %s",
			icon,
			statusStyle.Render(string(job.Status)),
			energyBar,
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

	energyBar := renderEnergyProgressBar(j.Progress, barWidth)
	progressLine := fmt.Sprintf("Progress: [%s] %d%%", energyBar, int(j.Progress*100))

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

func truncateANSI(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max])
}
