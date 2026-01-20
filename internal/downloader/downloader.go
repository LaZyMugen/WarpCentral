package downloader

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"
)

type Progress struct {
	Downloaded int64
	Total      int64 // -1 if unknown
	SpeedBps   float64
}

type Downloader struct {
	Client *http.Client
}

func New() *Downloader {
	return &Downloader{
		Client: &http.Client{
			Timeout: 0, // rely on context cancel
		},
	}
}

func (d *Downloader) Download(ctx context.Context, rawURL, outPath string, onProgress func(Progress)) error {
	// Basic validation
	if strings.TrimSpace(rawURL) == "" {
		return errors.New("empty url")
	}
	_, err := url.ParseRequestURI(rawURL)
	if err != nil {
		return fmt.Errorf("invalid url: %w", err)
	}

	// Request
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "WarpCentral/0.1")

	resp, err := d.Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("http error: %s", resp.Status)
	}

	// Output path
	if outPath == "" {
		outPath = guessFileName(resp, rawURL)
	}
	outPath = filepath.Clean(outPath)

	// Create file
	f, err := os.Create(outPath)
	if err != nil {
		return err
	}
	defer f.Close()

	total := resp.ContentLength // -1 if unknown

	// Shared progress counter (atomic because goroutine updates it)
	var downloaded int64

	// Progress ticker
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	// Instant speed calculation state
	lastTime := time.Now()
	lastBytes := int64(0)

	emit := func() {
		now := time.Now()
		dt := now.Sub(lastTime).Seconds()
		if dt <= 0 {
			return
		}

		cur := atomic.LoadInt64(&downloaded)
		delta := cur - lastBytes
		speed := float64(delta) / dt

		lastTime = now
		lastBytes = cur

		if onProgress != nil {
			onProgress(Progress{
				Downloaded: cur,
				Total:      total,
				SpeedBps:   speed,
			})
		}
	}

	// Copy in background so the main goroutine can:
	// - emit progress on a ticker
	// - exit cleanly on completion
	// - respond instantly to ctx cancel
	errCh := make(chan error, 1)
	go func() {
		buf := make([]byte, 256*1024) // 256KB
		for {
			n, rErr := resp.Body.Read(buf)
			if n > 0 {
				_, wErr := f.Write(buf[:n])
				if wErr != nil {
					errCh <- wErr
					return
				}
				atomic.AddInt64(&downloaded, int64(n))
			}

			if rErr == io.EOF {
				// Flush file to disk
				if syncErr := f.Sync(); syncErr != nil {
					errCh <- syncErr
					return
				}
				errCh <- nil
				return
			}

			if rErr != nil {
				errCh <- rErr
				return
			}
		}
	}()

	// Control loop (no blocking reads here)
	for {
		select {
		case <-ctx.Done():
			// If already complete, don't treat it as a failure.
			cur := atomic.LoadInt64(&downloaded)
			if total > 0 && cur >= total {
				emit()
				return nil
			}
			emit()
			return ctx.Err()

		case <-ticker.C:
			emit()

		case copyErr := <-errCh:
			// One final emit before exiting
			emit()
			return copyErr
		}
	}
}

func guessFileName(resp *http.Response, rawURL string) string {
	// Try Content-Disposition: filename=...
	cd := resp.Header.Get("Content-Disposition")
	if cd != "" {
		lower := strings.ToLower(cd)
		idx := strings.Index(lower, "filename=")
		if idx != -1 {
			part := cd[idx+len("filename="):]
			part = strings.TrimSpace(part)
			part = strings.Trim(part, `"'`)
			if part != "" {
				return part
			}
		}
	}

	// Fallback to URL path base
	u, err := url.Parse(rawURL)
	if err == nil {
		base := filepath.Base(u.Path)
		if base != "" && base != "/" && base != "." {
			return base
		}
	}

	return "download.bin"
}
