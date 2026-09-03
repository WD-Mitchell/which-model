//go:build !nousage

package service

import (
	"github.com/WD-Mitchell/which-model/internal/config"
	"github.com/WD-Mitchell/which-model/internal/usage/provider/codexbar"
)

var discoverBackendProviderIDs = func(backend config.UsageBackend) []string {
	if backend != config.UsageBackendCodexBar {
		return nil
	}
	return codexbar.SupportedProviders()
}
