# Runtime and observation identity

Status: **verified v0.3.0 checkpoint — live acceptance passed**  
Verified baseline: **v0.2.0** at `a6b1ee03be11fe716b2a30059e15701865a12d88`  
Authority change: **none**

## Goal

Make Actor Engine evidence unambiguous across process restarts and repeated KoL observations without turning identity into authorization.

```text
runtime identity       observation identity
------------------     -----------------------------
process lifetime       bounded KoL observation facts
fresh on restart       deterministic from content
random 128-bit value   SHA-256 content identity
not authority          not authority
```

## runtime_id

At process startup Actor Engine obtains 16 bytes from the operating system cryptographic random source and renders:

```text
run-<32 lowercase hexadecimal characters>
```

If secure random generation fails, startup fails. There is no timestamp or predictable fallback.

The runtime ID is exposed in `/health` and `/v1/state`, written into `runtime.started`, and propagated into proposals, receipts, and subsequent audit events.

A restart must produce a different runtime ID.

## observation_id

The bounded KoL observation currently contains:

```text
event
character
total_turns
ascension_turns
adventures
ascensions
breakfast
```

The identity algorithm is deliberately independent of Go map iteration and JSON key order:

1. sort field names lexicographically;
2. for every key, write its UTF-8 byte length as unsigned 64-bit big-endian;
3. write the UTF-8 key bytes;
4. write the value UTF-8 byte length the same way;
5. write the UTF-8 value bytes;
6. SHA-256 the resulting byte stream;
7. prefix the lowercase hex digest with `obs-`.

Consequences:

- identical bounded observation content → identical `observation_id`;
- changing any bounded field → different `observation_id`;
- restarting Actor Engine does not by itself change an observation's content identity;
- `observation_id` is not a freshness proof. Freshness still depends on when/where the observation was obtained.

## Binding

A release-stage proposal copies the current `runtime_id` and `observation_id`. The state digest also includes these snapshot fields, so a confirmation belongs to the process/observation context in which it was formed.

Receipts retain the originating proposal identities. Audit lifecycle records carry the identity of the runtime and observation context in which each event occurred.

Historical audit evidence can therefore show an old proposal's originating runtime even when a post-restart denial is produced under a new runtime.

## Safety boundary

Identity is evidence only:

```text
runtime_id != credential
observation_id != permission
state_digest != human confirmation
audit history != restored authority
```

The feature adds no release installation, KoLmafia restart, arbitrary ASH/gCLI, game mutation, social automation, or automatic confirmation authority.

## Acceptance result

The interactive live harness proved on 2026-09-29:

1. `runtime_id` is well formed and stable within one process;
2. `observation_id` independently recomputes from the returned bounded KoL state;
3. repeated identical `kol_actor sync` observations have the same identity;
4. changing only the event through `kol_actor turn` changes observation identity without spending an adventure;
5. returning to the same bounded manual observation returns to the same identity when no other bounded value changed;
6. a proposal/audit record carries its originating identities;
7. after Actor Engine restart, `runtime_id` changes;
8. a fresh identical KoL observation can retain its prior `observation_id`;
9. the pre-restart proposal remains unavailable after restart;
10. no release-stage proposal is confirmed or executed by the harness.

Harness: `tests/live/v0.3-identity/runtime-observation-identity-test.sh`.

## Live evidence checkpoint — 2026-09-29

**Result:** `PASS LIVE_RUNTIME_OBSERVATION_IDENTITY_TEST`  
**Feature implementation commit:** `e32b0118871c695a9f95e1a9efe8b32411af60cc`  
**Exact tested repository HEAD:** `bd473f58babf1bc68c3407c45d506243f2621086`  
**Authority expansion:** none

Observed identities:

```text
runtime before restart:
run-871adb21a6724de6de3820420149c433

runtime after restart:
run-eae31582ec1d3a4e12299d9dbabaaa79

stable manual observation:
obs-173a105577d0e666bc6eeccdb07faf5d234d5d8b1f4055f88ed266d0f8d8e873
```

The same bounded manual KoL facts produced the same `observation_id` before and after the Actor Engine restart, while the `runtime_id` changed. A temporary `event=after-adventure` observation produced a different identity, and returning to `event=manual` restored the original content identity.

The unconfirmed proposal used for provenance verification was:

```text
proposal_id   = p-a5aacc864ee2db95e53e3931
runtime_id    = run-871adb21a6724de6de3820420149c433
observation_id= obs-173a105577d0e666bc6eeccdb07faf5d234d5d8b1f4055f88ed266d0f8d8e873
state_digest  = c02fec452a09177ea564029725d1d45ce68d1b57ae8c44b5f0d98bc94a77326c
target        = KoLmafia-29315.jar / r29315
```

The harness did not confirm or successfully execute that proposal. After restart, executing its old ID returned:

```text
HTTP 409
proposal not found
```

Audit history retained the originating runtime/observation identity and recorded the post-restart denial under the new runtime.

This closes the v0.3 identity acceptance target.
