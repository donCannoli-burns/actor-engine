package audit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLedgerAppendReopenAndRecent(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "audit.jsonl")
	l, err := Open(path)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	first, err := l.Append(Event{Type: EventProposalCreated, ProposalID: "p-1", StateDigest: "abc", Result: "pending"})
	if err != nil {
		t.Fatalf("Append(first) error = %v", err)
	}
	second, err := l.Append(Event{Type: EventProposalConfirmed, ProposalID: "p-1", StateDigest: "abc", Actor: "human", Result: "confirmed"})
	if err != nil {
		t.Fatalf("Append(second) error = %v", err)
	}
	if first.Seq != 1 || second.Seq != 2 {
		t.Fatalf("sequences = %d,%d want 1,2", first.Seq, second.Seq)
	}
	if second.PrevHash != first.Hash {
		t.Fatalf("second.PrevHash = %q want %q", second.PrevHash, first.Hash)
	}

	reopened, err := Open(path)
	if err != nil {
		t.Fatalf("Open(reopen) error = %v", err)
	}
	status := reopened.Status()
	if status.Count != 2 || !status.Verified || status.HeadHash != second.Hash {
		t.Fatalf("Status() = %+v", status)
	}
	recent := reopened.Recent(1)
	if len(recent) != 1 || recent[0].Type != EventProposalConfirmed {
		t.Fatalf("Recent(1) = %+v", recent)
	}
}

func TestLedgerRejectsTampering(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "audit.jsonl")
	l, err := Open(path)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	if _, err := l.Append(Event{Type: EventProposalCreated, ProposalID: "p-1"}); err != nil {
		t.Fatalf("Append() error = %v", err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	tampered := strings.Replace(string(raw), EventProposalCreated, EventProposalConfirmed, 1)
	if tampered == string(raw) {
		t.Fatal("tamper replacement did not change ledger")
	}
	if err := os.WriteFile(path, []byte(tampered), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if _, err := Open(path); err == nil || !strings.Contains(err.Error(), "audit hash mismatch") {
		t.Fatalf("Open(tampered) error = %v, want hash mismatch", err)
	}
}
