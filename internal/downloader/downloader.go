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
			Timeout: 0,
		},
	}
}

func (d *Downloader) Download(ctx context.Context, rawURL, outPath string, onProgress func(Progress)) error {
	if strings.TrimSpace(rawURL) == "" {
		return errors.New("empty url")
	}
	_, err := url.ParseRequestURI(rawURL)
	if err != nil {
		return fmt.Errorf("invalid url: %w", err)
	}

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

	if outPath == "" {
		outPath = guessFileName(resp, rawURL)
	}
	outPath = filepath.Clean(outPath)

	f, err := os.Create(outPath)
	if err != nil {
		return err
	}
	defer f.Close()

	total := resp.ContentLength // -1 if unknown
	var downloaded int64

	start := time.Now()

	// For speed calculation: bytes over last interval
	lastTime := time.Now()
	lastBytes := int64(0)

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	emitProgress := func() {
		now := time.Now()
		dt := now.Sub(lastTime).Seconds()
		if dt <= 0 {
			return
		}
		delta := downloaded - lastBytes
		speed := float64(delta) / dt

		lastTime = now
		lastBytes = downloaded

		if onProgress != nil {
			onProgress(Progress{
				Downloaded: downloaded,
				Total:      total,
				SpeedBps:   speed,
			})
		}
	}

	// progress loop (NO goroutine needed)
	buf := make([]byte, 1024*256)

	for {
		select {
		case <-ctx.Done():
	// If we already fully downloaded the file, consider it a success.
	if total > 0 && downloaded >= total {
		emitProgress()
		return nil
	}
	emitProgress()
	return ctx.Err()

		case <-ticker.C:
			emitProgress()
		default:
			n, readErr := resp.Body.Read(buf)
			if n > 0 {
				_, wErr := f.Write(buf[:n])
				if wErr != nil {
					return wErr
				}
				downloaded += int64(n)
			}

			if readErr == io.EOF {
				break
			}
			if readErr != nil {
				return readErr
			}
		}
	}

	// final progress (avg-ish)
	elapsed := time.Since(start).Seconds()
	avgSpeed := float64(downloaded)
	if elapsed > 0 {
		avgSpeed = avgSpeed / elapsed
	}
	if onProgress != nil {
		onProgress(Progress{
			Downloaded: downloaded,
			Total:      total,
			SpeedBps:   avgSpeed,
		})
	}

	return nil
}

func guessFileName(resp *http.Response, rawURL string) string {
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

	u, err := url.Parse(rawURL)
	if err == nil {
		base := filepath.Base(u.Path)
		if base != "" && base != "/" && base != "." {
			return base
		}
	}

	return "download.bin"
}
