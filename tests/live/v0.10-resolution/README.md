# v0.10 human recovery resolution live acceptance

This harness extends the already verified v0.9 crash window with one explicit human governance action.

It uses a temporary Actor Engine, audit ledger, stage directory, and local blocking fixture for all execution/recovery work. The normal sidecar is used only to capture one fresh human-run `kol_actor sync`.

The sequence is:

```text
execution.started durable
fixture download entered
.part-* exists
SIGKILL isolated Actor Engine
    ↓
INTERRUPTED_UNKNOWN_OUTCOME
    ↓
human sees exact interruption digest + orphan path
    ↓
acknowledge_unknown_no_replay
    ↓
durable kol-actor/resolution-v1
    ↓
historical interruption remains unknown
orphan remains untouched
old proposal remains dead
    ↓
fresh preflight READY
new proposal may be formed
but is never confirmed or executed
    ↓
restart
resolution survives
old + new proposal authority absent
no replay
```

Expected terminal result:

```text
PASS LIVE_HUMAN_RECOVERY_RESOLUTION_TEST
Human resolution acknowledged the exact unknown interruption without asserting success or failure.
The resolution evidence was independently verifiable and durably cleared only the interruption admission block.
The orphaned partial artifact remained untouched and neither old nor new proposal authority was restored.
Resolution survived isolated restart; replay remained impossible and the workspace was cleaned only by the harness.
```
