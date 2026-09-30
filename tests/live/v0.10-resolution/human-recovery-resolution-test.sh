#!/usr/bin/env bash
set -euo pipefail

MAIN_BASE="${MAIN_BASE:-http://127.0.0.1:10424}"
PROJECT_ROOT="${PROJECT_ROOT:-$HOME/.kolmafia/kolmaf-ai}"
ACTOR_REPO="${ACTOR_REPO:-$PROJECT_ROOT/actor-engine}"
REQUIRED_BASE_COMMIT="${REQUIRED_BASE_COMMIT:-6c9de316b3b732c30d3d940354b95d13c438b239}"
EXPECTED_VERSION="${EXPECTED_VERSION:-0.10.0-dev}"
CONFIRMED_BY="${CONFIRMED_BY:-doncannoli-v10-crash-test}"
RESOLVED_BY="${RESOLVED_BY:-doncannoli-v10-resolution-test}"
RESOLUTION_NOTE="${RESOLUTION_NOTE:-reviewed exact interrupted execution; acknowledge unknown outcome and continue without replay}"

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
wait_url(){
  local url="$1"
  for _ in {1..60}; do
    if curl -fsS "$url" >/dev/null 2>&1; then return 0; fi
    sleep 0.2
  done
  return 1
}

for c in curl python3 git sha256sum go grep find; do need "$c"; done
[[ -d "$ACTOR_REPO/.git" ]] || fail "Actor Engine repo not found: $ACTOR_REPO"

WORK="$(mktemp -d)"
FIXTURE_PID=""
ACTOR_PID=""
EXEC_CURL_PID=""
cleanup(){
  set +e
  [[ -n "$EXEC_CURL_PID" ]] && kill "$EXEC_CURL_PID" 2>/dev/null
  [[ -n "$ACTOR_PID" ]] && kill "$ACTOR_PID" 2>/dev/null
  [[ -n "$FIXTURE_PID" ]] && kill "$FIXTURE_PID" 2>/dev/null
  [[ -n "$EXEC_CURL_PID" ]] && wait "$EXEC_CURL_PID" 2>/dev/null
  [[ -n "$ACTOR_PID" ]] && wait "$ACTOR_PID" 2>/dev/null
  [[ -n "$FIXTURE_PID" ]] && wait "$FIXTURE_PID" 2>/dev/null
  rm -rf "$WORK"
}
trap cleanup EXIT INT TERM

section "0. Baseline and static verification"
HEAD="$(git -C "$ACTOR_REPO" rev-parse HEAD)"
say "repo: $ACTOR_REPO"
say "HEAD: $HEAD"
git -C "$ACTOR_REPO" merge-base --is-ancestor "$REQUIRED_BASE_COMMIT" "$HEAD" || fail "verified v0.9.0 baseline is not an ancestor"
(
  cd "$ACTOR_REPO"
  go test ./...
  go test -race ./...
  go vet ./...
  sha256sum -c MANIFEST.sha256
  go build -o "$WORK/kol-actor-engine" ./cmd/kol-actor-engine
)
pass "static verification complete"

section "1. Capture fresh bounded observation from the normal sidecar"
curl -fsS "$MAIN_BASE/health" >/dev/null || fail "normal Actor Engine unavailable at $MAIN_BASE"
say "Run this exact command yourself in KoLmafia gCLI:"
say
say "  kol_actor sync"
say
if ! prompt_yes "Have you run 'kol_actor sync' and want the harness to capture the read-only state?"; then exit 3; fi
MAIN_STATE="$(curl -fsS "$MAIN_BASE/v1/state")"
python3 -c '
import json,sys
x=json.load(sys.stdin)
assert x.get("observation_id"),x
assert x.get("kol_state"),x
assert x.get("installed_revision"),x
' <<<"$MAIN_STATE"
REV="$(printf '%s' "$MAIN_STATE" | json_get installed_revision)"
JAR="${KOLMAFIA_JAR:-/opt/kolmafia/lib/app/KoLmafia-$REV.jar}"
[[ -f "$JAR" ]] || fail "installed KoLmafia jar not found at $JAR; set KOLMAFIA_JAR explicitly"
pass "captured fresh bounded state; installed revision $REV"

