package gate

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/donCannoli-burns/actor-engine/internal/protocol"
)

var ErrProposalNotFound = errors.New("proposal not found")

type Gate struct {
	mu       sync.Mutex
	pending  map[string]protocol.Proposal
	approved map[string]protocol.Confirmation
}

func New() *Gate {
	return &Gate{pending: map[string]protocol.Proposal{}, approved: map[string]protocol.Confirmation{}}
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

func (g *Gate) Confirm(c protocol.Confirmation) (protocol.Proposal, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	p, ok := g.pending[c.ProposalID]
	if !ok {
		return protocol.Proposal{}, ErrProposalNotFound
	}
	if time.Now().After(p.ExpiresAt) {
		delete(g.pending, c.ProposalID)
		delete(g.approved, c.ProposalID)
		return protocol.Proposal{}, fmt.Errorf("proposal %s expired", c.ProposalID)
	}
	if c.StateDigest != p.StateDigest {
		return protocol.Proposal{}, fmt.Errorf("state digest mismatch")
	}
	g.approved[c.ProposalID] = c
	return p, nil
}

func (g *Gate) Consume(id, currentDigest string) (protocol.Proposal, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	p, ok := g.pending[id]
	if !ok {
		return protocol.Proposal{}, ErrProposalNotFound
	}
	c, ok := g.approved[id]
	if !ok {
		return protocol.Proposal{}, fmt.Errorf("proposal %s is not confirmed", id)
	}
	if time.Now().After(p.ExpiresAt) {
		delete(g.pending, id)
		delete(g.approved, id)
		return protocol.Proposal{}, fmt.Errorf("proposal %s expired", id)
	}
	if c.StateDigest != p.StateDigest || currentDigest != p.StateDigest {
		delete(g.pending, id)
		delete(g.approved, id)
		return protocol.Proposal{}, fmt.Errorf("state changed after confirmation; approval invalidated")
	}
	delete(g.pending, id)
	delete(g.approved, id)
	return p, nil
}
