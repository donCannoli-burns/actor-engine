# Human recovery resolution

Status: **v0.10.0-dev — implementation built, live acceptance pending**  
Verified baseline: **v0.9.0** at `6c9de316b3b732c30d3d940354b95d13c438b239`  
Execution authority change: **none**  
Governance change: **explicit human acknowledgment may clear one interruption-specific admission block**

## Goal

v0.9 deliberately stops at:

```text
execution.started
    ↓
process disappears
    ↓
INTERRUPTED_UNKNOWN_OUTCOME
    ↓
new proposal admission blocked
```

v0.10 adds one narrowly defined human governance action:

```text
human reviews exact interruption
    ↓
acknowledge_unknown_no_replay
    ↓
durable resolution evidence
    ↓
that interruption no longer blocks future proposal admission
```

The action does **not** convert the interrupted execution into success or failure.

## Only supported decision

```text
acknowledge_unknown_no_replay
```

There is intentionally no:

```text
mark_succeeded
mark_failed
retry
resume
replay
cleanup
install
restart
```

resolution decision.

## Resolution evidence

`kol-actor/resolution-v1` binds:

```text
interruption_digest
proposal_id
operation
execution_digest
started_event_hash
decision
resolved_by
note
runtime_id
resolved_at

outcome_remains_unknown = true
artifact_state_remains_unknown = true
replay_permitted = false
authority_restorable = false
resolution_grants_execution_authority = false
interruption_block_cleared = true
```

The digest is SHA-256 over canonical sorted-key compact JSON excluding only `digest`.

Human identity is bounded to 128 bytes and the required resolution note is bounded to 512 bytes.

## Endpoint

```text
POST /v1/recovery/<interruption_digest>/resolve
```

Request body:

```json
{
  "interruption_digest": "sha256:...",
  "decision": "acknowledge_unknown_no_replay",
  "resolved_by": "human-identifier",
  "note": "reviewed the exact unknown outcome and choose to continue without replay"
}
```

The path digest and body digest must match exactly.

The endpoint:

1. serializes resolution requests;
2. finds the exact currently unresolved interruption;
3. creates first-class resolution evidence;
4. durably appends one `recovery.resolved` audit event;
5. returns the resulting recovery report.

If durable audit append fails, resolution is refused.

A second resolution for the same interruption returns HTTP 409 and does not append another event.

## What resolution changes

After a valid durable resolution:

```text
GET /v1/recovery
status = CLEAR
unresolved = []
resolved = [
  {
    interruption: {
      outcome_known: false,
      artifact_state: "unknown",
      ...
    },
    resolution: {
      decision: "acknowledge_unknown_no_replay",
      ...
    }
  }
]
```

Preflight no longer fails `no_interrupted_execution` for that interruption.

All other preflight checks still apply. Resolution alone is not READY and does not create a proposal.

## What resolution does not change

The original interrupted evidence remains unchanged:

```text
outcome_known  = false
artifact_state = unknown
```

The old proposal remains absent from the in-memory gate.

The endpoint does not:

- remove a `.part-*` file;
- rename or validate an artifact;
- recreate confirmation;
- execute or retry the operation;
- create a new proposal;
- bypass fresh observation/release/sidecar checks.

## Live acceptance

The v0.10 harness recreates the isolated v0.9 crash window:

```text
execution.started durable
fixture download entered
.part-* exists
SIGKILL isolated Actor Engine
```

After restart it proves the interruption is unknown and blocking. The harness then shows the exact interruption digest and orphan path and asks the human to acknowledge that exact interruption.

After human resolution it must prove:

- resolution SHA-256 independently recomputes;
- the durable `recovery.resolved` event carries the same object/digest;
- original interruption still says unknown outcome/artifact state;
- orphan `.part-*` still exists;
- repeated resolution is rejected;
- fresh preflight becomes READY;
- a **new** proposal may be formed but is never confirmed or executed;
- the old interrupted proposal remains non-executable;
- after another isolated restart, resolution evidence still clears the block but neither old nor new proposal authority survives;
- no automatic replay occurs.

The temporary orphan is removed only when the harness deletes its isolated workspace after all assertions.

## Acceptance target

```text
PASS LIVE_HUMAN_RECOVERY_RESOLUTION_TEST
Human resolution acknowledged the exact unknown interruption without asserting success or failure.
The resolution evidence was independently verifiable and durably cleared only the interruption admission block.
The orphaned partial artifact remained untouched and neither old nor new proposal authority was restored.
Resolution survived isolated restart; replay remained impossible and the workspace was cleaned only by the harness.
```
