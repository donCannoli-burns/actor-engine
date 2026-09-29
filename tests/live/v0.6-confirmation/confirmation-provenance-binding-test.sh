#!/usr/bin/env bash
set -euo pipefail

BASE="${BASE:-http://127.0.0.1:10424}"
PROJECT_ROOT="${PROJECT_ROOT:-$HOME/.kolmafia/kolmaf-ai}"
ACTOR_REPO="${ACTOR_REPO:-$PROJECT_ROOT/actor-engine}"
SETUP_DIR="${SETUP_DIR:-$PROJECT_ROOT/actor-engine-local-setup}"
REQUIRED_BASE_COMMIT="${REQUIRED_BASE_COMMIT:-4b3f7c3e06d2d2c10e57edadfdde1c6b2446b495}"
EXPECTED_VERSION="${EXPECTED_VERSION:-0.6.0-dev}"
CONFIRMED_BY="${CONFIRMED_BY:-doncannoli-v06-live-test}"

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
git -C "$ACTOR_REPO" merge-base --is-ancestor "$REQUIRED_BASE_COMMIT" "$HEAD" || fail "verified v0.5.0 baseline is not an ancestor"
pass "verified v0.5.0 baseline is an ancestor"

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
python3 -c 'import json,sys; x=json.load(sys.stdin); assert x["version"]==sys.argv[1],x; assert x["authority_restored_on_boot"] is False,x; assert x["live_kolmafia_mutation"] is False,x' "$EXPECTED_VERSION" <<<"$HEALTH"
RUNTIME_BEFORE="$(printf '%s' "$HEALTH" | json_get runtime_id)"
pass "running sidecar reports $EXPECTED_VERSION"

section "3. Establish fresh read-only evidence"
curl -fsS -X POST "$BASE/v1/release/refresh" >/dev/null || fail "release refresh failed"
curl -fsS "$BASE/v1/kingdomsitter/refresh" >/dev/null || fail "Kingdomsitter refresh failed"
say "Run this exact command yourself in KoLmafia gCLI:"
say
say "  kol_actor sync"
say
if ! prompt_yes "Have you run 'kol_actor sync' and want the harness to verify READY?"; then exit 3; fi
PREFLIGHT="$(curl -fsS "$BASE/v1/preflight")"
echo "$PREFLIGHT" | python3 -m json.tool
python3 -c 'import json,sys; x=json.load(sys.stdin); assert x["status"]=="READY",x; assert x["ready_for_proposal"] is True,x; assert x["authority"]["preflight_grants_authority"] is False,x' <<<"$PREFLIGHT"
pass "fresh bounded evidence is READY without granting authority"

section "4. Create admitted proposal"
PF="$(mktemp)"
PCODE="$(curl -sS -o "$PF" -w '%{http_code}' -X POST "$BASE/v1/proposals/release-stage" -H 'content-type: application/json' -d '{}')"
PROPOSAL="$(cat "$PF")"; rm -f "$PF"
[[ "$PCODE" == "201" ]] || fail "proposal creation returned HTTP $PCODE: $PROPOSAL"
echo "$PROPOSAL" | python3 -m json.tool
PID="$(printf '%s' "$PROPOSAL" | json_get id)"
STATE_DIGEST="$(python3 -c 'import json,sys; print(json.load(sys.stdin)["state_digest"])' <<<"$PROPOSAL")"
ADMISSION_DIGEST="$(python3 -c 'import json,sys; print(json.load(sys.stdin)["admission"]["digest"])' <<<"$PROPOSAL")"
PROPOSAL_RUNTIME="$(python3 -c 'import json,sys; print(json.load(sys.stdin)["runtime_id"])' <<<"$PROPOSAL")"
[[ "$PROPOSAL_RUNTIME" == "$RUNTIME_BEFORE" ]] || fail "proposal runtime $PROPOSAL_RUNTIME != current runtime $RUNTIME_BEFORE"
pass "proposal is bound to current runtime and admission evidence"

section "5. Human confirmation boundary"
say "The next request confirms proposal $PID for LOCAL release staging."
say "The harness will NOT execute the confirmed proposal while this approval is live."
say "After evidence verification, it will restart only Actor Engine to destroy the approval."
say
if ! prompt_yes "Confirm this one proposal as '$CONFIRMED_BY' now?"; then exit 5; fi
BODY="$(python3 -c 'import json,sys; print(json.dumps({"state_digest":sys.argv[1],"confirmed_by":sys.argv[2]}))' "$STATE_DIGEST" "$CONFIRMED_BY")"
CF="$(mktemp)"
CCODE="$(curl -sS -o "$CF" -w '%{http_code}' -X POST "$BASE/v1/proposals/$PID/confirm" -H 'content-type: application/json' -d "$BODY")"
CONFIRM_RESPONSE="$(cat "$CF")"; rm -f "$CF"
[[ "$CCODE" == "200" ]] || fail "confirmation returned HTTP $CCODE: $CONFIRM_RESPONSE"
echo "$CONFIRM_RESPONSE" | python3 -m json.tool

