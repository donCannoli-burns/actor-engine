package admission

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/donCannoli-burns/actor-engine/internal/preflight"
)

const Version = "kol-actor/admission-v1"

type Evidence struct {
	Version   string           `json:"version"`
	Digest    string           `json:"digest"`
	Preflight preflight.Result `json:"preflight"`
}

func Bind(result preflight.Result) (Evidence, error) {
	if result.Status != "READY" || !result.ReadyForProposal {
		return Evidence{}, fmt.Errorf("preflight is not READY")
	}
	if result.Authority.PreflightGrantsAuthority {
		return Evidence{}, fmt.Errorf("preflight unexpectedly grants authority")
	}
	if !result.Authority.HumanConfirmationRequired {
		return Evidence{}, fmt.Errorf("preflight unexpectedly removes human confirmation")
	}
	digest, err := Digest(result)
	if err != nil {
		return Evidence{}, err
	}
	return Evidence{Version: Version, Digest: digest, Preflight: result}, nil
}

func Verify(e Evidence) error {
	if e.Version != Version {
		return fmt.Errorf("admission version %q is unsupported", e.Version)
	}
	if e.Preflight.Status != "READY" || !e.Preflight.ReadyForProposal {
		return fmt.Errorf("admission preflight is not READY")
	}
	if e.Preflight.Authority.PreflightGrantsAuthority {
		return fmt.Errorf("admission preflight grants authority")
	}
	if !e.Preflight.Authority.HumanConfirmationRequired {
		return fmt.Errorf("admission preflight removes human confirmation")
	}
	want, err := Digest(e.Preflight)
	if err != nil {
		return err
	}
	if e.Digest != want {
		return fmt.Errorf("admission digest mismatch")
	}
	return nil
}

func Digest(result preflight.Result) (string, error) {
	canonical, err := canonicalJSON(result)
	if err != nil {
		return "", fmt.Errorf("canonicalize preflight: %w", err)
	}
	sum := sha256.Sum256(canonical)
	return "sha256:" + hex.EncodeToString(sum[:]), nil
}

func canonicalJSON(v any) ([]byte, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var generic any
	if err := json.Unmarshal(raw, &generic); err != nil {
		return nil, err
	}
	return json.Marshal(generic)
}
