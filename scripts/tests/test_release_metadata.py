"""Inventories describe the built artifact, including replacement modules."""
import hashlib
from pathlib import Path
import sys
import unittest
sys.path.insert(0, str(Path(__file__).resolve().parents[1]))
from release_metadata import make_sbom

class ReleaseMetadataTests(unittest.TestCase):
    def test_artifact_runtime_and_linked_modules(self):
        info = {"GoVersion":"go1.26.6", "Deps":[{"Path":"example.org/lib", "Version":"v1.2.3", "Sum":"h1:synthetic"}], "Settings":[{"Key":"GOOS","Value":"windows"},{"Key":"GOARCH","Value":"amd64"}]}
        bom=make_sbom('binary.exe',b'bytes',info,'1.2.3','a'*40)
        self.assertEqual(bom['bomFormat'],'CycloneDX')
        self.assertEqual(bom['specVersion'],'1.6')
        self.assertEqual(bom['metadata']['component']['hashes'][0]['content'],hashlib.sha256(b'bytes').hexdigest())
        self.assertEqual([(x['name'],x['version']) for x in bom['components']],[('go','go1.26.6'),('example.org/lib','v1.2.3')])
        self.assertNotIn('licenses',bom['components'][1])
    def test_versioned_replacement_is_inventory_source(self):
        info={"GoVersion":"go1.26.6","Deps":[{"Path":"old/lib","Version":"v1","Replace":{"Path":"new/lib","Version":"v2"}}]}
        self.assertEqual(make_sbom('binary',b'b',info,'1.0.0','a'*40)['components'][1]['name'],'new/lib')
        info['Deps'][0]['Replace']['Version']=''
        with self.assertRaises(ValueError): make_sbom('binary',b'b',info,'1.0.0','a'*40)
    def test_missing_build_info_is_not_an_empty_success(self):
        with self.assertRaises(ValueError): make_sbom('binary',b'b',{},'1.0.0','a'*40)

if __name__=='__main__': unittest.main()
