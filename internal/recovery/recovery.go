package recovery

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/donCannoli-burns/actor-engine/internal/audit"
)

const (
	Version             = "kol-actor/recovery-v1"
	InterruptionVersion = "kol-actor/interruption-v1"
	StatusClear         = "CLEAR"
	StatusInterrupted   = "INTERRUPTED_UNKNOWN_OUTCOME"
	ArtifactUnknown     = "unknown"
)

type AuthorityBoundary struct {
	ReplayPermitted      bool `json:"replay_permitted"`
	AuthorityRestorable  bool `json:"authority_restorable"`
	AutomaticResolution  bool `json:"automatic_resolution"`
	EvidenceOnly         bool `json:"evidence_only"`
}

type Interruption struct {
	Version             string    `json:"version"`
	Digest              string    `json:"digest"`
	ProposalID          string    `json:"proposal_id"`
	Operation           string    `json:"operation"`
	AdmissionDigest     string    `json:"admission_digest"`
	ConfirmationDigest  string    `json:"confirmation_digest"`
	ExecutionDigest     string    `json:"execution_digest"`
	OriginRuntimeID     string    `json:"origin_runtime_id"`
	ExecutionRuntimeID  string    `json:"execution_runtime_id"`
	ObservationID       string    `json:"observation_id,omitempty"`
	StartedSeq          uint64    `json:"started_seq"`
	StartedAt           time.Time `json:"started_at"`
	StartedEventHash    string    `json:"started_event_hash"`
	Status              string    `json:"status"`
	OutcomeKnown        bool      `json:"outcome_known"`
	ArtifactState       string    `json:"artifact_state"`
	ReplayPermitted     bool      `json:"replay_permitted"`
	AuthorityRestorable bool      `json:"authority_restorable"`
}

type Report struct {
	Version      string            `json:"version"`
	Status       string            `json:"status"`
	GeneratedAt  time.Time         `json:"generated_at"`
	RuntimeID    string            `json:"runtime_id"`
	Unresolved   []Interruption    `json:"unresolved"`
	Authority    AuthorityBoundary `json:"authority"`
}

type interruptionDigestPayload struct {
	Version             string    `json:"version"`
	ProposalID          string    `json:"proposal_id"`
	Operation           string    `json:"operation"`
	AdmissionDigest     string    `json:"admission_digest"`
	ConfirmationDigest  string    `json:"confirmation_digest"`
	ExecutionDigest     string    `json:"execution_digest"`
	OriginRuntimeID     string    `json:"origin_runtime_id"`
	ExecutionRuntimeID  string    `json:"execution_runtime_id"`
	ObservationID       string    `json:"observation_id,omitempty"`
	StartedSeq          uint64    `json:"started_seq"`
	StartedAt           time.Time `json:"started_at"`
	StartedEventHash    string    `json:"started_event_hash"`
	Status              string    `json:"status"`
	OutcomeKnown        bool      `json:"outcome_known"`
	ArtifactState       string    `json:"artifact_state"`
	ReplayPermitted     bool      `json:"replay_permitted"`
	AuthorityRestorable bool      `json:"authority_restorable"`
}

