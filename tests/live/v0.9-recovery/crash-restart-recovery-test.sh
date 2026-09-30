#!/usr/bin/env bash
set -euo pipefail

MAIN_BASE="${MAIN_BASE:-http://127.0.0.1:10424}"
PROJECT_ROOT="${PROJECT_ROOT:-$HOME/.kolmafia/kolmaf-ai}"
ACTOR_REPO="${ACTOR_REPO:-$PROJECT_ROOT/actor-engine}"
REQUIRED_BASE_COMMIT="${REQUIRED_BASE_COMMIT:-915f8a4900062c41b8ae5128b522157becbb4b3a}"
EXPECTED_VERSION="${EXPECTED_VERSION:-0.9.0}"
CONFIRMED_BY="${CONFIRMED_BY:-doncannoli-v09-live-test}"

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
git -C "$ACTOR_REPO" merge-base --is-ancestor "$REQUIRED_BASE_COMMIT" "$HEAD" || fail "verified v0.8.0 baseline is not an ancestor"
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

prefix=b"actor-engine-v09-partial-fixture\n"*2048
full=prefix + (b"x"*(1024*1024-len(prefix)))
digest=hashlib.sha256(full).hexdigest()

class H(BaseHTTPRequestHandler):
    def log_message(self, fmt, *args):
        pass
    def do_GET(self):
        port=self.server.server_address[1]
        if self.path == "/latest":
            payload={
                "tag_name":"r-v09-fixture",
                "name":"v09-fixture",
                "html_url":f"http://127.0.0.1:{port}/release/r-v09-fixture",
                "assets":[{
                    "name":"KoLmafia-v09-fixture.jar",
                    "browser_download_url":f"http://127.0.0.1:{port}/KoLmafia-v09-fixture.jar",
                    "digest":"sha256:"+digest,
                }],
            }
            raw=json.dumps(payload).encode()
            self.send_response(200)
            self.send_header("content-type","application/json")
            self.send_header("content-length",str(len(raw)))
            self.end_headers()
            self.wfile.write(raw)
        elif self.path == "/KoLmafia-v09-fixture.jar":
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
python3 -c 'import json,sys; x=json.load(sys.stdin); assert x["version"]==sys.argv[1],x; assert x["authority_restored_on_boot"] is False,x; assert x["automatic_execution_replay"] is False,x' "$EXPECTED_VERSION" <<<"$HEALTH"
ORIGINAL_RUNTIME="$(printf '%s' "$HEALTH" | json_get runtime_id)"
copy_bounded_state
PREFLIGHT="$(curl -fsS "$ISO_BASE/v1/preflight")"
python3 -c 'import json,sys; x=json.load(sys.stdin); assert x["status"]=="READY",x' <<<"$PREFLIGHT"
pass "isolated v0.9 Actor Engine is READY before interruption"

section "3. Create and confirm isolated fixture proposal"
PROPOSAL="$(curl -fsS -X POST "$ISO_BASE/v1/proposals/release-stage" -H 'content-type: application/json' -d '{}')"
echo "$PROPOSAL" | python3 -m json.tool
PID="$(printf '%s' "$PROPOSAL" | json_get id)"
STATE_DIGEST="$(python3 -c 'import json,sys; print(json.load(sys.stdin)["state_digest"])' <<<"$PROPOSAL")"
python3 -c 'import json,sys; x=json.load(sys.stdin); assert x["payload"]["asset"]=="KoLmafia-v09-fixture.jar",x' <<<"$PROPOSAL"
say "This proposal targets only:"
say "  $STAGE"
say "The harness will start the isolated execution, wait for durable execution.started + an actual .part-* file, then SIGKILL only the isolated Actor Engine."
if ! prompt_yes "Confirm this isolated crash-recovery fixture proposal as '$CONFIRMED_BY'?"; then exit 4; fi
BODY="$(python3 -c 'import json,sys; print(json.dumps({"state_digest":sys.argv[1],"confirmed_by":sys.argv[2]}))' "$STATE_DIGEST" "$CONFIRMED_BY")"
curl -fsS -X POST "$ISO_BASE/v1/proposals/$PID/confirm" -H 'content-type: application/json' -d "$BODY" >/dev/null
pass "isolated proposal confirmed"

section "4. Cross execution boundary and crash mid-download"
if ! prompt_yes "Start the isolated fixture execution and let the harness SIGKILL that isolated process after the partial file is proven?"; then exit 5; fi
curl -sS -X POST "$ISO_BASE/v1/proposals/$PID/execute" >"$WORK/execute.out" 2>"$WORK/execute.err" &
EXEC_CURL_PID=$!

STARTED=0
ENTERED=0
PARTFILE=""
for _ in {1..150}; do
  [[ -s "$WORK/download.entered" ]] && ENTERED=1
  PARTFILE="$(find "$STAGE" -maxdepth 1 -type f -name '.KoLmafia-v09-fixture.jar.part-*' -print -quit 2>/dev/null || true)"
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
pass "durable execution.started and partial staging file proven before crash"
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
pass "crash left durable start with no invented terminal record"

