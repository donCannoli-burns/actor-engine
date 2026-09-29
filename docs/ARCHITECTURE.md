# Architecture

```text
human / desktop / agents
        |
        | versioned HTTP + JSON protocol
        v
Go actor supervisor :10424
  |- release-watcher actor ----------> GitHub releases (read-only)
  |- kingdomsitter-watcher actor ----> :10423 /health + /v0/state (read-only)
  |- state plane ---------------------> concurrently active runtime facts
  |- proposal gate ------------------> exact state digest + human confirmation
  `- local release stager -----------> staging/ only; no install/restart
        ^
        |
ASH observation shim
  `- KoLmafia state snapshot only

KoLmafia remains runtime truth.
Kingdomsitter remains execution_authority=false.
```

The state plane intentionally follows the uploaded asyncmachine reference's central idea: useful behavior is bound to active states, and several states may be active at once. This prototype implements that model with a small standard-library state plane so the bootstrap has no third-party Go dependency. It can later be adapted to `pancsta/asyncmachine-go` without changing the external protocol.

## Active state examples

A normal session can simultaneously be:

- `ready`
- `observe_only`
- `kol_state_seen`
- `kingdomsitter_seen`
- `release_current` **or** `release_available`

A confirmed staging flow moves through:

`awaiting_confirmation -> proposal_ready -> executing -> reconciling -> release_staged`

These are state activations, not a claim that every pair is a legal global FSM edge.
