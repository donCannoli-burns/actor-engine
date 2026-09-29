package release

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestStageDigestMismatchLeavesNoArtifact(t *testing.T) {
	t.Parallel()
	body := []byte("fixture")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	dir := t.TempDir()
	_, _, err := c.Stage(context.Background(), Asset{
		Name:               "KoLmafia-test.jar",
		BrowserDownloadURL: srv.URL,
		Digest:             "sha256:" + strings.Repeat("0", 64),
	}, dir)
	if err == nil || !strings.Contains(err.Error(), "digest mismatch") {
		t.Fatalf("Stage() error = %v, want digest mismatch", err)
	}
	assertNoStageArtifacts(t, dir, "KoLmafia-test.jar")
}

func TestStageCancellationLeavesNoArtifact(t *testing.T) {
	t.Parallel()
	firstChunk := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("partial"))
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		close(firstChunk)
		<-r.Context().Done()
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	dir := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		<-firstChunk
		cancel()
	}()

	_, _, err := c.Stage(ctx, Asset{
		Name:               "KoLmafia-test.jar",
		BrowserDownloadURL: srv.URL,
	}, dir)
	if err == nil {
		t.Fatal("Stage() error = nil, want cancellation")
	}
	assertNoStageArtifacts(t, dir, "KoLmafia-test.jar")
}

func TestStageSuccessCommitsVerifiedArtifact(t *testing.T) {
	t.Parallel()
	body := []byte("complete-fixture")
	sum := sha256.Sum256(body)
	expected := hex.EncodeToString(sum[:])
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	c.downloadClient.Timeout = 2 * time.Second
	dir := t.TempDir()
	path, gotDigest, err := c.Stage(context.Background(), Asset{
		Name:               "KoLmafia-test.jar",
		BrowserDownloadURL: srv.URL,
		Digest:             "sha256:" + expected,
	}, dir)
	if err != nil {
		t.Fatalf("Stage() error = %v", err)
	}
	if gotDigest != expected {
		t.Fatalf("digest = %q, want %q", gotDigest, expected)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(got) != string(body) {
		t.Fatal("committed artifact contents differ")
	}
	matches, err := filepath.Glob(filepath.Join(dir, ".KoLmafia-test.jar.part-*"))
	if err != nil {
		t.Fatalf("Glob() error = %v", err)
	}
	if len(matches) != 0 {
		t.Fatalf("temporary stage files remain: %v", matches)
	}
}

func assertNoStageArtifacts(t *testing.T, dir, finalName string) {
	t.Helper()
	if _, err := os.Stat(filepath.Join(dir, finalName)); !os.IsNotExist(err) {
		t.Fatalf("final artifact exists after failed stage; stat err=%v", err)
	}
	matches, err := filepath.Glob(filepath.Join(dir, "."+finalName+".part-*"))
	if err != nil {
		t.Fatalf("Glob() error = %v", err)
	}
	if len(matches) != 0 {
		t.Fatalf("temporary stage files remain after failure: %v", matches)
	}
}
