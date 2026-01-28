package downloader

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/LaZyMugen/warpcentral/internal/chunker"
	"github.com/LaZyMugen/warpcentral/internal/resume"
	"github.com/LaZyMugen/warpcentral/internal/storage"
)

type metaSaver struct {
	lastSave time.Time
	interval time.Duration
}

func newMetaSaver() *metaSaver {
	return &metaSaver{
		lastSave: time.Now(),
		interval: time.Second,
	}
}

func (m *metaSaver) shouldSave() bool {
	return time.Since(m.lastSave) >= m.interval
}

func (m *metaSaver) markSaved() {
	m.lastSave = time.Now()
}



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

/* ---------- tuning ---------- */

func autoParts(total int64) int {
	const mb = 1024 * 1024
	const target = 16 * mb
	const min = 2 * mb

	if total <= 0 {
		return 1
	}

	parts := int(total / target)
	if total%target != 0 {
		parts++
	}

	if parts < 1 {
		parts = 1
	}
	if parts > 32 {
		parts = 32
	}

	for parts > 1 {
		if total/int64(parts) >= min {
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
	if parts > 12 {
		return 12
	}
	return parts
}

// guessFileNameFromURL picks a reasonable default filename from the URL path.
// Used by the chunked path so it behaves like the single-stream downloader.
func guessFileNameFromURL(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "download.bin"
	}

	name := filepath.Base(u.Path)
	if name == "" || name == "." || name == "/" {
		return "download.bin"
	}
	return name
}

/* ---------- public entry ---------- */

func (d *Downloader) DownloadSmart(
	ctx context.Context,
	rawURL, outPath string,
	onProgress func(Progress),
) error {

	info, err := d.Probe(ctx, rawURL)
	if err != nil || info.Size <= 0 || !info.AcceptRanges {
		return d.Download(ctx, rawURL, outPath, onProgress)
	}

	opt := defaultChunkedOptions()
	opt.Parts = autoParts(info.Size)
	opt.WorkerCount = clampWorkers(opt.Parts)

	if outPath == "" {
		outPath = guessFileNameFromURL(rawURL)
	}

	return d.downloadChunked(ctx, rawURL, outPath, info.Size, opt, onProgress)
}

/* ---------- chunked download ---------- */

func (d *Downloader) downloadChunked(
	ctx context.Context,
	rawURL string,
	outPath string,
	totalSize int64,
	opt ChunkedOptions,
	onProgress func(Progress),
) error {
	outPath = filepath.Clean(outPath)

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
		return fmt.Errorf("failed to split chunks")
	}

	/* ---------- meta ---------- */

	metaPath := resume.MetaPath(outPath)
	metaSave := newMetaSaver()
	meta := resume.Meta{
		Version:   1,
		URL:       rawURL,
		OutPath:   outPath,
		TotalSize: totalSize,
		Parts:     opt.Parts,
		Chunks:    make([]resume.ChunkState, 0, len(chunks)),
	}

	for _, c := range chunks {
		meta.Chunks = append(meta.Chunks, resume.ChunkState{
			ID:        c.ID,
			Start:     c.Start,
			End:       c.End,
			DoneBytes: 0,
		})
	}

	if metaSave.shouldSave() {
		_ = resume.Save(metaPath, meta)
		metaSave.markSaved()
	}

	var downloaded int64
	var metaMu sync.Mutex

	/* ---------- progress ---------- */

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
		speed := float64(cur-lastBytes) / dt

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

	/* ---------- workers ---------- */

	type job struct{ c chunker.Chunk }

	jobs := make(chan job)
	errCh := make(chan error, 1)

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var wg sync.WaitGroup

	worker := func() {
		defer wg.Done()

		for j := range jobs {
			var lastErr error

			for attempt := 0; attempt <= opt.MaxRetries; attempt++ {
				if ctx.Err() != nil {
					return
				}

				/* find resume offset */
				metaMu.Lock()
				startOffset := j.c.Start
				for _, mc := range meta.Chunks {
					if mc.ID == j.c.ID {
						startOffset = mc.Start + mc.DoneBytes
						break
					}
				}
				metaMu.Unlock()

				_, e := d.downloadRangeIntoFile(
					ctx,
					rawURL,
					f,
					j.c.Start,
					j.c.End,
					startOffset,
					func(n int64) {
						atomic.AddInt64(&downloaded, n)

						metaMu.Lock()
						for i := range meta.Chunks {
							if meta.Chunks[i].ID == j.c.ID {
								meta.Chunks[i].DoneBytes += n
								break
							}
						}
						if metaSave.shouldSave() {
							_ = resume.Save(metaPath, meta)
							metaSave.markSaved()
						}
						metaMu.Unlock()
					},
				)

				if e == nil {
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

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	for {
		select {
		case <-ctx.Done():
			emit()
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

			// Final checksum + meta update (best-effort)
			checksum, err := storage.SHA256(outPath)
			if err == nil {
				metaMu.Lock()
				meta.Checksum = checksum
				_ = resume.Save(metaPath, meta)
				metaMu.Unlock()
			}

			return nil
		}
	}
}

/* ---------- range download ---------- */

func (d *Downloader) downloadRangeIntoFile(
	ctx context.Context,
	rawURL string,
	f *os.File,
	start int64,
	end int64,
	startOffset int64,
	onBytes func(int64),
) (int64, error) {

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("User-Agent", "WarpCentral/0.1")
	req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", startOffset, end))

	resp, err := d.Client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusPartialContent {
		return 0, fmt.Errorf("range request failed: %s", resp.Status)
	}

	buf := make([]byte, 256*1024)
	var written int64
	offset := startOffset

	for {
		n, rErr := resp.Body.Read(buf)
		if n > 0 {
			w, wErr := f.WriteAt(buf[:n], offset)
			if wErr != nil {
				return written, wErr
			}
			if w != n {
				return written, fmt.Errorf("short write")
			}

			offset += int64(n)
			written += int64(n)

			if onBytes != nil {
				onBytes(int64(n))
			}
		}

		if rErr == io.EOF {
			break
		}
		if rErr != nil {
			return written, rErr
		}
	}

	expected := (end - startOffset) + 1
	if written != expected {
		return written, fmt.Errorf("chunk mismatch: got %d expected %d", written, expected)
	}

	return written, nil
}
