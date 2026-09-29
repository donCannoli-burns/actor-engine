# Read-only runtime preflight

Status: **verified v0.4.0 checkpoint — live acceptance passed**  
Verified baseline: **v0.3.0** at `28aa4b2d8c824f173d163c3194005f77a73cba08`  
Authority change: **none**

## Contract

```text
GET /v1/preflight
```

returns a machine-checkable report with:

- `status: READY | NOT_READY`;
- `ready_for_proposal: boolean`;
- current `runtime_id` and `observation_id`;
- individually named checks;
- deterministic reason codes for required failures;
- an explicit authority boundary.

`READY` means:

> Current local evidence is sufficient to form a proposal under the current policy.

It does **not** mean:

```text
permission to confirm
permission to execute
permission to install
permission to restart KoLmafia
permission to run arbitrary ASH/gCLI
permission to mutate live KoL
```

## Side-effect rule

The handler is pure with respect to the runtime. It only reads already-held state.

Calling it must not:

- contact GitHub;
- contact Kingdomsitter;
- invoke KoLmafia;
- append an audit record;
- create/drop a proposal;
- activate/deactivate an actor state;
- change confirmation or execution authority.

Dependency refreshes remain separate explicit endpoints.

## Required checks

| Check | Requirement |
| --- | --- |
| `runtime_identity` | current process has a runtime ID |
| `audit_ledger` | opened ledger remains verified |
| `kol_observation_present` | bounded KoL observation exists |
| `kol_observation_fresh` | observation age ≤ 5 minutes |
| `installed_revision` | installed KoLmafia revision is known |
| `release_metadata_present` | latest release metadata exists |
| `release_metadata_fresh` | release metadata age ≤ 30 minutes |
| `kingdomsitter_status_known` | sidecar transport has been checked |
| `kingdomsitter_status_fresh` | sidecar check age ≤ 2 minutes |
| `kingdomsitter_transport_healthy` | sidecar transport is healthy |
| `no_unresolved_fault` | no fault text / active fault state |
| `actor_ready` | state plane is ready and not booting |
| `proposal_gate_idle` | no pending/ready/executing/reconciling proposal |

## Reason codes

A NOT_READY response reports only failed **required** checks through stable reason codes such as:

```text
kol_observation_missing
kol_observation_stale
installed_revision_unknown
release_metadata_missing
release_metadata_stale
kingdomsitter_status_unknown
kingdomsitter_status_stale
kingdomsitter_transport_unhealthy
audit_ledger_unverified
unresolved_fault
actor_not_ready
proposal_gate_not_idle
```

## Authority boundary

Every result carries:

```json
{
  "mode": "proposal-and-confirmed-local-staging-only",
  "preflight_grants_authority": false,
  "human_confirmation_required": true,
  "live_kolmafia_mutation": false,
  "release_install": false,
  "kolmafia_restart": false,
  "arbitrary_ash": false,
  "arbitrary_gcli": false
}
```

## Acceptance result

The interactive v0.4 harness proved on the real runtime:

1. a fresh restart with no KoL observation reports NOT_READY;
2. explicit read-only release + Kingdomsitter refreshes do not make it READY without KoL evidence;
3. repeated calls to `/v1/preflight` do not change the audit ledger count;
4. human-run `kol_actor sync` makes the required evidence current and produces READY;
5. READY still reports `preflight_grants_authority: false`;
6. an automatically created but unconfirmed release-stage proposal makes preflight NOT_READY with `proposal_gate_not_idle`;
7. the proposal is never confirmed or successfully executed;
8. restarting only Actor Engine removes the ephemeral proposal;
9. after explicit refresh + human-run `kol_actor sync`, preflight returns READY again.

Freshness expiry itself is unit-tested with an injected clock so the live acceptance test does not waste minutes waiting for evidence to age.

## Live evidence checkpoint — 2026-09-29

**Result:** `PASS LIVE_READ_ONLY_PREFLIGHT_TEST`  
**Exact tested repository HEAD:** `57aa6f627d9edb765d9e44304bde060418eb20f4`  
**Authority expansion:** none

Observed terminal result:

```text
PASS LIVE_READ_ONLY_PREFLIGHT_TEST
Preflight moved NOT_READY -> READY -> NOT_READY -> READY for the expected evidence/gate changes.
Repeated preflight reads appended no audit evidence.
The test proposal was never confirmed or successfully executed.
READY never granted execution authority.
```

This closes the v0.4 preflight acceptance target. The meaning of `READY` remains deliberately narrow: evidence is sufficient to form a proposal under policy; human confirmation and execution authority remain separate.
