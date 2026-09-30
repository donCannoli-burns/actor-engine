package resolution

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

const (
	Version                           = "kol-actor/resolution-v1"
	DecisionQuarantineUnknownNoReplay = "quarantine_unknown_no_replay"
	DispositionQuarantined            = "quarantined"
	MaxActorLength                    = 128
	MaxNoteLength                     = 512
	MaxNameLength                     = 255
)

type Evidence struct {
	Version                            string    `json:"version"`
	Digest                             string    `json:"digest"`
	InterruptionDigest                 string    `json:"interruption_digest"`
	ProposalID                         string    `json:"proposal_id"`
	Operation                          string    `json:"operation"`
	ExecutionDigest                    string    `json:"execution_digest"`
	StartedEventHash                   string    `json:"started_event_hash"`
	Decision                           string    `json:"decision"`
	ResolvedBy                         string    `json:"resolved_by"`
	Note                               string    `json:"note"`
	RuntimeID                          string    `json:"runtime_id"`
	ResolvedAt                         time.Time `json:"resolved_at"`
	OutcomeRemainsUnknown              bool      `json:"outcome_remains_unknown"`
	ArtifactStateRemainsUnknown        bool      `json:"artifact_state_remains_unknown"`
	AmbiguousBytesDisposition          string    `json:"ambiguous_bytes_disposition"`
	OrphanName                         string    `json:"orphan_name"`
	FinalArtifactName                  string    `json:"final_artifact_name"`
	QuarantineName                     string    `json:"quarantine_name"`
	QuarantineSHA256                   string    `json:"quarantine_sha256"`
	OriginalPathAbsentVerified         bool      `json:"original_path_absent_verified"`
	FinalArtifactAbsentVerified        bool      `json:"final_artifact_absent_verified"`
	ActivePartFilesAbsentVerified      bool      `json:"active_part_files_absent_verified"`
	QuarantineHashVerified             bool      `json:"quarantine_hash_verified"`
	ReplayPermitted                    bool      `json:"replay_permitted"`
	AuthorityRestorable                bool      `json:"authority_restorable"`
	ResolutionGrantsExecutionAuthority bool      `json:"resolution_grants_execution_authority"`
	InterruptionBlockCleared           bool      `json:"interruption_block_cleared"`
}

type Input struct {
	InterruptionDigest            string
	ProposalID                    string
	Operation                     string
	ExecutionDigest               string
	StartedEventHash              string
	Decision                      string
	ResolvedBy                    string
	Note                          string
	RuntimeID                     string
	ResolvedAt                    time.Time
	OrphanName                    string
	FinalArtifactName             string
	QuarantineName                string
	QuarantineSHA256              string
	OriginalPathAbsentVerified    bool
	FinalArtifactAbsentVerified   bool
	ActivePartFilesAbsentVerified bool
	QuarantineHashVerified        bool
}

type digestPayload struct {
	Version                            string    `json:"version"`
	InterruptionDigest                 string    `json:"interruption_digest"`
	ProposalID                         string    `json:"proposal_id"`
	Operation                          string    `json:"operation"`
	ExecutionDigest                    string    `json:"execution_digest"`
	StartedEventHash                   string    `json:"started_event_hash"`
	Decision                           string    `json:"decision"`
	ResolvedBy                         string    `json:"resolved_by"`
	Note                               string    `json:"note"`
	RuntimeID                          string    `json:"runtime_id"`
	ResolvedAt                         time.Time `json:"resolved_at"`
	OutcomeRemainsUnknown              bool      `json:"outcome_remains_unknown"`
	ArtifactStateRemainsUnknown        bool      `json:"artifact_state_remains_unknown"`
	AmbiguousBytesDisposition          string    `json:"ambiguous_bytes_disposition"`
	OrphanName                         string    `json:"orphan_name"`
	FinalArtifactName                  string    `json:"final_artifact_name"`
	QuarantineName                     string    `json:"quarantine_name"`
	QuarantineSHA256                   string    `json:"quarantine_sha256"`
	OriginalPathAbsentVerified         bool      `json:"original_path_absent_verified"`
	FinalArtifactAbsentVerified        bool      `json:"final_artifact_absent_verified"`
	ActivePartFilesAbsentVerified      bool      `json:"active_part_files_absent_verified"`
	QuarantineHashVerified             bool      `json:"quarantine_hash_verified"`
	ReplayPermitted                    bool      `json:"replay_permitted"`
	AuthorityRestorable                bool      `json:"authority_restorable"`
	ResolutionGrantsExecutionAuthority bool      `json:"resolution_grants_execution_authority"`
	InterruptionBlockCleared           bool      `json:"interruption_block_cleared"`
}

