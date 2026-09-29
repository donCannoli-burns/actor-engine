# v0.3 runtime / observation identity live acceptance

This directory contains the interactive acceptance harness used to live-verify the v0.3.0 runtime / observation identity checkpoint.

The harness follows the same human-boundary convention as the verified v0.2 ledger test:

- safe/read-only checks are automated;
- KoLmafia gCLI commands are displayed for the human to run;
- every gCLI/restart boundary defaults to `N`;
- the harness never confirms or successfully executes its test `release.stage` proposal;
- no KoLmafia restart, release installation, arbitrary ASH/gCLI, or live game mutation is authorized.

Run:

```bash
./runtime-observation-identity-test.sh
```

Observed terminal success on 2026-09-29:

```text
PASS LIVE_RUNTIME_OBSERVATION_IDENTITY_TEST
Runtime identity changed across restart.
Observation identity matched bounded KoL observations deterministically.
Proposal evidence retained its originating runtime/observation identity.
No release.stage proposal was confirmed or executed by this test.
```

The result was obtained against exact repository HEAD `bd473f58babf1bc68c3407c45d506243f2621086`; v0.3.0 is therefore a verified checkpoint.