section "2. Launch isolated blocking fixture"
cat >"$WORK/fixture.py" <<'PY'
import hashlib, json, time
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path

prefix=b"actor-engine-v10-partial-fixture\n"*2048
full=prefix + (b"x"*(1024*1024-len(prefix)))
digest=hashlib.sha256(full).hexdigest()

class H(BaseHTTPRequestHandler):
    def log_message(self, fmt, *args):
        pass
    def do_GET(self):
        port=self.server.server_address[1]
        if self.path == "/latest":
            payload={
                "tag_name":"r-v10-fixture",
                "name":"v10-fixture",
                "html_url":f"http://127.0.0.1:{port}/release/r-v10-fixture",
                "assets":[{
                    "name":"KoLmafia-v10-fixture.jar",
                    "browser_download_url":f"http://127.0.0.1:{port}/KoLmafia-v10-fixture.jar",
                    "digest":"sha256:"+digest,
                }],
            }
            raw=json.dumps(payload).encode()
            self.send_response(200)
            self.send_header("content-type","application/json")
            self.send_header("content-length",str(len(raw)))
            self.end_headers()
            self.wfile.write(raw)
        elif self.path == "/KoLmafia-v10-fixture.jar":
            self.send_response(200)
            self.send_header("content-type","application/java-archive")
            self.send_header("content-length",str(len(full)))
            self.end_headers()
            self.wfile.write(prefix)
            self.wfile.flush()
            Path(ENTERED_FILE).write_text("entered")
            time.sleep(300)
        else:
            self.send_response(404)
            self.end_headers()

srv=ThreadingHTTPServer(("127.0.0.1",0),H)
Path(PORT_FILE).write_text(str(srv.server_address[1]))
srv.serve_forever()
PY
python3 -c 'from pathlib import Path; p=Path("'"$WORK"'/fixture.py"); s=p.read_text(); s="PORT_FILE="+repr("'"$WORK"'/fixture.port")+"\nENTERED_FILE="+repr("'"$WORK"'/download.entered")+"\n"+s; p.write_text(s)'
python3 "$WORK/fixture.py" >"$WORK/fixture.log" 2>&1 &
FIXTURE_PID=$!
for _ in {1..40}; do [[ -s "$WORK/fixture.port" ]] && break; sleep 0.1; done
[[ -s "$WORK/fixture.port" ]] || fail "fixture server did not publish a port"
FIXTURE_PORT="$(cat "$WORK/fixture.port")"
wait_url "http://127.0.0.1:$FIXTURE_PORT/latest" || fail "fixture server unavailable"
pass "blocking fixture server is ready"

ISO_PORT="$(python3 -c 'import socket; s=socket.socket(); s.bind(("127.0.0.1",0)); print(s.getsockname()[1]); s.close()')"
ISO_BASE="http://127.0.0.1:$ISO_PORT"
STAGE="$WORK/staging"
AUDIT="$WORK/audit.jsonl"
mkdir -p "$STAGE"

start_isolated(){
  "$WORK/kol-actor-engine"     -listen "127.0.0.1:$ISO_PORT"     -kolmafia_jar "$JAR"     -kingdomsitter "http://127.0.0.1:10423"     -release_url "http://127.0.0.1:$FIXTURE_PORT/latest"     -stage_dir "$STAGE"     -audit_log "$AUDIT"     -release_refresh 0     -sidecar_refresh 0     >"$WORK/actor.log" 2>&1 &
  ACTOR_PID=$!
  wait_url "$ISO_BASE/health" || { cat "$WORK/actor.log" >&2; fail "isolated Actor Engine unavailable"; }
}

copy_bounded_state(){
  curl -fsS -X POST "$ISO_BASE/v1/release/refresh" >/dev/null || fail "isolated release refresh failed"
  curl -fsS "$ISO_BASE/v1/kingdomsitter/refresh" >/dev/null || fail "isolated Kingdomsitter refresh failed"
  QUERY="$(python3 -c '
import json,sys,urllib.parse
x=json.load(sys.stdin)["kol_state"]
keys=["event","character","total_turns","ascension_turns","adventures","ascensions","breakfast"]
print(urllib.parse.urlencode({k:x.get(k,"") for k in keys}))
' <<<"$MAIN_STATE")"
  curl -fsS "$ISO_BASE/v1/kolmafia/update?$QUERY" >/dev/null || fail "isolated observation ingest failed"
}

