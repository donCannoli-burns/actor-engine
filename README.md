# Actor Engine — KoL actor runtime prototype v0.1.0

A Go + ASH actor-driven state plane for **monitoring and safely operating around KoLmafia**, designed to plug into Kolmaf-AI Desktop while preserving the existing authority model.

## What this prototype does

- runs a local Go actor supervisor on `127.0.0.1:10424`;
- monitors the official KoLmafia latest-release API;
- can inspect an installed KoLmafia JAR manifest for `Build-Revision`;
- monitors the proven Kingdomsitter Go/ASH sidecar on `127.0.0.1:10423`;
- accepts bounded KoLmafia observations from `kol_actor.ash`;
- represents runtime facts as **concurrently active states** rather than one giant edge-defined FSM;
- exposes one protocol to Go, Rust, Kotlin, C#, C++, C, and Swift clients;
- implements a state-bound human confirmation gate;
- implements one reversible write: **stage a KoLmafia release artifact** into a local staging directory and verify SHA-256 when GitHub supplies a digest;
- deliberately does **not** implement release install, restart, arbitrary gCLI, or arbitrary ASH execution.

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

## License

MIT. See [`LICENSE`](LICENSE).

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

See `integration/kolmaf-ai-desktop-tools.fragment.json`. It mirrors the desktop tool-manifest style but keeps this prototype's authority at `proposal-and-confirmed-local-staging-only`.

## Reference mapping

This build was grounded against the uploaded KoL App-Dir Matrix v0.2.1, Kingdomsitter's current Go/ASH boundary, Kolmaf-AI Desktop's current trust model, and the official KoLmafia release API. See `docs/REFERENCE-MAP.md` and `FOR-AGENT.html`.
