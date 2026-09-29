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

## v0.3.0 live acceptance — runtime / observation identity

**Status:** `PASS LIVE_RUNTIME_OBSERVATION_IDENTITY_TEST`  
**Date:** 2026-09-29  
**Feature implementation commit:** `e32b0118871c695a9f95e1a9efe8b32411af60cc`  
**Exact tested repository HEAD:** `bd473f58babf1bc68c3407c45d506243f2621086`  
**Authority expansion:** none

The interactive acceptance harness passed against the real local Actor Engine / KoLmafia runtime.

It independently verified the bounded observation hash instead of trusting the server's `observation_id`, repeated the same manual observation, changed only the observation event with `kol_actor turn`, returned to `kol_actor sync`, created an **unconfirmed and unexecuted** release-stage proposal for provenance inspection, restarted only Actor Engine, and re-observed the same bounded KoL facts.

Observed runtime identities:

```text
before restart: run-871adb21a6724de6de3820420149c433
after restart:  run-eae31582ec1d3a4e12299d9dbabaaa79
```

Stable manual observation identity:

```text
obs-173a105577d0e666bc6eeccdb07faf5d234d5d8b1f4055f88ed266d0f8d8e873
```

The same bounded manual observation retained that identity across different Actor Engine runtimes. The event-only `after-adventure` observation changed the identity, and returning to identical manual facts returned to the original identity.

Proposal provenance evidence:

```text
proposal      = p-a5aacc864ee2db95e53e3931
state_digest  = c02fec452a09177ea564029725d1d45ce68d1b57ae8c44b5f0d98bc94a77326c
target        = KoLmafia-29315.jar / r29315
asset_sha256  = d0b7fb3bf75768ad9918578ba979f627e4938c78cbdd1a05b0b9b180d0f65209
installed     = 29301
```

The proposal was never confirmed and never successfully executed. After the sidecar restart, its old ID returned `409 proposal not found`. Audit evidence preserved the proposal's originating runtime/observation identity and distinguished the subsequent denial as belonging to the new runtime.

Observed terminal result:

```text
PASS LIVE_RUNTIME_OBSERVATION_IDENTITY_TEST
Runtime identity changed across restart.
Observation identity matched bounded KoL observations deterministically.
Proposal evidence retained its originating runtime/observation identity.
No release.stage proposal was confirmed or executed by this test.
```

## v0.4.0 live acceptance — read-only runtime preflight

**Status:** `PASS LIVE_READ_ONLY_PREFLIGHT_TEST`  
**Date:** 2026-09-29  
**Exact tested repository HEAD:** `57aa6f627d9edb765d9e44304bde060418eb20f4`  
**Authority expansion:** none

The interactive v0.4 harness passed against the real local Actor Engine / KoLmafia runtime.

It verified the expected readiness transitions:

```text
NOT_READY
  -> fresh explicit evidence
READY
  -> unconfirmed pending proposal
NOT_READY
  -> Actor Engine restart
NOT_READY
  -> fresh explicit evidence
READY
```

Five repeated `GET /v1/preflight` reads did not change the audit ledger count. The test proposal was created only to prove that a non-idle proposal gate closes readiness; it was never confirmed and never successfully executed. The final READY result still explicitly reported that preflight grants no authority and that human confirmation remains required.

Observed terminal result:

```text
PASS LIVE_READ_ONLY_PREFLIGHT_TEST
Preflight moved NOT_READY -> READY -> NOT_READY -> READY for the expected evidence/gate changes.
Repeated preflight reads appended no audit evidence.
The test proposal was never confirmed or successfully executed.
READY never granted execution authority.
```


## v0.5.0 live acceptance — proposal admission / preflight binding

**Status:** `PASS LIVE_PROPOSAL_ADMISSION_BINDING_TEST`  
**Date:** 2026-09-29  
**Exact tested repository HEAD:** `19a43a36ab53367b8985de835f0e5cb893e060c3`  
**Authority expansion:** none

The real runtime verified that proposal formation is bound to a READY preflight without turning readiness into confirmation or execution authority.

Observed sequence:

```text
NOT_READY preflight
    -> proposal refused with HTTP 409
    -> no proposal.created audit evidence

fresh dependency evidence + human kol_actor sync
    -> READY

READY
    -> proposal created
    -> exact preflight embedded as kol-actor/admission-v1
    -> canonical SHA-256 independently recomputed
    -> same admission digest persisted in proposal.created audit evidence

unconfirmed execute probe
    -> HTTP 409
    -> proposal is not confirmed

Actor Engine restart
    -> prior proposal authority absent
    -> historical admission evidence still present
```

Observed proposal: `p-1c0b069655cda0e75d6af5d3`  
Observed admission digest: `sha256:f4c40c0010126f8803bf27d4bc64eee01e5789a8f9a557b66702f3b8013ee2bb`  
Runtime before restart: `run-29392a3578b6434c8eccf47f4204936e`  
Runtime after restart: `run-e5a33878b1316683899cb43a3fe2246e`

The test never called the confirmation endpoint and never successfully executed `release.stage`.

Observed terminal result:

```text
PASS LIVE_PROPOSAL_ADMISSION_BINDING_TEST
NOT_READY preflight could not form a proposal.
READY proposal admission carried an independently verified canonical digest and exact checkset.
Admission evidence propagated into durable audit history.
The test proposal was never confirmed or successfully executed.
Admission did not restore authority after restart.
```


## v0.6.0 live acceptance — confirmation provenance binding

**Status:** `PASS LIVE_CONFIRMATION_PROVENANCE_BINDING_TEST`  
**Date:** 2026-09-29  
**Exact tested repository HEAD:** `eee62dc77d35585188e495501ddca987a1943107`  
**Authority expansion:** none

The live harness verified the final human-controlled provenance boundary before execution:

```text
proposal + admission
    -> explicit human confirmation
    -> first-class confirmation evidence
    -> independently verifiable digest
    -> durable proposal.confirmed history

confirmed proposal
    -> never executed while approval was live

Actor Engine restart
    -> approval absent
    -> old execute = HTTP 409 proposal not found
    -> confirmation evidence remains valid history only
```

Observed terminal result:

```text
PASS LIVE_CONFIRMATION_PROVENANCE_BINDING_TEST
Human confirmation produced independently verifiable first-class evidence.
Confirmation evidence bound proposal, state, admission, human, runtime, and timestamp.
The confirmed proposal was never executed while approval was live.
Durable confirmation evidence survived restart but did not restore permission.
```


## v0.7.0 live acceptance — execution-attempt provenance

**Status:** `PASS LIVE_EXECUTION_ATTEMPT_PROVENANCE_TEST`  
**Date:** 2026-09-29  
**Exact tested repository HEAD:** `e55ddaab5043ada94628eb5016d4050a9eff9feb`  
**Authority expansion:** none

The real runtime exercised the execution boundary without successfully staging a release:

```text
READY proposal
    -> explicit human confirmation
    -> bounded observation change
    -> stale execute probe
    -> HTTP 409 approval invalidated
    -> first-class execution-attempt evidence
    -> no execution.started
    -> no execution.succeeded
    -> retry = proposal not found
    -> Actor Engine restart
    -> evidence survives, authority does not
```

Observed terminal result:

```text
PASS LIVE_EXECUTION_ATTEMPT_PROVENANCE_TEST
A stale confirmed proposal was denied before operation start.
The denial carried independently verifiable execution-attempt provenance.
No execution.started or execution.succeeded record existed for the test proposal.
The denial burned proposal authority; retry returned proposal not found.
```
