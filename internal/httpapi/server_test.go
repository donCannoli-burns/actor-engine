package httpapi

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/donCannoli-burns/actor-engine/internal/admission"
	"github.com/donCannoli-burns/actor-engine/internal/audit"
	"github.com/donCannoli-burns/actor-engine/internal/gate"
	"github.com/donCannoli-burns/actor-engine/internal/kingdomsitter"
	"github.com/donCannoli-burns/actor-engine/internal/preflight"
	"github.com/donCannoli-burns/actor-engine/internal/protocol"
	"github.com/donCannoli-burns/actor-engine/internal/release"
	"github.com/donCannoli-burns/actor-engine/internal/stateplane"
)

func TestReleaseStageFlow(t *testing.T) {
	t.Parallel()
	jar := []byte("not-a-real-jar-but-a-bounded-stage-fixture")
	sum := sha256.Sum256(jar)
	digest := hex.EncodeToString(sum[:])

	var upstream *httptest.Server
	upstream = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/latest":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"tag_name": "r29309",
				"name":     "29309",
				"html_url": upstream.URL + "/release/r29309",
				"assets": []map[string]any{{
					"name":                 "KoLmafia-29309.jar",
					"browser_download_url": upstream.URL + "/KoLmafia-29309.jar",
					"digest":               "sha256:" + digest,
				}},
			})
		case "/KoLmafia-29309.jar":
			_, _ = w.Write(jar)
		case "/health":
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "execution_authority": false})
		case "/v0/state":
			_ = json.NewEncoder(w).Encode(map[string]any{"available": false, "execution_authority": false})
		default:
			http.NotFound(w, r)
		}
	}))
	defer upstream.Close()

	plane := stateplane.New(stateplane.StateReady, stateplane.StateObserveOnly)
	stageDir := t.TempDir()
	ledger, err := audit.Open(filepath.Join(t.TempDir(), "audit.jsonl"))
	if err != nil {
		t.Fatalf("open audit ledger: %v", err)
	}
	s := New(
		plane,
		gate.New(),
		release.NewClient(upstream.URL+"/latest"),
		kingdomsitter.NewClient(upstream.URL),
		stageDir,
		ledger,
		"run-test-release",
	)
	s.SetInstalledRevision("29301")
	h := s.Handler()

	sidecarRR := httptest.NewRecorder()
	h.ServeHTTP(sidecarRR, httptest.NewRequest(http.MethodGet, "/v1/kingdomsitter/refresh", nil))
	if sidecarRR.Code != http.StatusOK {
		t.Fatalf("sidecar refresh status = %d, want %d; body=%s", sidecarRR.Code, http.StatusOK, sidecarRR.Body.String())
	}

	observeRR := httptest.NewRecorder()
	h.ServeHTTP(observeRR, httptest.NewRequest(http.MethodGet, "/v1/kolmafia/update?event=manual&character=doncannoli&total_turns=522711&ascension_turns=20439&adventures=219&ascensions=349&breakfast=true", nil))
	if observeRR.Code != http.StatusOK {
		t.Fatalf("observe status = %d, want %d; body=%s", observeRR.Code, http.StatusOK, observeRR.Body.String())
	}
	var observed struct {
		RuntimeID     string `json:"runtime_id"`
		ObservationID string `json:"observation_id"`
	}
	if err := json.Unmarshal(observeRR.Body.Bytes(), &observed); err != nil {
		t.Fatalf("decode observe response: %v", err)
	}
	if observed.RuntimeID != "run-test-release" || observed.ObservationID == "" {
		t.Fatalf("observe identity = %+v", observed)
	}

	refresh := httptest.NewRecorder()
	h.ServeHTTP(refresh, httptest.NewRequest(http.MethodPost, "/v1/release/refresh", nil))
	if refresh.Code != http.StatusOK {
		t.Fatalf("release refresh status = %d, want %d; body=%s", refresh.Code, http.StatusOK, refresh.Body.String())
	}

	proposalRR := httptest.NewRecorder()
	h.ServeHTTP(proposalRR, httptest.NewRequest(http.MethodPost, "/v1/proposals/release-stage", strings.NewReader(`{}`)))
	if proposalRR.Code != http.StatusCreated {
		t.Fatalf("create proposal status = %d, want %d; body=%s", proposalRR.Code, http.StatusCreated, proposalRR.Body.String())
	}
	var proposal protocol.Proposal
	if err := json.Unmarshal(proposalRR.Body.Bytes(), &proposal); err != nil {
		t.Fatalf("decode proposal: %v", err)
	}
	if proposal.RuntimeID != observed.RuntimeID || proposal.ObservationID != observed.ObservationID {
		t.Fatalf("proposal identity = runtime %q observation %q, want %q %q", proposal.RuntimeID, proposal.ObservationID, observed.RuntimeID, observed.ObservationID)
	}
	if proposal.Admission.Version != admission.Version {
		t.Fatalf("proposal admission version = %q, want %q", proposal.Admission.Version, admission.Version)
	}
	if proposal.Admission.Preflight.Status != "READY" || !proposal.Admission.Preflight.ReadyForProposal {
		t.Fatalf("proposal admission preflight = %+v", proposal.Admission.Preflight)
	}
	if proposal.Admission.Preflight.RuntimeID != proposal.RuntimeID || proposal.Admission.Preflight.ObservationID != proposal.ObservationID {
		t.Fatalf("proposal admission identity does not match proposal: %+v", proposal.Admission)
	}
	if err := admission.Verify(proposal.Admission); err != nil {
		t.Fatalf("proposal admission verification failed: %v", err)
	}

	confirmBody := fmt.Sprintf(`{"state_digest":%q,"confirmed_by":"test-human"}`, proposal.StateDigest)
	confirmRR := httptest.NewRecorder()
	h.ServeHTTP(confirmRR, httptest.NewRequest(http.MethodPost, "/v1/proposals/"+proposal.ID+"/confirm", strings.NewReader(confirmBody)))
	if confirmRR.Code != http.StatusOK {
		t.Fatalf("confirm proposal status = %d, want %d; body=%s", confirmRR.Code, http.StatusOK, confirmRR.Body.String())
	}

	executeRR := httptest.NewRecorder()
	h.ServeHTTP(executeRR, httptest.NewRequest(http.MethodPost, "/v1/proposals/"+proposal.ID+"/execute", nil))
	if executeRR.Code != http.StatusOK {
		t.Fatalf("execute proposal status = %d, want %d; body=%s", executeRR.Code, http.StatusOK, executeRR.Body.String())
	}
	var receipt protocol.Receipt
	if err := json.Unmarshal(executeRR.Body.Bytes(), &receipt); err != nil {
		t.Fatalf("decode receipt: %v", err)
	}
	if !receipt.Success {
		t.Fatalf("receipt.Success = false, want true; detail=%s", receipt.Detail)
	}
	if receipt.SHA256 != digest {
		t.Fatalf("receipt.SHA256 = %q, want %q", receipt.SHA256, digest)
	}
	if receipt.RuntimeID != proposal.RuntimeID || receipt.ObservationID != proposal.ObservationID {
		t.Fatalf("receipt identity = runtime %q observation %q, want %q %q", receipt.RuntimeID, receipt.ObservationID, proposal.RuntimeID, proposal.ObservationID)
	}
	if receipt.AdmissionDigest != proposal.Admission.Digest {
		t.Fatalf("receipt admission digest = %q, want %q", receipt.AdmissionDigest, proposal.Admission.Digest)
	}
	got, err := os.ReadFile(filepath.Join(stageDir, "KoLmafia-29309.jar"))
	if err != nil {
		t.Fatalf("read staged file: %v", err)
	}
	if string(got) != string(jar) {
		t.Fatal("staged file contents differ from upstream fixture")
	}

	auditRR := httptest.NewRecorder()
	h.ServeHTTP(auditRR, httptest.NewRequest(http.MethodGet, "/v1/audit/recent?limit=10", nil))
	if auditRR.Code != http.StatusOK {
		t.Fatalf("audit status = %d, want %d; body=%s", auditRR.Code, http.StatusOK, auditRR.Body.String())
	}
	var auditOut struct {
		Events []audit.Event `json:"events"`
	}
	if err := json.Unmarshal(auditRR.Body.Bytes(), &auditOut); err != nil {
		t.Fatalf("decode audit response: %v", err)
	}
	wantTypes := []string{audit.EventProposalCreated, audit.EventProposalConfirmed, audit.EventExecutionStarted, audit.EventExecutionSucceeded}
	if len(auditOut.Events) != len(wantTypes) {
		t.Fatalf("audit events = %d, want %d: %+v", len(auditOut.Events), len(wantTypes), auditOut.Events)
	}
	for i, want := range wantTypes {
		if auditOut.Events[i].Type != want {
			t.Fatalf("audit event[%d].Type = %q, want %q", i, auditOut.Events[i].Type, want)
		}
		if auditOut.Events[i].RuntimeID != proposal.RuntimeID {
			t.Fatalf("audit event[%d].RuntimeID = %q, want %q", i, auditOut.Events[i].RuntimeID, proposal.RuntimeID)
		}
		if auditOut.Events[i].ObservationID != proposal.ObservationID {
			t.Fatalf("audit event[%d].ObservationID = %q, want %q", i, auditOut.Events[i].ObservationID, proposal.ObservationID)
		}
		if auditOut.Events[i].AdmissionDigest != proposal.Admission.Digest {
			t.Fatalf("audit event[%d].AdmissionDigest = %q, want %q", i, auditOut.Events[i].AdmissionDigest, proposal.Admission.Digest)
		}
	}
}

