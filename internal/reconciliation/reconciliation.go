package reconciliation

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

const Version = "kol-actor/reconciliation-v1"

const (
	OutcomeSucceeded = "succeeded"
	OutcomeFailed    = "failed"
)

type Evidence struct {
	Version                 string    `json:"version"`
	Digest                  string    `json:"digest"`
	ProposalID              string    `json:"proposal_id"`
	Operation               string    `json:"operation"`
	AdmissionDigest         string    `json:"admission_digest"`
	ConfirmationDigest      string    `json:"confirmation_digest"`
	ExecutionDigest         string    `json:"execution_digest"`
	RuntimeID               string    `json:"runtime_id"`
	ObservationID           string    `json:"observation_id,omitempty"`
	Success                 bool      `json:"success"`
	Outcome                 string    `json:"outcome"`
	Detail                  string    `json:"detail"`
	ArtifactPath            string    `json:"artifact_path,omitempty"`
	SHA256                  string    `json:"sha256,omitempty"`
	CompletedAt             time.Time `json:"completed_at"`
	ArtifactCommitted       bool      `json:"artifact_committed"`
	EvidenceGrantsAuthority bool      `json:"evidence_grants_authority"`
	AuthorityRestorable     bool      `json:"authority_restorable"`
}

type Input struct {
	ProposalID         string
	Operation          string
	AdmissionDigest    string
	ConfirmationDigest string
	ExecutionDigest    string
	RuntimeID          string
	ObservationID      string
	Success            bool
	Detail             string
	ArtifactPath       string
	SHA256             string
	CompletedAt        time.Time
	ArtifactCommitted  bool
}

type digestPayload struct {
	Version                 string    `json:"version"`
	ProposalID              string    `json:"proposal_id"`
	Operation               string    `json:"operation"`
	AdmissionDigest         string    `json:"admission_digest"`
	ConfirmationDigest      string    `json:"confirmation_digest"`
	ExecutionDigest         string    `json:"execution_digest"`
	RuntimeID               string    `json:"runtime_id"`
	ObservationID           string    `json:"observation_id,omitempty"`
	Success                 bool      `json:"success"`
	Outcome                 string    `json:"outcome"`
	Detail                  string    `json:"detail"`
	ArtifactPath            string    `json:"artifact_path,omitempty"`
	SHA256                  string    `json:"sha256,omitempty"`
	CompletedAt             time.Time `json:"completed_at"`
	ArtifactCommitted       bool      `json:"artifact_committed"`
	EvidenceGrantsAuthority bool      `json:"evidence_grants_authority"`
	AuthorityRestorable     bool      `json:"authority_restorable"`
}

func Bind(in Input) (Evidence, error) {
	in.ProposalID = strings.TrimSpace(in.ProposalID)
	in.Operation = strings.TrimSpace(in.Operation)
	in.AdmissionDigest = strings.TrimSpace(in.AdmissionDigest)
	in.ConfirmationDigest = strings.TrimSpace(in.ConfirmationDigest)
	in.ExecutionDigest = strings.TrimSpace(in.ExecutionDigest)
	in.RuntimeID = strings.TrimSpace(in.RuntimeID)
	in.ObservationID = strings.TrimSpace(in.ObservationID)
	in.Detail = strings.TrimSpace(in.Detail)
	in.ArtifactPath = strings.TrimSpace(in.ArtifactPath)
	in.SHA256 = strings.TrimSpace(in.SHA256)

	switch {
	case in.ProposalID == "":
		return Evidence{}, fmt.Errorf("proposal id is empty")
	case in.Operation == "":
		return Evidence{}, fmt.Errorf("operation is empty")
	case in.AdmissionDigest == "":
		return Evidence{}, fmt.Errorf("admission digest is empty")
	case in.ConfirmationDigest == "":
		return Evidence{}, fmt.Errorf("confirmation digest is empty")
	case in.ExecutionDigest == "":
		return Evidence{}, fmt.Errorf("execution digest is empty")
	case in.RuntimeID == "":
		return Evidence{}, fmt.Errorf("runtime id is empty")
	case in.Detail == "":
		return Evidence{}, fmt.Errorf("detail is empty")
	case in.CompletedAt.IsZero():
		return Evidence{}, fmt.Errorf("completion timestamp is empty")
	case in.ArtifactCommitted && !in.Success:
		return Evidence{}, fmt.Errorf("failed outcome cannot mark artifact committed")
	case in.ArtifactCommitted && in.ArtifactPath == "":
		return Evidence{}, fmt.Errorf("committed artifact path is empty")
	}

	outcome := OutcomeFailed
	if in.Success {
		outcome = OutcomeSucceeded
	}

	e := Evidence{
		Version:                 Version,
		ProposalID:              in.ProposalID,
		Operation:               in.Operation,
		AdmissionDigest:         in.AdmissionDigest,
		ConfirmationDigest:      in.ConfirmationDigest,
		ExecutionDigest:         in.ExecutionDigest,
		RuntimeID:               in.RuntimeID,
		ObservationID:           in.ObservationID,
		Success:                 in.Success,
		Outcome:                 outcome,
		Detail:                  in.Detail,
		ArtifactPath:            in.ArtifactPath,
		SHA256:                  in.SHA256,
		CompletedAt:             in.CompletedAt.UTC(),
		ArtifactCommitted:       in.ArtifactCommitted,
		EvidenceGrantsAuthority: false,
		AuthorityRestorable:     false,
	}
	digest, err := Digest(e)
	if err != nil {
		return Evidence{}, err
	}
	e.Digest = digest
	return e, nil
}

