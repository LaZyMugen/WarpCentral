package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/LaZyMugen/warpcentral/internal/daemon"
)

type CategoryTab int

const (
	TabQueued CategoryTab = 0
	TabActive CategoryTab = 1
	TabDone   CategoryTab = 2
)

type Model struct {
	width       int
	height      int
	ready       bool
	activeTab   CategoryTab
	cursor      int
	jobs        []Job
	daemonState DaemonState
	logs        []daemon.LogEntry
	inputMode   bool
	urlInput    string
	port        int
}

func New() Model {
	port := 1700
	dState := fetchDaemonStatus(port)
	jobs, fromDaemon := loadJobsFromDaemon(port)
	if !fromDaemon {
		jobs = loadJobsFallback()
	}
	logs := fetchDaemonLogs(port)

	return Model{
		activeTab:   TabActive,
		jobs:        jobs,
		daemonState: dState,
		logs:        logs,
		port:        port,
	}
}

func NewWithPort(port int) Model {
	if port <= 0 {
		port = 1700
	}
	m := New()
	m.port = port
	return m
}

func (m Model) Init() tea.Cmd {
	return tick()
}