CONFIRM_DIGEST="$(python3 -c 'import json,sys; print(json.load(sys.stdin)["confirmation"]["digest"])' <<<"$CONFIRM_RESPONSE")"
python3 -c '
import hashlib,json,sys
x=json.load(sys.stdin)
p=x["proposal"]
e=x["confirmation"]
confirmed_by=sys.argv[1]
assert x["confirmed"] is True,x
assert e["version"]=="kol-actor/confirmation-v1",e
assert e["proposal_id"]==p["id"],(e,p)
assert e["state_digest"]==p["state_digest"],(e,p)
assert e["admission_digest"]==p["admission"]["digest"],(e,p)
assert e["confirmed_by"]==confirmed_by,e
assert e["runtime_id"]==p["runtime_id"],(e,p)
assert e["confirmed_at"],e
assert e["evidence_grants_authority"] is False,e
assert e["authority_restorable"] is False,e
payload={k:v for k,v in e.items() if k!="digest"}
canonical=json.dumps(payload,sort_keys=True,separators=(",",":"),ensure_ascii=False).encode()
want="sha256:"+hashlib.sha256(canonical).hexdigest()
assert e["digest"]==want,(e["digest"],want,canonical.decode())
' "$CONFIRMED_BY" <<<"$CONFIRM_RESPONSE"
pass "confirmation provenance digest independently recomputed"

section "6. Durable confirmation evidence"
AUDIT="$(curl -fsS "$BASE/v1/audit/recent?limit=100")"
python3 -c '
import json,sys
x=json.load(sys.stdin); pid,digest=sys.argv[1:]
assert x["authority_restored_from_audit"] is False,x
hits=[e for e in x["events"] if e.get("type")=="proposal.confirmed" and e.get("proposal_id")==pid]
assert hits,hits
event=hits[-1]
assert event.get("confirmation_digest")==digest,event
ce=event.get("confirmation")
assert ce and ce.get("digest")==digest,event
assert ce.get("evidence_grants_authority") is False,ce
assert ce.get("authority_restorable") is False,ce
started=[e for e in x["events"] if e.get("proposal_id")==pid and e.get("type") in ("execution.started","execution.succeeded")]
assert not started,started
' "$PID" "$CONFIRM_DIGEST" <<<"$AUDIT"
pass "full confirmation evidence is durable and no execution started"

STATE="$(curl -fsS "$BASE/v1/state")"
python3 -c 'import json,sys; x=json.load(sys.stdin); assert "proposal_ready" in x["active_states"],x; assert x["pending_proposal_id"]==sys.argv[1],x' "$PID" <<<"$STATE"
pass "confirmation created only an in-memory ready approval"

section "7. Restart only Actor Engine"
if ! prompt_yes "Restart only Actor Engine now to destroy the live approval?"; then exit 7; fi
(
  cd "$SETUP_DIR"
  ./stop.sh
  ./start.sh
)
wait_health || fail "Actor Engine did not become healthy after restart"
HEALTH2="$(curl -fsS "$BASE/health")"
echo "$HEALTH2" | python3 -m json.tool
RUNTIME_AFTER="$(printf '%s' "$HEALTH2" | json_get runtime_id)"
[[ "$RUNTIME_AFTER" != "$RUNTIME_BEFORE" ]] || fail "runtime_id did not change across restart"
python3 -c 'import json,sys; x=json.load(sys.stdin); assert x["authority_restored_on_boot"] is False,x' <<<"$HEALTH2"
pass "restart created a new runtime with no restored authority"

section "8. Durable evidence cannot rehydrate permission"
EF="$(mktemp)"
ECODE="$(curl -sS -o "$EF" -w '%{http_code}' -X POST "$BASE/v1/proposals/$PID/execute")"
EBODY="$(cat "$EF")"; rm -f "$EF"
say "HTTP $ECODE"
say "$EBODY"
[[ "$ECODE" == "409" ]] || fail "old proposal execute returned HTTP $ECODE, want 409"
grep -qi 'proposal not found' <<<"$EBODY" || fail "durable confirmation evidence restored authority unexpectedly"
pass "old confirmed proposal is not executable after restart"

AUDIT2="$(curl -fsS "$BASE/v1/audit/recent?limit=100")"
python3 -c '
import hashlib,json,sys
x=json.load(sys.stdin); pid,digest=sys.argv[1:]
assert x["authority_restored_from_audit"] is False,x
hits=[e for e in x["events"] if e.get("type")=="proposal.confirmed" and e.get("proposal_id")==pid]
assert hits,hits
ce=hits[-1].get("confirmation")
assert ce and ce.get("digest")==digest,ce
payload={k:v for k,v in ce.items() if k!="digest"}
canonical=json.dumps(payload,sort_keys=True,separators=(",",":"),ensure_ascii=False).encode()
assert "sha256:"+hashlib.sha256(canonical).hexdigest()==digest,(ce,canonical.decode())
success=[e for e in x["events"] if e.get("proposal_id")==pid and e.get("type")=="execution.succeeded"]
assert not success,success
' "$PID" "$CONFIRM_DIGEST" <<<"$AUDIT2"
pass "confirmation evidence survived restart as valid history only"

say
pass "LIVE_CONFIRMATION_PROVENANCE_BINDING_TEST"
say "Human confirmation produced independently verifiable first-class evidence."
say "Confirmation evidence bound proposal, state, admission, human, runtime, and timestamp."
say "The confirmed proposal was never executed while approval was live."
say "Durable confirmation evidence survived restart but did not restore permission."