func Bind(in Input) (Evidence, error) {
	in.InterruptionDigest = strings.TrimSpace(in.InterruptionDigest)
	in.ProposalID = strings.TrimSpace(in.ProposalID)
	in.Operation = strings.TrimSpace(in.Operation)
	in.ExecutionDigest = strings.TrimSpace(in.ExecutionDigest)
	in.StartedEventHash = strings.TrimSpace(in.StartedEventHash)
	in.Decision = strings.TrimSpace(in.Decision)
	in.ResolvedBy = strings.TrimSpace(in.ResolvedBy)
	in.Note = strings.TrimSpace(in.Note)
	in.RuntimeID = strings.TrimSpace(in.RuntimeID)
	in.OrphanName = strings.TrimSpace(in.OrphanName)
	in.FinalArtifactName = strings.TrimSpace(in.FinalArtifactName)
	in.QuarantineName = strings.TrimSpace(in.QuarantineName)
	in.QuarantineSHA256 = strings.ToLower(strings.TrimSpace(in.QuarantineSHA256))

	switch {
	case in.InterruptionDigest == "":
		return Evidence{}, fmt.Errorf("interruption digest is empty")
	case in.ProposalID == "":
		return Evidence{}, fmt.Errorf("proposal id is empty")
	case in.Operation == "":
		return Evidence{}, fmt.Errorf("operation is empty")
	case in.ExecutionDigest == "":
		return Evidence{}, fmt.Errorf("execution digest is empty")
	case in.StartedEventHash == "":
		return Evidence{}, fmt.Errorf("started event hash is empty")
	case in.Decision != DecisionQuarantineUnknownNoReplay:
		return Evidence{}, fmt.Errorf("unsupported resolution decision %q", in.Decision)
	case in.ResolvedBy == "":
		return Evidence{}, fmt.Errorf("resolved_by is empty")
	case len(in.ResolvedBy) > MaxActorLength:
		return Evidence{}, fmt.Errorf("resolved_by exceeds %d bytes", MaxActorLength)
	case in.Note == "":
		return Evidence{}, fmt.Errorf("resolution note is empty")
	case len(in.Note) > MaxNoteLength:
		return Evidence{}, fmt.Errorf("resolution note exceeds %d bytes", MaxNoteLength)
	case in.RuntimeID == "":
		return Evidence{}, fmt.Errorf("runtime id is empty")
	case in.ResolvedAt.IsZero():
		return Evidence{}, fmt.Errorf("resolution timestamp is empty")
	case in.OrphanName == "" || len(in.OrphanName) > MaxNameLength:
		return Evidence{}, fmt.Errorf("orphan name is invalid")
	case in.FinalArtifactName == "" || len(in.FinalArtifactName) > MaxNameLength:
		return Evidence{}, fmt.Errorf("final artifact name is invalid")
	case in.QuarantineName == "" || len(in.QuarantineName) > MaxNameLength:
		return Evidence{}, fmt.Errorf("quarantine name is invalid")
	case !validSHA256(in.QuarantineSHA256):
		return Evidence{}, fmt.Errorf("quarantine sha256 is invalid")
	case !in.OriginalPathAbsentVerified:
		return Evidence{}, fmt.Errorf("original active-staging path absence was not verified")
	case !in.FinalArtifactAbsentVerified:
		return Evidence{}, fmt.Errorf("final artifact absence was not verified")
	case !in.ActivePartFilesAbsentVerified:
		return Evidence{}, fmt.Errorf("active part-file absence was not verified")
	case !in.QuarantineHashVerified:
		return Evidence{}, fmt.Errorf("quarantine hash was not verified")
	}

	e := Evidence{
		Version:                            Version,
		InterruptionDigest:                 in.InterruptionDigest,
		ProposalID:                         in.ProposalID,
		Operation:                          in.Operation,
		ExecutionDigest:                    in.ExecutionDigest,
		StartedEventHash:                   in.StartedEventHash,
		Decision:                           in.Decision,
		ResolvedBy:                         in.ResolvedBy,
		Note:                               in.Note,
		RuntimeID:                          in.RuntimeID,
		ResolvedAt:                         in.ResolvedAt.UTC(),
		OutcomeRemainsUnknown:              true,
		ArtifactStateRemainsUnknown:        true,
		AmbiguousBytesDisposition:          DispositionQuarantined,
		OrphanName:                         in.OrphanName,
		FinalArtifactName:                  in.FinalArtifactName,
		QuarantineName:                     in.QuarantineName,
		QuarantineSHA256:                   in.QuarantineSHA256,
		OriginalPathAbsentVerified:         true,
		FinalArtifactAbsentVerified:        true,
		ActivePartFilesAbsentVerified:      true,
		QuarantineHashVerified:             true,
		ReplayPermitted:                    false,
		AuthorityRestorable:                false,
		ResolutionGrantsExecutionAuthority: false,
		InterruptionBlockCleared:           true,
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
		return fmt.Errorf("resolution version %q is unsupported", e.Version)
	}
	if e.Decision != DecisionQuarantineUnknownNoReplay {
		return fmt.Errorf("unsupported resolution decision %q", e.Decision)
	}
	if !e.OutcomeRemainsUnknown || !e.ArtifactStateRemainsUnknown {
		return fmt.Errorf("resolution unexpectedly asserts interrupted outcome or artifact state")
	}
	if e.AmbiguousBytesDisposition != DispositionQuarantined {
		return fmt.Errorf("resolution ambiguous bytes disposition %q is unsupported", e.AmbiguousBytesDisposition)
	}
	if !e.OriginalPathAbsentVerified || !e.FinalArtifactAbsentVerified ||
		!e.ActivePartFilesAbsentVerified || !e.QuarantineHashVerified {
		return fmt.Errorf("resolution lacks verified quarantine disposition")
	}
	if e.ReplayPermitted || e.AuthorityRestorable || e.ResolutionGrantsExecutionAuthority {
		return fmt.Errorf("resolution unexpectedly grants authority")
	}
	if !e.InterruptionBlockCleared {
		return fmt.Errorf("resolution does not clear interruption block")
	}
	if strings.TrimSpace(e.InterruptionDigest) == "" ||
		strings.TrimSpace(e.ProposalID) == "" ||
		strings.TrimSpace(e.Operation) == "" ||
		strings.TrimSpace(e.ExecutionDigest) == "" ||
		strings.TrimSpace(e.StartedEventHash) == "" ||
		strings.TrimSpace(e.ResolvedBy) == "" ||
		strings.TrimSpace(e.Note) == "" ||
		strings.TrimSpace(e.RuntimeID) == "" ||
		strings.TrimSpace(e.OrphanName) == "" ||
		strings.TrimSpace(e.FinalArtifactName) == "" ||
		strings.TrimSpace(e.QuarantineName) == "" ||
		e.ResolvedAt.IsZero() {
		return fmt.Errorf("resolution evidence is incomplete")
	}
	if len(e.ResolvedBy) > MaxActorLength || len(e.Note) > MaxNoteLength ||
		len(e.OrphanName) > MaxNameLength || len(e.FinalArtifactName) > MaxNameLength ||
		len(e.QuarantineName) > MaxNameLength {
		return fmt.Errorf("resolution evidence exceeds bounded text limits")
	}
	if !validSHA256(e.QuarantineSHA256) {
		return fmt.Errorf("resolution quarantine sha256 is invalid")
	}
	want, err := Digest(e)
	if err != nil {
		return err
	}
	if e.Digest != want {
		return fmt.Errorf("resolution digest mismatch")
	}
	return nil
}

