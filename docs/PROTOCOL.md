# Multi-language protocol

The canonical transport is local HTTP on `127.0.0.1:10424` and the canonical data model is `protocol/kol-actor-v1.schema.json`.

The non-Go clients are intentionally thin. They do **not** duplicate actor logic or mutation authority. Their job is to inspect state and submit typed requests to the Go supervisor.

Core endpoints:

- `GET /health`
- `GET /v1/state`
- `POST /v1/release/refresh`
- `GET /v1/release/latest`
- `GET /v1/kingdomsitter/refresh`
- `GET /v1/kolmafia/update?...` — ASH observation ingest
- `POST /v1/proposals/release-stage`
- `POST /v1/proposals/{id}/confirm`
- `POST /v1/proposals/{id}/execute`

The only implemented execution operation in v0.1.0 is `release.stage`.
