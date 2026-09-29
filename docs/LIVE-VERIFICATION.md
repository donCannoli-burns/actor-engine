# Actor Engine v0.1.0 — Live Verification Record

**Status:** `LIVE_VERIFICATION_PASS_FOR_RELEASE_STAGE_v0.1.0`  
**Date:** 2026-09-29  
**Hardened implementation commit:** `a7e9136b732229b5463466e5b236ef66e9d4cf58`  
**GitHub Actions run:** https://github.com/donCannoli-burns/actor-engine/actions/runs/36529056832 — completed / success

This record captures the bounded live verification performed against a real KoLmafia runtime. It is evidence for the named operations only. It is **not** a grant of authority for any adjacent capability.

## Environment observed

- Actor Engine: `v0.1.0`
- protocol: `kol-actor/v1`
- sidecar: `http://127.0.0.1:10424`
- KoLmafia installed revision: `29301`
- official latest release observed: `r29309`
- KoLmafia release asset: `KoLmafia-29309.jar`
- expected GitHub SHA-256: `c40cfe77bba26591d9f054b6e276998e9c60f61dfba1feb95e7cf2dfa6ccfc0b`
- local workspace shape: `~/.kolmafia/kolmaf-ai/`
- Kingdomsitter transport health: healthy during verification
- Kingdomsitter semantic state observed: `available: false`
- ASH execution authority: `false`
- health contract: `live_kolmafia_mutation: false`

## Static/build verification

The hardened commit passed locally:

```text
go test ./...          PASS
go test -race ./...    PASS
go vet ./...           PASS
go build               PASS
```

The same commit completed GitHub Actions CI successfully.

## Finding discovered during live testing

The first successful proposal/confirmation attempt exercised the old download implementation. The HTTP client timed out while reading the release body and the old staging code had already created the final path directly. This left a partial `KoLmafia-29309.jar` while the receipt correctly reported `success: false`.

That was treated as a real integrity failure in the staging implementation, not as a network-only failure.

Commit `a7e9136` repaired the path by changing staging to:

```text
download into hidden temporary file
        ↓
complete bounded transfer
        ↓
SHA-256 calculation + expected digest comparison
        ↓
file sync + close
        ↓
atomic rename to final staged filename
```

Any transfer error, cancellation, oversize condition, write error, close/sync error, or digest mismatch now removes the temporary file and does not commit the final artifact.

The same hardening commit changed stale-state consume behavior so a changed state destroys both the pending proposal and its approval instead of leaving the approval reusable until expiry.

## Live negative-path proof — stale state

A fresh release-stage proposal was created against a state in which the bounded KoL observation carried:

```text
event = after-adventure
```

Proposal:

```text
id           = p-3fd762f89046ccfcb5e27ac9
state_digest = 36a22b4ce60b71f73db11af302dab419d74e7942b88ca38f3336f4990613d2a8
operation    = release.stage
risk         = local-write-reversible
requires_confirmation = true
```

The human confirmation for that exact digest was accepted. Before execution, only the bounded observation was changed through `kol_actor sync`, producing:

```text
event = manual
```

The already-confirmed proposal was then executed.

First execute result:

```text
HTTP/1.1 409 Conflict
state changed after confirmation; approval invalidated
```

Immediate second execute result:

```text
HTTP/1.1 409 Conflict
proposal not found
```

Staging directory inspection after both attempts showed no file.

### Negative-path conclusion

Verified live:

```text
proposal bound to state A
→ exact human confirmation
→ observation becomes state B
→ execution denied
→ approval destroyed
→ retry impossible
→ no artifact written
```

## Live positive-path proof — unchanged state

A new proposal was created and confirmed without changing the bounded state between confirmation and execution.

Proposal:

```text
id           = p-6a994f8084b871ec2d392cf4
state_digest = dc233ed7fe1cb126adf071dabb1580d26b64fe9ed79cccf08ba1fa061664fde5
operation    = release.stage
risk         = local-write-reversible
requires_confirmation = true
```

