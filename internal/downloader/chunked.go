package downloader

import "io"

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/LaZyMugen/warpcentral/internal/chunker"
)

type ChunkedOptions struct {
	Parts       int
	MaxRetries  int
	WorkerCount int
}

func defaultChunkedOptions() ChunkedOptions {
	return ChunkedOptions{
		Parts:       8,
		MaxRetries:  3,
		WorkerCount: 8,
	}
}

func autoParts(total int64) int {
	const mb = 1024 * 1024

	// Aim: 8–16MB per chunk (good balance)
	const targetChunkSize = 16 * mb
	const minChunkSize = 2 * mb

	if total <= 0 {
		return 1
	}

	parts := int(total / targetChunkSize)
	if total%targetChunkSize != 0 {
		parts++
	}

	// Clamp parts range
	if parts < 1 {
		parts = 1
	}
	if parts > 32 {
		parts = 32
	}

	// Ensure chunks aren't too small
	// If parts make chunks smaller than minChunkSize, reduce parts.
	for parts > 1 {
		chunkSize := total / int64(parts)
		if chunkSize >= minChunkSize {
			break
		}
		parts--
	}

	return parts
}

func clampWorkers(parts int) int {
	if parts <= 0 {
		return 1
	}
	// hard cap for sanity
	if parts > 12 {
		return 12
	}
	return parts
}



func (d *Downloader) DownloadSmart(ctx context.Context, rawURL, outPath string, onProgress func(Progress)) error {
	info, err := d.Probe(ctx, rawURL)
	if err != nil {
		// If HEAD fails, fallback to single stream GET
		return d.Download(ctx, rawURL, outPath, onProgress)
	}

	// If no size or no range support -> fallback
	if info.Size <= 0 || !info.AcceptRanges {
		return d.Download(ctx, rawURL, outPath, onProgress)
	}

	// chunk download
	opt := defaultChunkedOptions()
	opt.Parts = autoParts(info.Size)
	opt.WorkerCount = clampWorkers(opt.Parts)
	return d.downloadChunked(ctx, rawURL, outPath, info.Size, opt, onProgress)}

func (d *Downloader) downloadChunked(
	ctx context.Context,
	rawURL string,
	outPath string,
	totalSize int64,
	opt ChunkedOptions,
	onProgress func(Progress),
) error {
	if opt.Parts <= 0 {
		opt.Parts = 8
	}
	if opt.WorkerCount <= 0 {
		opt.WorkerCount = opt.Parts
	}
	if opt.MaxRetries < 0 {
		opt.MaxRetries = 0
	}

	// Guess output if not provided
	if outPath == "" {
		outPath = "download.bin"
	}
	outPath = filepath.Clean(outPath)

	// Pre-allocate file
	f, err := os.OpenFile(outPath, os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	if err := f.Truncate(totalSize); err != nil {
		return err
	}

	chunks := chunker.Split(totalSize, opt.Parts)
	if len(chunks) == 0 {
		return fmt.Errorf("failed to split into chunks")
	}

	var downloaded int64 // atomic

	// progress ticker
	ticker := time.NewTicker(400 * time.Millisecond)
	defer ticker.Stop()

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
				Total:      totalSize,
				SpeedBps:   speed,
			})
		}
	}

	// worker pool
	type job struct {
		c chunker.Chunk
	}

	jobs := make(chan job)
	errCh := make(chan error, 1)

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var wg sync.WaitGroup

	worker := func() {
		defer wg.Done()

		for j := range jobs {
			// retry loop per chunk
			var lastErr error
			for attempt := 0; attempt <= opt.MaxRetries; attempt++ {
				if ctx.Err() != nil {
					return
				}

				n, e := d.downloadRangeIntoFile(ctx, rawURL, f, j.c.Start, j.c.End)
				if e == nil {
					atomic.AddInt64(&downloaded, n)
					lastErr = nil
					break
				}
				lastErr = e
			}

			if lastErr != nil {
				select {
				case errCh <- lastErr:
				default:
				}
				cancel()
				return
			}
		}
	}

	wg.Add(opt.WorkerCount)
	for i := 0; i < opt.WorkerCount; i++ {
		go worker()
	}

	// feed jobs
	go func() {
		defer close(jobs)
		for _, c := range chunks {
			select {
			case <-ctx.Done():
				return
			case jobs <- job{c: c}:
			}
		}
	}()

	// control loop
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	for {
		select {
		case <-ctx.Done():
			emit()
			// If cancellation was due to error, errCh handles it.
			select {
			case e := <-errCh:
				return e
			default:
				return ctx.Err()
			}

		case <-ticker.C:
			emit()

		case e := <-errCh:
			emit()
			return e

		case <-done:
			emit()
			_ = f.Sync()
			return nil
		}
	}
}

func (d *Downloader) downloadRangeIntoFile(
	ctx context.Context,
	rawURL string,
	f *os.File,
	start, end int64,
) (int64, error) {

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("User-Agent", "WarpCentral/0.1")
	req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", start, end))

	resp, err := d.Client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	// For range requests, servers should respond 206 Partial Content.
	// Some servers may respond 200 (bad range support). Treat that as failure here.
	if resp.StatusCode != http.StatusPartialContent {
		return 0, fmt.Errorf("range request failed: %s", resp.Status)
	}

	// WriteAt loop
	buf := make([]byte, 256*1024)
	var writtenTotal int64 = 0

	offset := start
	for {
		n, rErr := resp.Body.Read(buf)
		if n > 0 {
			w, wErr := f.WriteAt(buf[:n], offset)
			if wErr != nil {
				return writtenTotal, wErr
			}
			if w != n {
				return writtenTotal, fmt.Errorf("short write: wrote %d expected %d", w, n)
			}
			offset += int64(n)
			writtenTotal += int64(n)
		}

		if rErr == io.EOF {
			break
		}
		if rErr != nil {
			return writtenTotal, rErr
		}
	}

	// sanity: should match requested length
	expected := (end - start) + 1
	if writtenTotal != expected {
		return writtenTotal, fmt.Errorf("chunk size mismatch: got %d expected %d", writtenTotal, expected)
	}

	return writtenTotal, nil
}
