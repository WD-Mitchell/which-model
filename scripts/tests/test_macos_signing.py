import importlib.util
import json
import os
import subprocess
import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch

spec = importlib.util.spec_from_file_location('macos_signing', Path(__file__).resolve().parents[1] / 'sign-macos.py')
signing = importlib.util.module_from_spec(spec)
spec.loader.exec_module(signing)


class MacOSSigningTests(unittest.TestCase):
    def test_missing_ci_credentials_fails_before_running_commands(self):
        with patch.object(signing, 'run') as run:
            with self.assertRaisesRegex(ValueError, 'Missing signing credential'):
                with signing.ci_identity({}):
                    self.fail('missing credentials accepted')
            run.assert_not_called()

    def test_tool_error_does_not_expose_password_or_command_output(self):
        secret = 'PASSWORD_CANARY'
        result = subprocess.CompletedProcess([], 1, secret, secret)
        with patch('subprocess.run', return_value=result):
            with self.assertRaises(RuntimeError) as error:
                signing.run(['security', 'import', '--password', secret], 'Certificate import')
        self.assertNotIn(secret, str(error.exception))

    def test_ci_cleanup_on_success_and_handled_failures(self):
        import base64
        credentials = {name: 'canary' for name in signing.CREDENTIALS}
        credentials.update(APPLE_TEAM_ID='WJUCH8XDFT',
                           BUILD_CERTIFICATE_BASE64=base64.b64encode(b'fake p12').decode())
        for failure in (None, 'Import signing certificate', 'Validate notarization credentials', 'signing body'):
            with self.subTest(failure=failure):
                calls = []
                paths = []
                def run(args, phase, **kwargs):
                    calls.append(args)
                    if phase == 'Import signing certificate':
                        paths.append(Path(args[2]))
                        if os.name == 'posix':
                            self.assertEqual(paths[-1].stat().st_mode & 0o777, 0o600)
                    if phase == failure:
                        raise RuntimeError('expected failure')
                    stdout = ''
                    if phase == 'Read keychain search list':
                        stdout = '"/tmp/original keychain"'
                    if phase == 'Find signing identity':
                        stdout = 'A' * 40 + ' "Developer ID Application: Example (WJUCH8XDFT)"'
                    return subprocess.CompletedProcess(args, 0, stdout, '')
                with patch.object(signing, 'run', side_effect=run):
                    try:
                        with signing.ci_identity(credentials) as identity:
                            self.assertEqual(identity[:2], ('A' * 40, 'WJUCH8XDFT'))
                            self.assertFalse(paths[0].exists())
                            if failure == 'signing body':
                                raise RuntimeError('expected failure')
                    except RuntimeError:
                        if failure is None:
                            raise
                    else:
                        self.assertIsNone(failure)
                self.assertEqual(calls[-2], ['security', 'list-keychains', '-d', 'user', '-s', '/tmp/original keychain'])
                self.assertEqual(calls[-1][:2], ['security', 'delete-keychain'])
                self.assertFalse(Path(calls[-1][-1]).parent.exists())
                self.assertTrue(all(not path.exists() for path in paths))

    def scenario(self, directory, *, status='Accepted', team='WJUCH8XDFT', fail=None):
        app = Path(directory) / 'which-model.app'
        exe = app / 'Contents/MacOS/which-model-desktop'
        exe.parent.mkdir(parents=True)
        exe.write_bytes(b'signed executable')
        import plistlib
        (app / 'Contents/Info.plist').write_bytes(plistlib.dumps({
            'CFBundleExecutable': exe.name, 'CFBundleIdentifier': 'com.wdmitchell.which-model'}))
        calls = []

        def run(args, phase, **kwargs):
            calls.append(args)
            if fail and fail in args:
                raise RuntimeError('tool failed')
            stdout = ''
            stderr = ''
            if args[:2] == ['codesign', '--display']:
                stderr = f'Authority=Developer ID Application: Example ({team})\nTeamIdentifier={team}\nflags=0x10000(runtime)\nTimestamp=Sep 14, 2026\n'
            if args[:3] == ['xcrun', 'notarytool', 'submit']:
                stdout = json.dumps({'id': '12345678-1234-1234-1234-123456789abc', 'status': status})
            if args[:3] == ['xcrun', 'notarytool', 'log']:
                Path(args[4]).write_text(json.dumps({'status': status}))
            return subprocess.CompletedProcess(args, 0, stdout, stderr)
        return app, calls, run

    def test_accepted_submission_is_stapled_and_assessed_before_receipt(self):
        with tempfile.TemporaryDirectory() as td:
            app, calls, run = self.scenario(td)
            receipt = Path(td) / 'receipt.json'
            with patch.object(signing, 'run', side_effect=run):
                signing.sign_app(app, 'identity', 'WJUCH8XDFT', 'profile', None, receipt)
            value = json.loads(receipt.read_text())
            self.assertEqual(value['status'], 'Accepted')
            self.assertEqual(value['team_id'], 'WJUCH8XDFT')
            self.assertEqual(value['executable_sha256'], signing.digest(app / 'Contents/MacOS/which-model-desktop'))
            self.assertTrue(any(c[:3] == ['xcrun', 'stapler', 'validate'] for c in calls))
            self.assertTrue(any(c[0] == 'spctl' for c in calls))
            self.assertLess(next(i for i,c in enumerate(calls) if c[0]=='codesign'),
                            next(i for i,c in enumerate(calls) if c[:3]==['xcrun','notarytool','submit']))

    def test_rejected_notarization_never_staples_or_writes_success(self):
        with tempfile.TemporaryDirectory() as td:
            app, calls, run = self.scenario(td, status='Invalid')
            receipt = Path(td) / 'receipt.json'
            with patch.object(signing, 'run', side_effect=run):
                with self.assertRaisesRegex(RuntimeError, 'not accepted'):
                    signing.sign_app(app, 'identity', 'WJUCH8XDFT', 'profile', None, receipt)
            self.assertFalse(receipt.exists())
            self.assertFalse(any(c[:2] == ['xcrun', 'stapler'] for c in calls))

    def test_wrong_team_refuses_submission(self):
        with tempfile.TemporaryDirectory() as td:
            app, calls, run = self.scenario(td, team='OTHERTEAM00')
            with patch.object(signing, 'run', side_effect=run):
                with self.assertRaisesRegex(ValueError, 'signing identity'):
                    signing.sign_app(app, 'identity', 'WJUCH8XDFT', 'profile', None, Path(td)/'receipt.json')
            self.assertFalse(any(c[:2] == ['xcrun','notarytool'] for c in calls))

    def test_each_signing_or_verification_failure_blocks_success(self):
        for failure in ('--sign', 'staple', 'validate', '--assess'):
            with self.subTest(failure=failure), tempfile.TemporaryDirectory() as td:
                app, calls, run = self.scenario(td, fail=failure)
                receipt = Path(td)/'receipt.json'
                with patch.object(signing, 'run', side_effect=run):
                    with self.assertRaises(RuntimeError):
                        signing.sign_app(app, 'identity', 'WJUCH8XDFT', 'profile', None, receipt)
                self.assertFalse(receipt.exists())
