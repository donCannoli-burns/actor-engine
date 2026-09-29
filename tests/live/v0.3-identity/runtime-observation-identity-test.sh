#!/usr/bin/env bash
set -euo pipefail

BASE="${BASE:-http://127.0.0.1:10424}"
PROJECT_ROOT="${PROJECT_ROOT:-$HOME/.kolmafia/kolmaf-ai}"
ACTOR_REPO="${ACTOR_REPO:-$PROJECT_ROOT/actor-engine}"
SETUP_DIR="${SETUP_DIR:-$PROJECT_ROOT/actor-engine-local-setup}"
REQUIRED_BASE_COMMIT="${REQUIRED_BASE_COMMIT:-a6b1ee03be11fe716b2a30059e15701865a12d88}"
EXPECTED_VERSION="${EXPECTED_VERSION:-0.3.0}"

RED=$'\e[31m'; GREEN=$'\e[32m'; YELLOW=$'\e[33m'; CYAN=$'\e[36m'; RESET=$'\e[0m'
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
observation_digest(){
python3 -c '
import hashlib,json,struct,sys
x=json.load(sys.stdin)
state=x.get("kol_state") or {}
h=hashlib.sha256()
for k in sorted(state):
    kb=k.encode("utf-8"); vb=str(state[k]).encode("utf-8")
    h.update(struct.pack(">Q",len(kb))); h.update(kb)
    h.update(struct.pack(">Q",len(vb))); h.update(vb)
print("obs-"+h.hexdigest())
'
}

for c in curl python3 git sha256sum go grep; do need "$c"; done
[[ -d "$ACTOR_REPO/.git" ]] || fail "Actor Engine repo not found: $ACTOR_REPO"
[[ -x "$SETUP_DIR/start.sh" && -x "$SETUP_DIR/stop.sh" ]] || fail "setup start/stop scripts not found: $SETUP_DIR"

section "0. Preflight"
HEAD="$(git -C "$ACTOR_REPO" rev-parse HEAD)"
say "repo: $ACTOR_REPO"
say "HEAD: $HEAD"
if git -C "$ACTOR_REPO" merge-base --is-ancestor "$REQUIRED_BASE_COMMIT" "$HEAD" 2>/dev/null; then
  pass "verified v0.2.0 baseline is an ancestor"
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

section "2. Runtime identity"
HEALTH1="$(curl -fsS "$BASE/health")" || fail "Actor Engine health unavailable"
echo "$HEALTH1" | python3 -m json.tool
RUNTIME1="$(printf '%s' "$HEALTH1" | json_get runtime_id)"
python3 -c 'import json,re,sys; x=json.load(sys.stdin); assert x["version"]==sys.argv[1],x; assert x["live_kolmafia_mutation"] is False,x; assert re.fullmatch(r"run-[0-9a-f]{32}",x["runtime_id"]),x' "$EXPECTED_VERSION" <<<"$HEALTH1"
STATE0="$(curl -fsS "$BASE/v1/state")"
python3 -c 'import json,sys; x=json.load(sys.stdin); assert x["runtime_id"]==sys.argv[1],x' "$RUNTIME1" <<<"$STATE0"
pass "runtime_id is well formed and consistent within the process"

section "3. First manual KoL observation"
say "Run this exact command yourself in KoLmafia gCLI:"
say
say "  kol_actor sync"
say
if ! prompt_yes "Have you run 'kol_actor sync' and want the harness to verify it?"; then exit 3; fi
S1="$(curl -fsS "$BASE/v1/state")"
echo "$S1" | python3 -m json.tool
OBS1="$(printf '%s' "$S1" | json_get observation_id)"
CALC1="$(printf '%s' "$S1" | observation_digest)"
[[ "$OBS1" == "$CALC1" ]] || fail "observation_id does not match independent recomputation"
python3 -c 'import json,re,sys; x=json.load(sys.stdin); assert x["runtime_id"]==sys.argv[1]; assert re.fullmatch(r"obs-[0-9a-f]{64}",x["observation_id"]); assert (x.get("kol_state") or {}).get("event")=="manual",x' "$RUNTIME1" <<<"$S1"
STATE1_PAYLOAD="$(python3 -c 'import json,sys; print(json.dumps(json.load(sys.stdin).get("kol_state") or {},sort_keys=True,separators=(",",":")))' <<<"$S1")"
pass "manual observation identity independently recomputed"

