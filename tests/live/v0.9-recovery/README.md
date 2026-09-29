# v0.9 crash/restart recovery live acceptance

This harness tests the ambiguity window after durable `execution.started` but before a terminal audit record.

It uses only an isolated temporary Actor Engine, stage directory, ledger, and local blocking fixture server for the execution. The normal Actor Engine is used only to capture one fresh human-run `kol_actor sync`.

The harness explicitly waits for all three conditions before SIGKILL:

```text
execution.started durable
fixture download entered
.part-* file exists
```

After restart, success requires `INTERRUPTED_UNKNOWN_OUTCOME`, stable independently recomputed interruption evidence, blocked preflight, no replay/restoration authority, and preservation of the orphaned temporary file until the harness itself removes the temporary workspace.

Expected terminal result:

```text
PASS LIVE_CRASH_RESTART_RECOVERY_TEST
The isolated process was killed only after durable execution.started and a partial staging file existed.
Restart classified the execution as INTERRUPTED_UNKNOWN_OUTCOME without inferring success or failure.
Recovery evidence was stable, read-only, and blocked new proposal admission without restoring replay authority.
A second restart preserved the interruption digest; the temporary workspace was cleaned only by the harness.
```
