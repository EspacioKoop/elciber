#!/usr/bin/env python3
"""Download a pinned, upstream EasyTier runtime. Never installs a service/driver."""
import argparse
import hashlib
import io
import os
from pathlib import Path
import platform
import shutil
import tempfile
import time
import urllib.request
import zipfile

VERSION = "2.6.4"
ARTIFACTS = {
    "linux": ("easytier-linux-x86_64", "61b659eaedba658fa66fe47d17e1426cdd77e5d02fa15fed447bb4357c09dfd6", ("easytier-core", "easytier-cli")),
    "windows": ("easytier-windows-x86_64", "27af91e270e554709b048bd32327fefd2dfce5062ae1e8701af7550c6f525f84", ("easytier-core.exe", "easytier-cli.exe", "wintun.dll")),
}
MAX_BYTES = 160 * 1024 * 1024


def fetch(target, destination):
    folder, expected, names = ARTIFACTS[target]
    destination = Path(destination).absolute()
    if destination.exists():
        raise ValueError("El destino ya existe. No se sobrescribe un motor instalado; usa otro directorio para actualizar.")
    url = f"https://github.com/EasyTier/EasyTier/releases/download/v{VERSION}/{folder}-v{VERSION}.zip"
    started = time.monotonic()
    chunks, size = [], 0
    with urllib.request.urlopen(url, timeout=30) as response:
        while chunk := response.read(1024 * 1024):
            chunks.append(chunk)
            size += len(chunk)
            if size > MAX_BYTES or time.monotonic() - started > 120:
                raise ValueError("Descarga fuera de los límites de tamaño o tiempo.")
    data = b"".join(chunks)
    if hashlib.sha256(data).hexdigest() != expected:
        raise ValueError("SHA-256 incorrecto. No se ha instalado nada.")
    destination.parent.mkdir(parents=True, exist_ok=True)
    staging = Path(tempfile.mkdtemp(prefix=".elciber-engine-", dir=destination.parent))
    try:
        with zipfile.ZipFile(io.BytesIO(data)) as archive:
            for name in names:
                path = f"{folder}/{name}"
                entries = [entry for entry in archive.infolist() if entry.filename == path]
                if len(entries) != 1 or not 0 < entries[0].file_size <= MAX_BYTES:
                    raise ValueError("Estructura inesperada del paquete oficial.")
                # Exact allowlist; no extractall, links or archive-provided paths.
                with archive.open(entries[0]) as source, (staging / name).open("xb") as out:
                    shutil.copyfileobj(source, out)
                if target == "linux":
                    (staging / name).chmod(0o700)
        if destination.exists():
            raise ValueError("El destino apareció durante la descarga. No se sobrescribe.")
        os.rename(staging, destination)
    finally:
        if staging.exists():
            shutil.rmtree(staging)
    return len(names)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--platform", choices=ARTIFACTS)
    parser.add_argument("--destination", type=Path, default=Path(__file__).resolve().parents[1] / "engines" / "easytier")
    args = parser.parse_args()
    target = args.platform or platform.system().lower()
    if target not in ARTIFACTS or (not args.platform and platform.machine().lower() not in ("x86_64", "amd64")):
        parser.error("Solo se han fijado artefactos x86-64 de Windows y Linux.")
    try:
        count = fetch(target, args.destination)
    except Exception as exc:
        # Never echo environment/configuration, HTTP response bodies or credentials.
        print(f"Descarga no completada ({type(exc).__name__}). Comprueba conexión, destino y checksum oficial.")
        return 1
    print(f"EasyTier {VERSION}: SHA-256 verificado; {count} archivos. No se ha ejecutado el motor ni instalado controladores.")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
