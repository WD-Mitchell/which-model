//go:build nousage

// Package offlinedesktop exposes only bundled, in-memory ranking operations.
// It does not import the full desktop service or provider registries.
package offlinedesktop

import (
	"bytes"
	"encoding/json"
	"errors"
	"strconv"

	"github.com/WD-Mitchell/which-model/pkg/scoreonly"
)

type API struct{}

func run(args ...string) (json.RawMessage, error) {
	var out, diagnostic bytes.Buffer
	if scoreonly.Run(args, &out, &diagnostic) != 0 {
		return nil, errors.New(diagnostic.String())
	}
	return json.RawMessage(out.Bytes()), nil
}
func (a *API) Profiles() ([]string, error) {
	raw, err := run("profiles", "--json")
	if err != nil {
		return nil, err
	}
	var result struct {
		Profiles []string `json:"profiles"`
	}
	err = json.Unmarshal(raw, &result)
	return result.Profiles, err
}
func (a *API) Rank(profile string, top int) (json.RawMessage, error) {
	if len(profile) > 128 || top < 1 || top > 100 {
		return nil, errors.New("choose a built-in profile and between 1 and 100 results")
	}
	return run("pick", "--profile", profile, "--top", strconv.Itoa(top), "--json")
}
func (a *API) Capabilities() (json.RawMessage, error) {
	raw, err := run("capabilities", "--json")
	if err != nil {
		return nil, err
	}
	var result map[string]any
	if err = json.Unmarshal(raw, &result); err != nil {
		return nil, err
	}
	result["artifact"] = "which-model-score-only-desktop"
	result["runtime_boundary"] = "Bundled ranking only. The native webview/runtime is additional to the restricted CLI; OS webview storage and local application transport are outside the ranking engine's no-I/O boundary. No provider, credential, company-policy, update, or execution services are registered."
	// The engine excludes network and persistence; the GUI uses a native webview.
	// Do not carry the CLI's process-wide exclusion claim into a different host.
	result["excluded"] = []string{"provider_authentication", "provider_usage", "codexbar", "harness_execution", "skill_installation", "hook_installation", "configuration_files", "external_catalog_refresh"}
	return json.Marshal(result)
}
