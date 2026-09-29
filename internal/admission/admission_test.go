package admission

import (
	"strings"
	"testing"
	"time"

	"github.com/donCannoli-burns/actor-engine/internal/preflight"
)

func readyResult() preflight.Result {
	return preflight.Result{
		Version:          "kol-actor/preflight-v1",
		Status:           "READY",
		ReadyForProposal: true,
		GeneratedAt:      time.Date(2026, 9, 29, 18, 0, 0, 0, time.UTC),
		RuntimeID:        "run-0123456789abcdef0123456789abcdef",
		ObservationID:    "obs-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		Checks: []preflight.Check{{
			Name:     "runtime_identity",
			OK:       true,
			Required: true,
			Detail:   "runtime_id must identify the current Actor Engine process",
		}},
		Reasons: []string{},
		Authority: preflight.AuthorityBoundary{
			Mode:                      "proposal-and-confirmed-local-staging-only",
			PreflightGrantsAuthority:  false,
			HumanConfirmationRequired: true,
			LiveKoLmafiaMutation:      false,
		},
	}
}

func TestBindAndVerify(t *testing.T) {
	t.Parallel()
	result := readyResult()
	e, err := Bind(result)
	if err != nil {
		t.Fatalf("Bind() error = %v", err)
	}
	if e.Version != Version {
		t.Fatalf("Version = %q, want %q", e.Version, Version)
	}
	if !strings.HasPrefix(e.Digest, "sha256:") || len(e.Digest) != len("sha256:")+64 {
		t.Fatalf("Digest = %q", e.Digest)
	}
	if err := Verify(e); err != nil {
		t.Fatalf("Verify() error = %v", err)
	}
}

func TestVerifyRejectsChangedPreflight(t *testing.T) {
	t.Parallel()
	e, err := Bind(readyResult())
	if err != nil {
		t.Fatalf("Bind() error = %v", err)
	}
	e.Preflight.ObservationID = "obs-bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	if err := Verify(e); err == nil || !strings.Contains(err.Error(), "digest mismatch") {
		t.Fatalf("Verify() error = %v, want digest mismatch", err)
	}
}

func TestBindRejectsNotReadyOrAuthorityGrant(t *testing.T) {
	t.Parallel()
	notReady := readyResult()
	notReady.Status = "NOT_READY"
	notReady.ReadyForProposal = false
	notReady.Reasons = []string{"kol_observation_missing"}
	if _, err := Bind(notReady); err == nil {
		t.Fatal("Bind() accepted NOT_READY preflight")
	}

	grants := readyResult()
	grants.Authority.PreflightGrantsAuthority = true
	if _, err := Bind(grants); err == nil {
		t.Fatal("Bind() accepted authority-granting preflight")
	}
}
