package downloader

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/LaZyMugen/warpcentral/internal/chunker"
	"github.com/LaZyMugen/warpcentral/internal/resume"
	"github.com/LaZyMugen/warpcentral/internal/storage"
)

func (d *Downloader) ResumeFromMeta(ctx context.Context, metaPath string, onProgress func(Progress)) error {
	m, err := resume.Load(metaPath)
	if err != nil {
		return err
	}

	// Option A validation (strict)
	if m.Version != 1 {
		return fmt.Errorf("unsupported meta version: %d", m.Version)
	}
	if m.URL == "" || m.OutPath == "" || m.TotalSize <= 0 {
		return fmt.Errorf("invalid meta file")
	}

	// Open existing file WITHOUT truncation
	outPath := filepath.Clean(m.OutPath)

	f, err := os.OpenFile(outPath, os.O_RDWR, 0644)
	if err != nil {
		return fmt.Errorf("failed to open output file: %w", err)
	}
	defer f.Close()

	// Validate file size
	st, err := f.Stat()
	if err != nil {
		return err
	}
	if st.Size() != m.TotalSize {
		return fmt.Errorf("file size mismatch: have %d expected %d", st.Size(), m.TotalSize)
	}

	// Optional checksum verification (if present in meta)
	if m.Checksum != "" {
		checksum, err := storage.SHA256(outPath)
		if err != nil {
			return fmt.Errorf("failed to compute checksum: %w", err)
		}
		if checksum != m.Checksum {
			return fmt.Errorf("checksum mismatch: have %s expected %s", checksum, m.Checksum)
		}
	}

	// Build chunks from meta (only those not fully done)
	metaSave := newMetaSaver()
	chunks := make([]chunker.Chunk, 0, len(m.Chunks))
	var alreadyDone int64
	for _, c := range m.Chunks {
		length := (c.End - c.Start + 1)
		doneBytes := c.DoneBytes
		if doneBytes > length {
			doneBytes = length
		}
		alreadyDone += doneBytes

		if doneBytes >= length {
			// this chunk is fully done
			continue
		}

		chunks = append(chunks, chunker.Chunk{
			ID:    c.ID,
			Start: c.Start,
			End:   c.End,
		})
	}

	if alreadyDone >= m.TotalSize || len(chunks) == 0 {
		// everything is already complete
		if onProgress != nil {
			onProgress(Progress{
				Downloaded: m.TotalSize,
				Total:      m.TotalSize,
				SpeedBps:   0,
			})
		}
		return nil
	}

	// Setup options (safe defaults)
	opt := defaultChunkedOptions()
	opt.Parts = m.Parts
	opt.WorkerCount = clampWorkers(opt.Parts)

	// Start progress from bytes already done
	var downloaded int64 = alreadyDone
	var metaMu sync.Mutex

	// Progress ticker
	ticker := time.NewTicker(400 * time.Millisecond)
	defer ticker.Stop()

	lastTime := time.Now()
	lastBytes := int64(downloaded)

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
				Total:      m.TotalSize,
				SpeedBps:   speed,
			})
		}
	}

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

				// Determine current resume offset for this chunk
				metaMu.Lock()
				startOffset := j.c.Start
				for _, mc := range m.Chunks {
					if mc.ID == j.c.ID {
						startOffset = mc.Start + mc.DoneBytes
						break
					}
				}
				metaMu.Unlock()

				_, e := d.downloadRangeIntoFile(ctx, m.URL, f, j.c.Start, j.c.End, startOffset, func(n int64) {
					atomic.AddInt64(&downloaded, n)

					metaMu.Lock()
					for i := range m.Chunks {
						if m.Chunks[i].ID == j.c.ID {
							m.Chunks[i].DoneBytes += n
							break
						}
					}
					if metaSave.shouldSave() {
						_ = resume.Save(metaPath, m)
						metaSave.markSaved()
					}
					metaMu.Unlock()
				})

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

			// Final checksum + meta update after resume (best-effort)
			checksum, err := storage.SHA256(outPath)
			if err == nil {
				metaMu.Lock()
				m.Checksum = checksum
				_ = resume.Save(metaPath, m)
				metaMu.Unlock()
			}

			return nil
		}
	}
}
