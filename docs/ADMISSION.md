# Proposal admission / preflight binding

Status: **v0.5.0-dev — implementation built, live acceptance pending**  
Verified baseline: **v0.4.0** at `4aa5cb1cd72d9c8356b242ce4b7b8b5c68eddd25`  
Authority change: **none**

## Goal

Create a machine-checkable chain from verified readiness to proposal formation:

```text
current evidence
    ↓
GET /v1/preflight
    ↓
READY
    ↓
canonical admission evidence
    ↓
proposal created
```

without collapsing that chain into:

```text
READY = permission
```

## Admission gate

Before a `release.stage` proposal can be constructed, Actor Engine builds the same read-only preflight result returned by `GET /v1/preflight`.

If it is not READY:

- HTTP status is `409 Conflict`;
- response error is `preflight_not_ready`;
- the response carries the exact current preflight report;
- no proposal is placed in the in-memory gate;
- no pending proposal is installed;
- no proposal state is activated;
- no `proposal.created` audit event is appended.

## Admission evidence

A successful proposal carries:

```json
{
  "admission": {
    "version": "kol-actor/admission-v1",
    "digest": "sha256:<64 lowercase hex>",
    "preflight": {
      "version": "kol-actor/preflight-v1",
      "status": "READY",
      "ready_for_proposal": true,
      "...": "the complete exact report"
    }
  }
}
```

The digest is computed over canonical JSON of the exact embedded preflight report:

1. serialize the typed preflight report;
2. decode it into generic JSON values;
3. re-encode it with deterministic object-key ordering and compact JSON;
4. SHA-256 the canonical bytes;
5. prefix the lowercase hexadecimal digest with `sha256:`.

Because the report contains `runtime_id`, `observation_id`, generated time, checks, freshness ages, reasons, and the explicit authority boundary, the digest binds the proposal to the exact readiness evidence used for admission.

## Lifecycle propagation

The admission digest is copied into:

- `proposal.created`;
- `proposal.confirmed`;
- `execution.started`;
- terminal execution audit evidence;
- execution receipts.

Before an already-confirmed proposal can proceed to its operation, Actor Engine re-verifies the embedded admission evidence and digest. Admission verification cannot create confirmation; it only rejects malformed/tampered admission evidence.

## Authority separation

```text
preflight READY
        ↓
admission
        ↓
proposal
        ↓
HUMAN CONFIRMATION
        ↓
state-bound consume
        ↓
execution
```

The stages remain intentionally distinct.

A valid admission:

- is not a credential;
- is not a capability token;
- is not accepted by the confirmation endpoint;
- cannot restore authority after restart;
- does not bypass state-digest invalidation;
- does not authorize release installation, KoLmafia restart, arbitrary ASH/gCLI, or live game mutation.

## Acceptance target

The interactive harness must prove:

1. a fresh runtime with NOT_READY preflight cannot create a proposal;
2. the denial returns the exact machine-readable preflight report;
3. admission denial does not append `proposal.created` evidence;
4. explicit release/Kingdomsitter refresh plus human-run `kol_actor sync` produces READY;
5. proposal creation then succeeds;
6. the proposal embeds the exact READY preflight report;
7. an independent Python implementation recomputes the same canonical admission SHA-256;
8. proposal runtime/observation IDs match the embedded preflight identities;
9. `proposal.created` audit evidence carries the same admission digest;
10. admission alone does not count as confirmation;
11. an optional explicit-human-approved negative execute returns 409 not-confirmed and performs no successful `release.stage`;
12. Actor Engine restart removes the ephemeral proposal while historical admission evidence remains.

The harness never confirms the test proposal.
