#!/usr/bin/env bash
set -euo pipefail

MAIN_BASE="${MAIN_BASE:-http://127.0.0.1:10424}"
PROJECT_ROOT="${PROJECT_ROOT:-$HOME/.kolmafia/kolmaf-ai}"
ACTOR_REPO="${ACTOR_REPO:-$PROJECT_ROOT/actor-engine}"
REQUIRED_BASE_COMMIT="${REQUIRED_BASE_COMMIT:-9e2b39f891e7ba969cce86c3f1355f113818fb15}"
EXPECTED_VERSION="${EXPECTED_VERSION:-0.10.0-dev}"
CONFIRMED_BY="${CONFIRMED_BY:-doncannoli-v10-crash-test}"
RESOLVED_BY="${RESOLVED_BY:-doncannoli-v10-resolution-test}"

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

section "1. Capture fresh bounded observation from normal sidecar"
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
python3 -c 'import json,sys; x=json.load(sys.stdin); assert x["version"]==sys.argv[1],x; assert x["automatic_execution_replay"] is False,x; assert x["automatic_recovery_resolution"] is False,x' "$EXPECTED_VERSION" <<<"$HEALTH"
copy_bounded_state
PF="$(curl -fsS "$ISO_BASE/v1/preflight")"
python3 -c 'import json,sys; x=json.load(sys.stdin); assert x["status"]=="READY",x' <<<"$PF"
pass "isolated v0.10 Actor Engine is READY before interruption"

section "3. Create, confirm, and crash isolated fixture"
PROPOSAL="$(curl -fsS -X POST "$ISO_BASE/v1/proposals/release-stage" -H 'content-type: application/json' -d '{}')"
PID="$(printf '%s' "$PROPOSAL" | json_get id)"
STATE_DIGEST="$(python3 -c 'import json,sys; print(json.load(sys.stdin)["state_digest"])' <<<"$PROPOSAL")"
python3 -c 'import json,sys; x=json.load(sys.stdin); assert x["payload"]["asset"]=="KoLmafia-v10-fixture.jar",x' <<<"$PROPOSAL"
say "This proposal targets only $STAGE."
if ! prompt_yes "Confirm this isolated crash fixture proposal as '$CONFIRMED_BY'?"; then exit 4; fi
BODY="$(python3 -c 'import json,sys; print(json.dumps({"state_digest":sys.argv[1],"confirmed_by":sys.argv[2]}))' "$STATE_DIGEST" "$CONFIRMED_BY")"
curl -fsS -X POST "$ISO_BASE/v1/proposals/$PID/confirm" -H 'content-type: application/json' -d "$BODY" >/dev/null
if ! prompt_yes "Start the isolated execution and let the harness SIGKILL it after durable execution.started + a real part file are proven?"; then exit 5; fi
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
' "$WORK/audit-live.json" "$PID"; then STARTED=1; fi
  fi
  [[ "$STARTED" == "1" && "$ENTERED" == "1" && -n "$PARTFILE" ]] && break
  sleep 0.1
done
[[ "$STARTED" == "1" && "$ENTERED" == "1" && -f "$PARTFILE" ]] || fail "crash preconditions were not proven"
kill -9 "$ACTOR_PID"
wait "$ACTOR_PID" 2>/dev/null || true
ACTOR_PID=""
wait "$EXEC_CURL_PID" 2>/dev/null || true
EXEC_CURL_PID=""
[[ -f "$PARTFILE" ]] || fail "partial file disappeared after SIGKILL"
pass "isolated crash left durable execution.started plus ambiguous active-staging bytes"

section "4. Restart and prove resolution is initially blocked"
start_isolated
RECOVERY="$(curl -fsS "$ISO_BASE/v1/recovery")"
INTERRUPTION_DIGEST="$(python3 -c '
import json,sys
x=json.load(sys.stdin); pid=sys.argv[1]
assert x["status"]=="INTERRUPTED_UNKNOWN_OUTCOME",x
assert len(x["unresolved"])==1,x
r=x["unresolved"][0]
assert r["proposal_id"]==pid,r
assert r["outcome_known"] is False,r
assert r["artifact_state"]=="unknown",r
assert r["replay_permitted"] is False,r
print(r["digest"])
' "$PID" <<<"$RECOVERY")"
PF_BLOCKED="$(curl -fsS "$ISO_BASE/v1/preflight")"
python3 -c 'import json,sys; x=json.load(sys.stdin); assert "interrupted_execution_unresolved" in x["reasons"],x' <<<"$PF_BLOCKED"
pass "unknown-outcome interruption blocks admission before human disposition"

