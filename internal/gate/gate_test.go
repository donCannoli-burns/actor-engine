package gate

import (
	"strings"
	"testing"
	"time"

	"github.com/donCannoli-burns/actor-engine/internal/protocol"
)

func TestGateRejectsChangedStateAndBurnsApproval(t *testing.T) {
	t.Parallel()
	g := New()
	p := protocol.Proposal{ID: "p1", StateDigest: "abc", ExpiresAt: time.Now().Add(time.Minute)}
	g.Put(p)
	if _, err := g.Confirm(protocol.Confirmation{ProposalID: "p1", StateDigest: "abc"}); err != nil {
		t.Fatalf("Gate.Confirm() error = %v, want nil", err)
	}
	if _, err := g.Consume("p1", "different"); err == nil || !strings.Contains(err.Error(), "approval invalidated") {
		t.Fatalf("Gate.Consume() error = %v, want invalidated state-change rejection", err)
	}
	if _, err := g.Consume("p1", "abc"); !errorsIsProposalNotFound(err) {
		t.Fatalf("second Gate.Consume() error = %v, want proposal not found after burn", err)
	}
}

func TestGateRejectsExpiredConfirmedProposal(t *testing.T) {
	t.Parallel()
	g := New()
	p := protocol.Proposal{ID: "p2", StateDigest: "abc", ExpiresAt: time.Now().Add(40 * time.Millisecond)}
	g.Put(p)
	if _, err := g.Confirm(protocol.Confirmation{ProposalID: "p2", StateDigest: "abc"}); err != nil {
		t.Fatalf("Gate.Confirm() error = %v, want nil", err)
	}
	time.Sleep(60 * time.Millisecond)
	if _, err := g.Consume("p2", "abc"); err == nil || !strings.Contains(err.Error(), "expired") {
		t.Fatalf("Gate.Consume() error = %v, want expiration rejection", err)
	}
	if _, err := g.Consume("p2", "abc"); !errorsIsProposalNotFound(err) {
		t.Fatalf("second Gate.Consume() error = %v, want proposal not found after expiration", err)
	}
}

func errorsIsProposalNotFound(err error) bool {
	return err == ErrProposalNotFound
}
