# v0.10 human quarantine recovery resolution live acceptance

This harness extends the isolated v0.9 crash test through a deliberately human-governed resolution.

It first proves an ambiguous crash:

```text
execution.started durable
+ active .part-* exists
→ isolated SIGKILL
→ INTERRUPTED_UNKNOWN_OUTCOME
```

Then, behind an explicit human prompt, it moves only the isolated ambiguous file into `.recovery-quarantine/` and submits `quarantine_unknown_no_replay`.

Actor Engine must independently verify active-staging absence, final-artifact absence, sibling-part absence, quarantine directory/file type, and SHA-256 before committing `recovery.resolved`.

The historical interruption remains unknown-outcome evidence. After fresh evidence, proposal formation may reopen, but the harness creates only a new **unconfirmed and unexecuted** proposal.

Expected terminal result:

```text
PASS LIVE_HUMAN_QUARANTINE_RESOLUTION_TEST
The human-governed quarantine disposition was independently verified before recovery resolution was committed.
Resolution preserved the interrupted operation's unknown outcome and granted no replay or execution authority.
Verified resolution cleared only the interruption admission block; fresh evidence reopened proposal formation.
Durable resolution and quarantined bytes survived restart while all proposal authority remained ephemeral.
```
