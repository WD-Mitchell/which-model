#!/usr/bin/env python3
"""Developer ID sign and notarize both desktop products before release inventory."""
import argparse
import base64
from contextlib import contextmanager
import hashlib
import json
import os
from pathlib import Path
import plistlib
import re
import secrets
import shlex
import subprocess
import sys
import tempfile

CREDENTIALS = ('BUILD_CERTIFICATE_BASE64', 'P12_PASSWORD', 'APPLE_ID',
               'APPLE_TEAM_ID', 'APPLE_APP_SPECIFIC_PASSWORD')


def digest(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def run(args, phase, timeout=120, allow_failure=False):
    # Neither command arguments nor raw tool diagnostics may expose credentials.
    env = {k: v for k, v in os.environ.items() if k not in CREDENTIALS}
    try:
        result = subprocess.run(args, capture_output=True, text=True, timeout=timeout, env=env)
    except (OSError, subprocess.TimeoutExpired):
        raise RuntimeError(f'{phase} could not complete') from None
    if result.returncode and not allow_failure:
        raise RuntimeError(f'{phase} failed (exit {result.returncode})')
    return result


@contextmanager
def ci_identity(credentials):
    for name in CREDENTIALS:
        if not credentials.get(name):
            raise ValueError(f'Missing signing credential: {name}')
    team = credentials['APPLE_TEAM_ID']
    if not re.fullmatch(r'[A-Z0-9]{10}', team):
        raise ValueError('Invalid Apple team ID')
    try:
        certificate = base64.b64decode(''.join(credentials['BUILD_CERTIFICATE_BASE64'].split()), validate=True)
    except ValueError:
        raise ValueError('Invalid signing certificate encoding') from None
    if not certificate:
        raise ValueError('Empty signing certificate')
    original = shlex.split(run(['security', 'list-keychains', '-d', 'user'], 'Read keychain search list').stdout)
    with tempfile.TemporaryDirectory(prefix='which-model-signing-', dir=os.environ.get('RUNNER_TEMP')) as td:
        root = Path(td)
        keychain = str(root/'signing.keychain-db')
        password = secrets.token_urlsafe(32)
        profile = 'which-model-ci-notary'
        p12 = root/'certificate.p12'
        p12.write_bytes(certificate)
        p12.chmod(0o600)
        created = False
        try:
            run(['security', 'create-keychain', '-p', password, keychain], 'Create signing keychain')
            created = True
            run(['security', 'set-keychain-settings', '-lut', '3600', keychain], 'Configure signing keychain')
            run(['security', 'unlock-keychain', '-p', password, keychain], 'Unlock signing keychain')
            run(['security', 'import', str(p12), '-k', keychain, '-P', credentials['P12_PASSWORD'],
                 '-T', '/usr/bin/codesign'], 'Import signing certificate')
            p12.unlink()
            run(['security', 'set-key-partition-list', '-S', 'apple-tool:,apple:', '-s', '-k', password, keychain],
                'Authorize signing tool')
            run(['security', 'list-keychains', '-d', 'user', '-s', keychain, *original], 'Enable signing keychain')
            identities = run(['security', 'find-identity', '-v', '-p', 'codesigning', keychain], 'Find signing identity').stdout
            matches = re.findall(r'([A-Fa-f0-9]{40}) "Developer ID Application: [^"\n]+ \(' + re.escape(team) + r'\)"', identities)
            if len(matches) != 1:
                raise ValueError('Certificate must contain exactly one Developer ID Application identity for the Apple team')
            run(['xcrun', 'notarytool', 'store-credentials', profile, '--keychain', keychain,
                 '--apple-id', credentials['APPLE_ID'], '--team-id', team,
                 '--password', credentials['APPLE_APP_SPECIFIC_PASSWORD']], 'Validate notarization credentials')
            yield matches[0], team, profile, keychain
        finally:
            if created:
                try:
                    run(['security', 'list-keychains', '-d', 'user', '-s', *original], 'Restore keychain search list')
                finally:
                    run(['security', 'delete-keychain', keychain], 'Delete signing keychain')


def sign_app(app, identity, team, profile, keychain, receipt):
    receipt.parent.mkdir(parents=True, exist_ok=True)
    receipt.unlink(missing_ok=True)
    log_path = receipt.with_suffix('.log.json')
    log_path.unlink(missing_ok=True)
    info = plistlib.loads((app/'Contents/Info.plist').read_bytes())
    executable_name = info['CFBundleExecutable']
    if Path(executable_name).name != executable_name:
        raise ValueError('Invalid bundle executable')
    executable = app/'Contents/MacOS'/executable_name
    if not executable.is_file():
        raise ValueError('Missing bundle executable')
    keychain_arg = ['--keychain', keychain] if keychain else []
    run(['codesign', '--force', '--options', 'runtime', '--timestamp', '--sign', identity,
         *keychain_arg, str(app)], 'Developer ID signing')
    run(['codesign', '--verify', '--deep', '--strict', str(app)], 'Signature verification')
    details = run(['codesign', '--display', '--verbose=4', str(app)], 'Inspect signing identity').stderr
    authority = next((line.removeprefix('Authority=') for line in details.splitlines()
                      if line.startswith('Authority=Developer ID Application: ')), None)
    if (not authority or f'TeamIdentifier={team}' not in details.splitlines()
            or '(runtime)' not in details or not any(line.startswith('Timestamp=') for line in details.splitlines())):
        raise ValueError('Invalid signing identity, hardened runtime or secure timestamp')
    with tempfile.TemporaryDirectory(prefix='which-model-notary-') as td:
        upload = Path(td)/'submission.zip'
        run(['ditto', '-c', '-k', '--keepParent', str(app), str(upload)], 'Prepare notarization archive')
        auth = ['--keychain-profile', profile, *keychain_arg]
        result = run(['xcrun', 'notarytool', 'submit', str(upload), *auth, '--wait', '--timeout', '20m',
                      '--output-format', 'json'], 'Apple notarization', timeout=1260, allow_failure=True)
        submission = json.loads(result.stdout)
        submission_id = submission.get('id', '')
        if not re.fullmatch(r'[a-fA-F0-9]{8}(?:-[a-fA-F0-9]{4}){3}-[a-fA-F0-9]{12}', submission_id):
            raise ValueError('Missing notarization submission ID')
        run(['xcrun', 'notarytool', 'log', submission_id, str(log_path), *auth], 'Retrieve notarization log')
        if result.returncode or submission.get('status') != 'Accepted':
            raise RuntimeError('Apple notarization not accepted; inspect the notarization log')
    run(['xcrun', 'stapler', 'staple', str(app)], 'Staple notarization ticket')
    run(['xcrun', 'stapler', 'validate', str(app)], 'Validate notarization ticket')
    run(['codesign', '--verify', '--deep', '--strict', str(app)], 'Verify stapled app signature')
    run(['spctl', '--assess', '--type', 'execute', '--verbose=4', str(app)], 'Gatekeeper assessment')
    receipt.write_text(json.dumps({
        'schema': 1, 'status': 'Accepted', 'submission_id': submission_id, 'team_id': team,
        'authority': authority, 'bundle_id': info['CFBundleIdentifier'],
        'executable_sha256': digest(executable), 'ticket_stapled': True, 'gatekeeper_accepted': True,
    }, indent=2, sort_keys=True)+'\n')
    print(f'{app.name}: Developer ID signed, notarized, stapled and Gatekeeper accepted.', flush=True)


def sign_products(identity, team, profile, keychain, arch, output):
    for product, app in [('desktop', 'which-model.app'), ('offline-desktop', 'which-model-offline.app')]:
        sign_app(Path('bin')/app, identity, team, profile, keychain,
                 output/f'which-model-{product}-darwin-{arch}.zip.notarization.json')


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--ci', action='store_true')
    parser.add_argument('--arch', required=True, choices=['arm64', 'x64'])
    parser.add_argument('--output-dir', type=Path, required=True)
    parser.add_argument('--identity')
    parser.add_argument('--team-id')
    parser.add_argument('--keychain-profile', default='which-model-notary')
    args = parser.parse_args()
    if args.ci:
        with ci_identity(os.environ) as identity:
            sign_products(*identity, args.arch, args.output_dir)
    else:
        if not args.identity or not re.fullmatch(r'[A-Z0-9]{10}', args.team_id or ''):
            parser.error('--identity and --team-id are required for local signing')
        sign_products(args.identity, args.team_id, args.keychain_profile, None, args.arch, args.output_dir)


if __name__ == '__main__':
    try:
        main()
    except RuntimeError as error:
        print(f'macOS signing: {error}', file=sys.stderr)
        sys.exit(1)
    except (ValueError, OSError, KeyError):
        # Detailed Apple rejection reasons are in the dedicated notary log. Never
        # print arbitrary exceptions from credential handling or subprocess args.
        print('macOS signing failed; check the signing inputs and notarization logs.', file=sys.stderr)
        sys.exit(1)
