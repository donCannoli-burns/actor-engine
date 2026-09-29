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