func TestAuditHistoryDoesNotRestoreAuthority(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "audit.jsonl")
	ledger, err := audit.Open(path)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	if _, err := ledger.Append(audit.Event{Type: audit.EventProposalCreated, ProposalID: "p-old", StateDigest: "old", Result: "pending"}); err != nil {
		t.Fatalf("append proposal: %v", err)
	}
	if _, err := ledger.Append(audit.Event{Type: audit.EventProposalConfirmed, ProposalID: "p-old", StateDigest: "old", Actor: "human", Result: "confirmed"}); err != nil {
		t.Fatalf("append confirmation: %v", err)
	}

	reopened, err := audit.Open(path)
	if err != nil {
		t.Fatalf("reopen audit ledger: %v", err)
	}
	plane := stateplane.New(stateplane.StateReady, stateplane.StateObserveOnly)
	s := New(
		plane,
		gate.New(),
		release.NewClient("http://127.0.0.1:1/latest"),
		kingdomsitter.NewClient("http://127.0.0.1:1"),
		t.TempDir(),
		reopened,
		"run-restarted",
	)

	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/v1/proposals/p-old/execute", nil))
	if rr.Code != http.StatusConflict {
		t.Fatalf("execute after restart status = %d, want %d; body=%s", rr.Code, http.StatusConflict, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "proposal not found") {
		t.Fatalf("execute after restart body = %q, want proposal not found", rr.Body.String())
	}
	recent := reopened.Recent(10)
	if recent[len(recent)-1].Type != audit.EventExecutionDenied {
		t.Fatalf("last audit event = %q, want %q", recent[len(recent)-1].Type, audit.EventExecutionDenied)
	}
	if recent[len(recent)-1].RuntimeID != "run-restarted" {
		t.Fatalf("last audit runtime id = %q, want run-restarted", recent[len(recent)-1].RuntimeID)
	}
}

