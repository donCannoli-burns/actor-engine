# KoLmafia checkout

`actor-engine` keeps its Go/multi-language development tree outside KoLmafia runtime directories. The root `manifest.json` exposes only `kolmafia/` to KoLmafia.

## Install the KoLmafia-facing shim

From KoLmafia's gCLI:

```text
git checkout https://github.com/donCannoli-burns/actor-engine.git main
```

This installs:

```text
scripts/kol_actor.ash
data/actor-engine/runtime.json
```

It does **not** install or launch the Go sidecar binary.

## Install the Go sidecar

From a shell with Go available:

```bash
go install github.com/donCannoli-burns/actor-engine/cmd/kol-actor-engine@latest
kol-actor-engine
```

By default the sidecar listens only on:

```text
127.0.0.1:10424
```

Optional installed-JAR inspection:

```bash
kol-actor-engine -kolmafia_jar /path/to/KoLmafia-XXXXX.jar
```

## Verify

In a shell:

```bash
curl http://127.0.0.1:10424/health
curl http://127.0.0.1:10424/v1/state
```

In KoLmafia gCLI:

```text
kol_actor status
kol_actor sync
```

`kol_actor sync` publishes a bounded read-only observation of current KoLmafia state to the local sidecar. The installed ASH shim contains no arbitrary gCLI or arbitrary ASH execution path.

## Update

From KoLmafia gCLI:

```text
git update actor-engine
```

For the separately installed Go binary:

```bash
go install github.com/donCannoli-burns/actor-engine/cmd/kol-actor-engine@latest
```

## Remove

Use KoLmafia's normal `git delete actor-engine` flow for the checked-out ASH/data package. Remove the Go binary separately if desired.
