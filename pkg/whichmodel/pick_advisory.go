package whichmodel

import (
	"github.com/WD-Mitchell/which-model/internal/advisory"
	"github.com/WD-Mitchell/which-model/internal/config"
	"time"
)

func pickQuotaReport(st *runState, candidate *Candidate) advisory.Report {
	maxAge := st.fetchOptions.MaxAge
	if maxAge <= 0 {
		maxAge = 15 * time.Minute
		if st.fetchOptions.Backend != config.UsageBackend("codexbar") {
			maxAge = pickProviderFreshness(candidate.Route.Provider)
		}
	}
	return advisory.Evaluate(st.usageEnabled, st.snapshots[candidate.Route.Provider], candidate.Route.WindowIDs, maxAge, time.Now())
}
