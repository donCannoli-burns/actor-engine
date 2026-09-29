# v0.5 proposal admission / preflight binding live acceptance

This harness is the live acceptance test used to verify the v0.5.0 admission checkpoint.

It proves the chain:

```text
NOT_READY -> proposal refused
READY -> exact preflight bound into admission -> proposal created
proposal admission != confirmation
restart -> proposal authority gone; admission evidence remains
```

The harness never calls the confirmation endpoint.

It includes one optional, explicit default-`N` negative execution probe. If approved, that probe attempts to execute the unconfirmed local `release.stage` proposal and requires HTTP 409 `not confirmed`. It is included specifically to prove that valid admission evidence does not substitute for human confirmation.

Observed terminal result on 2026-09-29:

```text
PASS LIVE_PROPOSAL_ADMISSION_BINDING_TEST
NOT_READY preflight could not form a proposal.
READY proposal admission carried an independently verified canonical digest and exact checkset.
Admission evidence propagated into durable audit history.
The test proposal was never confirmed or successfully executed.
Admission did not restore authority after restart.
```

The result was obtained against exact repository HEAD `19a43a36ab53367b8985de835f0e5cb893e060c3`. The optional negative execute probe was approved and returned HTTP 409 `not confirmed`; no confirmation endpoint was called and no release staging succeeded.
