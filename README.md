<p align="center">
  <img src="assets/actor-engine-header.webp" alt="Two people connected by a long tin-can telephone line — Actor Engine communication boundary" width="100%">
</p>

# Actor Engine — KoL actor runtime prototype v0.6.0-dev

A Go + ASH actor-driven state plane for **monitoring and safely operating around KoLmafia**, designed to plug into Kolmaf-AI Desktop while preserving the existing authority model.

> **Evidence is not authority.** Live verification proves only the operations named below. It does not grant install, restart, arbitrary ASH/gCLI, account, social, or live game-mutation authority.

## Live verification checkpoint — 2026-09-29

`release.stage` is now **live-validated for v0.1.0** at hardened commit [`a7e9136`](https://github.com/donCannoli-burns/actor-engine/commit/a7e9136b732229b5463466e5b236ef66e9d4cf58).

The real KoLmafia runtime test covered both sides of the confirmation contract:

```text
UNCHANGED STATE
proposal → exact-state confirmation → execute → verified local staging

CHANGED STATE
proposal → confirmation → observation changes → 409 reject
                                            ↓
                                  approval invalidated
                                            ↓
                                  retry = proposal not found
```

Verified during the live run:

- `go test ./...` — PASS
- `go test -race ./...` — PASS
- `go vet ./...` — PASS
- Go sidecar build — PASS
- GitHub Actions CI for `a7e9136` — PASS
- KoLmafia observation through `kol_actor.ash` — PASS
- Kingdomsitter observation — PASS
- installed KoLmafia revision remained `29301`
- official latest release observed as `r29309`
- stale-state execution returned `409` and burned the approval
- immediate retry returned `proposal not found`
- failed/invalid staging left no committed artifact
- successful stage produced `KoLmafia-29309.jar`
- independent SHA-256 matched GitHub exactly: `c40cfe77bba26591d9f054b6e276998e9c60f61dfba1feb95e7cf2dfa6ccfc0b`
- no `.part-*` file remained after successful staging
- state plane activated `release_staged`
- staging did **not** install or restart KoLmafia

The live run also found and repaired a real bug in the original staging path: a timed-out download could leave a partial JAR under the final filename. Commit `a7e9136` changed staging to **temporary file → complete transfer → SHA-256 verification → sync/close → atomic rename**, with cleanup on failure, and changed stale-state consume failures to invalidate the prior approval.

Full evidence, commands, observed outcomes, and authority boundary: [`docs/LIVE-VERIFICATION.md`](docs/LIVE-VERIFICATION.md).

## Verified v0.2.0 checkpoint — durable evidence, ephemeral authority

The v0.2.0 checkpoint adds a **hash-chained JSONL evidence ledger** without adding a new operation or restoring authority after restart. The restart/authority acceptance test passed live on 2026-09-29.

```text
evidence survives restart
authority does not
```

The ledger records proposal/execution lifecycle evidence, verifies its sequence and SHA-256 hash chain at startup, and exposes recent evidence read-only at:

```bash
curl 'http://127.0.0.1:10424/v1/audit/recent?limit=20'
```

With the tested runtime layout it defaults to `~/.kolmafia/kolmaf-ai/actor-engine-runtime/audit.jsonl`. A past `proposal.confirmed` record is historical evidence only: the in-memory gate is empty after restart and no confirmation authority is reconstructed.

Authority-bearing paths also fail closed if required audit evidence cannot be durably appended. See [`docs/AUDIT-LEDGER.md`](docs/AUDIT-LEDGER.md).

Live harness: [`tests/live/v0.2-ledger/live-ledger-restart-authority-test.sh`](tests/live/v0.2-ledger/live-ledger-restart-authority-test.sh). Observed terminal result: `PASS LIVE_LEDGER_RESTART_AUTHORITY_TEST`.


## Verified v0.3.0 checkpoint — runtime / observation identity

The next bounded layer adds identity to evidence without adding authority:

```text
runtime_id      = one Actor Engine process lifetime
observation_id  = deterministic identity of the bounded KoL observation payload
```

A fresh Actor Engine start receives a new cryptographically random `runtime_id`. A KoL observation receives an `observation_id` derived from the sorted, length-prefixed bounded observation fields. Identical observation content therefore produces the same observation identity even across different Actor Engine runtimes.

These IDs are **not credentials, capabilities, or permission tokens**. They are provenance primitives. Proposals, receipts, and audit events carry the runtime/observation identity that produced them so humans and agents can distinguish “same observed facts” from “same process lifetime.”

The live identity harness passed on the real runtime on 2026-09-29: `PASS LIVE_RUNTIME_OBSERVATION_IDENTITY_TEST`.

Design: [`docs/IDENTITY.md`](docs/IDENTITY.md)  
Acceptance harness: [`tests/live/v0.3-identity/runtime-observation-identity-test.sh`](tests/live/v0.3-identity/runtime-observation-identity-test.sh)

Observed live result: `PASS LIVE_RUNTIME_OBSERVATION_IDENTITY_TEST`. The harness confirmed runtime turnover, deterministic observation identity across repeated and cross-restart observations, proposal/audit provenance binding, and non-restoration of the pre-restart proposal. It did not confirm or successfully execute `release.stage`.

## Verified v0.4.0 checkpoint — read-only runtime preflight

The next bounded layer adds one **read-only** readiness surface:

```text
GET /v1/preflight
```

It answers whether current evidence is sufficient to **form a proposal under policy**. It does not refresh dependencies, create a proposal, confirm anything, execute anything, or grant authority.

Required checks include:

```text
runtime identity established
audit ledger verified
bounded KoL observation present + <= 5m old
installed KoLmafia revision known
release metadata present + <= 30m old
Kingdomsitter status known + <= 2m old + transport healthy
no unresolved fault
actor state plane ready
proposal gate idle
```

The response explicitly includes `preflight_grants_authority: false` and preserves human confirmation as a separate gate.

Design: [`docs/PREFLIGHT.md`](docs/PREFLIGHT.md)  
Acceptance harness: [`tests/live/v0.4-preflight/read-only-preflight-test.sh`](tests/live/v0.4-preflight/read-only-preflight-test.sh)

Observed live result: `PASS LIVE_READ_ONLY_PREFLIGHT_TEST`. The harness proved NOT_READY → READY → NOT_READY → READY across evidence and proposal-gate changes, repeated preflight reads appended no audit evidence, the test proposal was never confirmed or successfully executed, and READY never granted execution authority.

## Verified v0.5.0 checkpoint — proposal admission / preflight binding

The next bounded layer turns verified v0.4 readiness into **proposal admission evidence**, without turning readiness into authority.

A `release.stage` proposal can now be created only when the current read-only preflight is `READY`. Every admitted proposal carries:

```text
admission.version = kol-actor/admission-v1
admission.digest  = SHA-256(canonical exact preflight report)
admission.preflight
    ├── runtime_id
    ├── observation_id
    ├── generated_at
    ├── exact checkset
    ├── reasons
    └── explicit authority boundary
```

The admission digest is propagated into audit lifecycle evidence and receipts. It is independently verifiable and immutable evidence of **why proposal formation was allowed**.

It is deliberately not a confirmation token:

```text
READY preflight
    ↓
admission evidence
    ↓
proposal

proposal + admission
    ≠ human confirmation
    ≠ execution authority
```

A NOT_READY preflight returns HTTP 409 with `error: preflight_not_ready` and the exact failed preflight report; no proposal is created.

Design: [`docs/ADMISSION.md`](docs/ADMISSION.md)  
Acceptance harness: [`tests/live/v0.5-admission/proposal-admission-binding-test.sh`](tests/live/v0.5-admission/proposal-admission-binding-test.sh)

Observed live result: `PASS LIVE_PROPOSAL_ADMISSION_BINDING_TEST`. The real runtime proved NOT_READY proposal refusal, independently recomputable READY admission evidence, audit propagation of the admission digest, rejection of an unconfirmed execute with HTTP 409, and loss of proposal authority across Actor Engine restart while historical admission evidence remained.

## v0.6 development — confirmation provenance binding

The next bounded layer makes human confirmation a **first-class, digestible evidence object** while keeping the actual approval ephemeral and memory-only.

A successful confirmation now produces:

```text
confirmation.version = kol-actor/confirmation-v1
confirmation.digest  = SHA-256(canonical confirmation payload)

bound fields:
  proposal_id
  state_digest
  admission_digest
  confirmed_by
  runtime_id
  confirmed_at
```

The evidence object explicitly carries:

```text
evidence_grants_authority = false
authority_restorable      = false
```

The in-memory gate stores the verified confirmation evidence as the live approval. The durable audit ledger stores a copy as provenance. **Only the in-memory gate entry is authority-bearing.** On restart, the gate is empty and the durable confirmation record cannot recreate permission.

Execution lifecycle evidence and receipts carry the confirmation digest so the provenance chain becomes:

```text
observation
  → preflight
  → admission
  → proposal
  → human confirmation evidence
  → ephemeral in-memory approval
  → state-bound execution gate
```

Design: [`docs/CONFIRMATION.md`](docs/CONFIRMATION.md)  
Acceptance harness: [`tests/live/v0.6-confirmation/confirmation-provenance-binding-test.sh`](tests/live/v0.6-confirmation/confirmation-provenance-binding-test.sh)

## What this prototype does

- runs a local Go actor supervisor on `127.0.0.1:10424`;
- monitors the official KoLmafia latest-release API;
- can inspect an installed KoLmafia JAR manifest for `Build-Revision`;
- monitors the proven Kingdomsitter Go/ASH sidecar on `127.0.0.1:10423`;
- accepts bounded KoLmafia observations from `kol_actor.ash`;
- represents runtime facts as **concurrently active states** rather than one giant edge-defined FSM;
- exposes one protocol to Go, Rust, Kotlin, C#, C++, C, and Swift clients;
- implements a state-bound human confirmation gate;
- implements one reversible write: **atomically stage a KoLmafia release artifact** into a local staging directory and verify SHA-256 when GitHub supplies a digest;
- deliberately does **not** implement release install, restart, arbitrary gCLI, or arbitrary ASH execution.

## Authority boundary

### Live-validated through v0.5.0

- observation-only ASH → Go state publication;
- release metadata discovery;
- proposal creation bound to a state digest;
- explicit human confirmation against that digest;
- stale-state rejection and approval invalidation;
- atomic, digest-verified local release staging;
- receipt/state reconciliation after successful staging.

### Not validated and not authorized

- release installation;
- KoLmafia restart;
- arbitrary ASH execution;
- arbitrary gCLI execution;
- login/logout/account switching;
- live in-game mutation;
- social/chat/trade automation.

Restart semantics are intentionally fail-closed through v0.5.0: pending proposals are in-memory and disappear on Actor Engine restart. Cached release metadata also requires refresh after restart.

## Why Go + ASH

Kingdomsitter already demonstrates the useful split: Go owns orchestration/tooling while KoLmafia/ASH remains the runtime-facing language surface. This prototype extends that pattern into an actor state plane rather than creating another game client.

## Run

```bash
./scripts/smoke.sh
go run ./cmd/kol-actor-engine \
  -kolmafia_jar /path/to/KoLmafia-29309.jar
```

Then inspect:

```bash
curl http://127.0.0.1:10424/health
curl http://127.0.0.1:10424/v1/state
curl -X POST http://127.0.0.1:10424/v1/release/refresh
```

Install `kolmafia/scripts/kol_actor.ash` into KoLmafia's scripts directory, then:

```text
kol_actor status
kol_actor sync
```

## KoLmafia checkout

The repository is KoLmafia-package-aware. Its root `manifest.json` points KoLmafia at `kolmafia/`, so checkout installs only the runtime-facing ASH/data shim and does not project the Go or multi-language development tree into KoLmafia directories.

From KoLmafia gCLI:

```text
git checkout https://github.com/donCannoli-burns/actor-engine.git main
```

Then install/start the Go sidecar separately:

```bash
go install github.com/donCannoli-burns/actor-engine/cmd/kol-actor-engine@latest
kol-actor-engine
```

Verify from KoLmafia:

```text
kol_actor status
kol_actor sync
```

Update later with `git update actor-engine`. See [`docs/KOLMAFIA-CHECKOUT.md`](docs/KOLMAFIA-CHECKOUT.md) for the full install/update/remove split.

## Release staging flow

```bash
# 1. refresh live release metadata
curl -X POST http://127.0.0.1:10424/v1/release/refresh

# 2. create proposal
curl -X POST http://127.0.0.1:10424/v1/proposals/release-stage \
  -H 'content-type: application/json' -d '{}'

# 3. human reviews proposal and copies BOTH proposal id + state_digest
curl -X POST http://127.0.0.1:10424/v1/proposals/<id>/confirm \
  -H 'content-type: application/json' \
  -d '{"state_digest":"<digest>","confirmed_by":"human"}'

# 4. execute exactly that confirmed proposal
curl -X POST http://127.0.0.1:10424/v1/proposals/<id>/execute
```

The result is a staged file + receipt. KoLmafia is not installed, restarted, logged in/out, or commanded.

## Multi-language clients

The `clients/` directory contains minimal protocol clients for:

- Rust
- Kotlin/JVM
- C#/.NET
- C++20
- C11/POSIX
- Swift/Foundation

They are transport clients, not authority domains. All policy and execution decisions remain in the Go supervisor.

## Integration with Kolmaf-AI Desktop

Desktop project page: https://doncannoli-burns.github.io/kolmaf-ai-desktop/  
Actor Engine page inside the Desktop site: https://doncannoli-burns.github.io/kolmaf-ai-desktop/actor-engine.html  
Desktop repository: https://github.com/donCannoli-burns/kolmaf-ai-desktop

See `integration/kolmaf-ai-desktop-tools.fragment.json`. It mirrors the desktop tool-manifest style but keeps this prototype's authority at `proposal-and-confirmed-local-staging-only`.

## Reference mapping

This build was grounded against the uploaded KoL App-Dir Matrix, Kingdomsitter's current Go/ASH boundary, Kolmaf-AI Desktop's current trust model, and the official KoLmafia release API. See `docs/REFERENCE-MAP.md` and `FOR-AGENT.html`.

## License

MIT. See [`LICENSE`](LICENSE).
