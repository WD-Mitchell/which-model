package service

import (
	"github.com/WD-Mitchell/which-model/internal/company"
	"github.com/WD-Mitchell/which-model/internal/privacy"
)

// MaintenanceResult preserves partial category results even when cleanup fails.
type MaintenanceResult struct {
	Managed     bool                      `json:"managed"`
	Categories  map[string]privacy.Report `json:"categories"`
	Operation   string                    `json:"operation"`
	CompletedAt string                    `json:"completed_at"`
	Error       string                    `json:"error,omitempty"`
}
type AdministrationStatus struct {
	Policy          company.Snapshot   `json:"policy"`
	NativeKeychain  bool               `json:"native_keychain"`
	UseKeychain     bool               `json:"use_keychain"`
	LastMaintenance *MaintenanceResult `json:"last_maintenance,omitempty"`
}
type DelegationCheck struct {
	Path       string `json:"path"`
	ConfigPath string `json:"config_path"`
	Verified   bool   `json:"verified"`
	Error      string `json:"error,omitempty"`
}
