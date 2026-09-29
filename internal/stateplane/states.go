package stateplane

const (
	StateBooting              = "booting"
	StateReady                = "ready"
	StateObserveOnly          = "observe_only"
	StateKoLStateSeen         = "kol_state_seen"
	StateKingdomsitterSeen    = "kingdomsitter_seen"
	StateReleaseCurrent       = "release_current"
	StateReleaseAvailable     = "release_available"
	StateReleaseStaged        = "release_staged"
	StateProposalReady        = "proposal_ready"
	StateAwaitingConfirmation = "awaiting_confirmation"
	StateExecuting            = "executing"
	StateReconciling          = "reconciling"
	StateFaulted              = "faulted"
)

var ReleaseStates = []string{StateReleaseCurrent, StateReleaseAvailable}
var ProposalStates = []string{StateProposalReady, StateAwaitingConfirmation, StateExecuting, StateReconciling}
