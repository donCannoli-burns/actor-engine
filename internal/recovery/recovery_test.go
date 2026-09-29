package recovery

import (
	"strings"
	"testing"
	"time"

	"github.com/donCannoli-burns/actor-engine/internal/audit"
	"github.com/donCannoli-burns/actor-engine/internal/execution"
)

func startedFixture() audit.Event {
	at := time.Date(2026, 9, 29, 23, 30, 0, 0, time.UTC)
	return audit.Event{
		Seq:                7,
		At:                 at,
		Type:               audit.EventExecutionStarted,
		ProposalID:         "p-crash",
		RuntimeID:          "run-origin",
		ObservationID:      "obs-origin",
		AdmissionDigest:    "sha256:admission",
		ConfirmationDigest: "sha256:confirmation",
		ExecutionDigest:    "sha256:execution",
		Operation:          "release.stage",
		Hash:               "ledger-hash-7",
		Execution: &execution.Evidence{
			OriginRuntimeID:    "run-origin",
			ExecutionRuntimeID: "run-origin",
		},
	}
}

func TestBuildDetectsUnmatchedExecutionStarted(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 9, 29, 23, 31, 0, 0, time.UTC)
	report := Build([]audit.Event{startedFixture()}, "run-recovery", now)
	if report.Status != StatusInterrupted || len(report.Unresolved) != 1 {
		t.Fatalf("report = %+v", report)
	}
	item := report.Unresolved[0]
	if err := Verify(item); err != nil {
		t.Fatalf("Verify() error = %v", err)
	}
	if item.OutcomeKnown || item.ReplayPermitted || item.AuthorityRestorable {
		t.Fatalf("unsafe interruption flags = %+v", item)
	}
	if item.ArtifactState != ArtifactUnknown {
		t.Fatalf("ArtifactState = %q", item.ArtifactState)
	}
}

func TestBuildIgnoresTerminalExecution(t *testing.T) {
	t.Parallel()
	started := startedFixture()
	failed := audit.Event{
		Seq:             8,
		Type:            audit.EventExecutionFailed,
		ProposalID:      started.ProposalID,
		ExecutionDigest: started.ExecutionDigest,
	}
	report := Build([]audit.Event{started, failed}, "run-recovery", time.Now())
	if report.Status != StatusClear || len(report.Unresolved) != 0 {
		t.Fatalf("report = %+v", report)
	}
}

func TestInterruptionDigestStableAcrossRecoveryRuntimes(t *testing.T) {
	t.Parallel()
	events := []audit.Event{startedFixture()}
	a := Build(events, "run-a", time.Date(2026, 9, 29, 23, 31, 0, 0, time.UTC))
	b := Build(events, "run-b", time.Date(2026, 9, 29, 23, 32, 0, 0, time.UTC))
	if len(a.Unresolved) != 1 || len(b.Unresolved) != 1 {
		t.Fatalf("unexpected reports: a=%+v b=%+v", a, b)
	}
	if a.Unresolved[0].Digest != b.Unresolved[0].Digest {
		t.Fatalf("interruption digest changed across recovery runtimes: %q != %q", a.Unresolved[0].Digest, b.Unresolved[0].Digest)
	}
}

func TestVerifyRejectsTampering(t *testing.T) {
	t.Parallel()
	report := Build([]audit.Event{startedFixture()}, "run-recovery", time.Now())
	item := report.Unresolved[0]
	item.ArtifactState = "missing"
	if err := Verify(item); err == nil || !strings.Contains(err.Error(), "artifact state") {
		t.Fatalf("Verify() error = %v, want artifact state failure", err)
	}
}
