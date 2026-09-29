#!/usr/bin/env bash
set -euo pipefail

# Actor Engine v0.2 live ledger verification
# Safety posture:
# - read-only HTTP checks unless explicitly creating/confirming a release.stage proposal
# - NEVER executes the release.stage proposal in this test
# - gCLI steps are manual and require explicit y/N confirmation
# - restart is deliberate to prove evidence survives while authority does not

BASE="${BASE:-http://127.0.0.1:10424}"
PROJECT_ROOT="${PROJECT_ROOT:-$HOME/.kolmafia/kolmaf-ai}"
ACTOR_REPO="${ACTOR_REPO:-$PROJECT_ROOT/actor-engine}"
SETUP_DIR="${SETUP_DIR:-$PROJECT_ROOT/actor-engine-local-setup}"
RUNTIME_DIR="${RUNTIME_DIR:-$PROJECT_ROOT/actor-engine-runtime}"
LEDGER="${LEDGER:-$RUNTIME_DIR/audit.jsonl}"
REQUIRED_IMPLEMENTATION_COMMIT="${REQUIRED_IMPLEMENTATION_COMMIT:-5c97d185987b58403f643165f83eb321dde53aa8}"

RED=$'\e[31m'; GREEN=$'\e[32m'; YELLOW=$'\e[33m'; CYAN=$'\e[36m'; RESET=$'\e[0m'

say(){ printf '%s\n' "$*"; }
info(){ printf '%s%s%s\n' "$CYAN" "$*" "$RESET"; }
pass(){ printf '%sPASS%s %s\n' "$GREEN" "$RESET" "$*"; }
warn(){ printf '%sWARN%s %s\n' "$YELLOW" "$RESET" "$*"; }
fail(){ printf '%sFAIL%s %s\n' "$RED" "$RESET" "$*" >&2; exit 1; }

need(){ command -v "$1" >/dev/null 2>&1 || fail "missing command: $1"; }
need curl
need python3
need git
need sha256sum
need go

prompt_yes(){
  local q="$1" ans
  printf '%s%s%s [y/N]: ' "$YELLOW" "$q" "$RESET"
  read -r ans || true
  case "${ans:-}" in y|Y|yes|YES|Yes) return 0;; *) return 1;; esac
}

json_get(){
  local key="$1"
  python3 -c 'import json,sys; print(json.load(sys.stdin)[sys.argv[1]])' "$key"
}

http_json(){
  local method="$1" url="$2" body="${3:-}"
  local tmp code
  tmp="$(mktemp)"
  if [[ -n "$body" ]]; then
    code="$(curl -sS -o "$tmp" -w '%{http_code}' -X "$method" "$url" -H 'content-type: application/json' -d "$body")"
  else
    code="$(curl -sS -o "$tmp" -w '%{http_code}' -X "$method" "$url")"
  fi
  printf '%s\n' "$code"
  cat "$tmp"
  rm -f "$tmp"
}

section(){ say; say "========== $* =========="; }

section "0. Preflight"
[[ -d "$ACTOR_REPO/.git" ]] || fail "Actor Engine repo not found: $ACTOR_REPO"
[[ -x "$SETUP_DIR/start.sh" && -x "$SETUP_DIR/stop.sh" ]] || fail "setup helper missing start/stop scripts: $SETUP_DIR"

CURRENT_HEAD="$(git -C "$ACTOR_REPO" rev-parse HEAD)"
say "repo: $ACTOR_REPO"
say "HEAD: $CURRENT_HEAD"
if git -C "$ACTOR_REPO" merge-base --is-ancestor "$REQUIRED_IMPLEMENTATION_COMMIT" "$CURRENT_HEAD" 2>/dev/null; then
  pass "verified v0.2 audit-ledger implementation is present in this checkout"
else
  warn "required implementation commit is not an ancestor: $REQUIRED_IMPLEMENTATION_COMMIT"
  if ! prompt_yes "Continue against a checkout that does not contain the verified implementation commit?"; then exit 2; fi
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

section "2. Runtime health"
HEALTH="$(curl -fsS "$BASE/health")" || fail "Actor Engine health endpoint unavailable at $BASE"
echo "$HEALTH" | python3 -m json.tool
python3 -c 'import json,sys; x=json.load(sys.stdin); assert x.get("ok") is True, x; assert x.get("live_kolmafia_mutation") is False, x; assert x.get("evidence_persistence") == "hash-chained-jsonl", x; assert x.get("authority_restored_on_boot") is False, x; assert x.get("version") == "0.2.0", x' <<<"$HEALTH"
pass "health advertises durable evidence and non-restored authority"

section "3. Existing audit evidence"
AUDIT0="$(curl -fsS "$BASE/v1/audit/recent?limit=20")" || fail "audit endpoint unavailable"
echo "$AUDIT0" | python3 -m json.tool
python3 -c 'import json,sys; x=json.load(sys.stdin); assert x.get("authority_restored_from_audit") is False, x; ledger=x.get("ledger") or {}; assert ledger.get("verified" is True, x' <<<"$AUDIT0"
pass "audit ledger opens as verified and does not restore authority"

section "4. Refresh release metadata"
RELEASE="$(curl -fsS -X POST "$BASE/v1/release/refresh")" || fail "release refresh failed"
echo "$RELEASE" | python3 -m json.tool
pass "release metadata refreshed"