start_isolated
HEALTH="$(curl -fsS "$ISO_BASE/health")"
python3 -c 'import json,sys; x=json.load(sys.stdin); assert x["version"]==sys.argv[1],x; assert x["authority_restored_on_boot"] is False,x; assert x["automatic_execution_replay"] is False,x; assert x["automatic_recovery_resolution"] is False,x' "$EXPECTED_VERSION" <<<"$HEALTH"
copy_bounded_state
PREFLIGHT="$(curl -fsS "$ISO_BASE/v1/preflight")"
python3 -c 'import json,sys; x=json.load(sys.stdin); assert x["status"]=="READY",x' <<<"$PREFLIGHT"
pass "isolated v0.10 Actor Engine is READY before interruption"

section "3. Create and confirm isolated crash fixture proposal"
PROPOSAL="$(curl -fsS -X POST "$ISO_BASE/v1/proposals/release-stage" -H 'content-type: application/json' -d '{}')"
echo "$PROPOSAL" | python3 -m json.tool
PID="$(printf '%s' "$PROPOSAL" | json_get id)"
STATE_DIGEST="$(python3 -c 'import json,sys; print(json.load(sys.stdin)["state_digest"])' <<<"$PROPOSAL")"
python3 -c 'import json,sys; x=json.load(sys.stdin); assert x["payload"]["asset"]=="KoLmafia-v10-fixture.jar",x' <<<"$PROPOSAL"
say "This proposal targets only:"
say "  $STAGE"
if ! prompt_yes "Confirm this isolated crash fixture proposal as '$CONFIRMED_BY'?"; then exit 4; fi
BODY="$(python3 -c 'import json,sys; print(json.dumps({"state_digest":sys.argv[1],"confirmed_by":sys.argv[2]}))' "$STATE_DIGEST" "$CONFIRMED_BY")"
curl -fsS -X POST "$ISO_BASE/v1/proposals/$PID/confirm" -H 'content-type: application/json' -d "$BODY" >/dev/null
pass "isolated proposal confirmed"

section "4. Cross execution boundary and crash mid-download"
say "The harness will wait for durable execution.started, fixture-download entry, and a real .part-* file before SIGKILLing only the isolated Actor Engine."
if ! prompt_yes "Start the isolated fixture execution and allow that controlled crash?"; then exit 5; fi
curl -sS -X POST "$ISO_BASE/v1/proposals/$PID/execute" >"$WORK/execute.out" 2>"$WORK/execute.err" &
EXEC_CURL_PID=$!

STARTED=0
ENTERED=0
PARTFILE=""
for _ in {1..150}; do
  [[ -s "$WORK/download.entered" ]] && ENTERED=1
  PARTFILE="$(find "$STAGE" -maxdepth 1 -type f -name '.KoLmafia-v10-fixture.jar.part-*' -print -quit 2>/dev/null || true)"
  if curl -fsS "$ISO_BASE/v1/audit/recent?limit=100" >"$WORK/audit-live.json" 2>/dev/null; then
    if python3 -c '
import json,sys
x=json.load(open(sys.argv[1])); pid=sys.argv[2]
raise SystemExit(0 if any(e.get("proposal_id")==pid and e.get("type")=="execution.started" for e in x["events"]) else 1)
' "$WORK/audit-live.json" "$PID"; then
      STARTED=1
    fi
  fi
  if [[ "$STARTED" == "1" && "$ENTERED" == "1" && -n "$PARTFILE" ]]; then break; fi
  sleep 0.1
