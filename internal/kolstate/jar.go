package kolstate

import (
	"archive/zip"
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
)

func InstalledRevision(jarPath string) (string, error) {
	if jarPath == "" {
		return "", nil
	}
	if _, err := os.Stat(jarPath); err != nil {
		return "", fmt.Errorf("stat KoLmafia jar: %w", err)
	}
	zr, err := zip.OpenReader(jarPath)
	if err != nil {
		return "", fmt.Errorf("open KoLmafia jar: %w", err)
	}
	defer zr.Close()
	for _, f := range zr.File {
		if f.Name != "META-INF/MANIFEST.MF" {
			continue
		}
		r, err := f.Open()
		if err != nil {
			return "", fmt.Errorf("open manifest: %w", err)
		}
		rev, readErr := readRevision(r)
		closeErr := r.Close()
		if readErr != nil {
			return "", readErr
		}
		if closeErr != nil {
			return "", fmt.Errorf("close manifest: %w", closeErr)
		}
		return rev, nil
	}
	return "", fmt.Errorf("KoLmafia manifest not found")
}

func readRevision(r io.Reader) (string, error) {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := scanner.Text()
		for _, key := range []string{"Build-Revision:", "Implementation-Version:"} {
			if strings.HasPrefix(line, key) {
				return strings.TrimSpace(strings.TrimPrefix(line, key)), nil
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("read manifest: %w", err)
	}
	return "", nil
}
