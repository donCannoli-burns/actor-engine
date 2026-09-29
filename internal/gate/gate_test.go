package gate

import (
	"testing"
	"time"

	"github.com/donCannoli-burns/actor-engine/internal/protocol"
)

func TestGateRejectsChangedState(t *testing.T) {
	t.Parallel()
	g := New()
	p := protocol.Proposal{ID: "p1", StateDigest: "abc", ExpiresAt: time.Now().Add(time.Minute)}
	g.Put(p)
	if _, err := g.Confirm(protocol.Confirmation{ProposalID: "p1", StateDigest: "abc"}); err != nil {
		t.Fatalf("Gate.Confirm() error = %v, want nil", err)
	}
	if _, err := g.Consume("p1", "different"); err == nil {
		t.Fatal("Gate.Consume() error = nil, want state-change rejection")
	}
}
