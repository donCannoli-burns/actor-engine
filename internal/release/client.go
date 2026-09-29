package release

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const maxStageBytes int64 = 256 << 20

type Asset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Digest             string `json:"digest"`
}

type Info struct {
	TagName   string  `json:"tag_name"`
	Name      string  `json:"name"`
	HTMLURL   string  `json:"html_url"`
	Published string  `json:"published_at"`
	Assets    []Asset `json:"assets"`
}

type Client struct {
	httpClient     *http.Client
	downloadClient *http.Client
	latestURL      string
}

func NewClient(latestURL string) *Client {
	return &Client{
		httpClient:     &http.Client{Timeout: 15 * time.Second},
		downloadClient: &http.Client{Timeout: 2 * time.Minute},
		latestURL:      latestURL,
	}
}

func (c *Client) Latest(ctx context.Context) (Info, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.latestURL, nil)
	if err != nil {
		return Info{}, fmt.Errorf("creating latest release request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return Info{}, fmt.Errorf("fetching latest release: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Info{}, fmt.Errorf("latest release returned %s", resp.Status)
	}
	var info Info
	if err := json.NewDecoder(io.LimitReader(resp.Body, 2<<20)).Decode(&info); err != nil {
		return Info{}, fmt.Errorf("decoding latest release: %w", err)
	}
	return info, nil
}

func SelectJar(info Info) (Asset, error) {
	for _, asset := range info.Assets {
		if strings.HasSuffix(asset.Name, ".jar") {
			return asset, nil
		}
	}
	return Asset{}, fmt.Errorf("release %q has no jar asset", info.TagName)
}

func (c *Client) Stage(ctx context.Context, asset Asset, dir string) (string, string, error) {
	if asset.BrowserDownloadURL == "" || asset.Name == "" {
		return "", "", fmt.Errorf("release asset is incomplete")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", "", fmt.Errorf("creating staging directory: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, asset.BrowserDownloadURL, nil)
	if err != nil {
		return "", "", fmt.Errorf("creating stage request: %w", err)
	}
	resp, err := c.downloadClient.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("downloading release asset: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("release asset returned %s", resp.Status)
	}

	base := filepath.Base(asset.Name)
	finalPath := filepath.Join(dir, base)
	tmp, err := os.CreateTemp(dir, "."+base+".part-*")
	if err != nil {
		return "", "", fmt.Errorf("creating temporary staged asset: %w", err)
	}
	tmpPath := tmp.Name()
	committed := false
	defer func() {
		if !committed {
			_ = tmp.Close()
			_ = os.Remove(tmpPath)
		}
	}()

	h := sha256.New()
	n, copyErr := io.Copy(io.MultiWriter(tmp, h), io.LimitReader(resp.Body, maxStageBytes+1))
	if copyErr != nil {
		return "", "", fmt.Errorf("writing temporary staged asset: %w", copyErr)
	}
	if n > maxStageBytes {
		return "", "", fmt.Errorf("release asset exceeds %d-byte staging limit", maxStageBytes)
	}
	if err := tmp.Sync(); err != nil {
		return "", "", fmt.Errorf("syncing temporary staged asset: %w", err)
	}
	if err := tmp.Chmod(0o644); err != nil {
		return "", "", fmt.Errorf("setting staged asset mode: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return "", "", fmt.Errorf("closing temporary staged asset: %w", err)
	}

	digest := hex.EncodeToString(h.Sum(nil))
	if strings.HasPrefix(asset.Digest, "sha256:") && !strings.EqualFold(strings.TrimPrefix(asset.Digest, "sha256:"), digest) {
		return "", digest, fmt.Errorf("digest mismatch for %s", asset.Name)
	}
	if err := os.Rename(tmpPath, finalPath); err != nil {
		return "", digest, fmt.Errorf("committing staged asset: %w", err)
	}
	committed = true
	return finalPath, digest, nil
}
