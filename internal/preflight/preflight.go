package preflight

import (
	"fmt"
	"strings"
	"time"
)

const (
	ObservationMaxAge = 5 * time.Minute
	ReleaseMaxAge     = 30 * time.Minute
	SidecarMaxAge     = 2 * time.Minute
)

type Check struct {
	Name          string    `json:"name"`
	OK            bool      `json:"ok"`
	Required      bool      `json:"required"`
	Detail        string    `json:"detail"`
	ObservedAt    time.Time `json:"observed_at,omitempty"`
	AgeSeconds    int64     `json:"age_seconds,omitempty"`
	MaxAgeSeconds int64     `json:"max_age_seconds,omitempty"`
}

type AuthorityBoundary struct {
	Mode                      string `json:"mode"`
	PreflightGrantsAuthority  bool   `json:"preflight_grants_authority"`
	HumanConfirmationRequired bool   `json:"human_confirmation_required"`
	LiveKoLmafiaMutation      bool   `json:"live_kolmafia_mutation"`
	ReleaseInstall            bool   `json:"release_install"`
	KoLmafiaRestart           bool   `json:"kolmafia_restart"`
	ArbitraryASH              bool   `json:"arbitrary_ash"`
	ArbitraryGCLI             bool   `json:"arbitrary_gcli"`
}

type Result struct {
	Version          string            `json:"version"`
	Status           string            `json:"status"`
	ReadyForProposal bool              `json:"ready_for_proposal"`
	GeneratedAt      time.Time         `json:"generated_at"`
	RuntimeID        string            `json:"runtime_id"`
	ObservationID    string            `json:"observation_id,omitempty"`
	Checks           []Check           `json:"checks"`
	Reasons          []string          `json:"reasons"`
	Authority        AuthorityBoundary `json:"authority"`
}

type Input struct {
	Now                    time.Time
	RuntimeID              string
	ObservationID          string
	ObservationAt          time.Time
	InstalledRevision      string
	LatestRelease          string
	ReleaseRefreshedAt     time.Time
	KingdomsitterCheckedAt time.Time
	KingdomsitterHealthy   bool
	AuditVerified          bool
	Fault                  string
	PendingProposalID      string
	ActiveStates           []string
}

