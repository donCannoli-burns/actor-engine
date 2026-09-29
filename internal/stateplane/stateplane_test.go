package stateplane

import "testing"

func TestPlaneSupportsConcurrentStates(t *testing.T) {
	t.Parallel()
	p := New(StateReady, StateObserveOnly)
	p.Activate(StateKoLStateSeen, StateKingdomsitterSeen)
	for _, state := range []string{StateReady, StateObserveOnly, StateKoLStateSeen, StateKingdomsitterSeen} {
		if !p.Has(state) {
			t.Fatalf("Plane.Has(%q) = false, want true", state)
		}
	}
}

func TestSetExclusive(t *testing.T) {
	t.Parallel()
	p := New(StateReleaseCurrent)
	p.SetExclusive(ReleaseStates, StateReleaseAvailable)
	if p.Has(StateReleaseCurrent) {
		t.Fatalf("Plane.Has(%q) = true, want false", StateReleaseCurrent)
	}
	if !p.Has(StateReleaseAvailable) {
		t.Fatalf("Plane.Has(%q) = false, want true", StateReleaseAvailable)
	}
}
