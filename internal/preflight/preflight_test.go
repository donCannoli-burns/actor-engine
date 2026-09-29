package preflight

import (
	"slices"
	"testing"
	"time"
)

func TestBuildReady(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 9, 29, 16, 0, 0, 0, time.UTC)
	got := Build(Input{
		Now:                    now,
		RuntimeID:              "run-0123456789abcdef0123456789abcdef",
		ObservationID:          "obs-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		ObservationAt:          now.Add(-time.Minute),
		InstalledRevision:      "29301",
		LatestRelease:          "r29315",
		ReleaseRefreshedAt:     now.Add(-10 * time.Minute),
		KingdomsitterCheckedAt: now.Add(-30 * time.Second),
		KingdomsitterHealthy:   true,
		AuditVerified:          true,
		ActiveStates:           []string{"ready", "observe_only", "kol_state_seen", "kingdomsitter_seen", "release_available"},
	})
	if got.Status != "READY" || !got.ReadyForProposal {
		t.Fatalf("Build() = status %q ready=%t reasons=%v", got.Status, got.ReadyForProposal, got.Reasons)
	}
	if got.Authority.PreflightGrantsAuthority {
		t.Fatal("preflight unexpectedly grants authority")
	}
	if !got.Authority.HumanConfirmationRequired {
		t.Fatal("preflight unexpectedly removed human confirmation requirement")
	}
}

func TestBuildExplainsMissingAndStaleEvidence(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 9, 29, 16, 0, 0, 0, time.UTC)
	got := Build(Input{
		Now:                    now,
		RuntimeID:              "run-0123456789abcdef0123456789abcdef",
		ObservationID:          "obs-bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		ObservationAt:          now.Add(-ObservationMaxAge - time.Second),
		InstalledRevision:      "",
		LatestRelease:          "r29315",
		ReleaseRefreshedAt:     now.Add(-ReleaseMaxAge - time.Second),
		KingdomsitterCheckedAt: now.Add(-SidecarMaxAge - time.Second),
		KingdomsitterHealthy:   false,
		AuditVerified:          true,
		Fault:                  "kingdomsitter unavailable",
		ActiveStates:           []string{"ready", "observe_only", "faulted"},
		PendingProposalID:      "p-pending",
	})
	if got.Status != "NOT_READY" || got.ReadyForProposal {
		t.Fatalf("Build() = status %q ready=%t", got.Status, got.ReadyForProposal)
	}
	for _, want := range []string{
		"kol_observation_stale",
		"installed_revision_unknown",
		"release_metadata_stale",
		"kingdomsitter_status_stale",
		"kingdomsitter_transport_unhealthy",
		"unresolved_fault",
		"proposal_gate_not_idle",
	} {
		if !slices.Contains(got.Reasons, want) {
			t.Fatalf("reasons=%v missing %q", got.Reasons, want)
		}
	}
}

func TestBuildRequiresEvidencePresence(t *testing.T) {
	t.Parallel()
	got := Build(Input{
		Now:           time.Date(2026, 9, 29, 16, 0, 0, 0, time.UTC),
		RuntimeID:     "run-0123456789abcdef0123456789abcdef",
		AuditVerified: true,
		ActiveStates:  []string{"ready"},
	})
	for _, want := range []string{
		"kol_observation_missing",
		"installed_revision_unknown",
		"release_metadata_missing",
		"kingdomsitter_status_unknown",
	} {
		if !slices.Contains(got.Reasons, want) {
			t.Fatalf("reasons=%v missing %q", got.Reasons, want)
		}
	}
}

func TestBuildBlocksUnresolvedInterruptedExecution(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 9, 29, 16, 0, 0, 0, time.UTC)
	got := Build(Input{
		Now:                    now,
		RuntimeID:              "run-0123456789abcdef0123456789abcdef",
		ObservationID:          "obs-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		ObservationAt:          now.Add(-time.Minute),
		InstalledRevision:      "29301",
		LatestRelease:          "r29315",
		ReleaseRefreshedAt:     now.Add(-10 * time.Minute),
		KingdomsitterCheckedAt: now.Add(-30 * time.Second),
		KingdomsitterHealthy:   true,
		AuditVerified:          true,
		UnresolvedExecutions:   1,
		ActiveStates:           []string{"ready", "observe_only", "kol_state_seen", "kingdomsitter_seen", "release_available"},
	})
	if got.Status != "NOT_READY" || got.ReadyForProposal {
		t.Fatalf("Build() = status %q ready=%t reasons=%v", got.Status, got.ReadyForProposal, got.Reasons)
	}
	if !slices.Contains(got.Reasons, "interrupted_execution_unresolved") {
		t.Fatalf("reasons=%v missing interrupted_execution_unresolved", got.Reasons)
	}
}
