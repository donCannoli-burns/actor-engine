# Confirmation provenance binding

Status: **v0.6.0-dev — implementation built, live acceptance pending**  
Verified baseline: **v0.5.0** at `4b3f7c3e06d2d2c10e57edadfdde1c6b2446b495`  
Authority change: **none**

## Goal

Carry provenance cleanly through the final human-controlled boundary before execution:

```text
READY preflight
    ↓
admission evidence
    ↓
proposal
    ↓
human confirmation
    ↓
confirmation evidence
    ↓
ephemeral in-memory approval
    ↓
state-bound execution
```

The new evidence object must explain the confirmation without itself becoming a reusable capability.

## Evidence schema

A successful confirmation produces:

```json
{
  "version": "kol-actor/confirmation-v1",
  "digest": "sha256:<64 lowercase hex>",
  "proposal_id": "p-...",
  "state_digest": "...",
  "admission_digest": "sha256:...",
  "confirmed_by": "human-identifier",
  "runtime_id": "run-...",
  "confirmed_at": "RFC3339 timestamp",
  "evidence_grants_authority": false,
  "authority_restorable": false
}
```

The digest excludes only the `digest` field itself. It is SHA-256 over canonical sorted-key compact JSON of the remaining fields.

## Binding rules

The gate constructs the evidence only after it has located the pending proposal and verified the request's state digest.

The evidence must match:

- the exact proposal ID;
- the proposal's state digest;
- the proposal's admission digest;
- the proposal's originating runtime ID;
- a non-empty human identifier;
- a non-zero UTC confirmation timestamp.

The gate stores the resulting evidence in its in-memory approval map.

Before approval consumption, the gate re-verifies both the digest and all proposal bindings. Invalid/tampered confirmation evidence burns the pending approval.

## Durable evidence vs authority

After confirmation succeeds:

- the **full confirmation evidence object** is appended to `proposal.confirmed`;
- the confirmation digest is propagated into execution lifecycle evidence and receipts;
- the in-memory gate holds the only authority-bearing approval state.

After Actor Engine restart:

```text
audit.jsonl
  contains confirmation evidence
        ↓
gate.New()
  approved = empty
        ↓
old proposal execution
  = proposal not found
```

There is intentionally no loader, rehydrator, replay hook, or API that converts historical confirmation evidence back into approval.

## Failure semantics

- empty confirming human identifier: confirmation refused;
- wrong proposal state digest: confirmation refused;
- proposal expired: confirmation refused and gate entry removed;
- confirmation evidence construction failure: confirmation refused;
- `proposal.confirmed` audit append failure: gate approval dropped and proposal invalidated;
- confirmation evidence tamper before consume: approval burned;
- restart: approval disappears even though evidence remains.

## Acceptance target

The interactive harness must prove on the real runtime:

1. fresh read-only evidence produces a READY preflight;
2. a READY-bound proposal is created with admission evidence;
3. confirmation is sent only after an explicit human `y/N` gate;
4. the confirmation response contains `kol-actor/confirmation-v1`;
5. an independent Python implementation recomputes the same canonical SHA-256;
6. proposal ID, state digest, admission digest, human identifier, runtime ID, and timestamp are all bound;
7. both authority flags are false;
8. durable `proposal.confirmed` audit evidence contains the exact confirmation object/digest;
9. no `execution.started` or `execution.succeeded` record exists for the test proposal;
10. Actor Engine restart changes runtime ID and clears the ephemeral approval;
11. execution of the old proposal after restart returns HTTP 409 `proposal not found`;
12. the prior confirmation evidence remains readable and valid in the ledger;
13. `authority_restored_from_audit` remains false.

The live harness never executes the confirmed proposal while its approval is live.
