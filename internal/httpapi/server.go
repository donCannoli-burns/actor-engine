package httpapi

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/donCannoli-burns/actor-engine/internal/audit"
	"github.com/donCannoli-burns/actor-engine/internal/gate"
	"github.com/donCannoli-burns/actor-engine/internal/identity"
	"github.com/donCannoli-burns/actor-engine/internal/kingdomsitter"
	"github.com/donCannoli-burns/actor-engine/internal/preflight"
	"github.com/donCannoli-burns/actor-engine/internal/protocol"
	"github.com/donCannoli-burns/actor-engine/internal/release"
	"github.com/donCannoli-burns/actor-engine/internal/stateplane"
)

type Server struct {
	mu                     sync.RWMutex
	plane                  *stateplane.Plane
	gate                   *gate.Gate
	releases               *release.Client
	kingdom                *kingdomsitter.Client
	stageDir               string
	audit                  *audit.Ledger
	runtimeID              string
	snapshot               protocol.Snapshot
	latest                 release.Info
	observationAt          time.Time
	releaseRefreshedAt     time.Time
	kingdomsitterCheckedAt time.Time
	pending                *protocol.Proposal
	lastReceipt            *protocol.Receipt
	now                    func() time.Time
}

func New(plane *stateplane.Plane, g *gate.Gate, releases *release.Client, kingdom *kingdomsitter.Client, stageDir string, ledger *audit.Ledger, runtimeID string) *Server {
	return &Server{
		plane:     plane,
		gate:      g,
		releases:  releases,
		kingdom:   kingdom,
		stageDir:  stageDir,
		audit:     ledger,
		runtimeID: runtimeID,
		snapshot:  protocol.Snapshot{Version: "kol-actor/v1", RuntimeID: runtimeID, UpdatedAt: time.Now().UTC()},
		now:       time.Now,
	}
}

func (s *Server) SetInstalledRevision(rev string) {
	s.mu.Lock()
	s.snapshot.InstalledRevision = rev
	s.mu.Unlock()
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", s.health)
	mux.HandleFunc("GET /v1/state", s.state)
	mux.HandleFunc("GET /v1/preflight", s.preflight)
	mux.HandleFunc("GET /v1/audit/recent", s.recentAudit)
	mux.HandleFunc("GET /v1/release/latest", s.latestRelease)
	mux.HandleFunc("POST /v1/release/refresh", s.refreshRelease)
	mux.HandleFunc("POST /v1/proposals/release-stage", s.proposeReleaseStage)
	mux.HandleFunc("POST /v1/proposals/{id}/confirm", s.confirmProposal)
	mux.HandleFunc("POST /v1/proposals/{id}/execute", s.executeProposal)
	mux.HandleFunc("GET /v1/kingdomsitter/refresh", s.refreshKingdomsitter)
	mux.HandleFunc("GET /v1/kolmafia/update", s.ingestKoLState)
	return mux
}

func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":                         true,
		"name":                       "kol-actor-engine",
		"version":                    "0.4.0",
		"runtime_id":                 s.runtimeID,
		"execution_authority":        "gated-local-operations-only",
		"live_kolmafia_mutation":     false,
		"evidence_persistence":       "hash-chained-jsonl",
		"authority_restored_on_boot": false,
	})
}