done
[[ "$STARTED" == "1" ]] || fail "execution.started was not observed durably"
[[ "$ENTERED" == "1" ]] || fail "fixture download handler was not entered"
[[ -n "$PARTFILE" && -f "$PARTFILE" ]] || fail "partial staging file was not proven before crash"
kill -9 "$ACTOR_PID"
wait "$ACTOR_PID" 2>/dev/null || true
ACTOR_PID=""
wait "$EXEC_CURL_PID" 2>/dev/null || true
EXEC_CURL_PID=""
[[ -f "$PARTFILE" ]] || fail "partial file unexpectedly disappeared after SIGKILL"
python3 -c '
import json,sys
pid=sys.argv[2]
events=[json.loads(line) for line in open(sys.argv[1]) if line.strip()]
mine=[e for e in events if e.get("proposal_id")==pid]
assert any(e.get("type")=="execution.started" for e in mine),mine
assert not any(e.get("type") in ("execution.succeeded","execution.failed") for e in mine),mine
' "$AUDIT" "$PID"
pass "controlled crash left durable start, no terminal result, and a real orphan part file"

section "5. Restart and prove the unresolved block"
start_isolated
RECOVERY_BEFORE="$(curl -fsS "$ISO_BASE/v1/recovery")"
INTERRUPTION_DIGEST="$(python3 -c '
import hashlib,json,sys
x=json.load(sys.stdin); pid=sys.argv[1]
assert x["status"]=="INTERRUPTED_UNKNOWN_OUTCOME",x
assert len(x["unresolved"])==1,x
assert len(x.get("resolved",[]))==0,x
r=x["unresolved"][0]
assert r["proposal_id"]==pid,r
assert r["outcome_known"] is False,r
assert r["artifact_state"]=="unknown",r
assert r["replay_permitted"] is False,r
assert r["authority_restorable"] is False,r
payload={k:v for k,v in r.items() if k!="digest"}
want="sha256:"+hashlib.sha256(json.dumps(payload,sort_keys=True,separators=(",",":"),ensure_ascii=False).encode()).hexdigest()
assert r["digest"]==want,(r["digest"],want)
print(want)
' "$PID" <<<"$RECOVERY_BEFORE")"
[[ -f "$PARTFILE" ]] || fail "read-only recovery removed orphan before resolution"
pass "restart reports exact unresolved unknown outcome"

copy_bounded_state
PF_BLOCKED="$(curl -fsS "$ISO_BASE/v1/preflight")"
python3 -c 'import json,sys; x=json.load(sys.stdin); assert x["status"]=="NOT_READY",x; assert "interrupted_execution_unresolved" in x["reasons"],x' <<<"$PF_BLOCKED"
pass "interruption blocks proposal admission before human resolution"

section "6. Explicit human resolution"
say "Interruption digest:"
say "  $INTERRUPTION_DIGEST"
say "Original proposal:"
say "  $PID"
say "Orphaned ambiguous partial artifact:"
say "  $PARTFILE"
say
say "Allowed decision:"
say "  acknowledge_unknown_no_replay"
say
say "This does NOT assert success or failure and does NOT remove the orphan."
if ! prompt_yes "Acknowledge this exact unknown interruption as '$RESOLVED_BY' and clear only its admission block?"; then exit 6; fi