func Build(events []audit.Event, runtimeID string, now time.Time) Report {
	if now.IsZero() {
		now = time.Now().UTC()
	} else {
		now = now.UTC()
	}

	terminal := make(map[string]struct{})
	for _, event := range events {
		if event.ExecutionDigest == "" {
			continue
		}
		if event.Type == audit.EventExecutionSucceeded || event.Type == audit.EventExecutionFailed {
			terminal[event.ExecutionDigest] = struct{}{}
		}
	}

	seen := make(map[string]struct{})
	unresolved := make([]Interruption, 0)
	for _, event := range events {
		if event.Type != audit.EventExecutionStarted || strings.TrimSpace(event.ExecutionDigest) == "" {
			continue
		}
		if _, ok := terminal[event.ExecutionDigest]; ok {
			continue
		}
		if _, ok := seen[event.ExecutionDigest]; ok {
			continue
		}
		seen[event.ExecutionDigest] = struct{}{}

		originRuntimeID := event.RuntimeID
		executionRuntimeID := event.RuntimeID
		if event.Execution != nil {
			if event.Execution.OriginRuntimeID != "" {
				originRuntimeID = event.Execution.OriginRuntimeID
			}
			if event.Execution.ExecutionRuntimeID != "" {
				executionRuntimeID = event.Execution.ExecutionRuntimeID
			}
		}

		item := Interruption{
			Version:             InterruptionVersion,
			ProposalID:          event.ProposalID,
			Operation:           event.Operation,
			AdmissionDigest:     event.AdmissionDigest,
			ConfirmationDigest:  event.ConfirmationDigest,
			ExecutionDigest:     event.ExecutionDigest,
			OriginRuntimeID:     originRuntimeID,
			ExecutionRuntimeID:  executionRuntimeID,
			ObservationID:       event.ObservationID,
			StartedSeq:          event.Seq,
			StartedAt:           event.At.UTC(),
			StartedEventHash:    event.Hash,
			Status:              StatusInterrupted,
			OutcomeKnown:        false,
			ArtifactState:       ArtifactUnknown,
			ReplayPermitted:     false,
			AuthorityRestorable: false,
		}
		digest, err := Digest(item)
		if err != nil {
			continue
		}
		item.Digest = digest
		unresolved = append(unresolved, item)
	}

	sort.Slice(unresolved, func(i, j int) bool {
		if unresolved[i].StartedSeq == unresolved[j].StartedSeq {
			return unresolved[i].ExecutionDigest < unresolved[j].ExecutionDigest
		}
		return unresolved[i].StartedSeq < unresolved[j].StartedSeq
	})

	status := StatusClear
	if len(unresolved) > 0 {
		status = StatusInterrupted
	}
	return Report{
		Version:     Version,
		Status:      status,
		GeneratedAt: now,
		RuntimeID:   runtimeID,
		Unresolved:  unresolved,
		Authority: AuthorityBoundary{
			ReplayPermitted:     false,
			AuthorityRestorable: false,
			AutomaticResolution: false,
			EvidenceOnly:        true,
		},
	}
}

func Verify(item Interruption) error {
	if item.Version != InterruptionVersion {
		return fmt.Errorf("interruption version %q is unsupported", item.Version)
	}
	if item.Status != StatusInterrupted {
		return fmt.Errorf("interruption status %q is unsupported", item.Status)
	}
	if item.OutcomeKnown {
		return fmt.Errorf("interrupted execution unexpectedly claims a known outcome")
	}
	if item.ArtifactState != ArtifactUnknown {
		return fmt.Errorf("interrupted execution artifact state %q is unsupported", item.ArtifactState)
	}
	if item.ReplayPermitted || item.AuthorityRestorable {
		return fmt.Errorf("interrupted execution unexpectedly grants authority")
	}
	if strings.TrimSpace(item.ProposalID) == "" ||
		strings.TrimSpace(item.Operation) == "" ||
		strings.TrimSpace(item.ExecutionDigest) == "" ||
		strings.TrimSpace(item.ExecutionRuntimeID) == "" ||
		item.StartedSeq == 0 ||
		item.StartedAt.IsZero() ||
		strings.TrimSpace(item.StartedEventHash) == "" {
		return fmt.Errorf("interruption evidence is incomplete")
	}
	want, err := Digest(item)
	if err != nil {
		return err
	}
	if item.Digest != want {
		return fmt.Errorf("interruption digest mismatch")
	}
	return nil
}

func Digest(item Interruption) (string, error) {
	payload := interruptionDigestPayload{
		Version:             item.Version,
		ProposalID:          item.ProposalID,
		Operation:           item.Operation,
		AdmissionDigest:     item.AdmissionDigest,
		ConfirmationDigest:  item.ConfirmationDigest,
		ExecutionDigest:     item.ExecutionDigest,
		OriginRuntimeID:     item.OriginRuntimeID,
		ExecutionRuntimeID:  item.ExecutionRuntimeID,
		ObservationID:       item.ObservationID,
		StartedSeq:          item.StartedSeq,
		StartedAt:           item.StartedAt.UTC(),
		StartedEventHash:    item.StartedEventHash,
		Status:              item.Status,
		OutcomeKnown:        item.OutcomeKnown,
		ArtifactState:       item.ArtifactState,
		ReplayPermitted:     item.ReplayPermitted,
		AuthorityRestorable: item.AuthorityRestorable,
	}
	raw, err := canonicalJSON(payload)
	if err != nil {
		return "", fmt.Errorf("canonicalize interruption: %w", err)
	}
	sum := sha256.Sum256(raw)
	return "sha256:" + hex.EncodeToString(sum[:]), nil
}

func canonicalJSON(v any) ([]byte, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var generic any
	if err := json.Unmarshal(raw, &generic); err != nil {
		return nil, err
	}
	return json.Marshal(generic)
}
