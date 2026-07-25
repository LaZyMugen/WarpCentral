package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/LaZyMugen/warpcentral/internal/downloader"
	qstore "github.com/LaZyMugen/warpcentral/internal/queue"
	"github.com/LaZyMugen/warpcentral/internal/resume"
	"github.com/LaZyMugen/warpcentral/internal/storage"
)

type LogEntry struct {
	Timestamp string `json:"timestamp"`
	Message   string `json:"message"`
}

type TaskDetail struct {
	ID                 string              `json:"id"`
	URL                string              `json:"url"`
	OutPath            string              `json:"outPath"`
	FileName           string              `json:"fileName"`
	MetaPath           string              `json:"metaPath"`
	TotalSize          int64               `json:"totalSize"`
	Downloaded         int64               `json:"downloaded"`
	SpeedBps           float64             `json:"speedBps"`
	SpeedFormatted     string              `json:"speedFormatted"`
	Progress           float64             `json:"progress"` // 0.0 to 1.0
	Status             string              `json:"status"`   // queued, active, paused, done, failed
	Error              string              `json:"error,omitempty"`
	Conns              int                 `json:"conns"`
	TimeElapsedSeconds int64               `json:"timeElapsedSeconds"`
	ETASeconds         int64               `json:"etaSeconds"`
	ETAFormatted       string              `json:"etaFormatted"`
	Chunks             []resume.ChunkState `json:"chunks"`
}

type SystemStatus struct {
	ServingAt       string    `json:"servingAt"`
	Port            int       `json:"port"`
	UptimeSeconds   int64     `json:"uptimeSeconds"`
	ActiveDownloads int       `json:"activeDownloads"`
	TotalSpeedBps   float64   `json:"totalSpeedBps"`
	TopSpeedBps     float64   `json:"topSpeedBps"`
	TotalSessionMB  float64   `json:"totalSessionMb"`
	SpeedHistory    []float64 `json:"speedHistory"`
}

type Daemon struct {
	port          int
	listener      net.Listener
	server        *http.Server
	startTime     time.Time
	mu            sync.RWMutex
	logs          []LogEntry
	speedHistory  []float64
	topSpeedBps   float64
	totalSession  int64
	activeCancels map[string]context.CancelFunc
	taskSpeeds    map[string]float64
	taskStart     map[string]time.Time
	downloader    *downloader.Downloader
}

func New(port int) *Daemon {
	if port <= 0 {
		port = 1700
	}
	return &Daemon{
		port:          port,
		startTime:     time.Now(),
		logs:          make([]LogEntry, 0, 100),
		speedHistory:  make([]float64, 30),
		activeCancels: make(map[string]context.CancelFunc),
		taskSpeeds:    make(map[string]float64),
		taskStart:     make(map[string]time.Time),
		downloader:    downloader.New(),
	}
}

func (d *Daemon) Log(msg string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	ts := time.Now().Format("15:04:05")
	entry := LogEntry{Timestamp: ts, Message: msg}
	d.logs = append(d.logs, entry)
	if len(d.logs) > 100 {
		d.logs = d.logs[len(d.logs)-100:]
	}
	log.Printf("[%s] %s", ts, msg)
}

func (d *Daemon) Start() error {
	addr := fmt.Sprintf("127.0.0.1:%d", d.port)
	l, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to bind port %d: %w", d.port, err)
	}
	d.listener = l

	mux := http.NewServeMux()
	mux.HandleFunc("/api/status", d.handleStatus)
	mux.HandleFunc("/api/tasks", d.handleTasks)
	mux.HandleFunc("/api/tasks/add", d.handleTaskAdd)
	mux.HandleFunc("/api/tasks/pause", d.handleTaskPause)
	mux.HandleFunc("/api/tasks/resume", d.handleTaskResume)
	mux.HandleFunc("/api/tasks/delete", d.handleTaskDelete)
	mux.HandleFunc("/api/logs", d.handleLogs)

	d.server = &http.Server{
		Handler: mux,
	}

	d.Log(fmt.Sprintf("WarpCentral Daemon serving at %s", addr))

	// Background metrics sampler
	go d.startMetricsSampler()

	return d.server.Serve(l)
}

func (d *Daemon) Stop() error {
	if d.server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		return d.server.Shutdown(ctx)
	}
	return nil
}

func (d *Daemon) startMetricsSampler() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		d.mu.Lock()
		var currentTotalSpeed float64
		for _, speed := range d.taskSpeeds {
			currentTotalSpeed += speed
		}

		if currentTotalSpeed > d.topSpeedBps {
			d.topSpeedBps = currentTotalSpeed
		}

		d.speedHistory = append(d.speedHistory[1:], currentTotalSpeed)
		d.mu.Unlock()
	}
}

