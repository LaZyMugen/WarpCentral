package tui

import (
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/LaZyMugen/warpcentral/internal/daemon"
	"github.com/LaZyMugen/warpcentral/internal/resume"
	"github.com/LaZyMugen/warpcentral/internal/storage"
)

type DaemonState struct {
	ServingAt       string
	Port            int
	UptimeSeconds   int64
	ActiveDownloads int
	TotalSpeedBps   float64
	TopSpeedBps     float64
	TotalSessionMB  float64
	SpeedHistory    []float64
	IsOnline        bool
}

func fetchDaemonStatus(port int) DaemonState {
	if port <= 0 {
		port = 1700
	}
	url := fmt.Sprintf("http://127.0.0.1:%d/api/status", port)
	client := http.Client{Timeout: 500 * time.Millisecond}
	resp, err := client.Get(url)
	if err != nil {
		return DaemonState{ServingAt: fmt.Sprintf("127.0.0.1:%d", port), Port: port, IsOnline: false}
	}
	defer resp.Body.Close()

	var st daemon.SystemStatus
	if err := json.NewDecoder(resp.Body).Decode(&st); err != nil {
		return DaemonState{ServingAt: fmt.Sprintf("127.0.0.1:%d", port), Port: port, IsOnline: false}
	}

	return DaemonState{
		ServingAt:       st.ServingAt,
		Port:            st.Port,
		UptimeSeconds:   st.UptimeSeconds,
		ActiveDownloads: st.ActiveDownloads,
		TotalSpeedBps:   st.TotalSpeedBps,
		TopSpeedBps:     st.TopSpeedBps,
		TotalSessionMB:  st.TotalSessionMB,
		SpeedHistory:    st.SpeedHistory,
		IsOnline:        true,
	}
}

func fetchDaemonLogs(port int) []daemon.LogEntry {
	if port <= 0 {
		port = 1700
	}
	url := fmt.Sprintf("http://127.0.0.1:%d/api/logs", port)
	client := http.Client{Timeout: 500 * time.Millisecond}
	resp, err := client.Get(url)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()

	var logs []daemon.LogEntry
	_ = json.NewDecoder(resp.Body).Decode(&logs)
	return logs
}

func loadJobsFromDaemon(port int) ([]Job, bool) {
	if port <= 0 {
		port = 1700
	}
	url := fmt.Sprintf("http://127.0.0.1:%d/api/tasks", port)
	client := http.Client{Timeout: 500 * time.Millisecond}
	resp, err := client.Get(url)
	if err != nil {
		return nil, false
	}
	defer resp.Body.Close()

	var details []daemon.TaskDetail
	if err := json.NewDecoder(resp.Body).Decode(&details); err != nil {
		return nil, false
	}

	out := make([]Job, 0, len(details))
	for _, d := range details {
		out = append(out, Job{
			ID:          d.ID,
			Name:        d.FileName,
			URL:         d.URL,
			OutPath:     d.OutPath,
			MetaPath:    d.MetaPath,
			Status:      JobStatus(d.Status),
			Progress:    d.Progress,
			Speed:       d.SpeedFormatted,
			SpeedBps:    d.SpeedBps,
			Downloaded:  d.Downloaded,
			TotalSize:   d.TotalSize,
			Conns:       d.Conns,
			TimeElapsed: formatSec(d.TimeElapsedSeconds),
			ETA:         d.ETAFormatted,
			Error:       d.Error,
			Chunks:      d.Chunks,
		})
	}

	return out, true
}

func loadJobsFallback() []Job {
	jobs, err := storage.LoadJobs()
	if err != nil {
		return nil
	}

	out := make([]Job, 0, len(jobs))
	for _, j := range jobs {
		meta, err := resume.Load(j.MetaPath)
		if err != nil {
			continue
		}

		var doneBytes int64
		for _, c := range meta.Chunks {
			doneBytes += c.DoneBytes
		}

		progress := 0.0
		if meta.TotalSize > 0 {
			progress = float64(doneBytes) / float64(meta.TotalSize)
		}

		if progress > 1 {
			progress = 1
		}
		if progress < 0 {
			progress = 0
		}

		conns := meta.Parts
		if conns <= 0 {
			conns = 1
		}

		fileName := filepath.Base(meta.OutPath)
		if fileName == "" || fileName == "." {
			fileName = trimMeta(filepath.Base(j.MetaPath))
		}

		out = append(out, Job{
			ID:          j.ID,
			Name:        fileName,
			URL:         meta.URL,
			OutPath:     meta.OutPath,
			MetaPath:    j.MetaPath,
			Status:      JobStatus(j.Status),
			Progress:    progress,
			Speed:       "0 B/s",
			Downloaded:  doneBytes,
			TotalSize:   meta.TotalSize,
			Conns:       conns,
			TimeElapsed: "0:00",
			ETA:         "0:00",
			Error:       meta.Error,
			Chunks:      meta.Chunks,
		})
	}

	return out
}

func trimMeta(name string) string {
	const suffix = ".warp.meta.json"
	if strings.HasSuffix(name, suffix) {
		return strings.TrimSuffix(name, suffix)
	}
	return name
}

func formatSec(sec int64) string {
	if sec <= 0 {
		return "0:00"
	}
	m := sec / 60
	s := sec % 60
	return fmt.Sprintf("%d:%02d", m, s)
}
