#!/usr/bin/env bash
set -euo pipefail

BASE="${BASE:-http://127.0.0.1:10424}"
PROJECT_ROOT="${PROJECT_ROOT:-$HOME/.kolmafia/kolmaf-ai}"
ACTOR_REPO="${ACTOR_REPO:-$PROJECT_ROOT/actor-engine}"
SETUP_DIR="${SETUP_DIR:-$PROJECT_ROOT/actor-engine-local-setup}"
REQUIRED_BASE_COMMIT="${REQUIRED_BASE_COMMIT:-28aa4b2d8c824f173d163c3194005f77a73cba08}"
EXPECTED_VERSION="${EXPECTED_VERSION:-0.4.0-dev}"

RED=$'\e[31m'; GREEN=$'\e[32m'; YELLOW=$'\e[33m'; RESET=$'\e[0m'
say(){ printf '%s\n' "$*"; }
pass(){ printf '%sPASS%s %s\n' "$GREEN" "$RESET" "$*"; }
warn(){ printf '%sWARN%s %s\n' "$YELLOW" "$RESET" "$*"; }
fail(){ printf '%sFAIL%s %s\n' "$RED" "$RESET" "$*" >&2; exit 1; }
section(){ say; say "========== $* =========="; }
need(){ command -v "$1" >/dev/null 2>&1 || fail "missing command: $1"; }
prompt_yes(){
  local q="$1" ans
  printf '%s%s%s [y/N]: ' "$YELLOW" "$q" "$RESET"
  read -r ans || true
  case "${ans:-}" in y|Y|yes|YES|Yes) return 0;; *) return 1;; esac
}
json_get(){ local key="$1"; python3 -c 'import json,sys; print(json.load(sys.stdin)[sys.argv[1]])' "$key"; }
audit_count(){ curl -fsS "$BASE/v1/audit/recent?limit=1" | python3 -c 'import json,sys; print(json.load(sys.stdin)["ledger"]["count"])'; }
wait_health(){
  for _ in {1..30}; do
    if curl -fsS "$BASE/health" >/dev/null 2>&1; then return 0; fi
    sleep 1
  done
  return 1
}

for c in curl python3 git sha256sum go grep; do need "$c"; done
[[ -d "$ACTOR_REPO/.git" ]] || fail "Actor Engine repo not found: $ACTOR_REPO"
[[ -x "$SETUP_DIR/start.sh" && -x "$SETUP_DIR/stop.sh" ]] || fail "setup start/stop scripts not found: $SETUP_DIR"

section "0. Preflight"
HEAD="$(git -C "$ACTOR_REPO" rev-parse HEAD)"
say "repo: $ACTOR_REPO"
say "HEAD: $HEAD"
if git -C "$ACTOR_REPO" merge-base --is-ancestor "$REQUIRED_BASE_COMMIT" "$HEAD" 2>/dev/null; then
  pass "verified v0.3.0 baseline is an ancestor"
else
  warn "verified baseline is not an ancestor: $REQUIRED_BASE_COMMIT"
  if ! prompt_yes "Continue against this different history?"; then exit 2; fi
fi

section "1. Static verification"
(
  cd "$ACTOR_REPO"
  go test ./...
  go test -race ./...
  go vet ./...
  sha256sum -c MANIFEST.sha256
)
pass "static verification complete"

section "2. Runtime version and authority boundary"
HEALTH="$(curl -fsS "$BASE/health")" || fail "Actor Engine health unavailable"
echo "$HEALTH" | python3 -m json.tool
python3 -c 'import json,sys; x=json.load(sys.stdin); assert x["version"]==sys.argv[1],x; assert x["live_kolmafia_mutation"] is False,x; assert x["authority_restored_on_boot"] is False,x' "$EXPECTED_VERSION" <<<"$HEALTH"
pass "running sidecar reports $EXPECTED_VERSION with no live KoL mutation authority"

section "3. Read-only preflight before KoL observation"
P0="$(curl -fsS "$BASE/v1/preflight")"
echo "$P0" | python3 -m json.tool
python3 -c 'import json,sys; x=json.load(sys.stdin); assert x["status"]=="NOT_READY",x; assert x["ready_for_proposal"] is False,x; assert x["authority"]["preflight_grants_authority"] is False,x; assert x["authority"]["human_confirmation_required"] is True,x; assert "kol_observation_missing" in x["reasons"],x' <<<"$P0"
pass "fresh runtime is NOT_READY without bounded KoL observation"

section "4. Explicit evidence refreshes"
curl -fsS -X POST "$BASE/v1/release/refresh" >/dev/null || fail "release refresh failed"
curl -fsS "$BASE/v1/kingdomsitter/refresh" >/dev/null || fail "Kingdomsitter refresh failed"
P1="$(curl -fsS "$BASE/v1/preflight")"
echo "$P1" | python3 -m json.tool
python3 -c 'import json,sys; x=json.load(sys.stdin); assert x["status"]=="NOT_READY",x; assert "kol_observation_missing" in x["reasons"],x' <<<"$P1"
pass "dependency refresh alone did not substitute for KoL observation"