func (s *Server) state(w http.ResponseWriter, _ *http.Request) {
	s.mu.RLock()
	out := s.snapshot
	if s.pending != nil {
		out.PendingProposalID = s.pending.ID
	}
	out.LastReceipt = s.lastReceipt
	s.mu.RUnlock()
	out.ActiveStates = s.plane.Snapshot()
	out.UpdatedAt = time.Now().UTC()
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) preflight(w http.ResponseWriter, _ *http.Request) {
	s.mu.RLock()
	pendingID := ""
	if s.pending != nil {
		pendingID = s.pending.ID
	}
	in := preflight.Input{
		Now:                    s.currentTime(),
		RuntimeID:              s.runtimeID,
		ObservationID:          s.snapshot.ObservationID,
		ObservationAt:          s.observationAt,
		InstalledRevision:      s.snapshot.InstalledRevision,
		LatestRelease:          s.latest.TagName,
		ReleaseRefreshedAt:     s.releaseRefreshedAt,
		KingdomsitterCheckedAt: s.kingdomsitterCheckedAt,
		KingdomsitterHealthy:   s.snapshot.KingdomsitterHealthy,
		AuditVerified:          s.audit.Status().Verified,
		Fault:                  s.snapshot.Fault,
		PendingProposalID:      pendingID,
		ActiveStates:           s.plane.Snapshot(),
	}
	s.mu.RUnlock()
	writeJSON(w, http.StatusOK, preflight.Build(in))
}

func (s *Server) recentAudit(w http.ResponseWriter, r *http.Request) {
	limit := 25
	if raw := r.URL.Query().Get("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 || n > audit.MaxRecent {
			http.Error(w, fmt.Sprintf("limit must be between 1 and %d", audit.MaxRecent), http.StatusBadRequest)
			return
		}
		limit = n
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ledger":                        s.audit.Status(),
		"events":                        s.audit.Recent(limit),
		"authority_restored_from_audit": false,
	})
}

