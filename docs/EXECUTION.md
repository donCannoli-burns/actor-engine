# Execution-attempt provenance

Status: **v0.7.0-dev — implementation built, live acceptance pending**  
Verified baseline: **v0.6.0** at `f749ba14049b23bf82a4085f6727737758406f9f`  
Authority change: **none**

## Goal

Complete the provenance chain through the execution boundary without requiring a successful write for live acceptance.

```text
observation
  → preflight
  → admission
  → proposal
  → confirmation
  → execution attempt
```

## Evidence schema

```json
{
  "version": "kol-actor/execution-attempt-v1",
  "digest": "sha256:<64 lowercase hex>",
  "proposal_id": "p-...",
  "operation": "release.stage",
  "proposal_state_digest": "...",
  "admission_digest": "sha256:...",
  "confirmation_digest": "sha256:...",
  "origin_runtime_id": "run-...",
  "execution_runtime_id": "run-...",
  "current_state_digest": "...",
  "attempted_at": "RFC3339 timestamp",
  "gate_decision": "authorized|denied",
  "evidence_grants_authority": false
}
```

The digest excludes only the `digest` field itself and is independently reproducible from canonical sorted-key compact JSON.

## Denial-first live proof

The acceptance campaign does not need to successfully stage a release.

Instead:

1. establish fresh bounded evidence;
2. create a READY-admitted proposal;
3. explicitly confirm it;
4. use human-run `kol_actor turn` to publish `event=after-adventure` without spending an adventure;
5. verify the current `observation_id` differs from the proposal's originating observation;
6. explicitly approve one execute probe;
7. require HTTP 409 stale-state invalidation;
8. independently verify the denial's execution-attempt digest;
9. require no `execution.started` or `execution.succeeded` event for the test proposal;
10. retry and require `proposal not found`.

Because the observation identity is part of the proposal-bound snapshot, a changed observation ID is a precondition before the live execute probe.

## Gate decisions

`authorized` means the in-memory gate successfully consumed the confirmed proposal against the current state digest. It does **not** mean the operation succeeded.

`denied` means the known proposal/confirmation context reached the gate but was refused before operation execution.

## Digest propagation

For authorized attempts:

```text
execution.started
  contains full execution object
        ↓
execution_digest
        ↓
terminal audit event
        ↓
receipt
```

For stale-state denials:

```text
proposal.invalidated / execution.denied
  contains full execution object
        ↓
no execution.started
        ↓
no operation success
```

## Acceptance target

The interactive harness must prove:

- the proposal is admitted under READY preflight;
- confirmation evidence is present;
- bounded observation identity changes before execute;
- stale execute returns HTTP 409 and approval invalidation;
- denial contains full `kol-actor/execution-attempt-v1`;
- Python independently recomputes its SHA-256;
- proposal/admission/confirmation/runtime/state bindings match;
- `gate_decision=denied`;
- `evidence_grants_authority=false`;
- current state digest differs from proposal state digest;
- there is no `execution.started` or `execution.succeeded` for the proposal;
- retry returns `proposal not found`;
- after optional Actor Engine restart, historical execution evidence remains evidence only.

No successful `release.stage` is accepted by this harness.
