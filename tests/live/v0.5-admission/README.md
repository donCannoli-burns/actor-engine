# v0.5 proposal admission / preflight binding live acceptance

This harness ships with the admission implementation.

It proves the chain:

```text
NOT_READY -> proposal refused
READY -> exact preflight bound into admission -> proposal created
proposal admission != confirmation
restart -> proposal authority gone; admission evidence remains
```

The harness never calls the confirmation endpoint.

It includes one optional, explicit default-`N` negative execution probe. If approved, that probe attempts to execute the unconfirmed local `release.stage` proposal and requires HTTP 409 `not confirmed`. It is included specifically to prove that valid admission evidence does not substitute for human confirmation.

Expected terminal success:

```text
PASS LIVE_PROPOSAL_ADMISSION_BINDING_TEST
NOT_READY preflight could not form a proposal.
READY proposal admission carried an independently verified canonical digest and exact checkset.
Admission evidence propagated into durable audit history.
The test proposal was never confirmed or successfully executed.
Admission did not restore authority after restart.
```