func Build(in Input) Result {
	now := in.Now.UTC()
	if now.IsZero() {
		now = time.Now().UTC()
	}

	checks := make([]Check, 0, 12)
	reasons := make([]string, 0, 12)
	add := func(check Check, reason string) {
		checks = append(checks, check)
		if check.Required && !check.OK {
			reasons = append(reasons, reason)
		}
	}

	add(Check{
		Name:     "runtime_identity",
		OK:       strings.HasPrefix(in.RuntimeID, "run-") && len(in.RuntimeID) > len("run-"),
		Required: true,
		Detail:   "runtime_id must identify the current Actor Engine process",
	}, "runtime_identity_missing")

	add(Check{
		Name:     "audit_ledger",
		OK:       in.AuditVerified,
		Required: true,
		Detail:   "durable audit ledger must be verified",
	}, "audit_ledger_unverified")

	observationPresent := strings.HasPrefix(in.ObservationID, "obs-") && !in.ObservationAt.IsZero()
	add(Check{
		Name:       "kol_observation_present",
		OK:         observationPresent,
		Required:   true,
		Detail:     "bounded KoL observation and observation_id must be present",
		ObservedAt: in.ObservationAt.UTC(),
	}, "kol_observation_missing")
	if observationPresent {
		add(freshnessCheck("kol_observation_fresh", in.ObservationAt, now, ObservationMaxAge, true), "kol_observation_stale")
	}

	add(Check{
		Name:     "installed_revision",
		OK:       strings.TrimSpace(in.InstalledRevision) != "",
		Required: true,
		Detail:   fmt.Sprintf("installed KoLmafia revision=%q", in.InstalledRevision),
	}, "installed_revision_unknown")

	releasePresent := strings.TrimSpace(in.LatestRelease) != "" && !in.ReleaseRefreshedAt.IsZero()
	add(Check{
		Name:       "release_metadata_present",
		OK:         releasePresent,
		Required:   true,
		Detail:     fmt.Sprintf("latest release=%q", in.LatestRelease),
		ObservedAt: in.ReleaseRefreshedAt.UTC(),
	}, "release_metadata_missing")
	if releasePresent {
		add(freshnessCheck("release_metadata_fresh", in.ReleaseRefreshedAt, now, ReleaseMaxAge, true), "release_metadata_stale")
	}

	sidecarKnown := !in.KingdomsitterCheckedAt.IsZero()
	add(Check{
		Name:       "kingdomsitter_status_known",
		OK:         sidecarKnown,
		Required:   true,
		Detail:     "Kingdomsitter transport status must have been checked",
		ObservedAt: in.KingdomsitterCheckedAt.UTC(),
	}, "kingdomsitter_status_unknown")
	if sidecarKnown {
		add(freshnessCheck("kingdomsitter_status_fresh", in.KingdomsitterCheckedAt, now, SidecarMaxAge, true), "kingdomsitter_status_stale")
		add(Check{
			Name:     "kingdomsitter_transport_healthy",
			OK:       in.KingdomsitterHealthy,
			Required: true,
			Detail:   fmt.Sprintf("kingdomsitter transport healthy=%t", in.KingdomsitterHealthy),
		}, "kingdomsitter_transport_unhealthy")
	}

	faulted := strings.TrimSpace(in.Fault) != "" || hasState(in.ActiveStates, "faulted")
	add(Check{
		Name:     "no_unresolved_fault",
		OK:       !faulted,
		Required: true,
		Detail:   fmt.Sprintf("fault=%q", in.Fault),
	}, "unresolved_fault")

	actorReady := hasState(in.ActiveStates, "ready") && !hasState(in.ActiveStates, "booting")
	add(Check{
		Name:     "actor_ready",
		OK:       actorReady,
		Required: true,
		Detail:   "actor state plane must be ready and not booting",
	}, "actor_not_ready")

	gateIdle := strings.TrimSpace(in.PendingProposalID) == "" &&
		!hasState(in.ActiveStates, "proposal_ready") &&
		!hasState(in.ActiveStates, "awaiting_confirmation") &&
		!hasState(in.ActiveStates, "executing") &&
		!hasState(in.ActiveStates, "reconciling")
	add(Check{
		Name:     "proposal_gate_idle",
		OK:       gateIdle,
		Required: true,
		Detail:   fmt.Sprintf("pending_proposal_id=%q", in.PendingProposalID),
	}, "proposal_gate_not_idle")

	ready := len(reasons) == 0
	status := "NOT_READY"
	if ready {
		status = "READY"
	}

	return Result{
		Version:          "kol-actor/preflight-v1",
		Status:           status,
		ReadyForProposal: ready,
		GeneratedAt:      now,
		RuntimeID:        in.RuntimeID,
		ObservationID:    in.ObservationID,
		Checks:           checks,
		Reasons:          reasons,
		Authority: AuthorityBoundary{
			Mode:                      "proposal-and-confirmed-local-staging-only",
			PreflightGrantsAuthority:  false,
			HumanConfirmationRequired: true,
			LiveKoLmafiaMutation:      false,
			ReleaseInstall:            false,
			KoLmafiaRestart:           false,
			ArbitraryASH:              false,
			ArbitraryGCLI:             false,
		},
	}
}

func freshnessCheck(name string, observedAt, now time.Time, maxAge time.Duration, required bool) Check {
	age := now.Sub(observedAt.UTC())
	if age < 0 {
		age = 0
	}
	return Check{
		Name:          name,
		OK:            age <= maxAge,
		Required:      required,
		Detail:        fmt.Sprintf("age=%s max=%s", age.Round(time.Second), maxAge),
		ObservedAt:    observedAt.UTC(),
		AgeSeconds:    int64(age / time.Second),
		MaxAgeSeconds: int64(maxAge / time.Second),
	}
}

func hasState(states []string, target string) bool {
	for _, state := range states {
		if state == target {
			return true
		}
	}
	return false
}
