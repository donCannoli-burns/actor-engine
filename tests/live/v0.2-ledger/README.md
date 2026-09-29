# v0.2 live ledger acceptance

This directory preserves the interactive harness used to live-verify the v0.2 durable-evidence / ephemeral-authority boundary.

Run:

```bash
./live-ledger-restart-authority-test.sh
```

The script keeps gCLI under human control, defaults every human boundary to `N`, never executes the confirmed `release.stage` proposal, and restarts only the Actor Engine sidecar after explicit approval.

Verified implementation commit: `5c97d185987b58403f643165f83eb321dde53aa8`.

Observed live result on 2026-09-29:

```text
PASS LIVE_LEDGER_RESTART_AUTHORITY_TEST
Evidence survived restart. Proposal/confirmation authority did not.
No release.stage proposal was executed by this test.
```
