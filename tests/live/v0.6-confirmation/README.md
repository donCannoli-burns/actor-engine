# v0.6 confirmation provenance binding live acceptance

This harness is the live acceptance test used to verify the v0.6.0 confirmation-provenance checkpoint.

It verifies:

```text
READY admission
    ↓
proposal
    ↓ explicit human y/N
confirmation
    ↓
kol-actor/confirmation-v1
    ↓
independently recomputable SHA-256
    ↓
full durable proposal.confirmed evidence

Actor Engine restart
    ↓
approval disappears
    ↓
old execute = 409 proposal not found
    ↓
confirmation evidence remains valid history
```

The confirmed proposal is **never executed while its approval is live**. The only execute request happens after Actor Engine restart, when the gate must be empty, and is required to fail.

Observed terminal result on 2026-09-29:

```text
PASS LIVE_CONFIRMATION_PROVENANCE_BINDING_TEST
Human confirmation produced independently verifiable first-class evidence.
Confirmation evidence bound proposal, state, admission, human, runtime, and timestamp.
The confirmed proposal was never executed while approval was live.
Durable confirmation evidence survived restart but did not restore permission.
```

The result was obtained against exact repository HEAD `eee62dc77d35585188e495501ddca987a1943107`. The old confirmed proposal returned HTTP 409 `proposal not found` after Actor Engine restart, proving the durable evidence did not restore approval.
