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
	httpClient *http.Client
	latestURL  string
}

func NewClient(latestURL string) *Client {
	return &Client{
		httpClient: &http.Client{Timeout: 15 * time.Second},
		latestURL:  latestURL,
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
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("downloading release asset: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("release asset returned %s", resp.Status)
	}
	path := filepath.Join(dir, filepath.Base(asset.Name))
	f, err := os.Create(path)
	if err != nil {
		return "", "", fmt.Errorf("creating staged asset: %w", err)
	}
	h := sha256.New()
	_, copyErr := io.Copy(io.MultiWriter(f, h), io.LimitReader(resp.Body, 256<<20))
	closeErr := f.Close()
	if copyErr != nil {
		return "", "", fmt.Errorf("writing staged asset: %w", copyErr)
	}
	if closeErr != nil {
		return "", "", fmt.Errorf("closing staged asset: %w", closeErr)
	}
	digest := hex.EncodeToString(h.Sum(nil))
	if strings.HasPrefix(asset.Digest, "sha256:") && !strings.EqualFold(strings.TrimPrefix(asset.Digest, "sha256:"), digest) {
		return path, digest, fmt.Errorf("digest mismatch for %s", asset.Name)
	}
	return path, digest, nil
}
