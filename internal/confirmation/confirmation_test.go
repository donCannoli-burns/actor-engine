package confirmation

import (
	"strings"
	"testing"
	"time"
)

func fixture() Input {
	return Input{
		ProposalID:      "p-123",
		StateDigest:     "state-abc",
		AdmissionDigest: "sha256:admission",
		ConfirmedBy:     "human-operator",
		RuntimeID:       "run-0123456789abcdef",
		ConfirmedAt:     time.Date(2026, 9, 29, 19, 0, 0, 123, time.UTC),
	}
}

func TestBindVerifyAndMatch(t *testing.T) {
	t.Parallel()
	in := fixture()
	e, err := Bind(in)
	if err != nil {
		t.Fatalf("Bind() error = %v", err)
	}
	if e.Version != Version {
		t.Fatalf("Version = %q, want %q", e.Version, Version)
	}
	if !strings.HasPrefix(e.Digest, "sha256:") || len(e.Digest) != len("sha256:")+64 {
		t.Fatalf("Digest = %q", e.Digest)
	}
	if e.EvidenceGrantsAuthority || e.AuthorityRestorable {
		t.Fatalf("confirmation evidence authority flags = %+v", e)
	}
	if err := Verify(e); err != nil {
		t.Fatalf("Verify() error = %v", err)
	}
	if err := Matches(e, in.ProposalID, in.StateDigest, in.AdmissionDigest, in.RuntimeID); err != nil {
		t.Fatalf("Matches() error = %v", err)
	}
}

func TestVerifyRejectsTampering(t *testing.T) {
	t.Parallel()
	e, err := Bind(fixture())
	if err != nil {
		t.Fatalf("Bind() error = %v", err)
	}
	e.ConfirmedBy = "different-human"
	if err := Verify(e); err == nil || !strings.Contains(err.Error(), "digest mismatch") {
		t.Fatalf("Verify() error = %v, want digest mismatch", err)
	}
}

func TestVerifyRejectsAuthorityFlags(t *testing.T) {
	t.Parallel()
	e, err := Bind(fixture())
	if err != nil {
		t.Fatalf("Bind() error = %v", err)
	}
	e.EvidenceGrantsAuthority = true
	if err := Verify(e); err == nil || !strings.Contains(err.Error(), "grants authority") {
		t.Fatalf("Verify() error = %v, want authority rejection", err)
	}
}

func TestBindRequiresHumanAndTimestamp(t *testing.T) {
	t.Parallel()
	in := fixture()
	in.ConfirmedBy = ""
	if _, err := Bind(in); err == nil || !strings.Contains(err.Error(), "human identifier") {
		t.Fatalf("Bind() error = %v, want human identifier rejection", err)
	}
	in = fixture()
	in.ConfirmedAt = time.Time{}
	if _, err := Bind(in); err == nil || !strings.Contains(err.Error(), "timestamp") {
		t.Fatalf("Bind() error = %v, want timestamp rejection", err)
	}
}
