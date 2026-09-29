package reconciliation

import (
	"strings"
	"testing"
	"time"
)

func fixture(success bool) Input {
	return Input{
		ProposalID:         "p-reconcile",
		Operation:          "release.stage",
		AdmissionDigest:    "sha256:admission",
		ConfirmationDigest: "sha256:confirmation",
		ExecutionDigest:    "sha256:execution",
		RuntimeID:          "run-test",
		ObservationID:      "obs-test",
		Success:            success,
		Detail:             "terminal result",
		CompletedAt:        time.Date(2026, 9, 29, 21, 30, 0, 123, time.UTC),
		ArtifactCommitted:  false,
	}
}

func TestBindVerifyFailed(t *testing.T) {
	t.Parallel()
	e, err := Bind(fixture(false))
	if err != nil {
		t.Fatalf("Bind() error = %v", err)
	}
	if e.Outcome != OutcomeFailed {
		t.Fatalf("Outcome = %q, want %q", e.Outcome, OutcomeFailed)
	}
	if e.EvidenceGrantsAuthority || e.AuthorityRestorable || e.ArtifactCommitted {
		t.Fatalf("unsafe flags = %+v", e)
	}
	if !strings.HasPrefix(e.Digest, "sha256:") {
		t.Fatalf("Digest = %q", e.Digest)
	}
	if err := Verify(e); err != nil {
		t.Fatalf("Verify() error = %v", err)
	}
}

func TestBindVerifySucceeded(t *testing.T) {
	t.Parallel()
	in := fixture(true)
	in.ArtifactPath = "/tmp/staging/KoLmafia-test.jar"
	in.SHA256 = strings.Repeat("a", 64)
	in.ArtifactCommitted = true
	e, err := Bind(in)
	if err != nil {
		t.Fatalf("Bind() error = %v", err)
	}
	if e.Outcome != OutcomeSucceeded || !e.ArtifactCommitted {
		t.Fatalf("evidence = %+v", e)
	}
	if err := Verify(e); err != nil {
		t.Fatalf("Verify() error = %v", err)
	}
}

func TestVerifyRejectsTampering(t *testing.T) {
	t.Parallel()
	e, err := Bind(fixture(false))
	if err != nil {
		t.Fatalf("Bind() error = %v", err)
	}
	e.Detail = "tampered"
	if err := Verify(e); err == nil || !strings.Contains(err.Error(), "digest mismatch") {
		t.Fatalf("Verify() error = %v, want digest mismatch", err)
	}
}

func TestBindRejectsCommittedFailedArtifact(t *testing.T) {
	t.Parallel()
	in := fixture(false)
	in.ArtifactCommitted = true
	in.ArtifactPath = "/tmp/should-not-exist"
	if _, err := Bind(in); err == nil || !strings.Contains(err.Error(), "failed outcome") {
		t.Fatalf("Bind() error = %v, want failed outcome", err)
	}
}
