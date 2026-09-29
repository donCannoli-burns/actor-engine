# v0.4 read-only preflight live acceptance

This directory contains the interactive acceptance harness used to live-verify the v0.4.0 read-only preflight checkpoint.

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

Observed terminal result on 2026-09-29:

```text
PASS LIVE_READ_ONLY_PREFLIGHT_TEST
Preflight moved NOT_READY -> READY -> NOT_READY -> READY for the expected evidence/gate changes.
Repeated preflight reads appended no audit evidence.
The test proposal was never confirmed or successfully executed.
READY never granted execution authority.
```

The result was obtained against exact repository HEAD `57aa6f627d9edb765d9e44304bde060418eb20f4`; v0.4.0 is therefore a verified checkpoint.
