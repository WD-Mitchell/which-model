//go:build nousage

package service

import (
	"context"

	"github.com/WD-Mitchell/which-model/internal/config"
	"github.com/WD-Mitchell/which-model/internal/routing"
)

var discoverBackendProviderIDs = func(config.UsageBackend) []string { return nil }

var discoverLiveProviderModels = func(context.Context, string) []routing.ModelEntry { return nil }
