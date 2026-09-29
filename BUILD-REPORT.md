# Actor Engine v0.1.0 — checkout/push report

Date: 2026-09-29
Target repository: `donCannoli-burns/actor-engine`
License: MIT

## Packaging

The repository is prepared as a normal Go/multi-language source tree plus a KoLmafia package projection:

```text
manifest.json -> { "root_directory": "kolmafia" }
kolmafia/scripts/kol_actor.ash
kolmafia/data/actor-engine/runtime.json
```

KoLmafia checkout therefore installs only the runtime-facing ASH/data files. It does not project the Go source tree, clients, tests, docs, or CI files into KoLmafia runtime directories.

Checkout command:

```text
git checkout https://github.com/donCannoli-burns/actor-engine.git main
```

Separate Go sidecar installation:

```bash
go install github.com/donCannoli-burns/actor-engine/cmd/kol-actor-engine@latest
```

## Verification

- `gofmt` — PASS
- `go test ./...` — PASS
- `go test -race ./...` — PASS
- `go vet ./...` — PASS
- `go build ./cmd/kol-actor-engine` — PASS
- isolated `go install ./cmd/kol-actor-engine` — PASS
- C11 client compile — PASS
- C++20 client compile — PASS
- Kotlin client compile — PASS
- Swift client compile — PASS
- Rust compile — NOT RUN (`rustc` unavailable)
- C# compile — NOT RUN (`dotnet` unavailable)
- KoLmafia manifest parse/root guard — PASS
- ASH mutation guard — PASS
- local `/health` — PASS
- ASH-shaped state observation ingest — PASS
- `/v1/state` readback — PASS

## Authority boundary

The public checkout does not add arbitrary gCLI/ASH execution. `kol_actor.ash` is observation-only and reports `execution_authority=false`. The Go engine's only implemented write remains state-bound, human-confirmed local release staging; it does not install/restart KoLmafia or mutate game state.

## CI

`.github/workflows/ci.yml` repeats Go formatting/tests/race/vet/build, C/C++ client builds, JSON manifest validation, and the ASH static mutation guard on pushes and pull requests.
