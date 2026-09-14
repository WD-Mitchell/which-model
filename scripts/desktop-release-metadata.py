#!/usr/bin/env python3
"""Inventory desktop archives and their embedded Go executables."""
import argparse
import json
import re
import subprocess
from pathlib import Path
from release_metadata import digest, make_sbom, write_json


def inventory(archive, app, version, commit):
    identity = json.loads((app / 'Contents/Resources/build-identity.json').read_text())
    if identity['version'] != version or identity['source_commit'] != commit:
        raise ValueError('desktop bundle identity does not match release')
    executables = list((app / 'Contents/MacOS').iterdir())
    if len(executables) != 1 or not executables[0].is_file():
        raise ValueError('desktop bundle requires one executable')
    executable = executables[0]
    info = json.loads(subprocess.check_output(['go', 'version', '-m', '-json', str(executable)], text=True))
    sbom = make_sbom(archive.name, archive.read_bytes(), info, version, commit)
    sbom['components'].append({'type': 'application', 'name': executable.name, 'version': version,
                               'bom-ref': 'desktop-executable', 'hashes': [{'alg': 'SHA-256', 'content': digest(executable.read_bytes())}]})
    sbom['metadata']['properties'].append({'name': 'which-model:desktop-signing', 'value': 'ad-hoc; not Apple-notarized'})
    sbom_path = archive.with_name(archive.name + '.cdx.json')
    write_json(sbom_path, sbom)
    return {'name': archive.name, 'sha256': digest(archive.read_bytes()), 'sbom': sbom_path.name,
            'sbom_sha256': digest(sbom_path.read_bytes())}


def merge(directory, version, commit, ref):
    manifest_path = directory / 'release-manifest.json'
    manifest = json.loads(manifest_path.read_text())
    if (manifest['version'], manifest['source_digest'], manifest['source_ref']) != (version, commit, ref):
        raise ValueError('CLI release identity mismatch')
    for arch in ('arm64', 'x64'):
        part = json.loads((directory / f'desktop-manifest-{arch}.json').read_text())
        if (part['version'], part['source_digest'], part['source_ref']) != (version, commit, ref):
            raise ValueError('desktop release identity mismatch')
        expected = {f'which-model-{product}-darwin-{arch}.zip' for product in ('desktop', 'offline-desktop')}
        if {a['name'] for a in part['artifacts']} != expected or len(part['artifacts']) != 2:
            raise ValueError('missing or duplicate desktop release bundle')
        for artifact in part['artifacts']:
            name = artifact['name']
            if artifact['sbom'] != name + '.cdx.json':
                raise ValueError('unexpected desktop inventory path')
            for key, hashkey in [('name', 'sha256'), ('sbom', 'sbom_sha256')]:
                if digest((directory / artifact[key]).read_bytes()) != artifact[hashkey]:
                    raise ValueError('desktop artifact checksum mismatch')
            if not (directory / (name + '.govulncheck.txt')).is_file():
                raise ValueError('desktop dependency review evidence missing')
        manifest['artifacts'].extend(part['artifacts'])
    names = [a['name'] for a in manifest['artifacts']]
    if len(set(names)) != len(names):
        raise ValueError('duplicate release artifact')
    write_json(manifest_path, manifest)
    # Recomputed before attestation, so both desktop and CLI subjects are covered.
    (directory / 'checksums.txt').write_text(''.join(f'{digest(p.read_bytes())}  {p.name}\n' for p in sorted(directory.iterdir())
        if p.is_file() and p.name not in ('checksums.txt', 'provenance.jsonl', 'verification.txt')))


def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument('operation', choices=['inventory', 'merge'])
    p.add_argument('directory', type=Path)
    p.add_argument('version'); p.add_argument('commit'); p.add_argument('ref')
    p.add_argument('--arch', choices=['arm64', 'x64'])
    args = p.parse_args()
    if not re.fullmatch(r'\d+\.\d+\.\d+(?:-[0-9A-Za-z][0-9A-Za-z.-]*)?', args.version) or not re.fullmatch('[0-9a-f]{40}', args.commit) or not re.fullmatch(r'refs/(heads|tags)/[A-Za-z0-9][A-Za-z0-9._/-]*', args.ref):
        p.error('invalid release identity')
    if args.operation == 'merge':
        merge(args.directory, args.version, args.commit, args.ref)
        return
    if not args.arch:
        p.error('--arch required for inventory')
    artifacts = []
    for product, app in [('desktop', 'which-model.app'), ('offline-desktop', 'which-model-offline.app')]:
        name = f'which-model-{product}-darwin-{args.arch}.zip'
        artifacts.append(inventory(args.directory / name, Path('bin') / app, args.version, args.commit))
    write_json(args.directory / f'desktop-manifest-{args.arch}.json', {
        'version': args.version, 'source_digest': args.commit, 'source_ref': args.ref, 'artifacts': artifacts})

if __name__ == '__main__':
    main()
