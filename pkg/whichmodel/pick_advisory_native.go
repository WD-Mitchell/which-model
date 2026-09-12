//go:build !nousage

package whichmodel

import (
	"github.com/WD-Mitchell/which-model/internal/usage"
	"time"
)

func pickProviderFreshness(provider string) time.Duration {
	if desc, err := usage.Get(provider); err == nil {
		return desc.CacheTTL
	}
	return 0
}
