#!/usr/bin/env python3
"""Exercise the restricted binary with hostile ambient configuration.

On Linux, --trace also requires a separate network namespace and checks strace
evidence for application-file, network and child-process side effects.
"""
import argparse
import hashlib
import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import tempfile


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('binary', type=Path)
    parser.add_argument('--trace', action='store_true')
    args = parser.parse_args()
    binary = args.binary.resolve(strict=True)
    if args.trace:
        assert os.readlink('/proc/self/ns/net') != os.readlink('/proc/1/ns/net'), 'a separate network namespace is required'
        assert shutil.which('strace'), 'strace is required'
    with tempfile.TemporaryDirectory(prefix='score-only-canary-') as temp:
        root = Path(temp)
        home, project, helpers = (root / n for n in ('home', 'project', 'helpers'))
        for directory in (home, project, helpers):
            directory.mkdir()
        # Invalid inputs deliberately fail if any full-product loader is reached.
        for relative in ('.codex/auth.json', '.claude/.credentials.json', '.config/github-copilot/apps.json',
                         '.config/which-model/config.toml', '.config/which-model/credentials.json',
                         'Library/Application Support/which-model/config.toml'):
            file = home / relative
            file.parent.mkdir(parents=True, exist_ok=True)
            file.write_text('SYNTHETIC_CREDENTIAL_CANARY invalid data', encoding='utf-8')
        for name in ('which-model.toml', '.which-model/config.toml', 'config.toml'):
            (project / name).parent.mkdir(parents=True, exist_ok=True)
            (project / name).write_text('invalid TOML [; launch must not run', encoding='utf-8')
        for name in ('codexbar', 'CodexBar', 'codex', 'claude', 'gh', 'sh', 'cmd.exe'):
            file = helpers / name
            file.write_text('#!/bin/sh\nexit 97\n', encoding='utf-8')
            file.chmod(0o755)
        # These are child-only test environments. Real user credentials are never
        # inherited; Windows system variables remain for the native process loader.
        clean = {key: os.environ[key] for key in ('SystemRoot', 'WINDIR', 'COMSPEC', 'TEMP', 'TMP') if key in os.environ}
        clean.update({'HOME': str(home), 'USERPROFILE': str(home), 'XDG_CONFIG_HOME': str(home / '.config'),
                      'APPDATA': str(home), 'LOCALAPPDATA': str(home), 'PATH': str(helpers)})
        hostile = dict(clean, WHICH_MODEL_USAGE_ENABLED='true', WHICH_MODEL_USAGE_BACKEND='codexbar',
                       WHICH_MODEL_CONFIG=str(project / 'which-model.toml'),
                       CODEX_HOME=str(home / '.codex'), CLAUDE_CONFIG_DIR=str(home / '.claude'),
                       CODEX_ACCESS_TOKEN='SYNTHETIC_ENV_CANARY', CLAUDE_ACCESS_TOKEN='SYNTHETIC_ENV_CANARY',
                       WHICH_MODEL_CLAUDE_OAUTH_TOKEN='SYNTHETIC_ENV_CANARY',
                       GH_TOKEN='SYNTHETIC_ENV_CANARY', GITHUB_TOKEN='SYNTHETIC_ENV_CANARY',
                       HTTP_PROXY='http://127.0.0.1:1', HTTPS_PROXY='http://127.0.0.1:1')
        before = {str(p): hashlib.sha256(p.read_bytes()).hexdigest() for p in root.rglob('*') if p.is_file()}
        traces = 0

        def run(command, expected=0, env=hostile):
            nonlocal traces
            argv = [str(binary), *command]
            # Trace files are outside the canary tree; only the harness writes them.
            with tempfile.TemporaryDirectory(prefix='score-only-trace-') as trace_dir:
                trace = Path(trace_dir) / 'trace.txt'
                if args.trace:
                    argv = [shutil.which('strace'), '-f', '-qq', '-e', 'trace=%file,%network,%process', '-o', str(trace), '--', *argv]
                result = subprocess.run(argv, cwd=project, env=env, capture_output=True, text=True, timeout=20)
                assert result.returncode == expected, (command, result.returncode, result.stderr)
                assert 'SYNTHETIC_' not in result.stdout + result.stderr
                if args.trace:
                    evidence = trace.read_text(encoding='utf-8')
                    assert len(re.findall(r'\bexecve\(', evidence)) == 1, evidence
                    assert not re.search(r'\b(execveat|fork|vfork|socket|socketpair|connect|sendto|sendmsg|recvfrom|recvmsg|bind|listen|accept|accept4)\(', evidence), evidence
                    for line in evidence.splitlines():
                        if re.search(r'\bclone3?\(', line):
                            assert 'CLONE_THREAD' in line, line
                    # All credential/config locations live under these roots;
                    # neither reads nor probes (including failed stat/open) are allowed.
                    assert str(root) not in evidence, evidence
                    traces += 1
                return result.stdout

        plain = run(['pick', '--json'], env=clean)
        assert plain == run(['pick', '--json']) == run(['pick', '--json'])
        doc = json.loads(plain)
        assert doc['artifact'] == 'which-model-score-only'
        assert doc['usage_enabled'] is False and doc['usage_disabled_reason'] == 'compiled_out'
        assert doc['ranking']['candidate_count'] > 0 and 'unverified' in doc['notice']
        manifest = json.loads(run(['capabilities', '--json']))
        source = Path(__file__).resolve().parents[1] / manifest['catalog']['source_path']
        assert hashlib.sha256(source.read_bytes()).hexdigest() == manifest['catalog']['sha256'] == doc['catalog_sha256']
        assert manifest['profiles']['sha256'] == doc['profiles_sha256']
        assert {'network','provider_authentication','provider_usage','codexbar','harness_execution','skill_installation','hook_installation','configuration_files','persistence'} == set(manifest['excluded'])
        for command in (['version','--json'], ['profiles','--json'], ['help']):
            run(command)
        for command in (['auth','login'], ['usage'], ['catalog','refresh'], ['routes'], ['codexbar'],
                        ['skills','install'], ['hooks','install'], ['launch'], ['run'],
                        ['pick','--config','untrusted.toml'], ['pick','--usage'], ['pick','--harness','custom']):
            assert run(command, 2) == ''
        after = {str(p): hashlib.sha256(p.read_bytes()).hexdigest() for p in root.rglob('*') if p.is_file()}
        assert before == after, 'restricted invocation changed the canary filesystem'
        print(f'Restricted ranking and capability refusals passed; {traces} isolated syscall traces checked.')


if __name__ == '__main__':
    main()
