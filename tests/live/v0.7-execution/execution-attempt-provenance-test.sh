#!/usr/bin/env bash
set -euo pipefail

BASE="${BASE:-http://127.0.0.1:10424}"
PROJECT_ROOT="${PROJECT_ROOT:-$HOME/.kolmafia/kolmaf-ai}"
ACTOR_REPO="${ACTOR_REPO:-$PROJECT_ROOT/actor-engine}"
SETUP_DIR="${SETUP_DIR:-$PROJECT_ROOT/actor-engine-local-setup}"
REQUIRED_BASE_COMMIT="${REQUIRED_BASE_COMMIT:-f749ba14049b23bf82a4085f6727737758406f9f}"
EXPECTED_VERSION="${EXPECTED_VERSION:-0.7.0}"
CONFIRMED_BY="${CONFIRMED_BY:-doncannoli-v07-live-test}"

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
git -C "$ACTOR_REPO" merge-base --is-ancestor "$REQUIRED_BASE_COMMIT" "$HEAD" || fail "verified v0.6.0 baseline is not an ancestor"
pass "verified v0.6.0 baseline is an ancestor"

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
python3 -c 'import json,sys; x=json.load(sys.stdin); assert x["version"]==sys.argv[1],x; assert x["live_kolmafia_mutation"] is False,x' "$EXPECTED_VERSION" <<<"$HEALTH"
RUNTIME_ID="$(printf '%s' "$HEALTH" | json_get runtime_id)"
pass "running sidecar reports $EXPECTED_VERSION"

section "3. Fresh evidence and READY"
curl -fsS -X POST "$BASE/v1/release/refresh" >/dev/null || fail "release refresh failed"
curl -fsS "$BASE/v1/kingdomsitter/refresh" >/dev/null || fail "Kingdomsitter refresh failed"
say "Run this exact command yourself in KoLmafia gCLI:"
say
say "  kol_actor sync"
say
if ! prompt_yes "Have you run 'kol_actor sync' and want the harness to verify READY?"; then exit 3; fi
PF="$(curl -fsS "$BASE/v1/preflight")"
python3 -c 'import json,sys; x=json.load(sys.stdin); assert x["status"]=="READY",x; assert x["ready_for_proposal"] is True,x' <<<"$PF"
pass "preflight is READY"

section "4. Create proposal and confirm"
TMP="$(mktemp)"
PCODE="$(curl -sS -o "$TMP" -w '%{http_code}' -X POST "$BASE/v1/proposals/release-stage" -H 'content-type: application/json' -d '{}')"
PROPOSAL="$(cat "$TMP")"; rm -f "$TMP"
[[ "$PCODE" == "201" ]] || fail "proposal creation HTTP $PCODE: $PROPOSAL"
echo "$PROPOSAL" | python3 -m json.tool
PID="$(printf '%s' "$PROPOSAL" | json_get id)"
PROPOSAL_STATE="$(python3 -c 'import json,sys; print(json.load(sys.stdin)["state_digest"])' <<<"$PROPOSAL")"
PROPOSAL_OBS="$(python3 -c 'import json,sys; print(json.load(sys.stdin)["observation_id"])' <<<"$PROPOSAL")"
ADMISSION_DIGEST="$(python3 -c 'import json,sys; print(json.load(sys.stdin)["admission"]["digest"])' <<<"$PROPOSAL")"

say "The next request confirms proposal $PID, but the harness will deliberately invalidate it before execution."
if ! prompt_yes "Confirm this proposal as '$CONFIRMED_BY' now?"; then exit 4; fi
BODY="$(python3 -c 'import json,sys; print(json.dumps({"state_digest":sys.argv[1],"confirmed_by":sys.argv[2]}))' "$PROPOSAL_STATE" "$CONFIRMED_BY")"
TMP="$(mktemp)"
CCODE="$(curl -sS -o "$TMP" -w '%{http_code}' -X POST "$BASE/v1/proposals/$PID/confirm" -H 'content-type: application/json' -d "$BODY")"
CONFIRM="$(cat "$TMP")"; rm -f "$TMP"
[[ "$CCODE" == "200" ]] || fail "confirmation HTTP $CCODE: $CONFIRM"
echo "$CONFIRM" | python3 -m json.tool
CONFIRM_DIGEST="$(python3 -c 'import json,sys; print(json.load(sys.stdin)["confirmation"]["digest"])' <<<"$CONFIRM")"
pass "proposal confirmed with first-class confirmation evidence"

section "5. Deliberately change bounded observation"
say "'kol_actor turn' in this shim publishes event=after-adventure; it does NOT spend an adventure."
say "Run:"
say
say "  kol_actor turn"
say
if ! prompt_yes "Have you run 'kol_actor turn' and want the harness to verify the observation changed?"; then exit 5; fi
STATE="$(curl -fsS "$BASE/v1/state")"
CURRENT_OBS="$(printf '%s' "$STATE" | json_get observation_id)"
[[ -n "$CURRENT_OBS" && "$CURRENT_OBS" != "$PROPOSAL_OBS" ]] || fail "observation_id did not change; refusing execute probe"
pass "observation identity changed before execute probe"

