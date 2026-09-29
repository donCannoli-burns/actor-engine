package protocol

import (
	"time"

	"github.com/donCannoli-burns/actor-engine/internal/admission"
)

type MessageKind string

const (
	KindObserve MessageKind = "observe"
	KindPropose MessageKind = "propose"
	KindConfirm MessageKind = "confirm"
	KindExecute MessageKind = "execute"
	KindReceipt MessageKind = "receipt"
)

type Envelope struct {
	Version     string         `json:"version"`
	ID          string         `json:"id"`
	Kind        MessageKind    `json:"kind"`
	Actor       string         `json:"actor"`
	Target      string         `json:"target"`
	Operation   string         `json:"operation"`
	StateDigest string         `json:"state_digest,omitempty"`
	Payload     map[string]any `json:"payload,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
}

type Snapshot struct {
	Version              string            `json:"version"`
	RuntimeID            string            `json:"runtime_id"`
	ObservationID        string            `json:"observation_id,omitempty"`
	UpdatedAt            time.Time         `json:"updated_at"`
	ActiveStates         []string          `json:"active_states"`
	InstalledRevision    string            `json:"installed_revision,omitempty"`
	LatestRelease        string            `json:"latest_release,omitempty"`
	LatestReleaseURL     string            `json:"latest_release_url,omitempty"`
	KingdomsitterHealthy bool              `json:"kingdomsitter_healthy"`
	KingdomsitterState   map[string]any    `json:"kingdomsitter_state,omitempty"`
	KoLState             map[string]string `json:"kol_state,omitempty"`
	PendingProposalID    string            `json:"pending_proposal_id,omitempty"`
	LastReceipt          *Receipt          `json:"last_receipt,omitempty"`
	Fault                string            `json:"fault,omitempty"`
}

type Proposal struct {
	ID              string             `json:"id"`
	RuntimeID       string             `json:"runtime_id"`
	ObservationID   string             `json:"observation_id,omitempty"`
	Operation       string             `json:"operation"`
	Risk            string             `json:"risk"`
	StateDigest     string             `json:"state_digest"`
	Payload         map[string]any     `json:"payload"`
	CreatedAt       time.Time          `json:"created_at"`
	ExpiresAt       time.Time          `json:"expires_at"`
	HumanSummary    string             `json:"human_summary"`
	RequiresConfirm bool               `json:"requires_confirmation"`
	Admission       admission.Evidence `json:"admission"`
}

type Confirmation struct {
	ProposalID  string    `json:"proposal_id"`
	StateDigest string    `json:"state_digest"`
	ConfirmedBy string    `json:"confirmed_by"`
	ConfirmedAt time.Time `json:"confirmed_at"`
}

type Receipt struct {
	ProposalID         string    `json:"proposal_id"`
	RuntimeID          string    `json:"runtime_id"`
	ObservationID      string    `json:"observation_id,omitempty"`
	AdmissionDigest    string    `json:"admission_digest,omitempty"`
	ConfirmationDigest string    `json:"confirmation_digest,omitempty"`
	ExecutionDigest    string    `json:"execution_digest,omitempty"`
	Operation          string    `json:"operation"`
	Success            bool      `json:"success"`
	Detail             string    `json:"detail"`
	ArtifactPath       string    `json:"artifact_path,omitempty"`
	SHA256             string    `json:"sha256,omitempty"`
	CompletedAt        time.Time `json:"completed_at"`
}
