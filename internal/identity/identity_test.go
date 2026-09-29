package identity

import (
	"strings"
	"testing"
)

func TestObservationIDStableAcrossMapOrder(t *testing.T) {
	t.Parallel()
	a := map[string]string{
		"event":       "manual",
		"character":   "doncannoli",
		"adventures":  "219",
		"total_turns": "522711",
	}
	b := map[string]string{
		"total_turns": "522711",
		"adventures":  "219",
		"character":   "doncannoli",
		"event":       "manual",
	}
	if got, want := ObservationID(a), ObservationID(b); got != want {
		t.Fatalf("ObservationID map-order mismatch: %q != %q", got, want)
	}
}

func TestObservationIDChangesWhenBoundedStateChanges(t *testing.T) {
	t.Parallel()
	a := map[string]string{"event": "manual", "total_turns": "10"}
	b := map[string]string{"event": "after-adventure", "total_turns": "10"}
	if ObservationID(a) == ObservationID(b) {
		t.Fatal("ObservationID did not change when bounded observation changed")
	}
}

func TestNewRuntimeIDIsFreshAndWellFormed(t *testing.T) {
	t.Parallel()
	a, err := NewRuntimeID()
	if err != nil {
		t.Fatalf("NewRuntimeID() error = %v", err)
	}
	b, err := NewRuntimeID()
	if err != nil {
		t.Fatalf("NewRuntimeID() second error = %v", err)
	}
	if a == b {
		t.Fatalf("runtime ids unexpectedly equal: %q", a)
	}
	for _, id := range []string{a, b} {
		if !strings.HasPrefix(id, RuntimePrefix) || len(id) != len(RuntimePrefix)+32 {
			t.Fatalf("runtime id %q has unexpected format", id)
		}
	}
}