section "6. Stale-state execution probe"
say "The next request calls the existing execute endpoint on the now-stale confirmed proposal."
say "Expected result: HTTP 409 approval invalidated BEFORE release.stage starts."
say "If that invariant were broken, this endpoint could stage a local release artifact."
if ! prompt_yes "Attempt the stale confirmed proposal now?"; then exit 6; fi
TMP="$(mktemp)"
ECODE="$(curl -sS -o "$TMP" -w '%{http_code}' -X POST "$BASE/v1/proposals/$PID/execute")"
EBODY="$(cat "$TMP")"; rm -f "$TMP"
say "HTTP $ECODE"
say "$EBODY"
[[ "$ECODE" == "409" ]] || fail "stale execute returned HTTP $ECODE, want 409"
grep -qi 'approval invalidated' <<<"$EBODY" || fail "stale execute did not report approval invalidated"
pass "stale execution was denied before operation start"

section "7. Independently verify execution-attempt provenance"
AUDIT="$(curl -fsS "$BASE/v1/audit/recent?limit=100")"
EXEC_DIGEST="$(python3 -c '
import json,sys
x=json.load(sys.stdin); pid=sys.argv[1]
hits=[e for e in x["events"] if e.get("proposal_id")==pid and e.get("type") in ("proposal.invalidated","execution.denied") and e.get("execution")]
assert hits,hits
print(hits[-1]["execution_digest"])
' "$PID" <<<"$AUDIT")"
python3 -c '
import hashlib,json,sys
x=json.load(sys.stdin)
pid,proposal_state,admission_digest,confirmation_digest,runtime_id,expected_digest=sys.argv[1:]
hits=[e for e in x["events"] if e.get("proposal_id")==pid and e.get("type") in ("proposal.invalidated","execution.denied") and e.get("execution")]
assert hits,hits
event=hits[-1]
e=event["execution"]
assert e["version"]=="kol-actor/execution-attempt-v1",e
assert e["proposal_id"]==pid,e
assert e["operation"]=="release.stage",e
assert e["proposal_state_digest"]==proposal_state,e
assert e["admission_digest"]==admission_digest,e
assert e["confirmation_digest"]==confirmation_digest,e
assert e["origin_runtime_id"]==runtime_id,e
assert e["execution_runtime_id"]==runtime_id,e
assert e["current_state_digest"]!=proposal_state,e
assert e["gate_decision"]=="denied",e
assert e["evidence_grants_authority"] is False,e
payload={k:v for k,v in e.items() if k!="digest"}
canonical=json.dumps(payload,sort_keys=True,separators=(",",":"),ensure_ascii=False).encode()
want="sha256:"+hashlib.sha256(canonical).hexdigest()
assert e["digest"]==want,(e["digest"],want,canonical.decode())
assert event["execution_digest"]==want,event
started=[z for z in x["events"] if z.get("proposal_id")==pid and z.get("type") in ("execution.started","execution.succeeded")]
assert not started,started
' "$PID" "$PROPOSAL_STATE" "$ADMISSION_DIGEST" "$CONFIRM_DIGEST" "$RUNTIME_ID" "$EXEC_DIGEST" <<<"$AUDIT"
pass "denied execution attempt carries independently verifiable chained provenance"

section "8. Approval was burned"
TMP="$(mktemp)"
RCODE="$(curl -sS -o "$TMP" -w '%{http_code}' -X POST "$BASE/v1/proposals/$PID/execute")"
RBODY="$(cat "$TMP")"; rm -f "$TMP"
[[ "$RCODE" == "409" ]] || fail "retry returned HTTP $RCODE, want 409"
grep -qi 'proposal not found' <<<"$RBODY" || fail "retry did not report proposal not found"
pass "stale-state denial burned proposal authority"

section "9. Optional restart durability proof"
if prompt_yes "Restart only Actor Engine and verify execution-attempt evidence survives as history?"; then
  (
    cd "$SETUP_DIR"
    ./stop.sh
    ./start.sh
  )
  wait_health || fail "Actor Engine did not become healthy"
  AUDIT2="$(curl -fsS "$BASE/v1/audit/recent?limit=100")"
  python3 -c '
import json,sys
x=json.load(sys.stdin); pid,digest=sys.argv[1:]
assert x["authority_restored_from_audit"] is False,x
hits=[e for e in x["events"] if e.get("proposal_id")==pid and e.get("execution_digest")==digest]
assert hits,hits
success=[e for e in x["events"] if e.get("proposal_id")==pid and e.get("type")=="execution.succeeded"]
assert not success,success
' "$PID" "$EXEC_DIGEST" <<<"$AUDIT2"
  pass "execution-attempt evidence survived restart without restoring authority"
else
  warn "restart durability proof skipped; ledger durability remains covered by unit and prior live checkpoints"
fi

say
pass "LIVE_EXECUTION_ATTEMPT_PROVENANCE_TEST"
say "A stale confirmed proposal was denied before operation start."
say "The denial carried independently verifiable execution-attempt provenance."
say "No execution.started or execution.succeeded record existed for the test proposal."
say "The denial burned proposal authority; retry returned proposal not found."
