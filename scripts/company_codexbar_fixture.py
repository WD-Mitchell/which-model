#!/usr/bin/env python3
"""Protect a synthetic delegated native image/config on disposable CI runners."""
import argparse
import hashlib
import os
from pathlib import Path
import shutil
import sys
import company_policy_fixture as policy_fixture


def main():
    if os.environ.get('GITHUB_ACTIONS') != 'true':
        raise RuntimeError('CodexBar fixture requires a disposable GitHub Actions runner')
    parser = argparse.ArgumentParser()
    parser.add_argument('action', choices=['prepare', 'cleanup'])
    parser.add_argument('--binary')
    parser.add_argument('--github-env')
    args = parser.parse_args()
    if args.action == 'cleanup':
        policy_fixture.cleanup()
        return
    policy_fixture.prepare(args.github_env)
    fixture, live = policy_fixture.locations()
    directory = fixture / 'codexbar'
    directory.mkdir()
    policy_fixture.protect(directory)
    image = directory / 'approved.exe'
    shutil.copyfile(Path(args.binary).resolve(), image)
    policy_fixture.protect(image)
    if sys.platform != 'win32':
        image.chmod(0o755)
    output = Path.cwd() / 'company-codexbar-output.json'
    if output.exists():
        raise RuntimeError('native capture path already exists')
    config = directory / 'approved.json'
    policy_fixture.write(config, {'version': 1, 'fixtureOutput': str(output)})
    for kind, original, extension in [('image', image, 'exe'), ('config', config, 'json')]:
        for condition in ['changed', 'writable']:
            path = directory / f'{condition}-{kind}.{extension}'
            shutil.copyfile(original, path)
            if condition == 'changed':
                with path.open('ab') as target:
                    target.write(b'changed')
            policy_fixture.protect(path, writable=condition == 'writable')
            if kind == 'image' and sys.platform != 'win32' and condition != 'writable':
                path.chmod(0o755)
        os.symlink(original, directory / f'redirect-{kind}.{extension}')
    policy_fixture.write(live / 'managed' / 'policy.json', {
        'schema_version': 1, 'allowed_providers': ['antigravity'],
        'codexbar_installations': [{
            'path': str(image), 'sha256': hashlib.sha256(image.read_bytes()).hexdigest(),
            'config': {'path': str(config), 'sha256': hashlib.sha256(config.read_bytes()).hexdigest()}
        }]
    })
    with open(args.github_env, 'a', encoding='utf-8') as env:
        env.write(f'COMPANY_CODEXBAR_TEST_DIR={directory}\nCOMPANY_CODEXBAR_OUTPUT={output}\n')


if __name__ == '__main__':
    main()