section "5. Human-governed quarantine disposition"
ORPHAN_NAME="$(basename "$PARTFILE")"
ORPHAN_SHA="$(sha256sum "$PARTFILE" | awk '{print $1}')"
QDIR="$STAGE/.recovery-quarantine"
QNAME="${INTERRUPTION_DIGEST#sha256:}.part"
QPATH="$QDIR/$QNAME"
say "Ambiguous isolated bytes:"
say "  source:     $PARTFILE"
say "  sha256:     $ORPHAN_SHA"
say "  quarantine: $QPATH"
say "The historical operation outcome will remain UNKNOWN. This action only removes ambiguous bytes from active staging and records a no-replay human resolution."
if ! prompt_yes "Move this isolated orphan into quarantine and record the verified human resolution?"; then exit 6; fi
mkdir -p "$QDIR"
mv "$PARTFILE" "$QPATH"
[[ ! -e "$PARTFILE" && -f "$QPATH" ]] || fail "quarantine move did not complete"
QSHA="$(sha256sum "$QPATH" | awk '{print $1}')"
[[ "$QSHA" == "$ORPHAN_SHA" ]] || fail "quarantine move changed bytes"

RESOLVE_BODY="$(python3 -c '
import json,sys
print(json.dumps({
  "interruption_digest":sys.argv[1],
  "decision":"quarantine_unknown_no_replay",
  "resolved_by":sys.argv[2],
  "note":"isolated ambiguous part quarantined; abandon replay without classifying historical outcome",
  "orphan_name":sys.argv[3],
  "quarantine_name":sys.argv[4],
  "quarantine_sha256":sys.argv[5],
}))
' "$INTERRUPTION_DIGEST" "$RESOLVED_BY" "$ORPHAN_NAME" "$QNAME" "$QSHA")"
TMP="$(mktemp)"
RCODE="$(curl -sS -o "$TMP" -w '%{http_code}' -X POST "$ISO_BASE/v1/recovery/$INTERRUPTION_DIGEST/resolve" -H 'content-type: application/json' -d "$RESOLVE_BODY")"
RESOLUTION_RESPONSE="$(cat "$TMP")"; rm -f "$TMP"
[[ "$RCODE" == "200" ]] || fail "resolution HTTP $RCODE: $RESOLUTION_RESPONSE"
echo "$RESOLUTION_RESPONSE" | python3 -m json.tool

RESOLUTION_DIGEST="$(python3 -c '
import hashlib,json,sys
x=json.load(sys.stdin)
assert x["resolved"] is True,x
r=x["resolution"]
assert r["version"]=="kol-actor/resolution-v1",r
assert r["decision"]=="quarantine_unknown_no_replay",r
assert r["outcome_remains_unknown"] is True,r
assert r["artifact_state_remains_unknown"] is True,r
assert r["ambiguous_bytes_disposition"]=="quarantined",r
assert r["original_path_absent_verified"] is True,r
assert r["final_artifact_absent_verified"] is True,r
assert r["active_part_files_absent_verified"] is True,r
assert r["quarantine_hash_verified"] is True,r
assert r["replay_permitted"] is False,r
assert r["authority_restorable"] is False,r
assert r["resolution_grants_execution_authority"] is False,r
assert r["interruption_block_cleared"] is True,r
payload={k:v for k,v in r.items() if k!="digest"}
canonical=json.dumps(payload,sort_keys=True,separators=(",",":"),ensure_ascii=False).encode()
want="sha256:"+hashlib.sha256(canonical).hexdigest()
assert r["digest"]==want,(r["digest"],want,canonical.decode())
rec=x["recovery"]
assert rec["status"]=="CLEAR",rec
assert len(rec["unresolved"])==0,rec
assert len(rec["resolved"])==1,rec
assert rec["resolved"][0]["interruption"]["outcome_known"] is False,rec
assert rec["resolved"][0]["interruption"]["artifact_state"]=="unknown",rec
print(want)
' <<<"$RESOLUTION_RESPONSE")"
pass "Actor Engine independently verified quarantine disposition and canonical resolution evidence"

