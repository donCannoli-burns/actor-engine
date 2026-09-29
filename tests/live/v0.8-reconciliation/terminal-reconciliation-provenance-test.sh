#!/usr/bin/env bash
set -euo pipefail

MAIN_BASE="${MAIN_BASE:-http://127.0.0.1:10424}"
PROJECT_ROOT="${PROJECT_ROOT:-$HOME/.kolmafia/kolmaf-ai}"
ACTOR_REPO="${ACTOR_REPO:-$PROJECT_ROOT/actor-engine}"
REQUIRED_BASE_COMMIT="${REQUIRED_BASE_COMMIT:-41f9864d3aa2b0b0daf6b63dc4fe3da1b661e9c8}"
EXPECTED_VERSION="${EXPECTED_VERSION:-0.8.0}"
CONFIRMED_BY="${CONFIRMED_BY:-doncannoli-v08-live-test}"

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
  for _ in {1..40}; do
    if curl -fsS "$url" >/dev/null 2>&1; then return 0; fi
    sleep 0.25
  done
  return 1
}

for c in curl python3 git sha256sum go grep find; do need "$c"; done
[[ -d "$ACTOR_REPO/.git" ]] || fail "Actor Engine repo not found: $ACTOR_REPO"

WORK="$(mktemp -d)"
FIXTURE_PID=""
ACTOR_PID=""
cleanup(){
  set +e
  [[ -n "$ACTOR_PID" ]] && kill "$ACTOR_PID" 2>/dev/null
  [[ -n "$FIXTURE_PID" ]] && kill "$FIXTURE_PID" 2>/dev/null
  wait "$ACTOR_PID" 2>/dev/null
  wait "$FIXTURE_PID" 2>/dev/null
  rm -rf "$WORK"
}
trap cleanup EXIT INT TERM

section "0. Baseline and static verification"
HEAD="$(git -C "$ACTOR_REPO" rev-parse HEAD)"
say "repo: $ACTOR_REPO"
say "HEAD: $HEAD"
git -C "$ACTOR_REPO" merge-base --is-ancestor "$REQUIRED_BASE_COMMIT" "$HEAD" || fail "verified v0.7.0 baseline is not an ancestor"
(
  cd "$ACTOR_REPO"
  go test ./...
  go test -race ./...
  go vet ./...
  sha256sum -c MANIFEST.sha256
  go build -o "$WORK/kol-actor-engine" ./cmd/kol-actor-engine
)
pass "static verification complete"

section "1. Capture a fresh bounded KoL observation from the normal sidecar"
curl -fsS "$MAIN_BASE/health" >/dev/null || fail "normal Actor Engine sidecar unavailable at $MAIN_BASE"
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

section "2. Launch isolated bad-digest fixture"
cat >"$WORK/fixture.py" <<'PY'
import hashlib, json
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path

body=b"actor-engine-v08-intentional-bad-digest-fixture\n"
actual=hashlib.sha256(body).hexdigest()
wrong="0"*64
if actual == wrong:
    wrong="1"*64

class H(BaseHTTPRequestHandler):
    def log_message(self, fmt, *args):
        pass
    def do_GET(self):
        port=self.server.server_address[1]
        if self.path == "/latest":
            payload={
                "tag_name":"r-v08-fixture",
                "name":"v08-fixture",
                "html_url":f"http://127.0.0.1:{port}/release/r-v08-fixture",
                "assets":[{
                    "name":"KoLmafia-v08-fixture.jar",
                    "browser_download_url":f"http://127.0.0.1:{port}/KoLmafia-v08-fixture.jar",
                    "digest":"sha256:"+wrong,
                }],
            }
            raw=json.dumps(payload).encode()
            self.send_response(200); self.send_header("content-type","application/json"); self.send_header("content-length",str(len(raw))); self.end_headers(); self.wfile.write(raw)
        elif self.path == "/KoLmafia-v08-fixture.jar":
            self.send_response(200); self.send_header("content-type","application/java-archive"); self.send_header("content-length",str(len(body))); self.end_headers(); self.wfile.write(body)
        else:
            self.send_response(404); self.end_headers()

