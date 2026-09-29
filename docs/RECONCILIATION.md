# Terminal receipt / reconciliation provenance

Status: **verified v0.8.0 checkpoint — live acceptance passed**  
Verified baseline: **v0.7.0** at `41f9864d3aa2b0b0daf6b63dc4fe3da1b661e9c8`  
Authority change: **none**

## Goal

Close the provenance chain through the terminal operation result:

```text
observation
  → preflight
  → admission
  → proposal
  → confirmation
  → execution attempt
  → reconciliation / receipt
```

## Evidence schema

```json
{
  "version": "kol-actor/reconciliation-v1",
  "digest": "sha256:<64 lowercase hex>",
  "proposal_id": "p-...",
  "operation": "release.stage",
  "admission_digest": "sha256:...",
  "confirmation_digest": "sha256:...",
  "execution_digest": "sha256:...",
  "runtime_id": "run-...",
  "observation_id": "obs-...",
  "success": false,
  "outcome": "failed",
  "detail": "digest mismatch for KoLmafia-v08-fixture.jar",
  "sha256": "<actual downloaded SHA-256>",
  "completed_at": "RFC3339 timestamp",
  "artifact_committed": false,
  "evidence_grants_authority": false,
  "authority_restorable": false
}
```

For successful staging, `success=true`, `outcome=succeeded`, and `artifact_committed=true` with the committed artifact path and SHA-256.

The digest excludes only `digest` itself and is SHA-256 over canonical sorted-key compact JSON.

## Receipt propagation

The HTTP receipt carries both:

```text
reconciliation_digest
reconciliation
```

The terminal audit event carries the same digest/object. This makes the returned receipt independently comparable with durable history.

## Failure-path semantics

A digest-mismatch stage has useful terminal evidence even though the operation fails:

```text
execution.started
    ↓
download into temporary file
    ↓
actual SHA-256 computed
    ↓
expected SHA-256 mismatch
    ↓
temporary file removed
    ↓
execution.failed
    ↓
reconciliation:
  success=false
  artifact_committed=false
  sha256=<actual bytes>
```

No final-named staged artifact is committed.

If reconciliation evidence construction itself fails after an otherwise successful stage, Actor Engine removes the staged artifact and faults rather than returning a success without terminal provenance.

## Isolated live acceptance

The v0.8 harness intentionally avoids the official release endpoint and real Actor Engine staging directory.

It launches:

1. the normal Actor Engine remains untouched on its normal port;
2. a temporary local fixture server serves fake release metadata + fixture bytes;
3. a second isolated Actor Engine runs on another loopback port;
4. the isolated engine uses a temporary stage directory and audit ledger;
5. a fresh bounded observation is copied from the normal read-only state after human-run `kol_actor sync`;
6. the fixture metadata advertises an intentionally wrong SHA-256;
7. after explicit confirmation and execute approval, the isolated stage must fail digest verification;
8. the harness independently recomputes reconciliation SHA-256;
9. it proves the stage directory contains no committed artifact or `.part-*` file;
10. it restarts only the isolated Actor Engine and proves reconciliation evidence survives without restoring authority.

The acceptance test treats a successful fixture stage as failure.

## Acceptance result

```text
PASS LIVE_TERMINAL_RECONCILIATION_PROVENANCE_TEST
The isolated authorized attempt reached execution.started and failed on the intentional digest mismatch.
The terminal receipt carried independently verifiable reconciliation provenance.
No fixture artifact or temporary part file remained committed.
Durable reconciliation evidence survived isolated Actor Engine restart without restoring authority.
```


## Live evidence checkpoint — 2026-09-29

**Result:** `PASS LIVE_TERMINAL_RECONCILIATION_PROVENANCE_TEST`  
**Exact tested repository HEAD:** `b11b94417bcbfec34d485d91de987c163c32ae8f`  
**Authority expansion:** none

The live campaign ran an isolated Actor Engine against a local bad-digest fixture release. The normal sidecar was used only for a fresh read-only KoL observation handoff.

Observed path:

```text
execution.started
    ↓
intentional SHA-256 mismatch
    ↓
execution.failed
    ↓
kol-actor/reconciliation-v1
    ↓
no committed fixture artifact
no .part-* file
    ↓
isolated Actor Engine restart
    ↓
reconciliation evidence survives
authority does not
```

The terminal receipt's reconciliation digest was independently recomputed. Durable audit history linked the same execution digest through `execution.started` and `execution.failed`. After isolated restart, the historical reconciliation object remained readable and the old proposal was not executable.
