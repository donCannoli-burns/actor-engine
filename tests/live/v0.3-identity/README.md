# v0.3 runtime / observation identity live acceptance

This directory contains the interactive acceptance harness for the v0.3 identity development layer.

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

Expected terminal success:

```text
PASS LIVE_RUNTIME_OBSERVATION_IDENTITY_TEST
Runtime identity changed across restart.
Observation identity matched bounded KoL observations deterministically.
Proposal evidence retained its originating runtime/observation identity.
No release.stage proposal was confirmed or executed by this test.
```

Do not promote the feature from `0.3.0-dev` to a verified checkpoint until this live result is obtained.
