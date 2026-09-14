#!/usr/bin/env bash
set -euo pipefail
# The native webview imports transport/OS support. Constrain project packages,
# not Wails' standard-library implementation; the restricted CLI keeps its audit.
go list -tags nousage -deps ./cmd/which-model-score-only-desktop | python3 -c '
import sys
prefix="github.com/WD-Mitchell/which-model/"
allowed={"cmd/which-model-score-only-desktop","pkg/offlinedesktop","pkg/scoreonly","data","internal/catalog","internal/catalog/score","internal/catalog/csvschema","internal/catalog/identity","internal/config","internal/decimal","internal/output","internal/pick","internal/routing","internal/usage"}
packages={s.strip()[len(prefix):] for s in sys.stdin if s.startswith(prefix)}
bad=packages-allowed
if bad: raise SystemExit("Unexpected offline desktop dependencies: "+", ".join(sorted(bad)))
print("Offline desktop excludes full services, provider adapters, credentials, company policy and execution.")
'
go test -tags nousage ./pkg/offlinedesktop
