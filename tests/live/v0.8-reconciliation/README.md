# v0.8 terminal reconciliation provenance live acceptance

This harness tests terminal receipt/reconciliation provenance without touching the real Actor Engine staging directory.

It keeps the normal sidecar available for one human-run `kol_actor sync`, then launches a second isolated Actor Engine against a local fixture release server. The fixture intentionally advertises the wrong SHA-256.

Expected path:

```text
isolated READY proposal
  -> human confirmation
  -> execution.started
  -> temporary fixture download
  -> digest mismatch
  -> temporary file cleanup
  -> execution.failed
  -> kol-actor/reconciliation-v1
  -> isolated restart
  -> evidence survives, authority does not
```

A successful fixture stage or any file left in the temporary stage directory is a failure.

Expected terminal result:

```text
PASS LIVE_TERMINAL_RECONCILIATION_PROVENANCE_TEST
The isolated authorized attempt reached execution.started and failed on the intentional digest mismatch.
The terminal receipt carried independently verifiable reconciliation provenance.
No fixture artifact or temporary part file remained committed.
Durable reconciliation evidence survived isolated Actor Engine restart without restoring authority.
```