COUNT_BEFORE="$(curl -fsS "$ISO_BASE/v1/audit/recent?limit=1" | python3 -c 'import json,sys; print(json.load(sys.stdin)["ledger"]["count"])')"
RESOLVE_BODY="$(python3 -c 'import json,sys; print(json.dumps({"interruption_digest":sys.argv[1],"decision":"acknowledge_unknown_no_replay","resolved_by":sys.argv[2],"note":sys.argv[3]}))' "$INTERRUPTION_DIGEST" "$RESOLVED_BY" "$RESOLUTION_NOTE")"
TMP="$(mktemp)"
RESOLVE_CODE="$(curl -sS -o "$TMP" -w '%{http_code}' -X POST "$ISO_BASE/v1/recovery/$INTERRUPTION_DIGEST/resolve" -H 'content-type: application/json' -d "$RESOLVE_BODY")"
RESOLVE_RESPONSE="$(cat "$TMP")"; rm -f "$TMP"
[[ "$RESOLVE_CODE" == "200" ]] || fail "resolution HTTP $RESOLVE_CODE: $RESOLVE_RESPONSE"
echo "$RESOLVE_RESPONSE" | python3 -m json.tool
RESOLUTION_DIGEST="$(python3 -c '
import hashlib,json,sys
x=json.load(sys.stdin); interruption=sys.argv[1]
assert x["resolved"] is True,x
r=x["resolution"]
assert r["version"]=="kol-actor/resolution-v1",r
assert r["interruption_digest"]==interruption,r
assert r["decision"]=="acknowledge_unknown_no_replay",r
assert r["outcome_remains_unknown"] is True,r
assert r["artifact_state_remains_unknown"] is True,r
assert r["replay_permitted"] is False,r
assert r["authority_restorable"] is False,r
assert r["resolution_grants_execution_authority"] is False,r
assert r["interruption_block_cleared"] is True,r
payload={k:v for k,v in r.items() if k!="digest"}
want="sha256:"+hashlib.sha256(json.dumps(payload,sort_keys=True,separators=(",",":"),ensure_ascii=False).encode()).hexdigest()
assert r["digest"]==want,(r["digest"],want)
report=x["recovery"]
assert report["status"]=="CLEAR",report
assert len(report["unresolved"])==0,report
assert len(report["resolved"])==1,report
hist=report["resolved"][0]["interruption"]
assert hist["digest"]==interruption,hist
assert hist["outcome_known"] is False,hist
assert hist["artifact_state"]=="unknown",hist
print(want)
' "$INTERRUPTION_DIGEST" <<<"$RESOLVE_RESPONSE")"
COUNT_AFTER="$(curl -fsS "$ISO_BASE/v1/audit/recent?limit=1" | python3 -c 'import json,sys; print(json.load(sys.stdin)["ledger"]["count"])')"
[[ "$COUNT_AFTER" -eq $((COUNT_BEFORE + 1)) ]] || fail "resolution did not append exactly one audit event"
[[ -f "$PARTFILE" ]] || fail "resolution silently removed ambiguous orphan"
pass "human resolution is independently verifiable and preserved historical ambiguity"

AUDIT_AFTER="$(curl -fsS "$ISO_BASE/v1/audit/recent?limit=100")"
python3 -c '
import json,sys
x=json.load(sys.stdin); interruption,rd=sys.argv[1:]
hits=[e for e in x["events"] if e.get("type")=="recovery.resolved" and e.get("interruption_digest")==interruption]
assert len(hits)==1,hits
e=hits[0]
assert e["resolution_digest"]==rd,e
assert e["resolution"]["digest"]==rd,e
assert e["resolution"]["outcome_remains_unknown"] is True,e
assert e["resolution"]["artifact_state_remains_unknown"] is True,e
assert e["resolution"]["replay_permitted"] is False,e
' "$INTERRUPTION_DIGEST" "$RESOLUTION_DIGEST" <<<"$AUDIT_AFTER"
pass "durable audit history carries the same resolution evidence"

COUNT_DUP_BEFORE="$COUNT_AFTER"
TMP="$(mktemp)"
DUP_CODE="$(curl -sS -o "$TMP" -w '%{http_code}' -X POST "$ISO_BASE/v1/recovery/$INTERRUPTION_DIGEST/resolve" -H 'content-type: application/json' -d "$RESOLVE_BODY")"
DUP_BODY="$(cat "$TMP")"; rm -f "$TMP"
[[ "$DUP_CODE" == "409" ]] || fail "duplicate resolution HTTP $DUP_CODE, want 409"
COUNT_DUP_AFTER="$(curl -fsS "$ISO_BASE/v1/audit/recent?limit=1" | python3 -c 'import json,sys; print(json.load(sys.stdin)["ledger"]["count"])')"
[[ "$COUNT_DUP_BEFORE" == "$COUNT_DUP_AFTER" ]] || fail "duplicate resolution appended evidence"
pass "duplicate resolution is rejected without mutating history"

section "7. Fresh preflight can reopen proposal admission, not execution authority"
copy_bounded_state
PF_READY="$(curl -fsS "$ISO_BASE/v1/preflight")"
python3 -c '
import json,sys
x=json.load(sys.stdin)
assert x["status"]=="READY",x
assert x["ready_for_proposal"] is True,x
assert "interrupted_execution_unresolved" not in x["reasons"],x
checks={c["name"]:c for c in x["checks"]}
assert checks["no_interrupted_execution"]["ok"] is True,checks
assert x["authority"]["preflight_grants_authority"] is False,x
' <<<"$PF_READY"
pass "normal fresh preflight becomes READY only after durable human resolution"