func Matches(e Evidence, interruptionDigest, proposalID, operation, executionDigest, startedEventHash string) error {
	if err := Verify(e); err != nil {
		return err
	}
	switch {
	case e.InterruptionDigest != interruptionDigest:
		return fmt.Errorf("resolution interruption digest mismatch")
	case e.ProposalID != proposalID:
		return fmt.Errorf("resolution proposal mismatch")
	case e.Operation != operation:
		return fmt.Errorf("resolution operation mismatch")
	case e.ExecutionDigest != executionDigest:
		return fmt.Errorf("resolution execution digest mismatch")
	case e.StartedEventHash != startedEventHash:
		return fmt.Errorf("resolution started event hash mismatch")
	}
	return nil
}

func Digest(e Evidence) (string, error) {
	payload := digestPayload{
		Version:                            e.Version,
		InterruptionDigest:                 e.InterruptionDigest,
		ProposalID:                         e.ProposalID,
		Operation:                          e.Operation,
		ExecutionDigest:                    e.ExecutionDigest,
		StartedEventHash:                   e.StartedEventHash,
		Decision:                           e.Decision,
		ResolvedBy:                         e.ResolvedBy,
		Note:                               e.Note,
		RuntimeID:                          e.RuntimeID,
		ResolvedAt:                         e.ResolvedAt.UTC(),
		OutcomeRemainsUnknown:              e.OutcomeRemainsUnknown,
		ArtifactStateRemainsUnknown:        e.ArtifactStateRemainsUnknown,
		AmbiguousBytesDisposition:          e.AmbiguousBytesDisposition,
		OrphanName:                         e.OrphanName,
		FinalArtifactName:                  e.FinalArtifactName,
		QuarantineName:                     e.QuarantineName,
		QuarantineSHA256:                   strings.ToLower(e.QuarantineSHA256),
		OriginalPathAbsentVerified:         e.OriginalPathAbsentVerified,
		FinalArtifactAbsentVerified:        e.FinalArtifactAbsentVerified,
		ActivePartFilesAbsentVerified:      e.ActivePartFilesAbsentVerified,
		QuarantineHashVerified:             e.QuarantineHashVerified,
		ReplayPermitted:                    e.ReplayPermitted,
		AuthorityRestorable:                e.AuthorityRestorable,
		ResolutionGrantsExecutionAuthority: e.ResolutionGrantsExecutionAuthority,
		InterruptionBlockCleared:           e.InterruptionBlockCleared,
	}
	raw, err := canonicalJSON(payload)
	if err != nil {
		return "", fmt.Errorf("canonicalize resolution: %w", err)
	}
	sum := sha256.Sum256(raw)
	return "sha256:" + hex.EncodeToString(sum[:]), nil
}

func validSHA256(value string) bool {
	if len(value) != 64 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
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
