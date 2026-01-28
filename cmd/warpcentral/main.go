package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/LaZyMugen/warpcentral/internal/downloader"
	qstore "github.com/LaZyMugen/warpcentral/internal/queue"
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

func clearLine() {
	fmt.Printf("\r%-90s", "")
}

func usage() {
	fmt.Println("Usage:")
	fmt.Println("  warpcentral download <url> [output_file]")
	fmt.Println("  warpcentral resume <meta-file>")
	fmt.Println("  warpcentral queue add <url> [output_file]")
	fmt.Println("  warpcentral queue run")
}

func main() {
	if len(os.Args) < 2 {
		usage()
		return
	}

	cmd := strings.ToLower(os.Args[1])

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

	// shared progress printer
	var lastPrinted int64 = -1
	printProgress := func(p downloader.Progress) {
		if p.Downloaded == lastPrinted {
			return
		}
		lastPrinted = p.Downloaded

		clearLine()

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
	}

	switch cmd {
	case "download":
		if len(os.Args) < 3 {
			usage()
			return
		}

		rawURL := os.Args[2]
		out := ""
		if len(os.Args) >= 4 {
			out = os.Args[3]
		}

		fmt.Println("Starting download...")
		err := dl.DownloadSmart(ctx, rawURL, out, printProgress)
		if err != nil {
			fmt.Println("\nFailed:", err)
			return
		}
		clearLine()
		fmt.Println("\nDone.")

	case "resume":
		if len(os.Args) < 3 {
			usage()
			return
		}

		metaPath := os.Args[2]

		fmt.Println("Resuming download...")
		err := dl.ResumeFromMeta(ctx, metaPath, printProgress)
		if err != nil {
			fmt.Println("\nFailed:", err)
			return
		}
		clearLine()
		fmt.Println("\nDone.")

	case "queue":
		if len(os.Args) < 3 {
			usage()
			return
		}

		sub := strings.ToLower(os.Args[2])
		queuePath := qstore.DefaultPath()

		switch sub {
		case "add":
			if len(os.Args) < 4 {
				usage()
				return
			}
			rawURL := os.Args[3]
			out := ""
			if len(os.Args) >= 5 {
				out = os.Args[4]
			}

			item, err := qstore.Add(queuePath, rawURL, out)
			if err != nil {
				fmt.Println("Failed to add to queue:", err)
				return
			}
			fmt.Println("Queued:", item.URL, "->", item.OutPath)

		case "run":
			if err := runQueue(ctx, dl, queuePath, printProgress); err != nil {
				fmt.Println("\nQueue run failed:", err)
				return
			}
			clearLine()
			fmt.Println("\nQueue empty.")

		default:
			fmt.Println("Unknown queue subcommand:", sub)
			usage()
		}

	default:
		fmt.Println("Unknown command:", cmd)
		usage()
	}
}

// runQueue processes all pending items in the persistent queue file
// sequentially using the chunked downloader.
func runQueue(ctx context.Context, dl *downloader.Downloader, queuePath string, onProgress func(downloader.Progress)) error {
	for {
		q, err := qstore.Load(queuePath)
		if err != nil {
			return err
		}

		// Find next item to run: prefer pending; if none, treat any in_progress as pending.
		idx := -1
		for i := range q.Items {
			if q.Items[i].Status == qstore.StatusPending {
				idx = i
				break
			}
		}
		if idx == -1 {
			for i := range q.Items {
				if q.Items[i].Status == qstore.StatusInProgress {
					idx = i
					break
				}
			}
		}

		if idx == -1 {
			// nothing left to do
			return nil
		}

		item := &q.Items[idx]
		item.Status = qstore.StatusInProgress
		item.Error = ""
		if err := qstore.Save(queuePath, q); err != nil {
			return err
		}

		out := item.OutPath
		fmt.Println("\nStarting queued download:", item.URL, "->", out)

		// Run the download; DownloadSmart will choose single/ranged.
		perItemProgress := func(p downloader.Progress) {
			if onProgress != nil {
				onProgress(p)
			}
		}

		err = dl.DownloadSmart(ctx, item.URL, out, perItemProgress)
		if err != nil {
			item.Status = qstore.StatusFailed
			item.Error = err.Error()
		} else {
			item.Status = qstore.StatusDone
			item.Error = ""
		}

		if saveErr := qstore.Save(queuePath, q); saveErr != nil {
			return saveErr
		}

		// If context was cancelled, stop processing further items.
		if ctx.Err() != nil {
			return ctx.Err()
		}
	}
}