srv=ThreadingHTTPServer(("127.0.0.1",0),H)
Path(PORT_FILE).write_text(str(srv.server_address[1]))
Path(SHA_FILE).write_text(actual)
srv.serve_forever()
PY
# Inject paths without allowing shell interpolation inside the Python body.
python3 -c 'from pathlib import Path; p=Path("'"$WORK"'/fixture.py"); s=p.read_text(); s="PORT_FILE="+repr("'"$WORK"'/fixture.port")+"\nSHA_FILE="+repr("'"$WORK"'/fixture.sha")+"\n"+s; p.write_text(s)'
python3 "$WORK/fixture.py" >"$WORK/fixture.log" 2>&1 &
FIXTURE_PID=$!
for _ in {1..40}; do [[ -s "$WORK/fixture.port" ]] && break; sleep 0.1; done
[[ -s "$WORK/fixture.port" ]] || fail "fixture server did not publish a port"
FIXTURE_PORT="$(cat "$WORK/fixture.port")"
ACTUAL_SHA="$(cat "$WORK/fixture.sha")"
wait_url "http://127.0.0.1:$FIXTURE_PORT/latest" || fail "fixture server unavailable"
pass "local fixture is ready with intentionally wrong advertised SHA-256"

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
start_isolated
HEALTH="$(curl -fsS "$ISO_BASE/health")"
python3 -c 'import json,sys; x=json.load(sys.stdin); assert x["version"]==sys.argv[1],x; assert x["authority_restored_on_boot"] is False,x' "$EXPECTED_VERSION" <<<"$HEALTH"
ISO_RUNTIME="$(printf '%s' "$HEALTH" | json_get runtime_id)"
pass "isolated Actor Engine reports $EXPECTED_VERSION"

section "3. Copy only the bounded observation into the isolated engine"
curl -fsS -X POST "$ISO_BASE/v1/release/refresh" >/dev/null || fail "isolated release refresh failed"
curl -fsS "$ISO_BASE/v1/kingdomsitter/refresh" >/dev/null || fail "isolated Kingdomsitter refresh failed"
QUERY="$(python3 -c '
import json,sys,urllib.parse
x=json.load(sys.stdin)["kol_state"]
keys=["event","character","total_turns","ascension_turns","adventures","ascensions","breakfast"]
print(urllib.parse.urlencode({k:x.get(k,"") for k in keys}))
' <<<"$MAIN_STATE")"
curl -fsS "$ISO_BASE/v1/kolmafia/update?$QUERY" >/dev/null || fail "isolated observation ingest failed"
PREFLIGHT="$(curl -fsS "$ISO_BASE/v1/preflight")"
python3 -c 'import json,sys; x=json.load(sys.stdin); assert x["status"]=="READY",x; assert x["ready_for_proposal"] is True,x' <<<"$PREFLIGHT"
pass "isolated preflight is READY using copied bounded evidence"

section "4. Create and explicitly confirm the isolated fixture proposal"
TMP="$(mktemp)"
PCODE="$(curl -sS -o "$TMP" -w '%{http_code}' -X POST "$ISO_BASE/v1/proposals/release-stage" -H 'content-type: application/json' -d '{}')"
PROPOSAL="$(cat "$TMP")"; rm -f "$TMP"
[[ "$PCODE" == "201" ]] || fail "isolated proposal HTTP $PCODE: $PROPOSAL"
echo "$PROPOSAL" | python3 -m json.tool
PID="$(printf '%s' "$PROPOSAL" | json_get id)"
STATE_DIGEST="$(python3 -c 'import json,sys; print(json.load(sys.stdin)["state_digest"])' <<<"$PROPOSAL")"
python3 -c 'import json,sys; x=json.load(sys.stdin); assert x["payload"]["tag"]=="r-v08-fixture",x; assert x["payload"]["asset"]=="KoLmafia-v08-fixture.jar",x' <<<"$PROPOSAL"
say "This proposal targets ONLY the temporary fixture stage directory:"
say "  $STAGE"
say "The fixture advertises a deliberately wrong SHA-256, so execution must fail before final rename."
if ! prompt_yes "Confirm this isolated fixture proposal as '$CONFIRMED_BY'?"; then exit 4; fi
BODY="$(python3 -c 'import json,sys; print(json.dumps({"state_digest":sys.argv[1],"confirmed_by":sys.argv[2]}))' "$STATE_DIGEST" "$CONFIRMED_BY")"
CONFIRM="$(curl -fsS -X POST "$ISO_BASE/v1/proposals/$PID/confirm" -H 'content-type: application/json' -d "$BODY")"
CONFIRM_DIGEST="$(python3 -c 'import json,sys; print(json.load(sys.stdin)["confirmation"]["digest"])' <<<"$CONFIRM")"
pass "isolated proposal confirmed"

section "5. Execute the intentional digest-mismatch fixture"
say "This execute can only write inside the temporary directory shown above."
say "Expected result: execution.started, digest mismatch, execution.failed, no committed artifact."
if ! prompt_yes "Execute the isolated bad-digest fixture now?"; then exit 5; fi
TMP="$(mktemp)"
ECODE="$(curl -sS -o "$TMP" -w '%{http_code}' -X POST "$ISO_BASE/v1/proposals/$PID/execute")"
RECEIPT="$(cat "$TMP")"; rm -f "$TMP"
say "HTTP $ECODE"
echo "$RECEIPT" | python3 -m json.tool
[[ "$ECODE" == "200" ]] || fail "isolated execution returned HTTP $ECODE, want terminal receipt HTTP 200"
python3 -c '
import json,sys
x=json.load(sys.stdin)
assert x["success"] is False,x
assert "digest mismatch" in x["detail"],x
assert not x.get("artifact_path"),x
assert x["sha256"]==sys.argv[1],(x,sys.argv[1])
assert x["reconciliation_digest"],x
r=x["reconciliation"]
assert r["version"]=="kol-actor/reconciliation-v1",r
assert r["success"] is False,r
assert r["outcome"]=="failed",r
assert r["artifact_committed"] is False,r
assert r["evidence_grants_authority"] is False,r
assert r["authority_restorable"] is False,r
' "$ACTUAL_SHA" <<<"$RECEIPT"
pass "terminal receipt reports intentional failure with no committed artifact"

