#!/usr/bin/env bash
set -euo pipefail

BASE="${BASE:-http://127.0.0.1:10424}"
PROJECT_ROOT="${PROJECT_ROOT:-$HOME/.kolmafia/kolmaf-ai}"
ACTOR_REPO="${ACTOR_REPO:-$PROJECT_ROOT/actor-engine}"
SETUP_DIR="${SETUP_DIR:-$PROJECT_ROOT/actor-engine-local-setup}"
REQUIRED_BASE_COMMIT="${REQUIRED_BASE_COMMIT:-4aa5cb1cd72d9c8356b242ce4b7b8b5c68eddd25}"
EXPECTED_VERSION="${EXPECTED_VERSION:-0.5.0}"

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

section "0. Baseline"
HEAD="$(git -C "$ACTOR_REPO" rev-parse HEAD)"
say "repo: $ACTOR_REPO"
say "HEAD: $HEAD"
if git -C "$ACTOR_REPO" merge-base --is-ancestor "$REQUIRED_BASE_COMMIT" "$HEAD" 2>/dev/null; then
  pass "verified v0.4.0 baseline is an ancestor"
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

section "2. Runtime"
HEALTH="$(curl -fsS "$BASE/health")" || fail "Actor Engine health unavailable"
echo "$HEALTH" | python3 -m json.tool
python3 -c 'import json,sys; x=json.load(sys.stdin); assert x["version"]==sys.argv[1],x; assert x["live_kolmafia_mutation"] is False,x; assert x["authority_restored_on_boot"] is False,x' "$EXPECTED_VERSION" <<<"$HEALTH"
pass "running sidecar reports $EXPECTED_VERSION"

section "3. NOT_READY cannot form a proposal"
P0="$(curl -fsS "$BASE/v1/preflight")"
echo "$P0" | python3 -m json.tool
python3 -c 'import json,sys; x=json.load(sys.stdin); assert x["status"]=="NOT_READY",x; assert x["ready_for_proposal"] is False,x' <<<"$P0"
COUNT0="$(audit_count)"
DF="$(mktemp)"
DCODE="$(curl -sS -o "$DF" -w '%{http_code}' -X POST "$BASE/v1/proposals/release-stage" -H 'content-type: application/json' -d '{}')"
DBODY="$(cat "$DF")"; rm -f "$DF"
say "HTTP $DCODE"
echo "$DBODY" | python3 -m json.tool
[[ "$DCODE" == "409" ]] || fail "NOT_READY proposal returned HTTP $DCODE, want 409"
python3 -c 'import json,sys; x=json.load(sys.stdin); assert x["error"]=="preflight_not_ready",x; assert x["preflight"]["status"]=="NOT_READY",x; assert x["preflight"]["ready_for_proposal"] is False,x' <<<"$DBODY"
COUNT1="$(audit_count)"
[[ "$COUNT0" == "$COUNT1" ]] || fail "admission denial changed audit count: $COUNT0 -> $COUNT1"
pass "NOT_READY preflight refused proposal formation without proposal audit evidence"

section "4. Refresh read-only dependencies"
curl -fsS -X POST "$BASE/v1/release/refresh" >/dev/null || fail "release refresh failed"
curl -fsS "$BASE/v1/kingdomsitter/refresh" >/dev/null || fail "Kingdomsitter refresh failed"
P1="$(curl -fsS "$BASE/v1/preflight")"
python3 -c 'import json,sys; x=json.load(sys.stdin); assert x["status"]=="NOT_READY",x; assert "kol_observation_missing" in x["reasons"],x' <<<"$P1"
pass "dependency refresh alone did not satisfy admission"

section "5. Human KoL observation"
say "Run this exact command yourself in KoLmafia gCLI:"
say
say "  kol_actor sync"
say
if ! prompt_yes "Have you run 'kol_actor sync' and want the harness to verify READY admission evidence?"; then exit 5; fi
READY="$(curl -fsS "$BASE/v1/preflight")"
echo "$READY" | python3 -m json.tool
python3 -c 'import json,sys; x=json.load(sys.stdin); assert x["status"]=="READY",x; assert x["ready_for_proposal"] is True,x; assert x["authority"]["preflight_grants_authority"] is False,x; assert x["authority"]["human_confirmation_required"] is True,x' <<<"$READY"
pass "preflight is READY and still grants no authority"

