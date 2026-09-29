# Audit ledger — durable evidence, ephemeral authority

Status: **v0.2 development chunk**  
Authority change: **none**

## Purpose

The ledger preserves evidence about Actor Engine decisions and local writes across process restarts without preserving permission to repeat them.

```text
evidence survives restart
authority does not
```

## Storage

Use `-audit_log /path/to/audit.jsonl`. When omitted, the default is `audit.jsonl` beside `stage_dir`. For the tested local layout:

```text
~/.kolmafia/kolmaf-ai/actor-engine-runtime/
├── audit.jsonl
├── logs/
└── staging/
```

## Record and integrity model

Each JSONL record contains a sequence number, UTC timestamp, event type, relevant proposal/operation/state fields, `prev_hash`, and `hash`. Sequence numbers must be contiguous. `prev_hash` must point to the prior record. `hash` is SHA-256 over the record with its own hash field cleared.

Every append is flushed with `fsync` before it is considered successful. Opening an existing ledger verifies decoding, sequence continuity, the previous-hash chain, and every record hash. Corruption or tampering prevents startup rather than being skipped.

## Event vocabulary

```text
runtime.started
proposal.created
proposal.confirmed
proposal.confirmation_denied
proposal.invalidated
proposal.expired
execution.denied
execution.started
execution.succeeded
execution.failed
```

These events are evidence, not executable state.

## Read-only API

`GET /v1/audit/recent?limit=20` returns the ledger count, current head hash, verification status, recent events, and `authority_restored_from_audit: false`. Limits are bounded to 1–100.

The health response advertises `evidence_persistence: hash-chained-jsonl` and `authority_restored_on_boot: false`.

## Fail-closed interaction with authority

- Proposal creation is refused and its gate entry dropped if `proposal.created` cannot be made durable.
- A confirmed proposal is dropped if `proposal.confirmed` cannot be made durable.
- After the gate consumes a proposal, `execution.started` must be durable before the local stage begins.
- Terminal success/failure is recorded. If a stage succeeds but terminal evidence cannot be committed, Actor Engine attempts to remove the staged artifact and faults the operation.

## Restart semantics

A restart creates a fresh in-memory gate. Historical `proposal.created` and `proposal.confirmed` records are not reconstructed into pending or approved proposals. Executing an old proposal ID therefore returns `proposal not found`; that denial can itself become new evidence.

## Tests in this chunk

- append → reopen → chain/head preservation;
- bounded recent retrieval;
- tamper/hash mismatch rejection;
- release-stage lifecycle audit ordering;
- restart with historical proposal/confirmation evidence while execution remains unauthorized;
- existing unit, race, vet, build, C/C++ client, and KoLmafia package guards.

## Non-goals

The ledger does not install releases, restart KoLmafia, execute arbitrary ASH/gCLI, mutate the live game, restore confirmation after restart, or make historical evidence authoritative. It is an evidence primitive, not an authority store.
