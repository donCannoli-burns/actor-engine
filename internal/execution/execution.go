package execution

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

const Version = "kol-actor/execution-attempt-v1"

const (
	GateAuthorized = "authorized"
	GateDenied     = "denied"
)

type Evidence struct {
	Version                 string    `json:"version"`
	Digest                  string    `json:"digest"`
	ProposalID              string    `json:"proposal_id"`
	Operation               string    `json:"operation"`
	ProposalStateDigest     string    `json:"proposal_state_digest"`
	AdmissionDigest         string    `json:"admission_digest"`
	ConfirmationDigest      string    `json:"confirmation_digest"`
	OriginRuntimeID         string    `json:"origin_runtime_id"`
	ExecutionRuntimeID      string    `json:"execution_runtime_id"`
	CurrentStateDigest      string    `json:"current_state_digest"`
	AttemptedAt             time.Time `json:"attempted_at"`
	GateDecision            string    `json:"gate_decision"`
	EvidenceGrantsAuthority bool      `json:"evidence_grants_authority"`
}

type Input struct {
	ProposalID          string
	Operation           string
	ProposalStateDigest string
	AdmissionDigest     string
	ConfirmationDigest  string
	OriginRuntimeID     string
	ExecutionRuntimeID  string
	CurrentStateDigest  string
	AttemptedAt         time.Time
	GateDecision        string
}

type digestPayload struct {
	Version                 string    `json:"version"`
	ProposalID              string    `json:"proposal_id"`
	Operation               string    `json:"operation"`
	ProposalStateDigest     string    `json:"proposal_state_digest"`
	AdmissionDigest         string    `json:"admission_digest"`
	ConfirmationDigest      string    `json:"confirmation_digest"`
	OriginRuntimeID         string    `json:"origin_runtime_id"`
	ExecutionRuntimeID      string    `json:"execution_runtime_id"`
	CurrentStateDigest      string    `json:"current_state_digest"`
	AttemptedAt             time.Time `json:"attempted_at"`
	GateDecision            string    `json:"gate_decision"`
	EvidenceGrantsAuthority bool      `json:"evidence_grants_authority"`
}

func Bind(in Input) (Evidence, error) {
	in.ProposalID = strings.TrimSpace(in.ProposalID)
	in.Operation = strings.TrimSpace(in.Operation)
	in.ProposalStateDigest = strings.TrimSpace(in.ProposalStateDigest)
	in.AdmissionDigest = strings.TrimSpace(in.AdmissionDigest)
	in.ConfirmationDigest = strings.TrimSpace(in.ConfirmationDigest)
	in.OriginRuntimeID = strings.TrimSpace(in.OriginRuntimeID)
	in.ExecutionRuntimeID = strings.TrimSpace(in.ExecutionRuntimeID)
	in.CurrentStateDigest = strings.TrimSpace(in.CurrentStateDigest)
	in.GateDecision = strings.TrimSpace(in.GateDecision)

	switch {
	case in.ProposalID == "":
		return Evidence{}, fmt.Errorf("proposal id is empty")
	case in.Operation == "":
		return Evidence{}, fmt.Errorf("operation is empty")
	case in.ProposalStateDigest == "":
		return Evidence{}, fmt.Errorf("proposal state digest is empty")
	case in.AdmissionDigest == "":
		return Evidence{}, fmt.Errorf("admission digest is empty")
	case in.ConfirmationDigest == "":
		return Evidence{}, fmt.Errorf("confirmation digest is empty")
	case in.OriginRuntimeID == "":
		return Evidence{}, fmt.Errorf("origin runtime id is empty")
	case in.ExecutionRuntimeID == "":
		return Evidence{}, fmt.Errorf("execution runtime id is empty")
	case in.CurrentStateDigest == "":
		return Evidence{}, fmt.Errorf("current state digest is empty")
	case in.AttemptedAt.IsZero():
		return Evidence{}, fmt.Errorf("attempt timestamp is empty")
	case in.GateDecision != GateAuthorized && in.GateDecision != GateDenied:
		return Evidence{}, fmt.Errorf("unsupported gate decision %q", in.GateDecision)
	}

	e := Evidence{
		Version:                 Version,
		ProposalID:              in.ProposalID,
		Operation:               in.Operation,
		ProposalStateDigest:     in.ProposalStateDigest,
		AdmissionDigest:         in.AdmissionDigest,
		ConfirmationDigest:      in.ConfirmationDigest,
		OriginRuntimeID:         in.OriginRuntimeID,
		ExecutionRuntimeID:      in.ExecutionRuntimeID,
		CurrentStateDigest:      in.CurrentStateDigest,
		AttemptedAt:             in.AttemptedAt.UTC(),
		GateDecision:            in.GateDecision,
		EvidenceGrantsAuthority: false,
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
		return fmt.Errorf("execution attempt version %q is unsupported", e.Version)
	}
	if e.EvidenceGrantsAuthority {
		return fmt.Errorf("execution attempt evidence unexpectedly grants authority")
	}
	if strings.TrimSpace(e.ProposalID) == "" ||
		strings.TrimSpace(e.Operation) == "" ||
		strings.TrimSpace(e.ProposalStateDigest) == "" ||
		strings.TrimSpace(e.AdmissionDigest) == "" ||
		strings.TrimSpace(e.ConfirmationDigest) == "" ||
		strings.TrimSpace(e.OriginRuntimeID) == "" ||
		strings.TrimSpace(e.ExecutionRuntimeID) == "" ||
		strings.TrimSpace(e.CurrentStateDigest) == "" ||
		e.AttemptedAt.IsZero() {
		return fmt.Errorf("execution attempt evidence is incomplete")
	}
	if e.GateDecision != GateAuthorized && e.GateDecision != GateDenied {
		return fmt.Errorf("unsupported gate decision %q", e.GateDecision)
	}
	want, err := Digest(e)
	if err != nil {
		return err
	}
	if e.Digest != want {
		return fmt.Errorf("execution attempt digest mismatch")
	}
	return nil
}

func Digest(e Evidence) (string, error) {
	payload := digestPayload{
		Version:                 e.Version,
		ProposalID:              e.ProposalID,
		Operation:               e.Operation,
		ProposalStateDigest:     e.ProposalStateDigest,
		AdmissionDigest:         e.AdmissionDigest,
		ConfirmationDigest:      e.ConfirmationDigest,
		OriginRuntimeID:         e.OriginRuntimeID,
		ExecutionRuntimeID:      e.ExecutionRuntimeID,
		CurrentStateDigest:      e.CurrentStateDigest,
		AttemptedAt:             e.AttemptedAt.UTC(),
		GateDecision:            e.GateDecision,
		EvidenceGrantsAuthority: e.EvidenceGrantsAuthority,
	}
	raw, err := canonicalJSON(payload)
	if err != nil {
		return "", fmt.Errorf("canonicalize execution attempt: %w", err)
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
