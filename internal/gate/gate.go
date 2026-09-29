package gate

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/donCannoli-burns/actor-engine/internal/confirmation"
	"github.com/donCannoli-burns/actor-engine/internal/protocol"
)

var ErrProposalNotFound = errors.New("proposal not found")

type Gate struct {
	mu       sync.Mutex
	pending  map[string]protocol.Proposal
	approved map[string]confirmation.Evidence
}

func New() *Gate {
	return &Gate{pending: map[string]protocol.Proposal{}, approved: map[string]confirmation.Evidence{}}
}

func Digest(v any) (string, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return "", fmt.Errorf("marshal state for digest: %w", err)
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}

func (g *Gate) Put(p protocol.Proposal) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.pending[p.ID] = p
}

func (g *Gate) Drop(id string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	delete(g.pending, id)
	delete(g.approved, id)
}

func (g *Gate) Confirm(c protocol.Confirmation) (protocol.Proposal, confirmation.Evidence, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	p, ok := g.pending[c.ProposalID]
	if !ok {
		return protocol.Proposal{}, confirmation.Evidence{}, ErrProposalNotFound
	}
	if time.Now().After(p.ExpiresAt) {
		delete(g.pending, c.ProposalID)
		delete(g.approved, c.ProposalID)
		return protocol.Proposal{}, confirmation.Evidence{}, fmt.Errorf("proposal %s expired", c.ProposalID)
	}
	if c.StateDigest != p.StateDigest {
		return protocol.Proposal{}, confirmation.Evidence{}, fmt.Errorf("state digest mismatch")
	}
	evidence, err := confirmation.Bind(confirmation.Input{
		ProposalID:      p.ID,
		StateDigest:     p.StateDigest,
		AdmissionDigest: p.Admission.Digest,
		ConfirmedBy:     c.ConfirmedBy,
		RuntimeID:       p.RuntimeID,
		ConfirmedAt:     c.ConfirmedAt,
	})
	if err != nil {
		return protocol.Proposal{}, confirmation.Evidence{}, fmt.Errorf("confirmation evidence: %w", err)
	}
	g.approved[c.ProposalID] = evidence
	return p, evidence, nil
}

func (g *Gate) Consume(id, currentDigest string) (protocol.Proposal, confirmation.Evidence, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	p, ok := g.pending[id]
	if !ok {
		return protocol.Proposal{}, confirmation.Evidence{}, ErrProposalNotFound
	}
	evidence, ok := g.approved[id]
	if !ok {
		return p, confirmation.Evidence{}, fmt.Errorf("proposal %s is not confirmed", id)
	}
	if err := confirmation.Matches(evidence, p.ID, p.StateDigest, p.Admission.Digest, p.RuntimeID); err != nil {
		delete(g.pending, id)
		delete(g.approved, id)
		return p, evidence, fmt.Errorf("confirmation evidence invalid; approval invalidated: %w", err)
	}
	if time.Now().After(p.ExpiresAt) {
		delete(g.pending, id)
		delete(g.approved, id)
		return p, evidence, fmt.Errorf("proposal %s expired", id)
	}
	if evidence.StateDigest != p.StateDigest || currentDigest != p.StateDigest {
		delete(g.pending, id)
		delete(g.approved, id)
		return p, evidence, fmt.Errorf("state changed after confirmation; approval invalidated")
	}
	delete(g.pending, id)
	delete(g.approved, id)
	return p, evidence, nil
}
