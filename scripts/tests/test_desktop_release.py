import importlib.util
import json
from pathlib import Path
import sys
import tempfile
import unittest

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))
from release_metadata import digest
spec = importlib.util.spec_from_file_location('desktop_release', Path(__file__).resolve().parents[1] / 'desktop-release-metadata.py')
desktop = importlib.util.module_from_spec(spec)
spec.loader.exec_module(desktop)
assets_spec = importlib.util.spec_from_file_location('offline_assets', Path(__file__).resolve().parents[1] / 'copy-offline-assets.py')
assets = importlib.util.module_from_spec(assets_spec)
assets_spec.loader.exec_module(assets)

class DesktopReleaseTests(unittest.TestCase):
    def fixture(self, directory):
        identity = dict(version='2.6.0-beta.1', source_digest='a'*40, source_ref='refs/heads/review')
        (directory/'release-manifest.json').write_text(json.dumps(dict(identity, schema=1, artifacts=[])))
        for arch in ('arm64','x64'):
            rows=[]
            for product in ('desktop','offline-desktop'):
                name=f'which-model-{product}-darwin-{arch}.zip'
                (directory/name).write_bytes(b'archive')
                (directory/(name+'.cdx.json')).write_bytes(b'inventory')
                (directory/(name+'.govulncheck.txt')).write_bytes(b'No vulnerabilities found.')
                rows.append(dict(name=name,sha256=digest(b'archive'),sbom=name+'.cdx.json',sbom_sha256=digest(b'inventory')))
            (directory/f'desktop-manifest-{arch}.json').write_text(json.dumps(dict(identity,artifacts=rows)))
        return identity

    def test_merge_requires_both_products_on_both_architectures(self):
        with tempfile.TemporaryDirectory() as td:
            p=Path(td);i=self.fixture(p)
            desktop.merge(p,i['version'],i['source_digest'],i['source_ref'])
            m=json.loads((p/'release-manifest.json').read_text())
            self.assertEqual(len(m['artifacts']),4)
            self.assertIn('which-model-offline-desktop-darwin-x64.zip',(p/'checksums.txt').read_text())

    def test_tampered_bundle_refuses_combined_publication(self):
        with tempfile.TemporaryDirectory() as td:
            p=Path(td);i=self.fixture(p)
            (p/'which-model-desktop-darwin-arm64.zip').write_bytes(b'tampered')
            with self.assertRaisesRegex(ValueError,'checksum'):
                desktop.merge(p,i['version'],i['source_digest'],i['source_ref'])

    def test_wrong_source_refuses_combined_publication(self):
        with tempfile.TemporaryDirectory() as td:
            p=Path(td);i=self.fixture(p)
            part=p/'desktop-manifest-x64.json';m=json.loads(part.read_text());m['source_digest']='b'*40;part.write_text(json.dumps(m))
            with self.assertRaisesRegex(ValueError,'identity'):
                desktop.merge(p,i['version'],i['source_digest'],i['source_ref'])

    def test_offline_asset_copy_excludes_full_desktop_entries(self):
        with tempfile.TemporaryDirectory() as td:
            p=Path(td);source=p/'source';dest=p/'dest';(source/'.vite').mkdir(parents=True)
            (source/'.vite/manifest.json').write_text(json.dumps({'offline.html':{'file':'offline.js','imports':['shared']},'shared':{'file':'runtime.js'},'settings.html':{'file':'settings.js'}}))
            for name in ('offline.html','offline.js','runtime.js','settings.js'):(source/name).write_text(name)
            assets.copy_assets(source,dest)
            self.assertEqual({f.name for f in dest.iterdir()},{'offline.html','offline.js','runtime.js'})

    def test_offline_asset_copy_refuses_path_escape(self):
        with tempfile.TemporaryDirectory() as td:
            p=Path(td);(p/'.vite').mkdir();(p/'.vite/manifest.json').write_text(json.dumps({'offline.html':{'file':'../outside'}}))
            with self.assertRaisesRegex(ValueError,'unsafe'):
                assets.copy_assets(p,p/'dest')