func TestPreflightIsReadOnlyAndFreshnessAware(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 9, 29, 16, 0, 0, 0, time.UTC)

	var upstream *httptest.Server
	upstream = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/latest":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"tag_name": "r29315",
				"name":     "29315",
				"html_url": upstream.URL + "/release/r29315",
				"assets": []map[string]any{{
					"name":                 "KoLmafia-29315.jar",
					"browser_download_url": upstream.URL + "/KoLmafia-29315.jar",
					"digest":               "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
				}},
			})
		case "/health":
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "execution_authority": false})
		case "/v0/state":
			_ = json.NewEncoder(w).Encode(map[string]any{"available": false, "execution_authority": false})
		default:
			http.NotFound(w, r)
		}
	}))
	defer upstream.Close()

	ledger, err := audit.Open(filepath.Join(t.TempDir(), "audit.jsonl"))
	if err != nil {
		t.Fatalf("open audit ledger: %v", err)
	}
	s := New(
		stateplane.New(stateplane.StateReady, stateplane.StateObserveOnly),
		gate.New(),
		release.NewClient(upstream.URL+"/latest"),
		kingdomsitter.NewClient(upstream.URL),
		t.TempDir(),
		ledger,
		"run-preflight-test",
	)
	s.now = func() time.Time { return now }
	s.SetInstalledRevision("29301")
	h := s.Handler()

	getPreflight := func() preflight.Result {
		t.Helper()
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/v1/preflight", nil))
		if rr.Code != http.StatusOK {
			t.Fatalf("preflight status = %d, body=%s", rr.Code, rr.Body.String())
		}
		var out preflight.Result
		if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
			t.Fatalf("decode preflight: %v", err)
		}
		return out
	}

	initial := getPreflight()
	if initial.Status != "NOT_READY" || initial.ReadyForProposal {
		t.Fatalf("initial preflight = %+v", initial)
	}

	refreshRelease := httptest.NewRecorder()
	h.ServeHTTP(refreshRelease, httptest.NewRequest(http.MethodPost, "/v1/release/refresh", nil))
	if refreshRelease.Code != http.StatusOK {
		t.Fatalf("release refresh status = %d, body=%s", refreshRelease.Code, refreshRelease.Body.String())
	}
	refreshSidecar := httptest.NewRecorder()
	h.ServeHTTP(refreshSidecar, httptest.NewRequest(http.MethodGet, "/v1/kingdomsitter/refresh", nil))
	if refreshSidecar.Code != http.StatusOK {
		t.Fatalf("sidecar refresh status = %d, body=%s", refreshSidecar.Code, refreshSidecar.Body.String())
	}
	observe := httptest.NewRecorder()
	h.ServeHTTP(observe, httptest.NewRequest(http.MethodGet, "/v1/kolmafia/update?event=manual&character=doncannoli&total_turns=522711&ascension_turns=20439&adventures=219&ascensions=349&breakfast=true", nil))
	if observe.Code != http.StatusOK {
		t.Fatalf("observe status = %d, body=%s", observe.Code, observe.Body.String())
	}

	before := ledger.Status().Count
	ready := getPreflight()
	after := ledger.Status().Count
	if before != after {
		t.Fatalf("preflight mutated audit ledger count: before=%d after=%d", before, after)
	}
	if ready.Status != "READY" || !ready.ReadyForProposal {
		t.Fatalf("ready preflight = status %q ready=%t reasons=%v", ready.Status, ready.ReadyForProposal, ready.Reasons)
	}
	if ready.Authority.PreflightGrantsAuthority {
		t.Fatal("preflight unexpectedly grants authority")
	}

	now = now.Add(6 * time.Minute)
	stale := getPreflight()
	if stale.Status != "NOT_READY" || stale.ReadyForProposal {
		t.Fatalf("stale preflight = status %q ready=%t reasons=%v", stale.Status, stale.ReadyForProposal, stale.Reasons)
	}
	if !slices.Contains(stale.Reasons, "kol_observation_stale") {
		t.Fatalf("stale reasons=%v missing kol_observation_stale", stale.Reasons)
	}
	if !slices.Contains(stale.Reasons, "kingdomsitter_status_stale") {
		t.Fatalf("stale reasons=%v missing kingdomsitter_status_stale", stale.Reasons)
	}
}


