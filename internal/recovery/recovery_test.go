package recovery

import (
	"strings"
	"testing"
	"time"

	"github.com/donCannoli-burns/actor-engine/internal/audit"
	"github.com/donCannoli-burns/actor-engine/internal/execution"
	"github.com/donCannoli-burns/actor-engine/internal/resolution"
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

func TestBuildMovesVerifiedQuarantineResolutionOutOfUnresolvedSet(t *testing.T) {
	t.Parallel()
	started := startedFixture()
	initial := Build([]audit.Event{started}, "run-recovery", time.Date(2026, 9, 30, 1, 0, 0, 0, time.UTC))
	if len(initial.Unresolved) != 1 {
		t.Fatalf("initial report = %+v", initial)
	}
	interruption := initial.Unresolved[0]
	resolved, err := resolution.Bind(resolution.Input{
		InterruptionDigest:         interruption.Digest,
		ProposalID:                 interruption.ProposalID,
		Operation:                  interruption.Operation,
		ExecutionDigest:            interruption.ExecutionDigest,
		StartedEventHash:           interruption.StartedEventHash,
		Decision:                   resolution.DecisionQuarantineUnknownNoReplay,
		ResolvedBy:                 "test-human",
		Note:                       "quarantine verified; abandon replay",
		RuntimeID:                  "run-resolution",
		ResolvedAt:                 time.Date(2026, 9, 30, 1, 1, 0, 0, time.UTC),
		OrphanName:                 ".KoLmafia-test.jar.part-1234",
		QuarantineName:             "interrupt.part",
		QuarantineSHA256:           strings.Repeat("a", 64),
		OriginalPathAbsentVerified: true,
		QuarantineHashVerified:     true,
	})
	if err != nil {
		t.Fatalf("resolution.Bind() error = %v", err)
	}
	events := []audit.Event{
		started,
		{
			Seq:                8,
			At:                 resolved.ResolvedAt,
			Type:               audit.EventRecoveryResolved,
			ProposalID:         interruption.ProposalID,
			RuntimeID:          resolved.RuntimeID,
			ExecutionDigest:    interruption.ExecutionDigest,
			InterruptionDigest: interruption.Digest,
			ResolutionDigest:   resolved.Digest,
			Resolution:         &resolved,
			Operation:          interruption.Operation,
			Hash:               "resolution-event-hash",
		},
	}
	report := Build(events, "run-after-resolution", time.Date(2026, 9, 30, 1, 2, 0, 0, time.UTC))
	if report.Status != StatusClear || len(report.Unresolved) != 0 || len(report.Resolved) != 1 {
		t.Fatalf("resolved report = %+v", report)
	}
	if report.Resolved[0].Interruption.Digest != interruption.Digest {
		t.Fatalf("resolved interruption digest = %q want %q", report.Resolved[0].Interruption.Digest, interruption.Digest)
	}
	if err := resolution.Verify(report.Resolved[0].Resolution); err != nil {
		t.Fatalf("resolved evidence invalid: %v", err)
	}
	if report.Resolved[0].Interruption.OutcomeKnown ||
		report.Resolved[0].Interruption.ArtifactState != ArtifactUnknown {
		t.Fatalf("resolution rewrote historical ambiguity: %+v", report.Resolved[0].Interruption)
	}
}

func TestBuildRejectsResolutionWithoutVerifiedQuarantine(t *testing.T) {
	t.Parallel()
	started := startedFixture()
	initial := Build([]audit.Event{started}, "run-recovery", time.Now())
	interruption := initial.Unresolved[0]
	bad := resolution.Evidence{
		Version:                            resolution.Version,
		Digest:                             "sha256:bad",
		InterruptionDigest:                 interruption.Digest,
		ProposalID:                         interruption.ProposalID,
		Operation:                          interruption.Operation,
		ExecutionDigest:                    interruption.ExecutionDigest,
		StartedEventHash:                   interruption.StartedEventHash,
		Decision:                           resolution.DecisionQuarantineUnknownNoReplay,
		ResolvedBy:                         "test-human",
		Note:                               "not actually verified",
		RuntimeID:                          "run-resolution",
		ResolvedAt:                         time.Now().UTC(),
		OutcomeRemainsUnknown:              true,
		ArtifactStateRemainsUnknown:        true,
		AmbiguousBytesDisposition:          resolution.DispositionQuarantined,
		OrphanName:                         ".KoLmafia-test.jar.part-1",
		QuarantineName:                     "interrupt.part",
		QuarantineSHA256:                   strings.Repeat("a", 64),
		OriginalPathAbsentVerified:         false,
		QuarantineHashVerified:             false,
		ReplayPermitted:                    false,
		AuthorityRestorable:                false,
		ResolutionGrantsExecutionAuthority: false,
		InterruptionBlockCleared:           true,
	}
	report := Build([]audit.Event{
		started,
		{
			Seq:                8,
			Type:               audit.EventRecoveryResolved,
			ProposalID:         interruption.ProposalID,
			ExecutionDigest:    interruption.ExecutionDigest,
			InterruptionDigest: interruption.Digest,
			ResolutionDigest:   bad.Digest,
			Resolution:         &bad,
			Operation:          interruption.Operation,
		},
	}, "run-after-bad-resolution", time.Now())
	if report.Status != StatusInterrupted || len(report.Unresolved) != 1 || len(report.Resolved) != 0 {
		t.Fatalf("invalid resolution unexpectedly cleared interruption: %+v", report)
	}
}
