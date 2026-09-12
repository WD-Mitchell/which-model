#!/usr/bin/env python3
"""Generate CycloneDX 1.6 inventories from each released binary's Go metadata."""
import argparse
import hashlib
import json
from pathlib import Path
import re
import subprocess
from urllib.parse import quote


def digest(data):
    return hashlib.sha256(data).hexdigest()


def make_sbom(name, data, info, version, commit):
    if not info.get('GoVersion'):
        raise ValueError('binary has no Go build information')
    components = [{'type': 'library', 'name': 'go', 'version': info['GoVersion'],
                   'bom-ref': 'go-runtime'}]
    for module in sorted(info.get('Deps', []), key=lambda item: item['Path']):
        actual = module.get('Replace') or module
        if not actual.get('Version'):
            raise ValueError('unversioned/local replacement module is not releasable')
        purl = 'pkg:golang/' + quote(actual['Path'], safe='/') + '@' + quote(actual['Version'], safe='')
        component = {'type': 'library', 'name': actual['Path'], 'version': actual['Version'],
                     'bom-ref': purl, 'purl': purl}
        if actual.get('Sum'):
            component['properties'] = [{'name': 'go:module:sum', 'value': actual['Sum']}]
        if component not in components:
            components.append(component)
    return {'bomFormat': 'CycloneDX', 'specVersion': '1.6', 'version': 1,
            'metadata': {'component': {'type': 'application', 'name': name, 'version': version,
                                     'bom-ref': 'release-artifact',
                                     'hashes': [{'alg': 'SHA-256', 'content': digest(data)}]},
                         'properties': [{'name': 'which-model:source-commit', 'value': commit}]
                         + [{'name': 'go:build:' + item['Key'], 'value': item['Value']}
                            for item in info.get('Settings', []) if item['Key'] in ('GOOS', 'GOARCH', 'CGO_ENABLED')]},
            'components': components}


def write_json(path, value):
    path.write_text(json.dumps(value, indent=2, sort_keys=True) + '\n', encoding='utf-8')


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('directory', type=Path)
    parser.add_argument('version')
    parser.add_argument('source_digest')
    parser.add_argument('source_ref')
    args = parser.parse_args()
    if not re.fullmatch(r'\d+\.\d+\.\d+(?:-[0-9A-Za-z][0-9A-Za-z.-]*)?', args.version):
        parser.error('invalid version')
    if not re.fullmatch(r'[0-9a-f]{40}', args.source_digest):
        parser.error('full source commit required')
    if not re.fullmatch(r'refs/(heads|tags)/[A-Za-z0-9][A-Za-z0-9._/-]*', args.source_ref):
        parser.error('source branch or tag ref required')
    # Only binary basenames match; existing metadata cannot accidentally become a subject binary.
    binaries = sorted(p for p in args.directory.iterdir()
                      if re.fullmatch(r'which-model-(?:darwin|linux|windows)-(?:arm64|x64)(?:\.exe)?', p.name))
    if not binaries:
        parser.error('no release binaries found')
    artifacts = []
    for binary in binaries:
        info = json.loads(subprocess.check_output(['go', 'version', '-m', '-json', str(binary)], text=True))
        data = binary.read_bytes()
        sbom = binary.with_name(binary.name + '.cdx.json')
        write_json(sbom, make_sbom(binary.name, data, info, args.version, args.source_digest))
        artifacts.append({'name': binary.name, 'sha256': digest(data), 'sbom': sbom.name,
                          'sbom_sha256': digest(sbom.read_bytes())})
    write_json(args.directory / 'release-manifest.json', {
        'schema': 1, 'version': args.version, 'source_digest': args.source_digest,
        'source_ref': args.source_ref, 'artifacts': artifacts})
    print(f'Inventoried {len(artifacts)} release binaries with CycloneDX 1.6 SBOMs.')


if __name__ == '__main__':
    main()
