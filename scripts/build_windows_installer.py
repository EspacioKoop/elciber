#!/usr/bin/env python3
"""Build the real Windows web installer on Linux. Does not install or run it.

Tools: Go, NSIS 3 (makensis), and github.com/akavel/rsrc v0.10.2.
Third-party networking binaries are fetched by the installer, not bundled.
"""
import argparse
import hashlib
import os
from pathlib import Path
import shutil
import subprocess
import tempfile

ROOT = Path(__file__).resolve().parents[1]


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--go', default='go')
    parser.add_argument('--makensis', default='makensis')
    parser.add_argument('--rsrc', default='rsrc')
    parser.add_argument('--output', type=Path, default=ROOT / 'dist' / 'ElCiber-Setup-alpha.1.exe')
    args = parser.parse_args()
    args.output = args.output.resolve()
    args.output.parent.mkdir(parents=True, exist_ok=True)
    env = os.environ.copy()
    env.update({'GOMAXPROCS': '2', 'GOOS': 'windows', 'GOARCH': 'amd64', 'CGO_ENABLED': '0'})
    with tempfile.TemporaryDirectory(prefix='elciber-installer-build-') as tmp:
        stage = Path(tmp)
        source = stage / 'source'
        payload = stage / 'payload'
        source.mkdir()
        payload.mkdir()
        shutil.copyfile(ROOT / 'go.mod', source / 'go.mod')
        for path in ROOT.glob('*.go'):
            if not path.name.endswith('_test.go'):
                shutil.copyfile(path, source / path.name)
        shutil.copytree(ROOT / 'web', source / 'web')
        subprocess.run([args.rsrc, '-arch', 'amd64', '-manifest', str(ROOT / 'packaging/windows/elciber.manifest'), '-o', str(source / 'resource_windows_amd64.syso')], check=True, timeout=30)
        subprocess.run([args.go, 'build', '-p', '2', '-trimpath', '-ldflags=-H=windowsgui -s -w', '-o', str(payload / 'elciber.exe'), '.'], cwd=source, env=env, check=True, timeout=120)
        subprocess.run([args.go, 'build', '-p', '2', '-trimpath', '-ldflags=-H=windowsgui -s -w', '-o', str(payload / 'setup-engine.exe'), './cmd/setup-engine'], cwd=ROOT, env=env, check=True, timeout=120)
        go_root = Path(subprocess.check_output([args.go, 'env', 'GOROOT'], text=True).strip())
        license_text = (ROOT / 'LICENSE').read_text() + '\n\nGo standard library license\n\n' + (go_root / 'LICENSE').read_text()
        (payload / 'LICENSE.txt').write_text(license_text, encoding='utf-8')
        shutil.copyfile(ROOT / 'packaging/windows/THIRD-PARTY.txt', payload / 'THIRD-PARTY.txt')
        subprocess.run([args.makensis, '-V2', f'-DPAYLOAD={payload}', f'-DOUTPUT={args.output}', str(ROOT / 'packaging/windows/installer.nsi')], check=True, timeout=120)
    data = args.output.read_bytes()
    if not data.startswith(b'MZ') or len(data) < 100000:
        raise RuntimeError('NSIS output is not an executable installer')
    digest = hashlib.sha256(data).hexdigest()
    (args.output.parent / (args.output.name + '.sha256')).write_text(f'{digest}  {args.output.name}\n')
    print(f'Windows installer built: {args.output.name} ({len(data)} bytes). SHA-256 saved.')
    print('NOT installed or playtested on Windows. Automatic connection service is still pending.')


if __name__ == '__main__':
    main()