func (d *Daemon) handleStatus(w http.ResponseWriter, r *http.Request) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	activeCount := len(d.activeCancels)
	var currentSpeed float64
	for _, s := range d.taskSpeeds {
		currentSpeed += s
	}

	st := SystemStatus{
		ServingAt:       fmt.Sprintf("127.0.0.1:%d", d.port),
		Port:            d.port,
		UptimeSeconds:   int64(time.Since(d.startTime).Seconds()),
		ActiveDownloads: activeCount,
		TotalSpeedBps:   currentSpeed,
		TopSpeedBps:     d.topSpeedBps,
		TotalSessionMB:  float64(d.totalSession) / (1024 * 1024),
		SpeedHistory:    d.speedHistory,
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(st)
}

func (d *Daemon) handleLogs(w http.ResponseWriter, r *http.Request) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(d.logs)
}

func (d *Daemon) handleTasks(w http.ResponseWriter, r *http.Request) {
	jobs, err := storage.LoadJobs()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	d.mu.RLock()
	taskSpeeds := make(map[string]float64)
	for k, v := range d.taskSpeeds {
		taskSpeeds[k] = v
	}
	taskStarts := make(map[string]time.Time)
	for k, v := range d.taskStart {
		taskStarts[k] = v
	}
	d.mu.RUnlock()

	details := make([]TaskDetail, 0, len(jobs))
	for _, j := range jobs {
		metaPath := j.MetaPath
		meta, err := resume.Load(metaPath)
		if err != nil {
			continue
		}

		var doneBytes int64
		conns := meta.Parts
		if conns <= 0 {
			conns = 1
		}

		for _, c := range meta.Chunks {
			doneBytes += c.DoneBytes
		}

		progress := 0.0
		if meta.TotalSize > 0 {
			progress = float64(doneBytes) / float64(meta.TotalSize)
		}
		if progress > 1.0 {
			progress = 1.0
		}

		speedBps := taskSpeeds[metaPath]
		speedFormatted := formatSpeed(speedBps)

		var etaSec int64 = 0
		var elapsedSec int64 = 0
		if start, ok := taskStarts[metaPath]; ok {
			elapsedSec = int64(time.Since(start).Seconds())
		}

		if speedBps > 0 && meta.TotalSize > doneBytes {
			etaSec = int64(float64(meta.TotalSize-doneBytes) / speedBps)
		}

		fileName := filepath.Base(meta.OutPath)
		if fileName == "" || fileName == "." {
			fileName = trimMeta(filepath.Base(metaPath))
		}

		status := meta.Status
		if status == "" {
			if progress >= 1.0 {
				status = "done"
			} else if doneBytes > 0 {
				status = "paused"
			} else {
				status = "queued"
			}
		}

		details = append(details, TaskDetail{
			ID:                 metaPath,
			URL:                meta.URL,
			OutPath:            meta.OutPath,
			FileName:           fileName,
			MetaPath:           metaPath,
			TotalSize:          meta.TotalSize,
			Downloaded:         doneBytes,
			SpeedBps:           speedBps,
			SpeedFormatted:     speedFormatted,
			Progress:           progress,
			Status:             status,
			Error:              meta.Error,
			Conns:              conns,
			TimeElapsedSeconds: elapsedSec,
			ETASeconds:         etaSec,
			ETAFormatted:       formatDuration(etaSec),
			Chunks:             meta.Chunks,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(details)
}

type AddTaskReq struct {
	URL     string `json:"url"`
	OutPath string `json:"outPath,omitempty"`
}

func (d *Daemon) handleTaskAdd(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req AddTaskReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if req.URL == "" {
		http.Error(w, "URL is required", http.StatusBadRequest)
		return
	}

	outPath := req.OutPath
	if outPath == "" {
		outPath = downloader.GuessFileNameFromURL(req.URL)
	}

	item, err := qstore.Add(qstore.DefaultPath(), req.URL, outPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	d.Log(fmt.Sprintf("Queued download: %s", req.URL))

	// Auto-trigger background download run
	go d.startTaskDownload(req.URL, outPath)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(item)
}

type TaskActionReq struct {
	ID string `json:"id"`
}

func (d *Daemon) handleTaskPause(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req TaskActionReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	d.mu.Lock()
	cancel, active := d.activeCancels[req.ID]
	if active {
		cancel()
		delete(d.activeCancels, req.ID)
		delete(d.taskSpeeds, req.ID)
	}
	d.mu.Unlock()

	// Set status to paused in meta
	meta, err := resume.Load(req.ID)
	if err == nil {
		meta.Status = "paused"
		_ = resume.Save(req.ID, meta)
	}

	d.Log(fmt.Sprintf("Paused download: %s", req.ID))
	w.WriteHeader(http.StatusOK)
}

func (d *Daemon) handleTaskResume(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req TaskActionReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	metaPath := req.ID
	meta, err := resume.Load(metaPath)
	if err != nil {
		http.Error(w, "Meta file not found: "+err.Error(), http.StatusNotFound)
		return
	}

	d.Log(fmt.Sprintf("Resuming download: %s", meta.OutPath))
	go d.resumeTaskDownload(metaPath)

	w.WriteHeader(http.StatusOK)
}

func (d *Daemon) handleTaskDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req TaskActionReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	d.mu.Lock()
	if cancel, ok := d.activeCancels[req.ID]; ok {
		cancel()
		delete(d.activeCancels, req.ID)
		delete(d.taskSpeeds, req.ID)
	}
	d.mu.Unlock()

	metaPath := req.ID
	meta, err := resume.Load(metaPath)
	if err == nil {
		_ = os.Remove(meta.OutPath)
	}
	_ = os.Remove(metaPath)

	d.Log(fmt.Sprintf("Deleted download: %s", req.ID))
	w.WriteHeader(http.StatusOK)
}

func (d *Daemon) startTaskDownload(rawURL, outPath string) {
	if outPath == "" {
		outPath = downloader.GuessFileNameFromURL(rawURL)
	}
	metaPath := resume.MetaPath(outPath)

	// If meta file already exists, delegate to resumeTaskDownload to prevent truncating file
	if _, err := os.Stat(metaPath); err == nil {
		d.resumeTaskDownload(metaPath)
		return
	}

	ctx, cancel := context.WithCancel(context.Background())

	d.mu.Lock()
	d.activeCancels[metaPath] = cancel
	d.taskStart[metaPath] = time.Now()
	d.mu.Unlock()

	defer func() {
		d.mu.Lock()
		delete(d.activeCancels, metaPath)
		delete(d.taskSpeeds, metaPath)
		d.mu.Unlock()
	}()

	err := d.downloader.DownloadSmart(ctx, rawURL, outPath, func(p downloader.Progress) {
		d.mu.Lock()
		d.taskSpeeds[metaPath] = p.SpeedBps
		d.totalSession += int64(p.SpeedBps / 2) // approximate sampling accumulation
		d.mu.Unlock()
	})

	if err != nil {
		if errors.Is(err, context.Canceled) || ctx.Err() == context.Canceled {
			d.Log(fmt.Sprintf("Paused download: %s", outPath))
			if meta, mErr := resume.Load(metaPath); mErr == nil {
				meta.Status = "paused"
				meta.Error = ""
				_ = resume.Save(metaPath, meta)
			}
		} else {
			d.Log(fmt.Sprintf("Failed download %s: %v", rawURL, err))
			if meta, mErr := resume.Load(metaPath); mErr == nil {
				meta.Status = "failed"
				meta.Error = err.Error()
				_ = resume.Save(metaPath, meta)
			}
		}
	} else {
		d.Log(fmt.Sprintf("Finished download %s", rawURL))
		if meta, mErr := resume.Load(metaPath); mErr == nil {
			meta.Status = "done"
			meta.Error = ""
			_ = resume.Save(metaPath, meta)
		}
	}
}

func (d *Daemon) resumeTaskDownload(metaPath string) {
	ctx, cancel := context.WithCancel(context.Background())

	d.mu.Lock()
	d.activeCancels[metaPath] = cancel
	d.taskStart[metaPath] = time.Now()
	d.mu.Unlock()

	defer func() {
		d.mu.Lock()
		delete(d.activeCancels, metaPath)
		delete(d.taskSpeeds, metaPath)
		d.mu.Unlock()
	}()

	err := d.downloader.ResumeFromMeta(ctx, metaPath, func(p downloader.Progress) {
		d.mu.Lock()
		d.taskSpeeds[metaPath] = p.SpeedBps
		d.totalSession += int64(p.SpeedBps / 2)
		d.mu.Unlock()
	})

	if err != nil {
		if errors.Is(err, context.Canceled) || ctx.Err() == context.Canceled {
			d.Log(fmt.Sprintf("Paused download: %s", metaPath))
			if meta, mErr := resume.Load(metaPath); mErr == nil {
				meta.Status = "paused"
				meta.Error = ""
				_ = resume.Save(metaPath, meta)
			}
		} else {
			d.Log(fmt.Sprintf("Failed resume %s: %v", metaPath, err))
			if meta, mErr := resume.Load(metaPath); mErr == nil {
				meta.Status = "failed"
				meta.Error = err.Error()
				_ = resume.Save(metaPath, meta)
			}
		}
	} else {
		d.Log(fmt.Sprintf("Completed download %s", metaPath))
		if meta, mErr := resume.Load(metaPath); mErr == nil {
			meta.Status = "done"
			meta.Error = ""
			_ = resume.Save(metaPath, meta)
		}
	}
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

func formatSpeed(bps float64) string {
	if bps < 1024 {
		return fmt.Sprintf("%.0f B/s", bps)
	}
	if bps < 1024*1024 {
		return fmt.Sprintf("%.2f KB/s", bps/1024)
	}
	return fmt.Sprintf("%.2f MB/s", bps/(1024*1024))
}

func formatDuration(sec int64) string {
	if sec <= 0 {
		return "0:00"
	}
	m := sec / 60
	s := sec % 60
	return fmt.Sprintf("%d:%02d", m, s)
}

func trimMeta(name string) string {
	const suffix = ".warp.meta.json"
	if strings.HasSuffix(name, suffix) {
		return strings.TrimSuffix(name, suffix)
	}
	return name
}