NEW_PROPOSAL="$(curl -fsS -X POST "$ISO_BASE/v1/proposals/release-stage" -H 'content-type: application/json' -d '{}')"
NEW_PID="$(printf '%s' "$NEW_PROPOSAL" | json_get id)"
[[ "$NEW_PID" != "$PID" ]] || fail "new proposal id reused interrupted proposal id"
python3 -c '
import json,sys
x=json.load(sys.stdin)
assert x["requires_confirmation"] is True,x
assert x["admission"]["preflight"]["status"]=="READY",x
checks={c["name"]:c for c in x["admission"]["preflight"]["checks"]}
assert checks["no_interrupted_execution"]["ok"] is True,checks
' <<<"$NEW_PROPOSAL"
pass "new proposal formation reopened; it remains unconfirmed"

TMP="$(mktemp)"
OLD_CODE="$(curl -sS -o "$TMP" -w '%{http_code}' -X POST "$ISO_BASE/v1/proposals/$PID/execute")"
OLD_BODY="$(cat "$TMP")"; rm -f "$TMP"
[[ "$OLD_CODE" == "409" ]] || fail "old proposal execute HTTP $OLD_CODE, want 409"
grep -qi 'proposal not found' <<<"$OLD_BODY" || fail "old interrupted proposal authority was restored"
pass "old interrupted proposal remains non-executable"

section "8. Restart preserves resolution evidence but no proposal authority"
kill "$ACTOR_PID"
wait "$ACTOR_PID" 2>/dev/null || true
ACTOR_PID=""
start_isolated
RECOVERY_FINAL="$(curl -fsS "$ISO_BASE/v1/recovery")"
python3 -c '
import json,sys
x=json.load(sys.stdin); interruption,rd=sys.argv[1:]
assert x["status"]=="CLEAR",x
assert len(x["unresolved"])==0,x
assert len(x["resolved"])==1,x
pair=x["resolved"][0]
assert pair["interruption"]["digest"]==interruption,pair
assert pair["interruption"]["outcome_known"] is False,pair
assert pair["interruption"]["artifact_state"]=="unknown",pair
assert pair["resolution"]["digest"]==rd,pair
assert pair["resolution"]["replay_permitted"] is False,pair
assert pair["resolution"]["authority_restorable"] is False,pair
' "$INTERRUPTION_DIGEST" "$RESOLUTION_DIGEST" <<<"$RECOVERY_FINAL"
[[ -f "$PARTFILE" ]] || fail "restart after resolution removed ambiguous orphan"

for id in "$PID" "$NEW_PID"; do
  TMP="$(mktemp)"
  CODE="$(curl -sS -o "$TMP" -w '%{http_code}' -X POST "$ISO_BASE/v1/proposals/$id/execute")"
  BODY="$(cat "$TMP")"; rm -f "$TMP"
  [[ "$CODE" == "409" ]] || fail "proposal $id execute after restart HTTP $CODE, want 409"
  grep -qi 'proposal not found' <<<"$BODY" || fail "proposal $id authority survived restart"
done

FINAL_AUDIT="$(curl -fsS "$ISO_BASE/v1/audit/recent?limit=100")"
python3 -c '
import json,sys
x=json.load(sys.stdin); newpid=sys.argv[1]
assert not any(e.get("proposal_id")==newpid and e.get("type") in ("proposal.confirmed","execution.started","execution.succeeded","execution.failed") for e in x["events"]),x
' "$NEW_PID" <<<"$FINAL_AUDIT"
pass "resolution survived restart while old and new proposal authority remained absent"

say
pass "LIVE_HUMAN_RECOVERY_RESOLUTION_TEST"
say "Human resolution acknowledged the exact unknown interruption without asserting success or failure."
say "The resolution evidence was independently verifiable and durably cleared only the interruption admission block."
say "The orphaned partial artifact remained untouched and neither old nor new proposal authority was restored."
say "Resolution survived isolated restart; replay remained impossible and the workspace was cleaned only by the harness."
