package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/LaZyMugen/warpcentral/internal/downloader"
)

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

func main() {
	if len(os.Args) < 3 {
		fmt.Println("Usage:")
		fmt.Println("  warpcentral download <url> [output_file]")
		return
	}

	cmd := strings.ToLower(os.Args[1])
	if cmd != "download" {
		fmt.Println("Unknown command:", cmd)
		return
	}

	rawURL := os.Args[2]
	out := ""
	if len(os.Args) >= 4 {
		out = os.Args[3]
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Ctrl+C cancel
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigCh
		fmt.Println("\nStopping...")
		cancel()
	}()

	dl := downloader.New()

	fmt.Println("Starting download...")

	err := dl.Download(ctx, rawURL, out, func(p downloader.Progress) {
		if p.Total > 0 {
			percent := float64(p.Downloaded) * 100 / float64(p.Total)
			fmt.Printf("\r%s / %s (%.2f%%) | %s",
				formatBytes(p.Downloaded),
				formatBytes(p.Total),
				percent,
				formatSpeed(p.SpeedBps),
			)
		} else {
			fmt.Printf("\r%s / ? | %s",
				formatBytes(p.Downloaded),
				formatSpeed(p.SpeedBps),
			)
		}
	})

	if err != nil {
		fmt.Println("\nFailed:", err)
		return
	}

	fmt.Println("\nDone.")
}
