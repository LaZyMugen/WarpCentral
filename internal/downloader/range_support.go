package downloader

import (
	"context"
	"fmt"
	"net/http"
	"strings"
)

type RemoteInfo struct {
	Size         int64
	AcceptRanges bool
}

func (d *Downloader) Probe(ctx context.Context, rawURL string) (RemoteInfo, error) {
	// HEAD request first (cheap)
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, rawURL, nil)
	if err != nil {
		return RemoteInfo{}, err
	}
	req.Header.Set("User-Agent", "WarpCentral/0.1")

	resp, err := d.Client.Do(req)
	if err != nil {
		return RemoteInfo{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return RemoteInfo{}, fmt.Errorf("probe http error: %s", resp.Status)
	}

	size := resp.ContentLength

	// Many servers lie about Accept-Ranges.
	// We verify range support using a tiny GET request: bytes=0-0
	rangeOK := false

	testReq, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return RemoteInfo{}, err
	}
	testReq.Header.Set("User-Agent", "WarpCentral/0.1")
	testReq.Header.Set("Range", "bytes=0-0")

	testResp, err := d.Client.Do(testReq)
	if err == nil {
		defer testResp.Body.Close()

		// Correct response for a range request is 206 Partial Content
		if testResp.StatusCode == http.StatusPartialContent {
			rangeOK = true
		}
	}

	// fallback to header check if range test request fails but HEAD hinted bytes
	// (still not trustworthy, but better than giving up immediately)
	if !rangeOK {
		ar := strings.ToLower(resp.Header.Get("Accept-Ranges"))
		if strings.Contains(ar, "bytes") {
			rangeOK = true
		}
	}

	return RemoteInfo{
		Size:         size,
		AcceptRanges: rangeOK,
	}, nil
}
