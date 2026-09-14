#!/usr/bin/env python3
"""Copy only the offline Vite entry and its dependency closure for embedding."""
import json
from pathlib import Path
import shutil
import sys

def copy_assets(source, target):
    source, target = Path(source), Path(target)
    manifest = json.loads((source / '.vite/manifest.json').read_text())
    seen = set()
    def copy(name):
        path = Path(name)
        if path.is_absolute() or '..' in path.parts:
            raise ValueError('unsafe frontend asset path')
        destination = target / path
        destination.parent.mkdir(parents=True, exist_ok=True)
        shutil.copyfile(source / path, destination)
    def visit(key):
        if key in seen:
            return
        seen.add(key)
        entry = manifest[key]
        copy(entry['file'])
        for asset in entry.get('css', []) + entry.get('assets', []):
            copy(asset)
        for child in entry.get('imports', []) + entry.get('dynamicImports', []):
            visit(child)
    visit('offline.html')
    copy('offline.html')

if __name__ == '__main__':
    copy_assets(*sys.argv[1:])
