# v0.7 execution-attempt provenance live acceptance

This acceptance harness is built with the feature and deliberately uses a **stale-state denial** rather than a successful release stage.

It requires the human to:

1. publish a fresh `kol_actor sync`;
2. approve confirmation of one proposal;
3. publish `kol_actor turn` to change only the bounded observation;
4. approve one stale execute probe.

The harness refuses the execute probe unless the current `observation_id` differs from the proposal's originating observation.

Expected success:

```text
PASS LIVE_EXECUTION_ATTEMPT_PROVENANCE_TEST
A stale confirmed proposal was denied before operation start.
The denial carried independently verifiable execution-attempt provenance.
No execution.started or execution.succeeded record existed for the test proposal.
The denial burned proposal authority; retry returned proposal not found.
```

A successful `release.stage` is a test failure.
