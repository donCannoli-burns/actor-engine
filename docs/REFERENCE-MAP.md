# Reference map

## Uploaded matrix

The source matrix separates reference notes, Git-tree evidence, semantic evidence, and authority. This prototype keeps the same distinction. Its actor/state reference section specifically highlights state-bound code with multiple simultaneously active states, and its project shelf identifies Kingdomsitter as a Go-first ASH parser/IR sidecar.

## Kingdomsitter

Grounded current interface:

- Go-first ASH language sidecar.
- default local listener `127.0.0.1:10423`.
- `/health` and `/v0/state` expose observation state.
- `execution_authority` is false.
- the only host boundary is a Go `Host.Call(ctx, name, args)` interface; parser/analyzer/agent packages do not receive game authority.
- KoLmafia ASH controller publishes lightweight state updates to `/v0/update`.

KoL Actor Engine consumes this read-only surface rather than adding execution authority to Kingdomsitter.

## Kolmaf-AI Desktop

The desktop's existing policy says documentation is not authority and version-sensitive facts require runtime verification. The prototype's desktop manifest fragment follows that pattern.

## KoLmafia release monitor

The engine uses `https://api.github.com/repos/kolmafia/kolmafia/releases/latest` at runtime. During this prototype build, the latest release observed was `r29309` (published 2026-09-28), but that value is not hard-coded as truth; the actor refreshes the endpoint.