section "5. Restart and classify unknown outcome"
start_isolated
HEALTH2="$(curl -fsS "$ISO_BASE/health")"
RECOVERY_RUNTIME="$(printf '%s' "$HEALTH2" | json_get runtime_id)"
[[ "$RECOVERY_RUNTIME" != "$ORIGINAL_RUNTIME" ]] || fail "runtime_id did not change after isolated crash restart"
COUNT_BEFORE="$(curl -fsS "$ISO_BASE/v1/audit/recent?limit=1" | python3 -c 'import json,sys; print(json.load(sys.stdin)["ledger"]["count"])')"
RECOVERY1="$(curl -fsS "$ISO_BASE/v1/recovery")"
RECOVERY2="$(curl -fsS "$ISO_BASE/v1/recovery")"
COUNT_AFTER="$(curl -fsS "$ISO_BASE/v1/audit/recent?limit=1" | python3 -c 'import json,sys; print(json.load(sys.stdin)["ledger"]["count"])')"
[[ "$COUNT_BEFORE" == "$COUNT_AFTER" ]] || fail "read-only recovery reads changed audit ledger count"

INTERRUPTION_DIGEST="$(python3 -c '
import hashlib,json,sys
x=json.load(sys.stdin); pid=sys.argv[1]
assert x["version"]=="kol-actor/recovery-v1",x
assert x["status"]=="INTERRUPTED_UNKNOWN_OUTCOME",x
assert x["authority"]=={"replay_permitted":False,"authority_restorable":False,"automatic_resolution":False,"evidence_only":True},x
assert len(x["unresolved"])==1,x
r=x["unresolved"][0]
assert r["version"]=="kol-actor/interruption-v1",r
assert r["proposal_id"]==pid,r
assert r["status"]=="INTERRUPTED_UNKNOWN_OUTCOME",r
assert r["outcome_known"] is False,r
assert r["artifact_state"]=="unknown",r
assert r["replay_permitted"] is False,r
assert r["authority_restorable"] is False,r
payload={k:v for k,v in r.items() if k!="digest"}
canonical=json.dumps(payload,sort_keys=True,separators=(",",":"),ensure_ascii=False).encode()
want="sha256:"+hashlib.sha256(canonical).hexdigest()
assert r["digest"]==want,(r["digest"],want,canonical.decode())
print(want)
' "$PID" <<<"$RECOVERY1")"
python3 -c 'import json,sys; a=json.loads(sys.argv[1]); b=json.loads(sys.argv[2]); assert a["unresolved"][0]["digest"]==b["unresolved"][0]["digest"]' "$RECOVERY1" "$RECOVERY2"
[[ -f "$PARTFILE" ]] || fail "read-only recovery silently removed the orphaned part file"
pass "restart classified stable INTERRUPTED_UNKNOWN_OUTCOME evidence without mutation"

section "6. Recovery blocks new proposal admission and restores no old authority"
copy_bounded_state
PF2="$(curl -fsS "$ISO_BASE/v1/preflight")"
python3 -c '
import json,sys
x=json.load(sys.stdin)
assert x["status"]=="NOT_READY",x
assert x["ready_for_proposal"] is False,x
assert "interrupted_execution_unresolved" in x["reasons"],x
checks={c["name"]:c for c in x["checks"]}
assert checks["no_interrupted_execution"]["ok"] is False,checks
' <<<"$PF2"
TMP="$(mktemp)"
PCODE="$(curl -sS -o "$TMP" -w '%{http_code}' -X POST "$ISO_BASE/v1/proposals/release-stage" -H 'content-type: application/json' -d '{}')"
PBODY="$(cat "$TMP")"; rm -f "$TMP"
[[ "$PCODE" == "409" ]] || fail "new proposal during unresolved interruption returned HTTP $PCODE, want 409"
grep -q 'preflight_not_ready' <<<"$PBODY" || fail "new proposal denial did not expose preflight_not_ready"

TMP="$(mktemp)"
OCODE="$(curl -sS -o "$TMP" -w '%{http_code}' -X POST "$ISO_BASE/v1/proposals/$PID/execute")"
OBODY="$(cat "$TMP")"; rm -f "$TMP"
[[ "$OCODE" == "409" ]] || fail "old proposal after crash restart returned HTTP $OCODE, want 409"
grep -qi 'proposal not found' <<<"$OBODY" || fail "old proposal authority was reconstructed"
pass "recovery blocks new proposal admission and old proposal authority remains absent"

section "7. Second restart preserves interruption identity"
kill "$ACTOR_PID"
wait "$ACTOR_PID" 2>/dev/null || true
ACTOR_PID=""
start_isolated
RECOVERY3="$(curl -fsS "$ISO_BASE/v1/recovery")"
DIGEST3="$(python3 -c 'import json,sys; x=json.load(sys.stdin); print(x["unresolved"][0]["digest"])' <<<"$RECOVERY3")"
[[ "$DIGEST3" == "$INTERRUPTION_DIGEST" ]] || fail "interruption digest changed across recovery runtimes"
RUNTIME3="$(python3 -c 'import json,sys; print(json.load(sys.stdin)["runtime_id"])' <<<"$RECOVERY3")"
[[ "$RUNTIME3" != "$RECOVERY_RUNTIME" ]] || fail "second recovery runtime_id did not change"
[[ -f "$PARTFILE" ]] || fail "second read-only recovery restart removed orphaned part file"
pass "second restart changed report runtime while preserving interruption digest and artifact ambiguity"

say
pass "LIVE_CRASH_RESTART_RECOVERY_TEST"
say "The isolated process was killed only after durable execution.started and a partial staging file existed."
say "Restart classified the execution as INTERRUPTED_UNKNOWN_OUTCOME without inferring success or failure."
say "Recovery evidence was stable, read-only, and blocked new proposal admission without restoring replay authority."
say "A second restart preserved the interruption digest; the temporary workspace was cleaned only by the harness."