Execution receipt:

```text
success       = true
operation     = release.stage
artifact      = ~/.kolmafia/kolmaf-ai/actor-engine-runtime/staging/KoLmafia-29309.jar
sha256        = c40cfe77bba26591d9f054b6e276998e9c60f61dfba1feb95e7cf2dfa6ccfc0b
```

Independent `sha256sum` of the staged JAR returned exactly:

```text
c40cfe77bba26591d9f054b6e276998e9c60f61dfba1feb95e7cf2dfa6ccfc0b
```

Inspection found no hidden `.part-*` file remaining after commit.

The state plane then contained:

```text
release_available
release_staged
kol_state_seen
kingdomsitter_seen
observe_only
ready
```

Critically, `installed_revision` remained `29301`. The tested operation staged `r29309`; it did not install it or restart KoLmafia.

## Restart behavior observed

Pending proposals are in-memory in v0.1.0. Restarting Actor Engine removes them, so a prior proposal becomes `proposal not found`. This is intentionally fail-closed for the prototype.

Release metadata is also in-memory. After restart, `/v1/release/refresh` must run again before a release-stage proposal can be created. Attempting proposal creation without release metadata returns conflict rather than guessing.

## Validated scope

The live test supports these concrete claims:

- local Go sidecar health/readiness;
- installed KoLmafia revision observation;
- bounded ASH → Go KoL state publication;
- Kingdomsitter observation;
- official KoLmafia release discovery;
- proposal creation bound to observed state;
- exact-digest human confirmation;
- stale-state execution rejection;
- stale approval invalidation/burn;
- bounded local release download;
- digest verification against GitHub-supplied SHA-256;
- failure cleanup with no committed partial artifact under hardened staging;
- atomic commit of a verified staged JAR;
- receipt/state reconciliation;
- no KoLmafia install or restart during `release.stage`.

## Explicitly not validated / not authorized

This checkpoint does **not** validate or authorize:

- release installation;
- KoLmafia restart;
- arbitrary ASH execution;
- arbitrary gCLI execution;
- login/logout/account switching;
- live in-game mutation;
- combat/adventure/choice execution;
- chat, clan, trade, kmail, or other social automation;
- any authority inferred solely from evidence, documentation, state visibility, or transport reachability.

## Safety interpretation

The verification target is not “the Actor Engine can do more.” The target is:

```text
misuse should require replacement, not activation
```

A caller can observe state and present a bounded proposal. A human confirmation is tied to that exact state. If the state changes, the confirmation becomes unusable. Even on the successful path, the implemented write ends at a verified local staging boundary.

**Evidence remains separate from authority.**

## v0.2.0 live acceptance — durable evidence / ephemeral authority

**Status:** `PASS LIVE_LEDGER_RESTART_AUTHORITY_TEST`  
**Date:** 2026-09-29  
**Implementation under test:** `5c97d185987b58403f643165f83eb321dde53aa8`  
**Authority expansion:** none

The v0.2 interactive Bash harness was run against the real local Actor Engine/KoLmafia environment. The human retained the gCLI and restart boundaries through explicit `y/N` prompts.

Observed terminal output:

```text
PASS LIVE_LEDGER_RESTART_AUTHORITY_TEST
Evidence survived restart. Proposal/confirmation authority did not.
No release.stage proposal was executed by this test.
```

The live run established:

```text
proposal.created
proposal.confirmed
        ↓
Actor Engine restart
        ↓
historical evidence still present
        ↓
old proposal execute = 409 proposal not found
        ↓
execution.denied appended as new evidence
```

The test deliberately did **not** execute the confirmed `release.stage` proposal. It therefore tested the persistence/authority boundary without performing a new local staging write.

The acceptance harness is preserved at `tests/live/v0.2-ledger/live-ledger-restart-authority-test.sh`.
