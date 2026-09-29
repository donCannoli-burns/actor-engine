# v0.4 read-only preflight live acceptance

This directory ships with the v0.4 preflight implementation rather than being written after the feature.

The harness automates read-only HTTP verification and keeps KoLmafia gCLI / Actor Engine restart boundaries behind explicit default-`N` prompts.

It proves:

```text
restart/no KoL evidence     -> NOT_READY
fresh explicit evidence     -> READY
unconfirmed pending proposal-> NOT_READY
Actor Engine restart        -> proposal authority gone
fresh evidence again        -> READY
```

Five repeated preflight reads must leave the audit ledger count unchanged.

The harness creates one **unconfirmed** release-stage proposal only to prove that a non-idle proposal gate closes readiness. It never confirms or successfully executes that proposal.

Expected terminal result:

```text
PASS LIVE_READ_ONLY_PREFLIGHT_TEST
Preflight moved NOT_READY -> READY -> NOT_READY -> READY for the expected evidence/gate changes.
Repeated preflight reads appended no audit evidence.
The test proposal was never confirmed or successfully executed.
READY never granted execution authority.
```
