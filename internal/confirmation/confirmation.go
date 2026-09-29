package confirmation

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

const Version = "kol-actor/confirmation-v1"

type Evidence struct {
	Version                 string    `json:"version"`
	Digest                  string    `json:"digest"`
	ProposalID              string    `json:"proposal_id"`
	StateDigest             string    `json:"state_digest"`
	AdmissionDigest         string    `json:"admission_digest"`
	ConfirmedBy             string    `json:"confirmed_by"`
	RuntimeID               string    `json:"runtime_id"`
	ConfirmedAt             time.Time `json:"confirmed_at"`
	EvidenceGrantsAuthority bool      `json:"evidence_grants_authority"`
	AuthorityRestorable     bool      `json:"authority_restorable"`
}

type Input struct {
	ProposalID      string
	StateDigest     string
	AdmissionDigest string
	ConfirmedBy     string
	RuntimeID       string
	ConfirmedAt     time.Time
}

type digestPayload struct {
	Version                 string    `json:"version"`
	ProposalID              string    `json:"proposal_id"`
	StateDigest             string    `json:"state_digest"`
	AdmissionDigest         string    `json:"admission_digest"`
	ConfirmedBy             string    `json:"confirmed_by"`
	RuntimeID               string    `json:"runtime_id"`
	ConfirmedAt             time.Time `json:"confirmed_at"`
	EvidenceGrantsAuthority bool      `json:"evidence_grants_authority"`
	AuthorityRestorable     bool      `json:"authority_restorable"`
}

func Bind(in Input) (Evidence, error) {
	in.ProposalID = strings.TrimSpace(in.ProposalID)
	in.StateDigest = strings.TrimSpace(in.StateDigest)
	in.AdmissionDigest = strings.TrimSpace(in.AdmissionDigest)
	in.ConfirmedBy = strings.TrimSpace(in.ConfirmedBy)
	in.RuntimeID = strings.TrimSpace(in.RuntimeID)
	if in.ProposalID == "" {
		return Evidence{}, fmt.Errorf("proposal id is empty")
	}
	if in.StateDigest == "" {
		return Evidence{}, fmt.Errorf("state digest is empty")
	}
	if in.AdmissionDigest == "" {
		return Evidence{}, fmt.Errorf("admission digest is empty")
	}
	if in.ConfirmedBy == "" {
		return Evidence{}, fmt.Errorf("confirming human identifier is empty")
	}
	if in.RuntimeID == "" {
		return Evidence{}, fmt.Errorf("runtime id is empty")
	}
	if in.ConfirmedAt.IsZero() {
		return Evidence{}, fmt.Errorf("confirmation timestamp is empty")
	}
	e := Evidence{
		Version:                 Version,
		ProposalID:              in.ProposalID,
		StateDigest:             in.StateDigest,
		AdmissionDigest:         in.AdmissionDigest,
		ConfirmedBy:             in.ConfirmedBy,
		RuntimeID:               in.RuntimeID,
		ConfirmedAt:             in.ConfirmedAt.UTC(),
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
		return fmt.Errorf("confirmation version %q is unsupported", e.Version)
	}
	if e.EvidenceGrantsAuthority {
		return fmt.Errorf("confirmation evidence unexpectedly grants authority")
	}
	if e.AuthorityRestorable {
		return fmt.Errorf("confirmation evidence unexpectedly permits authority restoration")
	}
	if strings.TrimSpace(e.ProposalID) == "" ||
		strings.TrimSpace(e.StateDigest) == "" ||
		strings.TrimSpace(e.AdmissionDigest) == "" ||
		strings.TrimSpace(e.ConfirmedBy) == "" ||
		strings.TrimSpace(e.RuntimeID) == "" ||
		e.ConfirmedAt.IsZero() {
		return fmt.Errorf("confirmation evidence is incomplete")
	}
	want, err := Digest(e)
	if err != nil {
		return err
	}
	if e.Digest != want {
		return fmt.Errorf("confirmation digest mismatch")
	}
	return nil
}

func Matches(e Evidence, proposalID, stateDigest, admissionDigest, runtimeID string) error {
	if err := Verify(e); err != nil {
		return err
	}
	if e.ProposalID != proposalID {
		return fmt.Errorf("confirmation proposal id mismatch")
	}
	if e.StateDigest != stateDigest {
		return fmt.Errorf("confirmation state digest mismatch")
	}
	if e.AdmissionDigest != admissionDigest {
		return fmt.Errorf("confirmation admission digest mismatch")
	}
	if e.RuntimeID != runtimeID {
		return fmt.Errorf("confirmation runtime id mismatch")
	}
	return nil
}

func Digest(e Evidence) (string, error) {
	payload := digestPayload{
		Version:                 e.Version,
		ProposalID:              e.ProposalID,
		StateDigest:             e.StateDigest,
		AdmissionDigest:         e.AdmissionDigest,
		ConfirmedBy:             e.ConfirmedBy,
		RuntimeID:               e.RuntimeID,
		ConfirmedAt:             e.ConfirmedAt.UTC(),
		EvidenceGrantsAuthority: e.EvidenceGrantsAuthority,
		AuthorityRestorable:     e.AuthorityRestorable,
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("canonicalize confirmation evidence: %w", err)
	}
	sum := sha256.Sum256(raw)
	return "sha256:" + hex.EncodeToString(sum[:]), nil
}
