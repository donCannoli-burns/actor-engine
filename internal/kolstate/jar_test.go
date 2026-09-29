package kolstate

import (
	"strings"
	"testing"
)

func TestReadRevision(t *testing.T) {
	t.Parallel()
	got, err := readRevision(strings.NewReader("Manifest-Version: 1.0\nBuild-Revision: 29309\n"))
	if err != nil {
		t.Fatalf("readRevision() error = %v, want nil", err)
	}
	if got != "29309" {
		t.Fatalf("readRevision() = %q, want %q", got, "29309")
	}
}
