package audit

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/donCannoli-burns/actor-engine/internal/confirmation"
	"github.com/donCannoli-burns/actor-engine/internal/execution"
	"github.com/donCannoli-burns/actor-engine/internal/reconciliation"
	"github.com/donCannoli-burns/actor-engine/internal/resolution"
)

const MaxRecent = 100

const (
	EventRuntimeStarted      = "runtime.started"
	EventProposalCreated     = "proposal.created"
	EventProposalConfirmed   = "proposal.confirmed"
	EventConfirmationDenied  = "proposal.confirmation_denied"
	EventProposalInvalidated = "proposal.invalidated"
	EventProposalExpired     = "proposal.expired"
	EventExecutionDenied     = "execution.denied"
	EventExecutionStarted    = "execution.started"
	EventExecutionSucceeded  = "execution.succeeded"
	EventExecutionFailed     = "execution.failed"
	EventRecoveryResolved     = "recovery.resolved"
)

type Event struct {
	Seq                  uint64                   `json:"seq"`
	At                   time.Time                `json:"at"`
	Type                 string                   `json:"type"`
	ProposalID           string                   `json:"proposal_id,omitempty"`
	RuntimeID            string                   `json:"runtime_id,omitempty"`
	ObservationID        string                   `json:"observation_id,omitempty"`
	AdmissionDigest      string                   `json:"admission_digest,omitempty"`
	ConfirmationDigest   string                   `json:"confirmation_digest,omitempty"`
	Confirmation         *confirmation.Evidence   `json:"confirmation,omitempty"`
	ExecutionDigest      string                   `json:"execution_digest,omitempty"`
	Execution            *execution.Evidence      `json:"execution,omitempty"`
	ReconciliationDigest string                   `json:"reconciliation_digest,omitempty"`
	Reconciliation       *reconciliation.Evidence `json:"reconciliation,omitempty"`
	InterruptionDigest   string                   `json:"interruption_digest,omitempty"`
	ResolutionDigest     string                   `json:"resolution_digest,omitempty"`
	Resolution           *resolution.Evidence     `json:"resolution,omitempty"`
	Operation            string                   `json:"operation,omitempty"`
	StateDigest          string                   `json:"state_digest,omitempty"`
	Actor                string                   `json:"actor,omitempty"`
	Result               string                   `json:"result,omitempty"`
	Detail               string                   `json:"detail,omitempty"`
	ArtifactPath         string                   `json:"artifact_path,omitempty"`
	SHA256               string                   `json:"sha256,omitempty"`
	PrevHash             string                   `json:"prev_hash,omitempty"`
	Hash                 string                   `json:"hash,omitempty"`
}

type Status struct {
	Count    int    `json:"count"`
	HeadHash string `json:"head_hash,omitempty"`
	Verified bool   `json:"verified"`
}

type Ledger struct {
	mu       sync.Mutex
	path     string
	events   []Event
	lastHash string
	failed   error
}

func Open(path string) (*Ledger, error) {
	if strings.TrimSpace(path) == "" {
		return nil, fmt.Errorf("audit ledger path is empty")
	}
	if err := os.MkdirAll(filepath.Dir(filepath.Clean(path)), 0o755); err != nil {
		return nil, fmt.Errorf("creating audit ledger directory: %w", err)
	}

	l := &Ledger{path: filepath.Clean(path)}
	f, err := os.Open(l.path)
	if os.IsNotExist(err) {
		return l, nil
	}
	if err != nil {
		return nil, fmt.Errorf("opening audit ledger: %w", err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 64<<10), 1<<20)
	var expected uint64 = 1
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(strings.TrimSpace(string(line))) == 0 {
			return nil, fmt.Errorf("audit ledger contains blank record at sequence %d", expected)
		}
		var event Event
		if err := json.Unmarshal(line, &event); err != nil {
			return nil, fmt.Errorf("decoding audit record %d: %w", expected, err)
		}
		if event.Seq != expected {
			return nil, fmt.Errorf("audit sequence mismatch: got %d want %d", event.Seq, expected)
		}
		if event.PrevHash != l.lastHash {
			return nil, fmt.Errorf("audit chain mismatch at sequence %d", event.Seq)
		}
		want, err := eventHash(event)
		if err != nil {
			return nil, fmt.Errorf("hashing audit record %d: %w", event.Seq, err)
		}
		if event.Hash == "" || event.Hash != want {
			return nil, fmt.Errorf("audit hash mismatch at sequence %d", event.Seq)
		}
		l.events = append(l.events, event)
		l.lastHash = event.Hash
		expected++
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading audit ledger: %w", err)
	}
	return l, nil
}

func (l *Ledger) Append(event Event) (Event, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.failed != nil {
		return Event{}, fmt.Errorf("audit ledger unavailable: %w", l.failed)
	}
	if strings.TrimSpace(event.Type) == "" {
		return Event{}, fmt.Errorf("audit event type is empty")
	}

	event.Seq = uint64(len(l.events) + 1)
	if event.At.IsZero() {
		event.At = time.Now().UTC()
	} else {
		event.At = event.At.UTC()
	}
	event.PrevHash = l.lastHash
	event.Hash = ""
	hash, err := eventHash(event)
	if err != nil {
		return Event{}, err
	}
	event.Hash = hash
	line, err := json.Marshal(event)
	if err != nil {
		return Event{}, fmt.Errorf("encoding audit event: %w", err)
	}
	line = append(line, '\n')

	f, err := os.OpenFile(l.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		l.failed = err
		return Event{}, fmt.Errorf("opening audit ledger for append: %w", err)
	}
	if _, err := f.Write(line); err != nil {
		_ = f.Close()
		l.failed = err
		return Event{}, fmt.Errorf("writing audit ledger: %w", err)
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		l.failed = err
		return Event{}, fmt.Errorf("syncing audit ledger: %w", err)
	}
	if err := f.Close(); err != nil {
		l.failed = err
		return Event{}, fmt.Errorf("closing audit ledger: %w", err)
	}

	l.events = append(l.events, event)
	l.lastHash = event.Hash
	return event, nil
}

func (l *Ledger) Recent(limit int) []Event {
	l.mu.Lock()
	defer l.mu.Unlock()
	if limit <= 0 {
		limit = 25
	}
	if limit > MaxRecent {
		limit = MaxRecent
	}
	start := len(l.events) - limit
	if start < 0 {
		start = 0
	}
	out := make([]Event, len(l.events)-start)
	copy(out, l.events[start:])
	return out
}

func (l *Ledger) Events() []Event {
	l.mu.Lock()
	defer l.mu.Unlock()
	out := make([]Event, len(l.events))
	copy(out, l.events)
	return out
}

func (l *Ledger) Status() Status {
	l.mu.Lock()
	defer l.mu.Unlock()
	return Status{Count: len(l.events), HeadHash: l.lastHash, Verified: l.failed == nil}
}

func eventHash(event Event) (string, error) {
	event.Hash = ""
	b, err := json.Marshal(event)
	if err != nil {
		return "", fmt.Errorf("marshal audit event for hash: %w", err)
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}
