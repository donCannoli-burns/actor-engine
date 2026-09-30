# Human recovery resolution with verified quarantine

Status: **v0.10.0-dev — implementation built, live acceptance pending**  
Verified baseline: **v0.9.0** at `9e2b39f891e7ba969cce86c3f1355f113818fb15`  
Authority change: **bounded human recovery resolution only; no replay authority**

## Goal

Allow a human to clear the v0.9 interruption admission block without rewriting history or replaying the interrupted operation.

The supported decision is exactly:

```text
quarantine_unknown_no_replay
```

## Separation of facts

Resolution deliberately separates three facts:

```text
historical operation outcome
    remains UNKNOWN

historical artifact outcome
    remains UNKNOWN

ambiguous bytes found in active staging
    receive a VERIFIED QUARANTINE DISPOSITION
```

A resolution therefore carries:

```text
outcome_remains_unknown = true
artifact_state_remains_unknown = true
ambiguous_bytes_disposition = quarantined
replay_permitted = false
authority_restorable = false
resolution_grants_execution_authority = false
interruption_block_cleared = true
```

## Human action before API action

Actor Engine does not move the orphan.

The human/operator must first move the exact ambiguous part file from active staging into:

```text
<stage_dir>/.recovery-quarantine/<interruption-hex>.part
```

The resolution endpoint then verifies the disposition.

## Endpoint

```text
POST /v1/recovery/{interruption_digest}/resolve
```

Request:

```json
{
  "interruption_digest": "sha256:...",
  "decision": "quarantine_unknown_no_replay",
  "resolved_by": "human",
  "note": "reviewed isolated ambiguous bytes; quarantine and abandon replay",
  "orphan_name": ".KoLmafia-example.jar.part-1234",
  "quarantine_name": "<interruption-hex>.part",
  "quarantine_sha256": "<64 lowercase hex>"
}
```

The path digest and body digest must match exactly.

## Filesystem verification

Before durable resolution, Actor Engine derives the intended final artifact basename from the standard temporary-file contract:

```text
.<final-basename>.part-<suffix>
```

It then requires:

1. the supplied orphan name is a simple basename matching that contract;
2. the original orphan path no longer exists under active staging;
3. the derived final artifact path does not exist;
4. no sibling `.<final-basename>.part-*` entries remain in active staging;
5. `.recovery-quarantine` exists as a real directory and is not a symlink;
6. the quarantine filename equals `<interruption-digest-hex>.part`;
7. the quarantine target is a regular file;
8. Actor Engine recomputes its SHA-256;
9. the recomputed SHA-256 exactly equals the request.

Only then can `recovery.resolved` be appended.

## Resolution evidence

`kol-actor/resolution-v1` binds:

- interruption digest;
- proposal ID;
- operation;
- execution digest;
- original `execution.started` event hash;
- decision;
- resolving human;
- bounded human note;
- resolving runtime ID and timestamp;
- orphan basename;
- derived final artifact basename;
- quarantine basename and SHA-256;
- verified absence/hash flags;
- explicit no-replay/no-authority flags.

Its digest is SHA-256 over canonical sorted-key compact JSON excluding only `digest`.

## Recovery report after resolution

A valid durable resolution moves the interruption from:

```text
unresolved[]
```

to:

```text
resolved[]
  interruption: original unknown-outcome evidence
  resolution: verified quarantine evidence
```

The original interruption object is unchanged. It still states unknown outcome and unknown artifact state.

## Admission reopening

After resolution:

```text
unresolved_interrupted_executions = 0
```

The recovery-specific preflight block can clear.

This does not bypass any other preflight requirement. Fresh observation, release metadata, Kingdomsitter health, actor readiness, and idle proposal gate remain required.

## Live acceptance

The harness recreates the v0.9 crash in a fully isolated temporary Actor Engine:

```text
execution.started durable
+ active .part-* exists
→ SIGKILL isolated Actor Engine
→ INTERRUPTED_UNKNOWN_OUTCOME
```

Then, behind an explicit human prompt, the harness:

1. computes the partial file SHA-256;
2. shows source/quarantine destinations;
3. moves only that isolated temporary file into `.recovery-quarantine/`;
4. calls the resolution endpoint;
5. independently recomputes the resolution digest;
6. proves recovery becomes `CLEAR` with one resolved historical interruption;
7. refreshes bounded evidence and proves preflight becomes READY;
8. creates a **new unconfirmed proposal only** to prove proposal admission reopened;
9. restarts the isolated Actor Engine;
10. proves the resolution survives, the quarantine bytes/hash remain unchanged, and neither the interrupted nor new proposal authority survives restart.

No proposal after resolution is confirmed or executed by the acceptance test.

## Acceptance target

```text
PASS LIVE_HUMAN_QUARANTINE_RESOLUTION_TEST
The human-governed quarantine disposition was independently verified before recovery resolution was committed.
Resolution preserved the interrupted operation's unknown outcome and granted no replay or execution authority.
Verified resolution cleared only the interruption admission block; fresh evidence reopened proposal formation.
Durable resolution and quarantined bytes survived restart while all proposal authority remained ephemeral.
```
