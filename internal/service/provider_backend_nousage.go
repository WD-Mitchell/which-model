//go:build nousage

package service

import "github.com/WD-Mitchell/which-model/internal/config"

var discoverBackendProviderIDs = func(config.UsageBackend) []string { return nil }