section "6. Independently verify reconciliation provenance and cleanup"
python3 -c '
import hashlib,json,sys
x=json.load(sys.stdin)
r=x["reconciliation"]
payload={k:v for k,v in r.items() if k!="digest"}
canonical=json.dumps(payload,sort_keys=True,separators=(",",":"),ensure_ascii=False).encode()
want="sha256:"+hashlib.sha256(canonical).hexdigest()
assert r["digest"]==want,(r["digest"],want,canonical.decode())
assert x["reconciliation_digest"]==want,x
assert r["execution_digest"]==x["execution_digest"],(r,x)
assert r["confirmation_digest"]==x["confirmation_digest"],(r,x)
assert r["admission_digest"]==x["admission_digest"],(r,x)
' <<<"$RECEIPT"
if find "$STAGE" -mindepth 1 -print -quit | grep -q .; then
  find "$STAGE" -mindepth 1 -maxdepth 1 -ls >&2
  fail "temporary stage directory is not empty after digest mismatch"
fi
pass "reconciliation digest independently recomputed and stage directory is empty"

AUDIT_JSON="$(curl -fsS "$ISO_BASE/v1/audit/recent?limit=100")"
REC_DIGEST="$(python3 -c 'import json,sys; print(json.load(sys.stdin)["reconciliation_digest"])' <<<"$RECEIPT")"
EXEC_DIGEST="$(python3 -c 'import json,sys; print(json.load(sys.stdin)["execution_digest"])' <<<"$RECEIPT")"
python3 -c '
import json,sys
x=json.load(sys.stdin); pid,ed,rd=sys.argv[1:]
ev=[e for e in x["events"] if e.get("proposal_id")==pid]
started=[e for e in ev if e.get("type")=="execution.started"]
failed=[e for e in ev if e.get("type")=="execution.failed"]
succeeded=[e for e in ev if e.get("type")=="execution.succeeded"]
assert len(started)==1,started
assert len(failed)==1,failed
assert not succeeded,succeeded
assert started[0]["execution_digest"]==ed,started[0]
assert failed[0]["execution_digest"]==ed,failed[0]
assert failed[0]["reconciliation_digest"]==rd,failed[0]
r=failed[0]["reconciliation"]
assert r["digest"]==rd,r
assert r["artifact_committed"] is False,r
assert r["authority_restorable"] is False,r
' "$PID" "$EXEC_DIGEST" "$REC_DIGEST" <<<"$AUDIT_JSON"
pass "durable audit history binds execution.started to terminal reconciliation evidence"

section "7. Restart only the isolated Actor Engine"
kill "$ACTOR_PID"
wait "$ACTOR_PID" 2>/dev/null || true
ACTOR_PID=""
start_isolated
HEALTH2="$(curl -fsS "$ISO_BASE/health")"
RUNTIME2="$(printf '%s' "$HEALTH2" | json_get runtime_id)"
[[ "$RUNTIME2" != "$ISO_RUNTIME" ]] || fail "isolated runtime_id did not change"
AUDIT2="$(curl -fsS "$ISO_BASE/v1/audit/recent?limit=100")"
python3 -c '
import json,sys
x=json.load(sys.stdin); pid,rd=sys.argv[1:]
assert x["authority_restored_from_audit"] is False,x
hits=[e for e in x["events"] if e.get("proposal_id")==pid and e.get("reconciliation_digest")==rd]
assert hits,hits
assert hits[-1]["reconciliation"]["authority_restorable"] is False,hits[-1]
' "$PID" "$REC_DIGEST" <<<"$AUDIT2"
TMP="$(mktemp)"
RCODE="$(curl -sS -o "$TMP" -w '%{http_code}' -X POST "$ISO_BASE/v1/proposals/$PID/execute")"
RBODY="$(cat "$TMP")"; rm -f "$TMP"
[[ "$RCODE" == "409" ]] || fail "old proposal after isolated restart returned HTTP $RCODE, want 409"
grep -qi 'proposal not found' <<<"$RBODY" || fail "historical terminal evidence restored proposal authority"
pass "reconciliation evidence survived isolated restart without restoring authority"

say
pass "LIVE_TERMINAL_RECONCILIATION_PROVENANCE_TEST"
say "The isolated authorized attempt reached execution.started and failed on the intentional digest mismatch."
say "The terminal receipt carried independently verifiable reconciliation provenance."
say "No fixture artifact or temporary part file remained committed."
say "Durable reconciliation evidence survived isolated Actor Engine restart without restoring authority."
