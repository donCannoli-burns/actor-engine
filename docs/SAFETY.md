# Safety boundaries

1. **Evidence is not authority.** GitHub release metadata, the HTML matrix, Kingdomsitter analysis, and actor observations can inform a proposal but cannot grant execution permission.
2. **KoLmafia is authoritative for game state and live mutations.** The actor engine does not reimplement the game client.
3. **ASH is observation-only in v0.1.0.** `kol_actor.ash` publishes a small state snapshot and has no generic command executor.
4. **Kingdomsitter remains read-only.** The engine consumes `/health` and `/v0/state`; it does not add an execution endpoint to Kingdomsitter.
5. **Release staging is the only implemented write.** It downloads a release artifact into a staging directory after confirmation and verifies GitHub's SHA-256 digest when supplied.
6. **Install/restart/gCLI/arbitrary ASH are structurally unimplemented.** Adding them requires a new transport adapter, explicit policy, tests, and readback reconciliation.
7. **Confirmation is state-bound.** A confirmation carries the exact proposal state digest. If state changes before execution, the write is refused.