func (s *Server) refreshRelease(w http.ResponseWriter, r *http.Request) {
	info, err := s.releases.Latest(r.Context())
	if err != nil {
		s.setFault(err)
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	s.applyRelease(info)
	writeJSON(w, http.StatusOK, info)
}

func (s *Server) latestRelease(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	info := s.latest
	s.mu.RUnlock()
	if info.TagName == "" {
		s.refreshRelease(w, r)
		return
	}
	writeJSON(w, http.StatusOK, info)
}

func (s *Server) applyRelease(info release.Info) {
	s.mu.Lock()
	s.latest = info
	s.releaseRefreshedAt = s.currentTime()
	s.snapshot.LatestRelease = info.TagName
	s.snapshot.LatestReleaseURL = info.HTMLURL
	installed := normalizeRevision(s.snapshot.InstalledRevision)
	latest := normalizeRevision(info.TagName)
	s.snapshot.Fault = ""
	s.mu.Unlock()
	if installed != "" && installed == latest {
		s.plane.SetExclusive(stateplane.ReleaseStates, stateplane.StateReleaseCurrent)
		return
	}
	s.plane.SetExclusive(stateplane.ReleaseStates, stateplane.StateReleaseAvailable)
}

func (s *Server) refreshKingdomsitter(w http.ResponseWriter, r *http.Request) {
	checkedAt := s.currentTime()
	health, err := s.kingdom.Health(r.Context())
	if err != nil {
		s.mu.Lock()
		s.snapshot.KingdomsitterHealthy = false
		s.kingdomsitterCheckedAt = checkedAt
		s.snapshot.Fault = "kingdomsitter: " + err.Error()
		s.mu.Unlock()
		s.plane.Deactivate(stateplane.StateKingdomsitterSeen)
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	state, stateErr := s.kingdom.State(r.Context())
	s.mu.Lock()
	s.snapshot.KingdomsitterHealthy = true
	s.kingdomsitterCheckedAt = checkedAt
	s.snapshot.KingdomsitterState = state
	s.snapshot.Fault = ""
	s.mu.Unlock()
	s.plane.Activate(stateplane.StateKingdomsitterSeen)
	writeJSON(w, http.StatusOK, map[string]any{"health": health, "state": state, "state_error": errorText(stateErr)})
}

func (s *Server) ingestKoLState(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	allowed := []string{"event", "character", "total_turns", "ascension_turns", "adventures", "ascensions", "breakfast"}
	state := make(map[string]string, len(allowed))
	for _, key := range allowed {
		state[key] = q.Get(key)
	}
	observationID := identity.ObservationID(state)
	s.mu.Lock()
	s.snapshot.KoLState = state
	s.snapshot.ObservationID = observationID
	s.observationAt = s.currentTime()
	s.snapshot.Fault = ""
	s.mu.Unlock()
	s.plane.Activate(stateplane.StateKoLStateSeen)
	writeJSON(w, http.StatusOK, map[string]any{
		"accepted":            true,
		"runtime_id":          s.runtimeID,
		"observation_id":      observationID,
		"execution_authority": false,
	})
}

func (s *Server) proposeReleaseStage(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Asset string `json:"asset"`
	}
	if err := decodeJSON(r.Body, &in); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	s.mu.RLock()
	info := s.latest
	snapshot := s.snapshot
	s.mu.RUnlock()
	if info.TagName == "" {
		http.Error(w, "release state unavailable; refresh first", http.StatusConflict)
		return
	}
	asset, err := chooseAsset(info, in.Asset)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	digest, err := gate.Digest(snapshotForBinding(snapshot, s.plane.Snapshot()))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	p := protocol.Proposal{
		ID:              randomID(),
		RuntimeID:       snapshot.RuntimeID,
		ObservationID:   snapshot.ObservationID,
		Operation:       "release.stage",
		Risk:            "local-write-reversible",
		StateDigest:     digest,
		Payload:         map[string]any{"tag": info.TagName, "asset": asset.Name, "url": asset.BrowserDownloadURL, "digest": asset.Digest},
		CreatedAt:       time.Now().UTC(),
		ExpiresAt:       time.Now().UTC().Add(10 * time.Minute),
		HumanSummary:    fmt.Sprintf("Stage %s from %s into %s; this does not install or restart KoLmafia.", asset.Name, info.TagName, filepath.Clean(s.stageDir)),
		RequiresConfirm: true,
	}
	s.gate.Put(p)
	if _, err := s.audit.Append(s.withIdentity(audit.Event{
		Type:        audit.EventProposalCreated,
		ProposalID:  p.ID,
		Operation:   p.Operation,
		StateDigest: p.StateDigest,
		Result:      "pending",
		Detail:      p.HumanSummary,
	})); err != nil {
		s.gate.Drop(p.ID)
		s.setFault(fmt.Errorf("audit proposal creation: %w", err))
		http.Error(w, "audit ledger unavailable; proposal refused", http.StatusServiceUnavailable)
		return
	}
	s.mu.Lock()
	s.pending = &p
	s.mu.Unlock()
	s.plane.SetExclusive(stateplane.ProposalStates, stateplane.StateAwaitingConfirmation)
	writeJSON(w, http.StatusCreated, p)
}

func (s *Server) confirmProposal(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var in struct {
		StateDigest string `json:"state_digest"`
		ConfirmedBy string `json:"confirmed_by"`
	}
	if err := decodeJSON(r.Body, &in); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	confirmation := protocol.Confirmation{ProposalID: id, StateDigest: in.StateDigest, ConfirmedBy: in.ConfirmedBy, ConfirmedAt: time.Now().UTC()}
	p, err := s.gate.Confirm(confirmation)
	if err != nil {
		if _, auditErr := s.audit.Append(s.withIdentity(audit.Event{
			Type:        audit.EventConfirmationDenied,
			ProposalID:  id,
			StateDigest: in.StateDigest,
			Actor:       in.ConfirmedBy,
			Result:      "denied",
			Detail:      err.Error(),
		})); auditErr != nil {
			s.setFault(fmt.Errorf("audit confirmation denial: %w", auditErr))
			http.Error(w, "audit ledger unavailable; confirmation refused", http.StatusServiceUnavailable)
			return
		}
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}
	if _, auditErr := s.audit.Append(s.withIdentity(audit.Event{
		Type:        audit.EventProposalConfirmed,
		ProposalID:  p.ID,
		Operation:   p.Operation,
		StateDigest: p.StateDigest,
		Actor:       confirmation.ConfirmedBy,
		Result:      "confirmed",
	})); auditErr != nil {
		s.gate.Drop(p.ID)
		s.clearPending(p.ID)
		s.plane.SetExclusive(stateplane.ProposalStates, "")
		s.setFault(fmt.Errorf("audit proposal confirmation: %w", auditErr))
		http.Error(w, "audit ledger unavailable; confirmation invalidated", http.StatusServiceUnavailable)
		return
	}
	s.plane.SetExclusive(stateplane.ProposalStates, stateplane.StateProposalReady)
	writeJSON(w, http.StatusOK, map[string]any{"confirmed": true, "proposal": p})
}

func (s *Server) executeProposal(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	s.mu.RLock()
	snapshot := s.snapshot
	info := s.latest
	s.mu.RUnlock()
	currentDigest, err := gate.Digest(snapshotForBinding(snapshot, s.plane.Snapshot()))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	p, err := s.gate.Consume(id, currentDigest)
	if err != nil {
		eventType := audit.EventExecutionDenied
		switch {
		case strings.Contains(err.Error(), "approval invalidated"):
			eventType = audit.EventProposalInvalidated
		case strings.Contains(err.Error(), "expired"):
			eventType = audit.EventProposalExpired
		}
		if _, auditErr := s.audit.Append(s.withIdentity(audit.Event{
			Type:        eventType,
			ProposalID:  id,
			StateDigest: currentDigest,
			Result:      "denied",
			Detail:      err.Error(),
		})); auditErr != nil {
			s.setFault(fmt.Errorf("audit execution denial: %w", auditErr))
			http.Error(w, "audit ledger unavailable; execution refused", http.StatusServiceUnavailable)
			return
		}
		if err == gate.ErrProposalNotFound || strings.Contains(err.Error(), "approval invalidated") || strings.Contains(err.Error(), "expired") {
			s.clearPending(id)
			s.plane.SetExclusive(stateplane.ProposalStates, "")
		}
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}

	if _, auditErr := s.audit.Append(s.withIdentity(audit.Event{
		Type:        audit.EventExecutionStarted,
		ProposalID:  p.ID,
		Operation:   p.Operation,
		StateDigest: p.StateDigest,
		Result:      "started",
	})); auditErr != nil {
		s.clearPending(p.ID)
		s.plane.SetExclusive(stateplane.ProposalStates, "")
		s.setFault(fmt.Errorf("audit execution start: %w", auditErr))
		http.Error(w, "audit ledger unavailable; execution refused", http.StatusServiceUnavailable)
		return
	}

	s.plane.SetExclusive(stateplane.ProposalStates, stateplane.StateExecuting)
	var receipt protocol.Receipt
	switch p.Operation {
	case "release.stage":
		assetName, _ := p.Payload["asset"].(string)
		asset, chooseErr := chooseAsset(info, assetName)
		if chooseErr != nil {
			receipt = failedReceipt(p, chooseErr)
			break
		}
		path, sum, stageErr := s.releases.Stage(r.Context(), asset, s.stageDir)
		if stageErr != nil {
			receipt = failedReceipt(p, stageErr)
			break
		}
		receipt = protocol.Receipt{ProposalID: p.ID, RuntimeID: p.RuntimeID, ObservationID: p.ObservationID, Operation: p.Operation, Success: true, Detail: "release asset staged and digest checked when GitHub supplied one", ArtifactPath: path, SHA256: sum, CompletedAt: time.Now().UTC()}
	default:
		receipt = failedReceipt(p, fmt.Errorf("operation %q is not implemented", p.Operation))
	}

	terminalType := audit.EventExecutionFailed
	result := "failed"
	if receipt.Success {
		terminalType = audit.EventExecutionSucceeded
		result = "success"
	}
	if _, auditErr := s.audit.Append(s.withIdentity(audit.Event{
		Type:         terminalType,
		ProposalID:   p.ID,
		Operation:    p.Operation,
		StateDigest:  p.StateDigest,
		Result:       result,
		Detail:       receipt.Detail,
		ArtifactPath: receipt.ArtifactPath,
		SHA256:       receipt.SHA256,
	})); auditErr != nil {
		rollbackDetail := ""
		if receipt.Success && receipt.ArtifactPath != "" {
			if removeErr := os.Remove(receipt.ArtifactPath); removeErr != nil && !os.IsNotExist(removeErr) {
				rollbackDetail = "; staged artifact rollback failed: " + removeErr.Error()
			}
		}
		failed := failedReceipt(p, fmt.Errorf("audit ledger commit failed: %w%s", auditErr, rollbackDetail))
		s.mu.Lock()
		s.lastReceipt = &failed
		s.pending = nil
		s.mu.Unlock()
		s.plane.SetExclusive(stateplane.ProposalStates, "")
		s.plane.Activate(stateplane.StateFaulted)
		writeJSON(w, http.StatusServiceUnavailable, failed)
		return
	}

	s.mu.Lock()
	s.lastReceipt = &receipt
	s.pending = nil
	s.mu.Unlock()
	if receipt.Success {
		s.plane.SetExclusive(stateplane.ProposalStates, stateplane.StateReconciling)
		s.plane.Activate(stateplane.StateReleaseStaged)
		s.plane.SetExclusive(stateplane.ProposalStates, "")
	} else {
		s.plane.SetExclusive(stateplane.ProposalStates, "")
		s.plane.Activate(stateplane.StateFaulted)
	}
	writeJSON(w, http.StatusOK, receipt)
}

func (s *Server) currentTime() time.Time {
	if s.now == nil {
		return time.Now().UTC()
	}
	return s.now().UTC()
}

func (s *Server) withIdentity(event audit.Event) audit.Event {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if event.RuntimeID == "" {
		event.RuntimeID = s.runtimeID
	}
	if event.ObservationID == "" {
		event.ObservationID = s.snapshot.ObservationID
	}
	return event
}

func (s *Server) clearPending(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.pending != nil && s.pending.ID == id {
		s.pending = nil
	}
}

func (s *Server) setFault(err error) {
	s.mu.Lock()
	s.snapshot.Fault = err.Error()
	s.mu.Unlock()
	s.plane.Activate(stateplane.StateFaulted)
}

func chooseAsset(info release.Info, name string) (release.Asset, error) {
	if name == "" {
		return release.SelectJar(info)
	}
	for _, asset := range info.Assets {
		if asset.Name == name {
			return asset, nil
		}
	}
	return release.Asset{}, fmt.Errorf("asset %q not found in release %s", name, info.TagName)
}

func snapshotForBinding(snapshot protocol.Snapshot, active []string) protocol.Snapshot {
	proposalState := map[string]bool{
		stateplane.StateProposalReady:        true,
		stateplane.StateAwaitingConfirmation: true,
		stateplane.StateExecuting:            true,
		stateplane.StateReconciling:          true,
	}
	boundStates := make([]string, 0, len(active))
	for _, state := range active {
		if !proposalState[state] {
			boundStates = append(boundStates, state)
		}
	}
	snapshot.ActiveStates = boundStates
	snapshot.UpdatedAt = time.Time{}
	snapshot.PendingProposalID = ""
	snapshot.LastReceipt = nil
	return snapshot
}

func normalizeRevision(value string) string {
	value = strings.TrimSpace(strings.TrimPrefix(value, "r"))
	return strings.TrimPrefix(value, "KoLmafia-")
}

func failedReceipt(p protocol.Proposal, err error) protocol.Receipt {
	return protocol.Receipt{ProposalID: p.ID, RuntimeID: p.RuntimeID, ObservationID: p.ObservationID, Operation: p.Operation, Success: false, Detail: err.Error(), CompletedAt: time.Now().UTC()}
}

func errorText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func decodeJSON(r io.Reader, dst any) error {
	dec := json.NewDecoder(io.LimitReader(r, 1<<20))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return fmt.Errorf("decode request: %w", err)
	}
	return nil
}

func randomID() string {
	var b [12]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("p-%d", time.Now().UnixNano())
	}
	return "p-" + hex.EncodeToString(b[:])
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
