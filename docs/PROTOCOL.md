# Multi-language protocol

The canonical transport is local HTTP on `127.0.0.1:10424` and the canonical envelope model is `protocol/kol-actor-v1.schema.json`.

The non-Go clients are intentionally thin. They do **not** duplicate actor logic or mutation authority. Their job is to inspect state and submit typed requests to the Go supervisor.

Core endpoints:

- `GET /health`
- `GET /v1/state`
- `GET /v1/preflight` — read-only readiness report; never grants authority
- `GET /v1/audit/recent?limit=N`
- `POST /v1/release/refresh`
- `GET /v1/release/latest`
- `GET /v1/kingdomsitter/refresh`
- `GET /v1/kolmafia/update?...` — ASH observation ingest
- `POST /v1/proposals/release-stage`
- `POST /v1/proposals/{id}/confirm`
- `POST /v1/proposals/{id}/execute`

The only implemented execution operation through the verified v0.2 baseline is `release.stage`.

## Verified v0.3.0 identity fields

The verified v0.3.0 checkpoint adds two provenance fields to state/proposal/receipt/audit surfaces:

- `runtime_id`: `run-` + 128 random bits encoded as 32 lowercase hex characters. It is generated once per Actor Engine process start and changes across restarts.
- `observation_id`: `obs-` + SHA-256 of the bounded KoL observation map after deterministic sorting and length-prefix encoding.

`runtime_id` answers **which Actor Engine process lifetime produced this evidence?**

`observation_id` answers **which exact bounded KoL observation payload produced this evidence?**

Neither field grants authority. Neither is accepted as confirmation. Neither restores a proposal after restart.

See [`IDENTITY.md`](IDENTITY.md).

## Verified v0.4.0 preflight

`GET /v1/preflight` returns `kol-actor/preflight-v1`.

The endpoint is intentionally observational: it performs no network refresh, writes no audit event, creates no proposal, changes no state-plane state, and grants no authority. A `READY` result means only that current evidence satisfies the configured proposal-formation checks.

Freshness windows in the first contract are:

- KoL observation: 5 minutes;
- KoLmafia release metadata: 30 minutes;
- Kingdomsitter status: 2 minutes.

See [`PREFLIGHT.md`](PREFLIGHT.md).

## Verified v0.5.0 proposal admission

`POST /v1/proposals/release-stage` now requires a current `READY` result from the verified preflight contract.

A successful proposal includes `admission`:

- `version: kol-actor/admission-v1`;
- `digest: sha256:<hex>`;
- the complete exact preflight report used for admission.

The digest is SHA-256 over a canonical JSON representation of the embedded preflight report. It is also copied into proposal lifecycle audit records and receipts.

If preflight is NOT_READY, proposal formation returns HTTP 409 with:

```json
{
  "error": "preflight_not_ready",
  "preflight": { "...": "exact current preflight report" }
}
```

Admission is evidence of proposal eligibility only. The existing state-digest confirmation and execution gates remain separate.

See [`ADMISSION.md`](ADMISSION.md).

## Verified v0.6.0 confirmation provenance

A successful `POST /v1/proposals/{id}/confirm` response now includes a `confirmation` evidence object with:

- `version: kol-actor/confirmation-v1`;
- `digest: sha256:<hex>`;
- proposal ID;
- proposal state digest;
- admission digest;
- confirming human identifier;
- originating runtime ID;
- confirmation timestamp;
- `evidence_grants_authority: false`;
- `authority_restorable: false`.

The digest is independently reproducible from canonical JSON of those fields excluding the digest itself.

The durable `proposal.confirmed` audit record stores the full evidence object plus its digest. Execution lifecycle records and receipts carry the confirmation digest. The ledger is never consulted to reconstruct the in-memory approval map.

See [`CONFIRMATION.md`](CONFIRMATION.md).

## Verified v0.7.0 execution-attempt provenance

When an execute request has a known proposal and confirmation context, Actor Engine binds the attempt into `kol-actor/execution-attempt-v1`.

The evidence digest covers proposal/operation identity, proposal-state digest, admission digest, confirmation digest, origin runtime, execution runtime, execution-time state digest, timestamp, and gate decision.

For an authorized consume, the full object is stored on `execution.started` and its digest propagates to terminal audit evidence and the receipt. For a stale-state consume denial, the full object is stored on the denial/invalidation audit event with `gate_decision: denied`.

The evidence itself grants no authority.


## Verified v0.8.0 terminal reconciliation provenance

A terminal execution result can carry `kol-actor/reconciliation-v1` evidence. The canonical digest binds the complete provenance chain through the operation outcome:

```text
admission_digest
confirmation_digest
execution_digest
    ↓
runtime + observation identity
    ↓
success / failure + terminal detail
    ↓
artifact path + SHA-256 + committed flag
    ↓
completed_at
```

The full reconciliation object is returned in the receipt and stored on the terminal `execution.succeeded` or `execution.failed` audit event. Its digest is also exposed separately as `reconciliation_digest`.

Reconciliation evidence is historical evidence only. It cannot authorize, replay, or restore an operation.


## Verified v0.9.0 crash/restart recovery report

`GET /v1/recovery` is read-only. It scans the verified audit ledger for `execution.started` records whose `execution_digest` has no matching `execution.succeeded` or `execution.failed` record.

Each unresolved start becomes deterministic `kol-actor/interruption-v1` evidence bound to the original proposal, operation, admission/confirmation/execution digests, runtime/observation identity, audit sequence, timestamp, and hash.

The report deliberately states that outcome and artifact state are unknown. It grants no replay or restoration authority and performs no cleanup or resolution.


## v0.10 development human recovery resolution

`POST /v1/recovery/<interruption_digest>/resolve` accepts one explicit human decision: `acknowledge_unknown_no_replay`.

The request must repeat the exact interruption digest from the path and provide a bounded human identifier plus a non-empty bounded note. A successful request creates `kol-actor/resolution-v1` evidence and durably appends `recovery.resolved`.

The resolution object binds the interruption digest, proposal/operation/execution identity, original `execution.started` event hash, human identity, note, current runtime, and resolution timestamp.

Resolution keeps `outcome_remains_unknown=true`, `artifact_state_remains_unknown=true`, `replay_permitted=false`, `authority_restorable=false`, and `resolution_grants_execution_authority=false`.

A valid durable resolution removes only that interruption from the preflight-blocking set. The old proposal is never reconstructed.
