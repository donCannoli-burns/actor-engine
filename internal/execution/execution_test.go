package execution

import (
	"strings"
	"testing"
	"time"
)

func fixture() Input {
	return Input{
		ProposalID:          "p-exec",
		Operation:           "release.stage",
		ProposalStateDigest: "proposal-state",
		AdmissionDigest:     "sha256:admission",
		ConfirmationDigest:  "sha256:confirmation",
		OriginRuntimeID:     "run-origin",
		ExecutionRuntimeID:  "run-origin",
		CurrentStateDigest:  "current-state",
		AttemptedAt:         time.Date(2026, 9, 29, 20, 30, 0, 123, time.UTC),
		GateDecision:        GateDenied,
	}
}

func TestBindVerify(t *testing.T) {
	t.Parallel()
	e, err := Bind(fixture())
	if err != nil {
		t.Fatalf("Bind() error = %v", err)
	}
	if e.Version != Version {
		t.Fatalf("Version = %q, want %q", e.Version, Version)
	}
	if !strings.HasPrefix(e.Digest, "sha256:") || len(e.Digest) != len("sha256:")+64 {
		t.Fatalf("Digest = %q", e.Digest)
	}
	if e.EvidenceGrantsAuthority {
		t.Fatal("execution attempt evidence unexpectedly grants authority")
	}
	if err := Verify(e); err != nil {
		t.Fatalf("Verify() error = %v", err)
	}
}

func TestVerifyRejectsTampering(t *testing.T) {
	t.Parallel()
	e, err := Bind(fixture())
	if err != nil {
		t.Fatalf("Bind() error = %v", err)
	}
	e.CurrentStateDigest = "tampered"
	if err := Verify(e); err == nil || !strings.Contains(err.Error(), "digest mismatch") {
		t.Fatalf("Verify() error = %v, want digest mismatch", err)
	}
}

func TestBindRejectsInvalidDecision(t *testing.T) {
	t.Parallel()
	in := fixture()
	in.GateDecision = "maybe"
	if _, err := Bind(in); err == nil || !strings.Contains(err.Error(), "unsupported gate decision") {
		t.Fatalf("Bind() error = %v, want unsupported decision", err)
	}
}
