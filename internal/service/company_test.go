package service

import (
	"context"
	"errors"
	"testing"

	"github.com/WD-Mitchell/which-model/internal/company"
)

func TestCompanyRefusesLegacyExecutionBeforeDiscovery(t *testing.T) {
	old := readCompanyPolicy
	t.Cleanup(func() { readCompanyPolicy = old })
	p := company.Defaults()
	readCompanyPolicy = func() (company.Snapshot, error) { return company.Snapshot{Managed: true, Policy: &p}, nil }
	// A nil service and missing binary would fail differently if either operation
	// touched its normal execution/discovery path before consulting policy.
	_, err := (&HarnessService{}).Launch(context.Background(), "CANARY", "CANARY", "CANARY")
	var denied *company.Error
	if !errors.As(err, &denied) {
		t.Fatal("launch reached mutable service state", err)
	}
	_, err = runProviderModelCommandDefault(context.Background(), "CANARY")
	if !errors.As(err, &denied) {
		t.Fatal("model discovery attempted a command", err)
	}
	if got := ToErrorDTO(err); got.Code != "validation_failed" {
		t.Fatal("wrong desktop boundary code", got)
	}
}