func Verify(e Evidence) error {
	if e.Version != Version {
		return fmt.Errorf("reconciliation version %q is unsupported", e.Version)
	}
	if e.EvidenceGrantsAuthority {
		return fmt.Errorf("reconciliation evidence unexpectedly grants authority")
	}
	if e.AuthorityRestorable {
		return fmt.Errorf("reconciliation evidence unexpectedly restores authority")
	}
	if strings.TrimSpace(e.ProposalID) == "" ||
		strings.TrimSpace(e.Operation) == "" ||
		strings.TrimSpace(e.AdmissionDigest) == "" ||
		strings.TrimSpace(e.ConfirmationDigest) == "" ||
		strings.TrimSpace(e.ExecutionDigest) == "" ||
		strings.TrimSpace(e.RuntimeID) == "" ||
		strings.TrimSpace(e.Detail) == "" ||
		e.CompletedAt.IsZero() {
		return fmt.Errorf("reconciliation evidence is incomplete")
	}
	wantOutcome := OutcomeFailed
	if e.Success {
		wantOutcome = OutcomeSucceeded
	}
	if e.Outcome != wantOutcome {
		return fmt.Errorf("reconciliation outcome %q does not match success=%t", e.Outcome, e.Success)
	}
	if e.ArtifactCommitted && !e.Success {
		return fmt.Errorf("failed reconciliation marks artifact committed")
	}
	if e.ArtifactCommitted && strings.TrimSpace(e.ArtifactPath) == "" {
		return fmt.Errorf("committed artifact path is empty")
	}
	want, err := Digest(e)
	if err != nil {
		return err
	}
	if e.Digest != want {
		return fmt.Errorf("reconciliation digest mismatch")
	}
	return nil
}

func Digest(e Evidence) (string, error) {
	payload := digestPayload{
		Version:                 e.Version,
		ProposalID:              e.ProposalID,
		Operation:               e.Operation,
		AdmissionDigest:         e.AdmissionDigest,
		ConfirmationDigest:      e.ConfirmationDigest,
		ExecutionDigest:         e.ExecutionDigest,
		RuntimeID:               e.RuntimeID,
		ObservationID:           e.ObservationID,
		Success:                 e.Success,
		Outcome:                 e.Outcome,
		Detail:                  e.Detail,
		ArtifactPath:            e.ArtifactPath,
		SHA256:                  e.SHA256,
		CompletedAt:             e.CompletedAt.UTC(),
		ArtifactCommitted:       e.ArtifactCommitted,
		EvidenceGrantsAuthority: e.EvidenceGrantsAuthority,
		AuthorityRestorable:     e.AuthorityRestorable,
	}
	raw, err := canonicalJSON(payload)
	if err != nil {
		return "", fmt.Errorf("canonicalize reconciliation: %w", err)
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
