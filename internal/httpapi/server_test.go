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
	"strings"
	"testing"

	"github.com/donCannoli-burns/actor-engine/internal/gate"
	"github.com/donCannoli-burns/actor-engine/internal/kingdomsitter"
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
	s := New(
		plane,
		gate.New(),
		release.NewClient(upstream.URL+"/latest"),
		kingdomsitter.NewClient(upstream.URL),
		stageDir,
	)
	h := s.Handler()

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
	got, err := os.ReadFile(filepath.Join(stageDir, "KoLmafia-29309.jar"))
	if err != nil {
		t.Fatalf("read staged file: %v", err)
	}
	if string(got) != string(jar) {
		t.Fatal("staged file contents differ from upstream fixture")
	}
}