section "4. Repeat identical manual observation"
say "Run again in KoLmafia gCLI:"
say
say "  kol_actor sync"
say
if ! prompt_yes "Have you run the second 'kol_actor sync'?"; then exit 4; fi
S2="$(curl -fsS "$BASE/v1/state")"
OBS2="$(printf '%s' "$S2" | json_get observation_id)"
CALC2="$(printf '%s' "$S2" | observation_digest)"
[[ "$OBS2" == "$CALC2" ]] || fail "second observation_id does not match recomputation"
STATE2_PAYLOAD="$(python3 -c 'import json,sys; print(json.dumps(json.load(sys.stdin).get("kol_state") or {},sort_keys=True,separators=(",",":")))' <<<"$S2")"
if [[ "$STATE1_PAYLOAD" == "$STATE2_PAYLOAD" ]]; then
  [[ "$OBS1" == "$OBS2" ]] || fail "identical observation payloads produced different IDs"
  pass "identical observations produced identical observation_id"
else
  warn "bounded KoL facts changed between syncs; deterministic recomputation passed but equality comparison was not applicable"
fi

section "5. Change only the observation event"
say "'kol_actor turn' in this shim publishes event=after-adventure; it does NOT spend an adventure."
say "Run this exact command yourself in KoLmafia gCLI:"
say
say "  kol_actor turn"
say
if ! prompt_yes "Have you run 'kol_actor turn' and want the harness to verify the observation?"; then exit 5; fi
STURN="$(curl -fsS "$BASE/v1/state")"
OBSTURN="$(printf '%s' "$STURN" | json_get observation_id)"
CALCTURN="$(printf '%s' "$STURN" | observation_digest)"
[[ "$OBSTURN" == "$CALCTURN" ]] || fail "turn observation_id does not match recomputation"
python3 -c 'import json,sys; x=json.load(sys.stdin); assert (x.get("kol_state") or {}).get("event")=="after-adventure",x' <<<"$STURN"
[[ "$OBSTURN" != "$OBS2" ]] || fail "changed observation event did not change observation_id"
pass "bounded observation change produced a different observation_id"

section "6. Return to manual observation"
say "Run in KoLmafia gCLI:"
say
say "  kol_actor sync"
say
if ! prompt_yes "Have you run 'kol_actor sync' to return event=manual?"; then exit 6; fi
S3="$(curl -fsS "$BASE/v1/state")"
OBS3="$(printf '%s' "$S3" | json_get observation_id)"
CALC3="$(printf '%s' "$S3" | observation_digest)"
[[ "$OBS3" == "$CALC3" ]] || fail "restored manual observation_id does not match recomputation"
STATE3_PAYLOAD="$(python3 -c 'import json,sys; print(json.dumps(json.load(sys.stdin).get("kol_state") or {},sort_keys=True,separators=(",",":")))' <<<"$S3")"
if [[ "$STATE3_PAYLOAD" == "$STATE1_PAYLOAD" ]]; then
  [[ "$OBS3" == "$OBS1" ]] || fail "same original manual payload did not return to original observation_id"
  pass "returning to identical bounded facts returned to the original observation_id"
else
  warn "other bounded facts changed; identity still independently recomputed correctly"
fi

section "7. Proposal provenance without confirmation or execution"
curl -fsS -X POST "$BASE/v1/release/refresh" >/dev/null || fail "release refresh failed"
PF="$(mktemp)"
PCODE="$(curl -sS -o "$PF" -w '%{http_code}' -X POST "$BASE/v1/proposals/release-stage" -H 'content-type: application/json' -d '{}')"
PROPOSAL="$(cat "$PF")"; rm -f "$PF"
[[ "$PCODE" == "201" ]] || fail "proposal creation returned HTTP $PCODE: $PROPOSAL"
echo "$PROPOSAL" | python3 -m json.tool
PID="$(printf '%s' "$PROPOSAL" | json_get id)"
PRUN="$(printf '%s' "$PROPOSAL" | json_get runtime_id)"
POBS="$(printf '%s' "$PROPOSAL" | json_get observation_id)"
[[ "$PRUN" == "$RUNTIME1" ]] || fail "proposal runtime_id differs from current runtime"
[[ "$POBS" == "$OBS3" ]] || fail "proposal observation_id differs from current observation"
AUDIT1="$(curl -fsS "$BASE/v1/audit/recent?limit=100")"
python3 -c 'import json,sys; x=json.load(sys.stdin); pid,run,obs=sys.argv[1:]; hits=[e for e in x["events"] if e.get("type")=="proposal.created" and e.get("proposal_id")==pid]; assert hits,hits; e=hits[-1]; assert e.get("runtime_id")==run,e; assert e.get("observation_id")==obs,e' "$PID" "$RUNTIME1" "$OBS3" <<<"$AUDIT1"
pass "proposal.created retained originating runtime/observation identity"
say "The harness will NOT confirm this proposal and will NOT execute it."