func TestProposalAdmissionRejectsNotReadyPreflight(t *testing.T) {
	t.Parallel()
	ledger, err := audit.Open(filepath.Join(t.TempDir(), "audit.jsonl"))
	if err != nil {
		t.Fatalf("open audit ledger: %v", err)
	}
	s := New(
		stateplane.New(stateplane.StateReady, stateplane.StateObserveOnly),
		gate.New(),
		release.NewClient("http://127.0.0.1:1/latest"),
		kingdomsitter.NewClient("http://127.0.0.1:1"),
		t.TempDir(),
		ledger,
		"run-admission-not-ready",
	)

	before := ledger.Status().Count
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/v1/proposals/release-stage", strings.NewReader(`{}`)))
	if rr.Code != http.StatusConflict {
		t.Fatalf("proposal status = %d, want %d; body=%s", rr.Code, http.StatusConflict, rr.Body.String())
	}
	var out struct {
		Error     string           `json:"error"`
		Preflight preflight.Result `json:"preflight"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode admission denial: %v", err)
	}
	if out.Error != "preflight_not_ready" {
		t.Fatalf("error = %q, want preflight_not_ready", out.Error)
	}
	if out.Preflight.Status != "NOT_READY" || out.Preflight.ReadyForProposal {
		t.Fatalf("preflight = %+v", out.Preflight)
	}
	if ledger.Status().Count != before {
		t.Fatalf("admission denial appended audit evidence: before=%d after=%d", before, ledger.Status().Count)
	}
	if got := s.plane.Snapshot(); slices.Contains(got, stateplane.StateAwaitingConfirmation) || slices.Contains(got, stateplane.StateProposalReady) {
		t.Fatalf("admission denial changed proposal state: %v", got)
	}
}
