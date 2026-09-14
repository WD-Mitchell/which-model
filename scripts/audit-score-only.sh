#!/usr/bin/env bash
set -euo pipefail
task_tmp="$(mktemp -d)"
trap 'rm -rf "$task_tmp"' EXIT
task_suffix=""
case "$(go env GOOS)" in windows) task_suffix=".exe" ;; esac
task_binary="$task_tmp/which-model-score-only${task_suffix}"
CGO_ENABLED=0 go build -trimpath -buildvcs=false -tags nousage -o "$task_binary" ./cmd/which-model-score-only
go test -tags nousage ./pkg/scoreonly ./internal/pick ./internal/catalog/score
go list -tags nousage -deps ./cmd/which-model-score-only > "$task_tmp/deps.txt"
go tool nm "$task_binary" > "$task_tmp/symbols.txt"
python3 - "$task_tmp" "$task_binary" <<'PY'
from pathlib import Path
import sys
p = Path(sys.argv[1])
deps = (p / 'deps.txt').read_text().splitlines()
for package in deps:
    assert package not in ('os/exec', 'net/http', 'net'), package
    assert not any(x in package for x in ('/usage/credential', '/usage/fetch', '/usage/provider/', '/usage/codexbar',
                                          '/harness', '/hooks', '/skills', '/catalog/fetch', '/catalog/csvstore', '/internal/service', '/pkg/whichmodel',
                                          'go-keyring', 'go-gh')), package
symbols = (p / 'symbols.txt').read_text()
for name in ('net.Dial', 'net/http.(*Client).do', 'os/exec.', 'internal/config.Load',
             'internal/routing.LoadTable', 'internal/routing.SaveTable'):
    assert name not in symbols, name
binary = Path(sys.argv[2]).read_bytes()
for endpoint in (b'chatgpt.com/backend-api', b'api.anthropic.com', b'copilot_internal'):
    assert endpoint not in binary, endpoint
print('Restricted import, linked-symbol and provider-endpoint audits passed.')
PY
python3 scripts/score_only_smoke.py "$task_binary"
