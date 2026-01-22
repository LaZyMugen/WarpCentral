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

	ar := strings.ToLower(resp.Header.Get("Accept-Ranges"))
	return RemoteInfo{
		Size:         resp.ContentLength,
		AcceptRanges: strings.Contains(ar, "bytes"),
	}, nil
}
