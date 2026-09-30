package resolution

import (
	"strings"
	"testing"
	"time"
)

func fixture() Input {
	return Input{
		InterruptionDigest:            "sha256:interrupt",
		ProposalID:                    "p-interrupted",
		Operation:                     "release.stage",
		ExecutionDigest:               "sha256:execution",
		StartedEventHash:              "ledger-start-hash",
		Decision:                      DecisionQuarantineUnknownNoReplay,
		ResolvedBy:                    "test-human",
		Note:                          "quarantined ambiguous bytes; continue without replay",
		RuntimeID:                     "run-resolution",
		ResolvedAt:                    time.Date(2026, 9, 30, 1, 0, 0, 123, time.UTC),
		OrphanName:                    ".KoLmafia-test.jar.part-1234",
		FinalArtifactName:             "KoLmafia-test.jar",
		QuarantineName:                "interrupt.part",
		QuarantineSHA256:              strings.Repeat("a", 64),
		OriginalPathAbsentVerified:    true,
		FinalArtifactAbsentVerified:   true,
		ActivePartFilesAbsentVerified: true,
		QuarantineHashVerified:        true,
	}
}

func TestBindVerify(t *testing.T) {
	t.Parallel()
	e, err := Bind(fixture())
	if err != nil {
		t.Fatalf("Bind() error = %v", err)
	}
	if !strings.HasPrefix(e.Digest, "sha256:") {
		t.Fatalf("Digest = %q", e.Digest)
	}
	if !e.OutcomeRemainsUnknown || !e.ArtifactStateRemainsUnknown || !e.InterruptionBlockCleared {
		t.Fatalf("resolution flags = %+v", e)
	}
	if e.AmbiguousBytesDisposition != DispositionQuarantined ||
		!e.OriginalPathAbsentVerified || !e.FinalArtifactAbsentVerified ||
		!e.ActivePartFilesAbsentVerified || !e.QuarantineHashVerified {
		t.Fatalf("quarantine disposition = %+v", e)
	}
	if e.ReplayPermitted || e.AuthorityRestorable || e.ResolutionGrantsExecutionAuthority {
		t.Fatalf("unsafe resolution flags = %+v", e)
	}
	if err := Verify(e); err != nil {
		t.Fatalf("Verify() error = %v", err)
	}
}

func TestBindRejectsUnverifiedDisposition(t *testing.T) {
	t.Parallel()
	in := fixture()
	in.QuarantineHashVerified = false
	if _, err := Bind(in); err == nil || !strings.Contains(err.Error(), "quarantine hash") {
		t.Fatalf("Bind() error = %v, want quarantine verification failure", err)
	}
}

func TestBindRejectsOutcomeAssertionDecision(t *testing.T) {
	t.Parallel()
	in := fixture()
	in.Decision = "mark_failed"
	if _, err := Bind(in); err == nil || !strings.Contains(err.Error(), "unsupported resolution decision") {
		t.Fatalf("Bind() error = %v, want unsupported decision", err)
	}
}

func TestVerifyRejectsTampering(t *testing.T) {
	t.Parallel()
	e, err := Bind(fixture())
	if err != nil {
		t.Fatalf("Bind() error = %v", err)
	}
	e.QuarantineSHA256 = strings.Repeat("b", 64)
	if err := Verify(e); err == nil || !strings.Contains(err.Error(), "digest mismatch") {
		t.Fatalf("Verify() error = %v, want digest mismatch", err)
	}
}

func TestMatchesBindsExactInterruption(t *testing.T) {
	t.Parallel()
	e, err := Bind(fixture())
	if err != nil {
		t.Fatalf("Bind() error = %v", err)
	}
	if err := Matches(e, e.InterruptionDigest, e.ProposalID, e.Operation, e.ExecutionDigest, e.StartedEventHash); err != nil {
		t.Fatalf("Matches() error = %v", err)
	}
	if err := Matches(e, "sha256:other", e.ProposalID, e.Operation, e.ExecutionDigest, e.StartedEventHash); err == nil {
		t.Fatal("Matches() accepted wrong interruption digest")
	}
}