section "5. Optional KoLmafia observation verification"
say "For the restart/evidence test, a fresh KoL observation is useful but not mutation authority."
say "Recommended gCLI command:  kol_actor sync"
if prompt_yes "Have you run 'kol_actor sync' in KoLmafia gCLI and want me to verify the observed state now?"; then
  STATE="$(curl -fsS "$BASE/v1/state")" || fail "state endpoint unavailable"
  echo "$STATE" | python3 -m json.tool
  python3 -c 'import json,sys; x=json.load(sys.stdin); assert "kol_state" in x and x["kol_state"], x' <<<"$STATE"
  pass "KoL observation is present"
else
  warn "continuing without manual gCLI observation verification"
fi

section "6. Create proposal"
PFILE="$(mktemp)"
PCODE="$(curl -sS -o "$PFILE" -w '%{http_code}' -X POST "$BASE/v1/proposals/release-stage" -H 'content-type: application/json' -d '{}')"
cat "$PFILE"; say
[[ "$PCODE" == "201" ]] || { rm -f "$PFILE"; fail "proposal creation returned HTTP $PCODE"; }
PROPOSAL="$(cat "$PFILE")"; rm -f "$PFILE"
echo "$PROPOSAL" | python3 -m json.tool
PID="$(printf '%s' "$PROPOSAL" | json_get id)"
DIGEST="$(printf '%s' "$PROPOSAL" | json_get state_digest)"
say "proposal=$PID"
say "digest=$DIGEST"

section "7. Human confirmation gate"
say "This confirms only the current release.stage proposal."
say "This test will NOT execute it."
if ! prompt_yes "Confirm proposal $PID for the restart-authority test?"; then
  warn "test stopped before confirmation"
  exit 3
fi
CONFIRM="$(curl -fsS -X POST "$BASE/v1/proposals/$PID/confirm" -H 'content-type: application/json' -d "{\"state_digest\":\"$DIGEST\",\"confirmed_by\":\"human-live-ledger-test\"}")" || fail "confirmation failed"
echo "$CONFIRM" | python3 -m json.tool
pass "proposal confirmed in current process"

section "8. Verify durable evidence before restart"
AUDIT1="$(curl -fsS "$BASE/v1/audit/recent?limit=30")" || fail "audit endpoint unavailable"
echo "$AUDIT1" | python3 -m json.tool
python3 -c 'import json,sys; pid=sys.argv[1]; x=json.load(sys.stdin); e=[r for r in x.get("events",[]) if r.get("proposal_id")==pid]; t=[r.get("type") for r in e]; assert "proposal.created" in t, t; assert "proposal.confirmed" in t, t' "$PID" <<<"$AUDIT1"
pass "proposal.created and proposal.confirmed are durable evidence"

section "9. Restart Actor Engine"
say "The restart is the core test: evidence should persist; permission should disappear."
if ! prompt_yes "Restart Actor Engine now using the local setup stop/start scripts?"; then
  warn "test stopped before restart"
  exit 4
fi
(
  cd "$SETUP_DIR"
  ./stop.sh
  ./start.sh
)

for _ in {1..30}; do
  if curl -fsS "$BASE/health" >/dev/null 2>&1; then break; fi
  sleep 1
done
curl -fsS "$BASE/health" >/dev/null || fail "Actor Engine did not become healthy after restart"
pass "Actor Engine restarted"

section "10. Verify evidence survived restart"
AUDIT2="$(curl -fsS "$BASE/v1/audit/recent?limit=40")" || fail "audit endpoint unavailable after restart"
echo "$AUDIT2" | python3 -m json.tool
python3 -c 'import json,sys; pid=sys.argv[1]; x=json.load(sys.stdin); e=x.get("events",[]); t=[r.get("type") for r in e if r.get("proposal_id")==pid]; assert "proposal.created" in t, t; assert "proposal.confirmed" in t, t; runtime=[r for r in e if r.get("type")=="runtime.started"]; assert len(runtime)>=1, runtime; assert x.get("authority_restored_from_audit") is False, x' "$PID" <<<"$AUDIT2"
pass "historical evidence survived restart"

section "11. Prove authority did NOT survive"
EFILE="$(mktemp)"
ECODE="$(curl -sS -o "$EFILE" -w '%{http_code}' -X POST "$BASE/v1/proposals/$PID/execute")"
EBODY="$(cat "$EFILE")"; rm -f "$EFILE"
say "HTTP $ECODE"
say "$EBODY"
[[ "$ECODE" == "409" ]] || fail "old proposal execute returned HTTP $ECODE, want 409"
grep -qi 'proposal not found' <<<"$EBODY" || fail "old proposal was not rejected as 'proposal not found'"
pass "old confirmation authority was not restored"

section "12. Verify denial became new evidence"
AUDIT3="$(curl -fsS "$BASE/v1/audit/recent?limit=50")" || fail "audit endpoint unavailable"
echo "$AUDIT3" | python3 -m json.tool
python3 -c 'import json,sys; pid=sys.argv[1]; x=json.load(sys.stdin); e=[r for r in x.get("events",[]) if r.get("proposal_id")==pid]; t=[r.get("type") for r in e]; assert "proposal.created" in t, t; assert "proposal.confirmed" in t, t; assert "execution.denied" in t, t' "$PID" <<<"$AUDIT3"
pass "ledger contains historical confirmation plus post-restart execution denial"

section "13. Final safety checks"
if [[ -e "$LEDGER" ]]; then
  say "ledger: $LEDGER"
  tail -n 8 "$LEDGER" || true
else
  warn "expected ledger path not found at $LEDGER; custom -audit_log may be in use"
fi

say
pass "LIVE_LEDGER_RESTART_AUTHORITY_TEST"
say "Evidence survived restart. Proposal/confirmation authority did not."
say "No release.stage proposal was executed by this test."