section "5. Prove preflight reads are side-effect free"
COUNT_BEFORE="$(audit_count)"
for _ in 1 2 3 4 5; do curl -fsS "$BASE/v1/preflight" >/dev/null; done
COUNT_AFTER="$(audit_count)"
[[ "$COUNT_BEFORE" == "$COUNT_AFTER" ]] || fail "audit count changed across preflight reads: $COUNT_BEFORE -> $COUNT_AFTER"
pass "five preflight reads appended no audit evidence"

section "6. Human KoL observation"
say "Run this exact command yourself in KoLmafia gCLI:"
say
say "  kol_actor sync"
say
if ! prompt_yes "Have you run 'kol_actor sync' and want the harness to verify READY?"; then exit 3; fi
READY="$(curl -fsS "$BASE/v1/preflight")"
echo "$READY" | python3 -m json.tool
python3 -c '
import json,sys
x=json.load(sys.stdin)
assert x["status"]=="READY",x
assert x["ready_for_proposal"] is True,x
assert x["runtime_id"].startswith("run-"),x
assert x["observation_id"].startswith("obs-"),x
a=x["authority"]
assert a["preflight_grants_authority"] is False,a
assert a["human_confirmation_required"] is True,a
assert a["live_kolmafia_mutation"] is False,a
bad=[c for c in x["checks"] if c.get("required") and not c.get("ok")]
assert not bad,bad
' <<<"$READY"
pass "fresh bounded evidence produces READY without granting authority"

section "7. Pending proposal must make preflight NOT_READY"
PF="$(mktemp)"
PCODE="$(curl -sS -o "$PF" -w '%{http_code}' -X POST "$BASE/v1/proposals/release-stage" -H 'content-type: application/json' -d '{}')"
PROPOSAL="$(cat "$PF")"; rm -f "$PF"
[[ "$PCODE" == "201" ]] || fail "proposal creation returned HTTP $PCODE: $PROPOSAL"
echo "$PROPOSAL" | python3 -m json.tool
PID="$(printf '%s' "$PROPOSAL" | json_get id)"
PENDING="$(curl -fsS "$BASE/v1/preflight")"
echo "$PENDING" | python3 -m json.tool
python3 -c 'import json,sys; x=json.load(sys.stdin); assert x["status"]=="NOT_READY",x; assert x["ready_for_proposal"] is False,x; assert "proposal_gate_not_idle" in x["reasons"],x' <<<"$PENDING"
pass "pending unconfirmed proposal closes proposal-formation readiness"
say "The harness will NOT confirm and will NOT successfully execute proposal $PID."

section "8. Restart only Actor Engine to clear ephemeral proposal"
if ! prompt_yes "Restart only Actor Engine now using the local setup stop/start scripts?"; then exit 8; fi
(
  cd "$SETUP_DIR"
  ./stop.sh
  ./start.sh
)
wait_health || fail "Actor Engine did not become healthy after restart"
POST="$(curl -fsS "$BASE/v1/preflight")"
python3 -c 'import json,sys; x=json.load(sys.stdin); assert x["status"]=="NOT_READY",x; assert "kol_observation_missing" in x["reasons"],x; assert "proposal_gate_not_idle" not in x["reasons"],x' <<<"$POST"
pass "restart cleared ephemeral proposal and observation; authority was not restored"

section "9. Re-establish read-only evidence"
curl -fsS -X POST "$BASE/v1/release/refresh" >/dev/null || fail "post-restart release refresh failed"
curl -fsS "$BASE/v1/kingdomsitter/refresh" >/dev/null || fail "post-restart Kingdomsitter refresh failed"
say "Run this exact command yourself in KoLmafia gCLI:"
say
say "  kol_actor sync"
say
if ! prompt_yes "Have you run post-restart 'kol_actor sync'?"; then exit 9; fi
FINAL="$(curl -fsS "$BASE/v1/preflight")"
echo "$FINAL" | python3 -m json.tool
python3 -c 'import json,sys; x=json.load(sys.stdin); assert x["status"]=="READY",x; assert x["ready_for_proposal"] is True,x; assert x["authority"]["preflight_grants_authority"] is False,x; assert x["authority"]["human_confirmation_required"] is True,x' <<<"$FINAL"
pass "post-restart evidence returns preflight to READY"

say
pass "LIVE_READ_ONLY_PREFLIGHT_TEST"
say "Preflight moved NOT_READY -> READY -> NOT_READY -> READY for the expected evidence/gate changes."
say "Repeated preflight reads appended no audit evidence."
say "The test proposal was never confirmed or successfully executed."
say "READY never granted execution authority."
