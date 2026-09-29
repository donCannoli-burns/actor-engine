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

## v0.5 development proposal admission

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
