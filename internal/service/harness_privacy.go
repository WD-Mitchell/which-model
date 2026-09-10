package service

import (
	"encoding/json"
	"github.com/WD-Mitchell/which-model/internal/company"
	"github.com/WD-Mitchell/which-model/internal/privacy"
	"log"
	"os"
	"path/filepath"
)

// A nil file leaves exec.Cmd's stdout/stderr connected to the null device.
// Company mode never captures arbitrary harness payloads in a product log.
func (h *HarnessService) launchOutput(policy company.Snapshot) (*os.File, error) {
	if policy.Managed {
		return nil, nil
	}
	return os.OpenFile(filepath.Join(h.s.paths.StateDir, "launch.log"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
}

func (h *HarnessService) recordCompanyLaunch(slug, provider, model, profile, outcome string) error {
	policy, err := readCompanyPolicy()
	if err != nil {
		log.Print("company launch record unavailable")
		return err
	}
	if !policy.Managed {
		return nil
	}
	data, err := json.Marshal(map[string]any{"harness": slug, "provider": provider, "model_id": model, "profile": profile, "outcome": outcome})
	if err == nil {
		err = (privacy.Controller{Policy: policy}).Append(filepath.Join(h.s.paths.StateDir, "launch.jsonl"), privacy.Launch, data)
	}
	if err != nil {
		log.Print("company launch record failed; launch outcome is unchanged")
	}
	return err
}
