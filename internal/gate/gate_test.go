package gate

import (
	"strings"
	"testing"
	"time"

	"github.com/donCannoli-burns/actor-engine/internal/admission"
	"github.com/donCannoli-burns/actor-engine/internal/confirmation"
	"github.com/donCannoli-burns/actor-engine/internal/preflight"
	"github.com/donCannoli-burns/actor-engine/internal/protocol"
)

func TestGateRejectsChangedStateAndBurnsApproval(t *testing.T) {
	t.Parallel()
	g := New()
	p := gateTestProposal("p1", "abc", time.Now().Add(time.Minute))
	g.Put(p)
	if _, _, err := g.Confirm(protocol.Confirmation{ProposalID: "p1", StateDigest: "abc", ConfirmedBy: "human", ConfirmedAt: time.Now().UTC()}); err != nil {
		t.Fatalf("Gate.Confirm() error = %v, want nil", err)
	}
	if _, _, err := g.Consume("p1", "different"); err == nil || !strings.Contains(err.Error(), "approval invalidated") {
		t.Fatalf("Gate.Consume() error = %v, want invalidated state-change rejection", err)
	}
	if _, _, err := g.Consume("p1", "abc"); !errorsIsProposalNotFound(err) {
		t.Fatalf("second Gate.Consume() error = %v, want proposal not found after burn", err)
	}
}

func TestGateRejectsExpiredConfirmedProposal(t *testing.T) {
	t.Parallel()
	g := New()
	p := gateTestProposal("p2", "abc", time.Now().Add(40*time.Millisecond))
	g.Put(p)
	if _, _, err := g.Confirm(protocol.Confirmation{ProposalID: "p2", StateDigest: "abc", ConfirmedBy: "human", ConfirmedAt: time.Now().UTC()}); err != nil {
		t.Fatalf("Gate.Confirm() error = %v, want nil", err)
	}
	time.Sleep(60 * time.Millisecond)
	if _, _, err := g.Consume("p2", "abc"); err == nil || !strings.Contains(err.Error(), "expired") {
		t.Fatalf("Gate.Consume() error = %v, want expiration rejection", err)
	}
	if _, _, err := g.Consume("p2", "abc"); !errorsIsProposalNotFound(err) {
		t.Fatalf("second Gate.Consume() error = %v, want proposal not found after expiration", err)
	}
}

func errorsIsProposalNotFound(err error) bool {
	return err == ErrProposalNotFound
}

func TestGateConfirmationEvidenceIsBoundAndEphemeral(t *testing.T) {
	t.Parallel()
	g := New()
	p := gateTestProposal("p3", "state-3", time.Now().Add(time.Minute))
	g.Put(p)
	confirmedAt := time.Now().UTC()
	gotProposal, evidence, err := g.Confirm(protocol.Confirmation{
		ProposalID:  p.ID,
		StateDigest: p.StateDigest,
		ConfirmedBy: "human-operator",
		ConfirmedAt: confirmedAt,
	})
	if err != nil {
		t.Fatalf("Gate.Confirm() error = %v", err)
	}
	if gotProposal.ID != p.ID {
		t.Fatalf("confirmed proposal ID = %q, want %q", gotProposal.ID, p.ID)
	}
	if err := confirmation.Matches(evidence, p.ID, p.StateDigest, p.Admission.Digest, p.RuntimeID); err != nil {
		t.Fatalf("confirmation evidence mismatch: %v", err)
	}
	if evidence.ConfirmedBy != "human-operator" || !evidence.ConfirmedAt.Equal(confirmedAt) {
		t.Fatalf("confirmation human/timestamp = %+v", evidence)
	}

	freshGate := New()
	freshGate.Put(p)
	if _, _, err := freshGate.Consume(p.ID, p.StateDigest); err == nil || !strings.Contains(err.Error(), "not confirmed") {
		t.Fatalf("fresh gate consume error = %v, want not confirmed", err)
	}
}

func TestGateRejectsTamperedConfirmationEvidence(t *testing.T) {
	t.Parallel()
	g := New()
	p := gateTestProposal("p4", "state-4", time.Now().Add(time.Minute))
	g.Put(p)
	_, evidence, err := g.Confirm(protocol.Confirmation{
		ProposalID:  p.ID,
		StateDigest: p.StateDigest,
		ConfirmedBy: "human-operator",
		ConfirmedAt: time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("Gate.Confirm() error = %v", err)
	}
	evidence.ConfirmedBy = "tampered-human"
	g.mu.Lock()
	g.approved[p.ID] = evidence
	g.mu.Unlock()

	if _, _, err := g.Consume(p.ID, p.StateDigest); err == nil || !strings.Contains(err.Error(), "confirmation evidence invalid") {
		t.Fatalf("Gate.Consume() error = %v, want invalid confirmation evidence", err)
	}
	if _, _, err := g.Consume(p.ID, p.StateDigest); !errorsIsProposalNotFound(err) {
		t.Fatalf("second Gate.Consume() error = %v, want proposal not found after invalidation", err)
	}
}

func gateTestProposal(id, stateDigest string, expires time.Time) protocol.Proposal {
	return protocol.Proposal{
		ID:          id,
		RuntimeID:   "run-gate-test",
		StateDigest: stateDigest,
		ExpiresAt:   expires,
		Admission: admission.Evidence{
			Version: "kol-actor/admission-v1",
			Digest:  "sha256:admission-" + id,
			Preflight: preflight.Result{
				Status:           "READY",
				ReadyForProposal: true,
			},
		},
	}
}