section "6. Fresh evidence reopens proposal admission only"
copy_bounded_state
PF_READY="$(curl -fsS "$ISO_BASE/v1/preflight")"
python3 -c '
import json,sys
x=json.load(sys.stdin)
assert x["status"]=="READY",x
assert x["ready_for_proposal"] is True,x
checks={c["name"]:c for c in x["checks"]}
assert checks["no_interrupted_execution"]["ok"] is True,checks
assert x["authority"]["preflight_grants_authority"] is False,x
' <<<"$PF_READY"
NEW_PROPOSAL="$(curl -fsS -X POST "$ISO_BASE/v1/proposals/release-stage" -H 'content-type: application/json' -d '{}')"
NEW_PID="$(printf '%s' "$NEW_PROPOSAL" | json_get id)"
[[ -n "$NEW_PID" ]] || fail "new unconfirmed proposal was not created"
AUDIT_NOW="$(curl -fsS "$ISO_BASE/v1/audit/recent?limit=100")"
python3 -c '
import json,sys
x=json.load(sys.stdin); pid=sys.argv[1]
assert any(e.get("proposal_id")==pid and e.get("type")=="proposal.created" for e in x["events"]),x
assert not any(e.get("proposal_id")==pid and e.get("type")=="proposal.confirmed" for e in x["events"]),x
assert not any(e.get("proposal_id")==pid and e.get("type")=="execution.started" for e in x["events"]),x
' "$NEW_PID" <<<"$AUDIT_NOW"
pass "verified resolution reopened proposal formation but no new confirmation/execution authority"

section "7. Restart preserves resolution, quarantine, and ephemeral authority"
kill "$ACTOR_PID"
wait "$ACTOR_PID" 2>/dev/null || true
ACTOR_PID=""
start_isolated
REC2="$(curl -fsS "$ISO_BASE/v1/recovery")"
python3 -c '
import json,sys
x=json.load(sys.stdin); rd=sys.argv[1]
assert x["status"]=="CLEAR",x
assert len(x["unresolved"])==0,x
assert len(x["resolved"])==1,x
assert x["resolved"][0]["resolution"]["digest"]==rd,x
assert x["resolved"][0]["interruption"]["outcome_known"] is False,x
assert x["resolved"][0]["interruption"]["artifact_state"]=="unknown",x
' "$RESOLUTION_DIGEST" <<<"$REC2"
[[ -f "$QPATH" ]] || fail "quarantined bytes disappeared after restart"
[[ "$(sha256sum "$QPATH" | awk '{print $1}')" == "$QSHA" ]] || fail "quarantine hash changed after restart"

for old in "$PID" "$NEW_PID"; do
  TMP="$(mktemp)"
  CODE="$(curl -sS -o "$TMP" -w '%{http_code}' -X POST "$ISO_BASE/v1/proposals/$old/execute")"
  BODY="$(cat "$TMP")"; rm -f "$TMP"
  [[ "$CODE" == "409" ]] || fail "proposal $old after restart returned HTTP $CODE, want 409"
  grep -qi 'proposal not found' <<<"$BODY" || fail "proposal $old authority survived restart"
done
pass "durable resolution and quarantined bytes survived restart while all proposal authority remained ephemeral"

say
pass "LIVE_HUMAN_QUARANTINE_RESOLUTION_TEST"
say "The human-governed quarantine disposition was independently verified before recovery resolution was committed."
say "Resolution preserved the interrupted operation's unknown outcome and granted no replay or execution authority."
say "Verified resolution cleared only the interruption admission block; fresh evidence reopened proposal formation."
say "Durable resolution and quarantined bytes survived restart while all proposal authority remained ephemeral."
