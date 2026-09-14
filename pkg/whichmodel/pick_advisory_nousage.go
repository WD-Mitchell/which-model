//go:build nousage

package whichmodel

import "time"

func pickProviderFreshness(string) time.Duration { return 0 }