section "8. Restart Actor Engine"
if ! prompt_yes "Restart only Actor Engine now using the local setup stop/start scripts?"; then exit 8; fi
(
  cd "$SETUP_DIR"
  ./stop.sh
  ./start.sh
)
wait_health || fail "Actor Engine did not become healthy after restart"
HEALTH2="$(curl -fsS "$BASE/health")"
echo "$HEALTH2" | python3 -m json.tool
RUNTIME2="$(printf '%s' "$HEALTH2" | json_get runtime_id)"
[[ "$RUNTIME2" != "$RUNTIME1" ]] || fail "runtime_id did not change across Actor Engine restart"
pass "runtime_id changed across process restart"

POST0="$(curl -fsS "$BASE/v1/state")"
python3 -c 'import json,sys; x=json.load(sys.stdin); assert x["runtime_id"]==sys.argv[1],x; assert not x.get("observation_id"),x' "$RUNTIME2" <<<"$POST0"
pass "new runtime did not inherit prior in-memory observation"

section "9. Re-observe identical KoL facts in the new runtime"
say "Run in KoLmafia gCLI:"
say
say "  kol_actor sync"
say
if ! prompt_yes "Have you run post-restart 'kol_actor sync'?"; then exit 9; fi
S4="$(curl -fsS "$BASE/v1/state")"
OBS4="$(printf '%s' "$S4" | json_get observation_id)"
CALC4="$(printf '%s' "$S4" | observation_digest)"
[[ "$OBS4" == "$CALC4" ]] || fail "post-restart observation_id does not match recomputation"
STATE4_PAYLOAD="$(python3 -c 'import json,sys; print(json.dumps(json.load(sys.stdin).get("kol_state") or {},sort_keys=True,separators=(",",":")))' <<<"$S4")"
if [[ "$STATE4_PAYLOAD" == "$STATE3_PAYLOAD" ]]; then
  [[ "$OBS4" == "$OBS3" ]] || fail "identical observation content changed identity across runtime restart"
  pass "same bounded observation retained the same observation_id across different runtime_id values"
else
  warn "bounded KoL facts changed across restart; cross-runtime equality was not applicable, recomputation still passed"
fi

section "10. Old proposal authority remains absent"
EF="$(mktemp)"
ECODE="$(curl -sS -o "$EF" -w '%{http_code}' -X POST "$BASE/v1/proposals/$PID/execute")"
EBODY="$(cat "$EF")"; rm -f "$EF"
say "HTTP $ECODE"
say "$EBODY"
[[ "$ECODE" == "409" ]] || fail "old proposal execute returned HTTP $ECODE, want 409"
grep -qi 'proposal not found' <<<"$EBODY" || fail "old proposal was not rejected as proposal not found"
pass "pre-restart proposal authority was not restored"

section "11. Audit provenance across runtime boundary"
AUDIT2="$(curl -fsS "$BASE/v1/audit/recent?limit=100")"
python3 -c '
import json,sys
x=json.load(sys.stdin); pid,oldrun,newrun,oldobs,newobs=sys.argv[1:]
created=[e for e in x["events"] if e.get("type")=="proposal.created" and e.get("proposal_id")==pid]
assert created and created[-1].get("runtime_id")==oldrun and created[-1].get("observation_id")==oldobs,created
starts=[e for e in x["events"] if e.get("type")=="runtime.started" and e.get("runtime_id")==newrun]
assert starts,starts
denied=[e for e in x["events"] if e.get("type")=="execution.denied" and e.get("proposal_id")==pid]
assert denied and denied[-1].get("runtime_id")==newrun,denied
if newobs:
    assert denied[-1].get("observation_id")==newobs,denied[-1]
' "$PID" "$RUNTIME1" "$RUNTIME2" "$OBS3" "$OBS4" <<<"$AUDIT2"
pass "audit history distinguishes originating and post-restart runtime identities"

say
pass "LIVE_RUNTIME_OBSERVATION_IDENTITY_TEST"
say "Runtime identity changed across restart."
say "Observation identity matched bounded KoL observations deterministically."
say "Proposal evidence retained its originating runtime/observation identity."
say "No release.stage proposal was confirmed or executed by this test."
