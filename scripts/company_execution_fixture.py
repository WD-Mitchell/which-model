#!/usr/bin/env python3
"""Owned execution fixtures on disposable native CI runners only."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import shutil
import sys
import company_policy_fixture as policy_fixture


def main():
    if os.environ.get('GITHUB_ACTIONS') != 'true':
        raise RuntimeError('execution fixtures require disposable GitHub Actions runners')
    parser = argparse.ArgumentParser()
    parser.add_argument('action', choices=['prepare', 'disable-integrations', 'cleanup'])
    parser.add_argument('--binary')
    parser.add_argument('--github-env')
    args = parser.parse_args()
    fixture, live = policy_fixture.locations()
    if args.action == 'cleanup':
        policy_fixture.cleanup()
        return
    if args.action == 'disable-integrations':
        path = live / 'managed' / 'policy.json'
        policy = json.loads(path.read_text())
        policy['integrations'] = {'skill_installation': False, 'hook_installation': False, 'hook_use': False}
        policy_fixture.write(path, policy)
        return
    policy_fixture.prepare(args.github_env)
    directory = fixture / 'execution'
    directory.mkdir()
    policy_fixture.protect(directory)
    image = directory / 'company-harness.exe'
    shutil.copyfile(Path(args.binary).resolve(), image)
    policy_fixture.protect(image)
    if sys.platform != 'win32':
        image.chmod(0o755)
    digest = hashlib.sha256(image.read_bytes()).hexdigest()
    for name in ['changed.exe', 'writable.exe']:
        path = directory / name
        shutil.copyfile(image, path)
        if name == 'changed.exe':
            with path.open('ab') as target:
                target.write(b'changed')
        policy_fixture.protect(path, writable=name == 'writable.exe')
        if sys.platform != 'win32' and name != 'writable.exe':
            path.chmod(0o755)
    os.symlink(image, directory / 'redirect.exe')
    asset = directory / 'approved-input.txt'
    asset.write_text('approved synthetic input\n')
    policy_fixture.protect(asset)
    output = Path.cwd() / 'company-execution-output.json'
    project = Path.cwd() / 'company-execution-project'
    if output.exists() or project.exists():
        raise RuntimeError('execution output fixture already exists')
    policy = {
        'schema_version': 1, 'allowed_providers': ['codex'],
        'integrations': {'skill_installation': True, 'hook_installation': True, 'hook_use': True},
        'executables': [{
            'id': 'codex', 'path': str(image), 'sha256': digest,
            'args': ['-test.run=^TestApprovedExecutionChildHelper$', '--', str(output), '{model_id}', '{reasoning}', '{provider}', '{profile}'],
            'inputs': [{'path': str(asset), 'sha256': hashlib.sha256(asset.read_bytes()).hexdigest()}]
        }]
    }
    policy_fixture.write(live / 'managed' / 'policy.json', policy)
    with open(args.github_env, 'a', encoding='utf-8') as env:
        env.write(f'COMPANY_EXECUTION_TEST_DIR={directory}\nCOMPANY_EXECUTION_OUTPUT={output}\nCOMPANY_EXECUTION_PROJECT={project}\n')


if __name__ == '__main__':
    main()