section "6. Create admitted proposal"
PF="$(mktemp)"
PCODE="$(curl -sS -o "$PF" -w '%{http_code}' -X POST "$BASE/v1/proposals/release-stage" -H 'content-type: application/json' -d '{}')"
PROPOSAL="$(cat "$PF")"; rm -f "$PF"
[[ "$PCODE" == "201" ]] || fail "admitted proposal returned HTTP $PCODE: $PROPOSAL"
echo "$PROPOSAL" | python3 -m json.tool
PID="$(printf '%s' "$PROPOSAL" | json_get id)"
ADMISSION_DIGEST="$(python3 -c 'import json,sys; print(json.load(sys.stdin)["admission"]["digest"])' <<<"$PROPOSAL")"
python3 -c '
import hashlib,json,sys
p=json.load(sys.stdin)
a=p["admission"]
assert a["version"]=="kol-actor/admission-v1",a
pf=a["preflight"]
assert pf["status"]=="READY" and pf["ready_for_proposal"] is True,pf
assert pf["runtime_id"]==p["runtime_id"],(pf,p)
assert pf["observation_id"]==p["observation_id"],(pf,p)
assert pf["authority"]["preflight_grants_authority"] is False,pf
assert pf["authority"]["human_confirmation_required"] is True,pf
canonical=json.dumps(pf,sort_keys=True,separators=(",",":"),ensure_ascii=False).encode()
want="sha256:"+hashlib.sha256(canonical).hexdigest()
assert a["digest"]==want,(a["digest"],want,canonical.decode())
assert p["requires_confirmation"] is True,p
' <<<"$PROPOSAL"
pass "proposal embeds independently verifiable READY admission evidence"

section "7. Audit carries the admission digest"
AUDIT="$(curl -fsS "$BASE/v1/audit/recent?limit=100")"
python3 -c '
import json,sys
x=json.load(sys.stdin); pid,digest=sys.argv[1:]
hits=[e for e in x["events"] if e.get("type")=="proposal.created" and e.get("proposal_id")==pid]
assert hits,hits
e=hits[-1]
assert e.get("admission_digest")==digest,e
' "$PID" "$ADMISSION_DIGEST" <<<"$AUDIT"
pass "proposal.created audit evidence carries the same admission digest"

section "8. Admission did not become confirmation"
PENDING="$(curl -fsS "$BASE/v1/preflight")"
python3 -c 'import json,sys; x=json.load(sys.stdin); assert x["status"]=="NOT_READY",x; assert "proposal_gate_not_idle" in x["reasons"],x' <<<"$PENDING"
pass "admitted proposal is pending and closes further proposal readiness"

say
say "Optional negative execution probe:"
say "  POST /v1/proposals/$PID/execute"
say "Expected result is HTTP 409 because the proposal has NOT been confirmed."
say "This targets only the existing local release.stage gate; if the gate were broken, it could create a local staged file."
if prompt_yes "Attempt the unconfirmed execute now to prove admission does not substitute for confirmation?"; then
  EF="$(mktemp)"
  ECODE="$(curl -sS -o "$EF" -w '%{http_code}' -X POST "$BASE/v1/proposals/$PID/execute")"
  EBODY="$(cat "$EF")"; rm -f "$EF"
  say "HTTP $ECODE"
  say "$EBODY"
  [[ "$ECODE" == "409" ]] || fail "unconfirmed execute returned HTTP $ECODE, want 409"
  grep -qi 'not confirmed' <<<"$EBODY" || fail "unconfirmed execute was not rejected as not confirmed"
  pass "valid admission did not substitute for human confirmation"
else
  warn "negative execution probe skipped by human; admission/confirmation separation remains covered by static tests"
fi

section "9. Restart Actor Engine and verify ephemeral authority remains absent"
if ! prompt_yes "Restart only Actor Engine now using the local setup stop/start scripts?"; then exit 9; fi
(
  cd "$SETUP_DIR"
  ./stop.sh
  ./start.sh
)
wait_health || fail "Actor Engine did not become healthy after restart"
RF="$(mktemp)"
RCODE="$(curl -sS -o "$RF" -w '%{http_code}' -X POST "$BASE/v1/proposals/$PID/execute")"
RBODY="$(cat "$RF")"; rm -f "$RF"
[[ "$RCODE" == "409" ]] || fail "old proposal execute after restart returned HTTP $RCODE, want 409"
grep -qi 'proposal not found' <<<"$RBODY" || fail "old proposal authority survived restart"
pass "admitted proposal authority did not survive restart"

AUDIT2="$(curl -fsS "$BASE/v1/audit/recent?limit=100")"
python3 -c '
import json,sys
x=json.load(sys.stdin); pid,digest=sys.argv[1:]
hits=[e for e in x["events"] if e.get("type")=="proposal.created" and e.get("proposal_id")==pid]
assert hits and hits[-1].get("admission_digest")==digest,hits
' "$PID" "$ADMISSION_DIGEST" <<<"$AUDIT2"
pass "historical admission evidence survived restart without restoring authority"

say
pass "LIVE_PROPOSAL_ADMISSION_BINDING_TEST"
say "NOT_READY preflight could not form a proposal."
say "READY proposal admission carried an independently verified canonical digest and exact checkset."
say "Admission evidence propagated into durable audit history."
say "The test proposal was never confirmed or successfully executed."
say "Admission did not restore authority after restart."
