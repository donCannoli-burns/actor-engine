# Crash/restart recovery detection

Status: **v0.9.0-dev — implementation built, live acceptance pending**  
Verified baseline: **v0.8.0** at `915f8a4900062c41b8ae5128b522157becbb4b3a`  
Authority change: **none**

## Goal

Detect the ambiguity created when Actor Engine disappears after durable execution admission but before a durable terminal result.

The key rule is:

```text
absence of terminal evidence != evidence of failure
absence of terminal evidence != permission to retry
```

## Recovery classification

A supported interrupted execution is:

```text
execution.started
with non-empty execution_digest
and no later/matching:
  execution.succeeded
  execution.failed
```

The recovery report status becomes:

```text
INTERRUPTED_UNKNOWN_OUTCOME
```

A clean ledger reports `CLEAR`.

## Interruption evidence

`kol-actor/interruption-v1` binds:

```text
proposal_id
operation
admission_digest
confirmation_digest
execution_digest
origin_runtime_id
execution_runtime_id
observation_id
started_seq
started_at
started_event_hash
status = INTERRUPTED_UNKNOWN_OUTCOME
outcome_known = false
artifact_state = unknown
replay_permitted = false
authority_restorable = false
```

The digest is SHA-256 over canonical sorted-key compact JSON excluding only the `digest` field.

It deliberately excludes the *detecting* runtime and current wall-clock time, so the same durable interrupted start produces the same interruption digest across later restarts. The outer recovery report carries current `runtime_id` and `generated_at`.

## Read-only endpoint

```text
GET /v1/recovery
```

The endpoint:

- reads the verified ledger image;
- derives unresolved interruption evidence;
- performs no network refresh;
- appends no audit event;
- mutates no file;
- cleans no staging path;
- reconstructs no proposal or confirmation;
- performs no retry.

Authority response is explicit:

```json
{
  "replay_permitted": false,
  "authority_restorable": false,
  "automatic_resolution": false,
  "evidence_only": true
}
```

## Admission interaction

While `unresolved.length > 0`, preflight fails:

```text
check:  no_interrupted_execution
reason: interrupted_execution_unresolved
```

This blocks *new* proposals until a future, separately designed resolution mechanism exists.

v0.9 intentionally has **no resolution endpoint**.

## Why artifact state is unknown

A process can disappear anywhere after `execution.started`:

```text
before network request
during download
after bytes reach temp file
after digest verification
after final rename
before terminal audit append
```

Therefore ledger shape alone cannot establish whether an artifact exists or whether the underlying operation took effect.

v0.9 records `artifact_state=unknown` rather than guessing.

## Isolated live acceptance

The harness uses a temporary Actor Engine, audit ledger, staging directory, and local fixture server.

The fixture:

1. publishes a fake release;
2. returns HTTP 200 for the fake JAR;
3. writes an initial chunk;
4. flushes it;
5. blocks the remainder of the response.

The harness waits until:

```text
execution.started is durable
fixture download entered
.part-* file exists
```

Only then does it SIGKILL the **isolated** Actor Engine.

On restart it must prove:

- `GET /v1/recovery` reports exactly one unresolved interruption;
- interruption SHA-256 independently recomputes;
- outcome is unknown;
- artifact state is unknown;
- replay and restoration are false;
- repeated recovery reads add no audit evidence;
- fresh dependencies + observation still leave preflight NOT_READY solely because recovery is unresolved;
- old proposal execution returns `proposal not found`;
- a second restart changes report runtime but preserves the interruption digest;
- the isolated orphan `.part-*` remains untouched by read-only recovery detection.

The temporary workspace is deleted only by the test harness cleanup after all assertions.

## Acceptance target

```text
PASS LIVE_CRASH_RESTART_RECOVERY_TEST
The isolated process was killed only after durable execution.started and a partial staging file existed.
Restart classified the execution as INTERRUPTED_UNKNOWN_OUTCOME without inferring success or failure.
Recovery evidence was stable, read-only, and blocked new proposal admission without restoring replay authority.
A second restart preserved the interruption digest; the temporary workspace was cleaned only by the harness.
```
