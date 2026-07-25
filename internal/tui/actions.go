package tui

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"sync"
	"time"
)

var (
	runningProcsMu sync.Mutex
	runningProcs   = make(map[string]*exec.Cmd)
)

func pauseJob(id string, port int) {
	if port <= 0 {
		port = 1700
	}
	apiURL := fmt.Sprintf("http://127.0.0.1:%d/api/tasks/pause", port)
	payload, _ := json.Marshal(map[string]string{"id": id})

	client := http.Client{Timeout: 1 * time.Second}
	resp, err := client.Post(apiURL, "application/json", bytes.NewBuffer(payload))
	if err == nil && resp.StatusCode == http.StatusOK {
		resp.Body.Close()
		return
	}
	if resp != nil {
		resp.Body.Close()
	}

	// Fallback to local process signal if daemon not answering
	runningProcsMu.Lock()
	cmd, ok := runningProcs[id]
	runningProcsMu.Unlock()

	if !ok || cmd.Process == nil {
		log.Println("pause requested but no running process for", id)
		return
	}

	if err := cmd.Process.Signal(os.Interrupt); err != nil {
		_ = cmd.Process.Kill()
	}
}

func resumeJob(id string, port int) {
	if port <= 0 {
		port = 1700
	}
	apiURL := fmt.Sprintf("http://127.0.0.1:%d/api/tasks/resume", port)
	payload, _ := json.Marshal(map[string]string{"id": id})

	client := http.Client{Timeout: 1 * time.Second}
	resp, err := client.Post(apiURL, "application/json", bytes.NewBuffer(payload))
	if err == nil && resp.StatusCode == http.StatusOK {
		resp.Body.Close()
		return
	}
	if resp != nil {
		resp.Body.Close()
	}

	// Fallback: spawn CLI subprocess if daemon is not running
	cmd := exec.Command("warpcentral", "resume", id)

	runningProcsMu.Lock()
	if _, exists := runningProcs[id]; exists {
		runningProcsMu.Unlock()
		return
	}
	runningProcs[id] = cmd
	runningProcsMu.Unlock()

	go func() {
		_ = cmd.Run()
		runningProcsMu.Lock()
		delete(runningProcs, id)
		runningProcsMu.Unlock()
	}()
}

func addJob(urlStr, outPath string, port int) {
	if port <= 0 {
		port = 1700
	}
	apiURL := fmt.Sprintf("http://127.0.0.1:%d/api/tasks/add", port)
	payload, _ := json.Marshal(map[string]string{
		"url":     urlStr,
		"outPath": outPath,
	})

	client := http.Client{Timeout: 2 * time.Second}
	resp, err := client.Post(apiURL, "application/json", bytes.NewBuffer(payload))
	if err == nil && resp.StatusCode == http.StatusOK {
		resp.Body.Close()
		return
	}
	if resp != nil {
		resp.Body.Close()
	}

	// Fallback to CLI add
	cmd := exec.Command("warpcentral", "queue", "add", urlStr, outPath)
	_ = cmd.Run()
}

func deleteJob(id string, port int) {
	if port <= 0 {
		port = 1700
	}
	apiURL := fmt.Sprintf("http://127.0.0.1:%d/api/tasks/delete", port)
	payload, _ := json.Marshal(map[string]string{"id": id})

	client := http.Client{Timeout: 1 * time.Second}
	resp, err := client.Post(apiURL, "application/json", bytes.NewBuffer(payload))
	if err == nil && resp.StatusCode == http.StatusOK {
		resp.Body.Close()
	}
	if resp != nil {
		resp.Body.Close()
	}
}
