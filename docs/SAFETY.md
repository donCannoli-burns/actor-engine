# Safety boundaries

1. **Evidence is not authority.** GitHub release metadata, the HTML matrix, Kingdomsitter analysis, and actor observations can inform a proposal but cannot grant execution permission.
2. **KoLmafia is authoritative for game state and live mutations.** The actor engine does not reimplement the game client.
3. **ASH is observation-only in v0.1.0.** `kol_actor.ash` publishes a small state snapshot and has no generic command executor.
4. **Kingdomsitter remains read-only.** The engine consumes `/health` and `/v0/state`; it does not add an execution endpoint to Kingdomsitter.
5. **Release staging is the only implemented write.** It downloads a release artifact into a staging directory after confirmation and verifies GitHub's SHA-256 digest when supplied.
6. **Install/restart/gCLI/arbitrary ASH are structurally unimplemented.** Adding them requires a new transport adapter, explicit policy, tests, and readback reconciliation.
7. **Confirmation is state-bound.** A confirmation carries the exact proposal state digest. If state changes before execution, the write is refused.

8. **Audit persistence never rehydrates authority.** Restart may reload verified event history, but proposals and confirmations remain memory-only and are never reconstructed from the ledger.
9. **Authority-bearing paths fail closed on evidence failure.** If required audit evidence cannot be durably appended, proposal/confirmation/execution does not proceed as successful.
10. **Tampered evidence blocks startup.** The JSONL ledger is sequence-checked and hash-chain-verified when opened; a broken chain is an integrity fault, not a source to partially trust.

The v0.2 audit ledger adds durability to evidence, not durability to permission.
11. **Preflight is evidence, not permission.** A `READY` preflight means only that current evidence is sufficient to form a proposal under policy. It does not confirm, execute, install, restart, or authorize anything.
12. **Preflight is side-effect free.** Reading `/v1/preflight` must not refresh remote state, append audit evidence, create/drop proposals, or alter the actor state plane.

The v0.4 preflight layer adds readiness visibility, not execution authority.
13. **Admission is not confirmation.** A valid preflight admission digest proves only which READY evidence/checkset allowed proposal formation; it is never accepted as human confirmation or execution authority.
14. **NOT_READY cannot form a proposal.** Proposal admission must fail before gate insertion, pending-proposal state, or `proposal.created` audit evidence when the current preflight is not READY.
15. **Admission evidence is tamper-evident.** The embedded exact preflight report is canonically hashed and re-verified before execution after the ordinary confirmation/state gates have passed.

The v0.5 admission layer adds provenance to proposal formation, not authority.
16. **Confirmation evidence is not the approval store.** The durable confirmation object proves who confirmed which exact proposal/admission/state/runtime and when; only the in-memory gate entry carries live approval.
17. **Confirmation evidence cannot rehydrate authority.** Startup and audit-ledger replay never insert confirmation evidence into the gate. Historical confirmation evidence must remain executable only as history, never as permission.
18. **Confirmation provenance is tamper-evident.** Its canonical digest is independently verifiable and is checked by the gate before an approval can be consumed.

The v0.6 confirmation layer completes provenance through the human boundary without making permission durable.
19. **Execution-attempt evidence is not execution permission.** It records the facts presented at the execution boundary and the gate decision; it cannot authorize a proposal.
20. **Denied execution remains pre-operation.** A stale-state denial must be durably observable without reaching `execution.started` or the operation body.
21. **Execution provenance is chained, not substitutive.** Admission and confirmation digests are inputs to execution-attempt evidence, but none of those evidence objects can replace the in-memory gate checks.

The v0.7 execution layer adds boundary provenance, not capability.
22. **Terminal evidence is not authority.** A reconciliation object records what happened after an authorized attempt; it cannot authorize another attempt.
23. **Committed-artifact state is explicit.** Failure reconciliation must never claim an artifact was committed, and success evidence must bind the actual artifact path/hash when one exists.
24. **Terminal evidence is fail-closed.** If reconciliation provenance cannot be constructed before terminal audit commit, a successful staged artifact is removed rather than returned without provenance.
25. **Bad-digest acceptance is isolated.** The v0.8 live harness uses a temporary stage directory and a local fixture release server; it never points the acceptance execution at the real runtime staging directory.

The v0.8 reconciliation layer adds provenance, not capability.
26. **Missing terminal evidence means unknown outcome.** A durable `execution.started` without a matching terminal record must never be rewritten as success, failure, or safe-to-retry.
27. **Crash recovery is read-only in v0.9.** Recovery detection may classify evidence and block proposal admission, but it may not replay, clean, resolve, or authorize an operation.
28. **Interrupted execution blocks new proposals.** Preflight must fail closed while any supported durable execution-start record lacks terminal evidence.
29. **Artifact state remains unknown.** Recovery evidence does not infer artifact success/failure from ledger absence or from temporary-file naming.
30. **Historical authority stays dead.** Confirmation and execution evidence discovered after restart remain evidence only; no prior proposal is reconstructed into the gate.

The v0.9 recovery layer adds crash detection and admission blocking, not recovery authority.
31. **Resolution is not outcome classification.** v0.10 may clear an admission block only while preserving `outcome_remains_unknown=true` and `artifact_state_remains_unknown=true`.
32. **Resolution requires external human disposition first.** Actor Engine does not move, delete, or clean ambiguous bytes as part of the resolution endpoint.
33. **Active staging must be proven clear.** The original orphan, derived final artifact, and sibling part files must all be absent before resolution can commit.
34. **Quarantine is hash-bound.** Resolution requires a real non-symlink quarantine directory and a regular quarantined file whose SHA-256 exactly matches the request.
35. **Resolution cannot replay.** A durable `recovery.resolved` record clears only the interruption admission block; it does not reconstruct proposal, confirmation, or execution authority.

The v0.10 layer adds a human-governed evidence transition from unresolved ambiguity to quarantined ambiguity. It does not add operation replay or live KoL mutation.
