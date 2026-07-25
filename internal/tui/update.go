package tui

import (
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func tick() tea.Cmd {
	return tea.Tick(500*time.Millisecond, func(time.Time) tea.Msg {
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
		if m.inputMode {
			switch msg.String() {
			case "enter":
				urlStr := strings.TrimSpace(m.urlInput)
				m.inputMode = false
				m.urlInput = ""
				if urlStr != "" {
					return m, func() tea.Msg {
						return AddJobMsg{URL: urlStr, OutPath: ""}
					}
				}
				return m, nil

			case "esc":
				m.inputMode = false
				m.urlInput = ""
				return m, nil

			case "backspace":
				if len(m.urlInput) > 0 {
					m.urlInput = m.urlInput[:len(m.urlInput)-1]
				}
				return m, nil

			default:
				if len(msg.String()) == 1 {
					m.urlInput += msg.String()
				}
				return m, nil
			}
		}

		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit

		case "tab":
			m.activeTab = (m.activeTab + 1) % 3
			m.cursor = 0
			return m, nil

		case "shift+tab":
			if m.activeTab == 0 {
				m.activeTab = TabDone
			} else {
				m.activeTab--
			}
			m.cursor = 0
			return m, nil

		case "1":
			m.activeTab = TabQueued
			m.cursor = 0
			return m, nil
		case "2":
			m.activeTab = TabActive
			m.cursor = 0
			return m, nil
		case "3":
			m.activeTab = TabDone
			m.cursor = 0
			return m, nil

		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
			return m, nil

		case "down", "j":
			filtered := m.filteredJobs()
			if m.cursor < len(filtered)-1 {
				m.cursor++
			}
			return m, nil

		case "p":
			filtered := m.filteredJobs()
			if len(filtered) > 0 && m.cursor < len(filtered) {
				jobID := filtered[m.cursor].ID
				return m, func() tea.Msg {
					return PauseJobMsg{JobID: jobID}
				}
			}

		case "r":
			filtered := m.filteredJobs()
			if len(filtered) > 0 && m.cursor < len(filtered) {
				jobID := filtered[m.cursor].ID
				return m, func() tea.Msg {
					return ResumeJobMsg{JobID: jobID}
				}
			}

		case "d", "delete":
			filtered := m.filteredJobs()
			if len(filtered) > 0 && m.cursor < len(filtered) {
				jobID := filtered[m.cursor].ID
				return m, func() tea.Msg {
					return DeleteJobMsg{JobID: jobID}
				}
			}

		case "a":
			m.inputMode = true
			m.urlInput = ""
			return m, nil
		}

	case TickMsg:
		m.daemonState = fetchDaemonStatus(m.port)
		jobs, fromDaemon := loadJobsFromDaemon(m.port)
		if !fromDaemon {
			jobs = loadJobsFallback()
		}
		m.jobs = jobs
		m.logs = fetchDaemonLogs(m.port)

		filtered := m.filteredJobs()
		if m.cursor >= len(filtered) {
			m.cursor = max(0, len(filtered)-1)
		}
		return m, tick()

	case PauseJobMsg:
		go pauseJob(msg.JobID, m.port)
		return m, nil

	case ResumeJobMsg:
		go resumeJob(msg.JobID, m.port)
		return m, nil

	case DeleteJobMsg:
		go deleteJob(msg.JobID, m.port)
		return m, nil

	case AddJobMsg:
		go addJob(msg.URL, msg.OutPath, m.port)
		return m, nil
	}

	return m, nil
}

func (m Model) filteredJobs() []Job {
	out := make([]Job, 0)
	for _, j := range m.jobs {
		switch m.activeTab {
		case TabQueued:
			if j.Status == StatusQueued || j.Status == StatusPaused {
				out = append(out, j)
			}
		case TabActive:
			if j.Status == StatusActive {
				out = append(out, j)
			}
		case TabDone:
			if j.Status == StatusDone {
				out = append(out, j)
			}
		}
	}
	return out
}

func (m Model) countTab(tab CategoryTab) int {
	cnt := 0
	for _, j := range m.jobs {
		switch tab {
		case TabQueued:
			if j.Status == StatusQueued || j.Status == StatusPaused {
				cnt++
			}
		case TabActive:
			if j.Status == StatusActive {
				cnt++
			}
		case TabDone:
			if j.Status == StatusDone {
				cnt++
			}
		}
	}
	return cnt
}
