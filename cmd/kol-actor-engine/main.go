package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/donCannoli-burns/actor-engine/internal/actor"
	"github.com/donCannoli-burns/actor-engine/internal/audit"
	"github.com/donCannoli-burns/actor-engine/internal/gate"
	"github.com/donCannoli-burns/actor-engine/internal/httpapi"
	"github.com/donCannoli-burns/actor-engine/internal/identity"
	"github.com/donCannoli-burns/actor-engine/internal/kingdomsitter"
	"github.com/donCannoli-burns/actor-engine/internal/kolstate"
	"github.com/donCannoli-burns/actor-engine/internal/release"
	"github.com/donCannoli-burns/actor-engine/internal/stateplane"
)

type config struct {
	listenURL      string
	jarPath        string
	kingdomsitter  string
	latestRelease  string
	stageDir       string
	auditLog       string
	refreshRelease time.Duration
	refreshSidecar time.Duration
}

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	cfg := parseFlags()
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	plane := stateplane.New(stateplane.StateBooting, stateplane.StateObserveOnly)
	g := gate.New()
	runtimeID, err := identity.NewRuntimeID()
	if err != nil {
		return err
	}
	ledger, err := audit.Open(cfg.auditLog)
	if err != nil {
		return fmt.Errorf("open audit ledger: %w", err)
	}
	if _, err := ledger.Append(audit.Event{
		Type:      audit.EventRuntimeStarted,
		RuntimeID: runtimeID,
		Result:    "ok",
		Detail:    "process started; durable evidence loaded; proposal and confirmation authority not restored",
	}); err != nil {
		return fmt.Errorf("record runtime start: %w", err)
	}
	releases := release.NewClient(cfg.latestRelease)
	kingdom := kingdomsitter.NewClient(cfg.kingdomsitter)
	api := httpapi.New(plane, g, releases, kingdom, cfg.stageDir, ledger, runtimeID)
	if cfg.jarPath != "" {
		rev, err := kolstate.InstalledRevision(cfg.jarPath)
		if err != nil {
			return err
		}
		api.SetInstalledRevision(rev)
	}

	sys := actor.NewSystem()
	if err := spawnMonitors(ctx, sys, cfg, api); err != nil {
		return err
	}

	server := &http.Server{
		Addr:              cfg.listenURL,
		Handler:           api.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}
	plane.Deactivate(stateplane.StateBooting)
	plane.Activate(stateplane.StateReady)

	errCh := make(chan error, 1)
	go func() {
		errCh <- server.ListenAndServe()
	}()
	log.Printf("kol-actor-engine listening on %s", cfg.listenURL)

	select {
	case <-ctx.Done():
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("shutdown HTTP server: %w", err)
		}
		return nil
	case err := <-errCh:
		if err == http.ErrServerClosed {
			return nil
		}
		return fmt.Errorf("serve HTTP: %w", err)
	}
}

func spawnMonitors(ctx context.Context, sys *actor.System, cfg config, api *httpapi.Server) error {
	if err := sys.Spawn(ctx, "release-watcher", func(ctx context.Context, msg actor.Message) error {
		if msg.Topic != "tick" {
			return nil
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://"+cfg.listenURL+"/v1/release/refresh", nil)
		if err != nil {
			return err
		}
		resp, err := http.DefaultClient.Do(req)
		if err == nil && resp != nil {
			_ = resp.Body.Close()
		}
		return err
	}); err != nil {
		return err
	}
	if err := sys.Spawn(ctx, "kingdomsitter-watcher", func(ctx context.Context, msg actor.Message) error {
		if msg.Topic != "tick" {
			return nil
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+cfg.listenURL+"/v1/kingdomsitter/refresh", nil)
		if err != nil {
			return err
		}
		resp, err := http.DefaultClient.Do(req)
		if err == nil && resp != nil {
			_ = resp.Body.Close()
		}
		return err
	}); err != nil {
		return err
	}

	go tickerLoop(ctx, cfg.refreshRelease, func() { _ = sys.Send(ctx, "release-watcher", actor.Message{Topic: "tick"}) })
	go tickerLoop(ctx, cfg.refreshSidecar, func() { _ = sys.Send(ctx, "kingdomsitter-watcher", actor.Message{Topic: "tick"}) })
	_ = api
	return nil
}

func tickerLoop(ctx context.Context, d time.Duration, fn func()) {
	if d <= 0 {
		return
	}
	fn()
	t := time.NewTicker(d)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			fn()
		}
	}
}

func parseFlags() config {
	var cfg config
	flag.StringVar(&cfg.listenURL, "listen", "127.0.0.1:10424", "local HTTP listen address")
	flag.StringVar(&cfg.jarPath, "kolmafia_jar", "", "optional installed KoLmafia jar to inspect")
	flag.StringVar(&cfg.kingdomsitter, "kingdomsitter", "http://127.0.0.1:10423", "kingdomsitter sidecar base URL")
	flag.StringVar(&cfg.latestRelease, "release_url", "https://api.github.com/repos/kolmafia/kolmafia/releases/latest", "latest KoLmafia release API URL")
	flag.StringVar(&cfg.stageDir, "stage_dir", "./staging", "directory for confirmed release staging")
	flag.StringVar(&cfg.auditLog, "audit_log", "", "hash-chained JSONL evidence ledger; defaults beside stage_dir")
	flag.DurationVar(&cfg.refreshRelease, "release_refresh", 15*time.Minute, "release monitor cadence; 0 disables")
	flag.DurationVar(&cfg.refreshSidecar, "sidecar_refresh", 30*time.Second, "kingdomsitter monitor cadence; 0 disables")
	flag.Parse()
	if cfg.auditLog == "" {
		cfg.auditLog = filepath.Join(filepath.Dir(filepath.Clean(cfg.stageDir)), "audit.jsonl")
	}
	return cfg
}
