#!/usr/bin/env python3
"""Actualiza los paquetes de AUR a una versión publicada de openaula.

Uso:
    scripts/aur-bump.py 0.1.5
    scripts/aur-bump.py 0.1.5 --push

Descarga los hashes del release, reescribe PKGBUILD y .SRCINFO de
packaging/aur/{openaula,openaula-bin} y, si existen los clones en
~/Proyectos/aur/, los actualiza y opcionalmente los pushea.
"""

import argparse
import hashlib
import os
import re
import subprocess
import sys
import urllib.request

REPO = "ldgnu/openaula"
ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
AUR_DIR = os.path.join(ROOT, "packaging", "aur")
CLONES = os.path.expanduser("~/Proyectos/aur")

ARCH_ASSETS = {
    "amd64": ("x86_64", "9a0403082856604b9909958be03496e2888ec6a2c498b95722578726c2e152d7"),
    "arm64": ("aarch64", "3de7555804f558a8f3ed371f006e6697bda3bbccecd5ea7fc91dfa1f776b1553"),
    "arm": ("armv7h", "22cad51f4d0c4a99c2dc6137d0149dbf5e5bb0a21250df524368c85031ddd3a2"),
    "386": ("i686", "837a5134b84fbfcc0eac0fccb45fd9c1cb2677a7b93f8e65fae61d6b2822848e"),
}


def fetch_sha256(url: str) -> str:
    digest = hashlib.sha256()
    with urllib.request.urlopen(url, timeout=120) as resp:
        for chunk in iter(lambda: resp.read(1 << 20), b""):
            digest.update(chunk)
    return digest.hexdigest()


def hashes_from_release(version: str):
    """Lee checksums.txt del release para no descargar los binarios."""
    url = f"https://github.com/{REPO}/releases/download/v{version}/checksums.txt"
    with urllib.request.urlopen(url, timeout=60) as resp:
        lines = resp.read().decode().splitlines()
    sums = {}
    for line in lines:
        parts = line.split()
        if len(parts) == 2:
            sums[parts[1]] = parts[0]
    bin_sha = {}
    for asset in ARCH_ASSETS:
        bin_sha[asset] = sums[f"openaula_linux_{asset}.tar.gz"]
    return sums[f"openaula-{version}-vendor.tar.gz"] if f"openaula-{version}-vendor.tar.gz" in sums else "", bin_sha


def main() -> int:
    ap = argparse.ArgumentParser()
    ap.add_argument("version")
    ap.add_argument("--push", action="store_true", help="commitear y pushear los clones de ~/Proyectos/aur")
    ap.add_argument("--skip-download", action="store_true", help="usar hashes del release (checksums.txt)")
    args = ap.parse_args()
    v = args.version

    print(f"openaula {v}")
    src_sha = fetch_sha256(f"https://github.com/{REPO}/archive/refs/tags/v{v}.tar.gz")
    if args.skip_download:
        _, bin_sha = hashes_from_release(v)
    else:
        bin_sha = {}
        for asset in ARCH_ASSETS:
            bin_sha[asset] = fetch_sha256(
                f"https://github.com/{REPO}/releases/download/v{v}/openaula_linux_{asset}.tar.gz")
    print(f"  fuente  {src_sha}")
    for asset, (arch, _) in ARCH_ASSETS.items():
        print(f"  {arch:<8} {bin_sha[asset]}")

    # ---- PKGBUILD de fuente ----
    p = os.path.join(AUR_DIR, "openaula", "PKGBUILD")
    s = open(p).read()
    s = re.sub(r"pkgver=[^\n]+", f"pkgver={v}", s)
    s = re.sub(r"sha256sums=\('[^']*'\)", f"sha256sums=('{src_sha}')", s)
    open(p, "w").write(s)

    # ---- PKGBUILD binario ----
    p = os.path.join(AUR_DIR, "openaula-bin", "PKGBUILD")
    s = open(p).read()
    s = re.sub(r"pkgver=[^\n]+", f"pkgver={v}", s)
    for asset, (_, old) in ARCH_ASSETS.items():
        s = s.replace(old, bin_sha[asset])
    open(p, "w").write(s)

    # ---- .SRCINFO ----
    srcinfo = f"""pkgbase = openaula
\tpkgdesc = TUI, CLI y API para configurar el teclado AULA/Epomaker F108 Pro en Linux
\tpkgver = {v}
\tpkgrel = 1
\turl = https://github.com/{REPO}
\tarch = x86_64
\tarch = aarch64
\tarch = armv7h
\tarch = i686
\tlicense = MIT
\tmakedepends = go
\tconflicts = openaula-bin
\tprovides = openaula
\tsource = openaula-{v}.tar.gz::https://github.com/{REPO}/archive/refs/tags/v{v}.tar.gz
\tsha256sums = {src_sha}

pkgname = openaula
"""
    open(os.path.join(AUR_DIR, "openaula", ".SRCINFO"), "w").write(srcinfo)

    bininfo = f"""pkgbase = openaula-bin
\tpkgdesc = TUI, CLI y API para configurar el teclado AULA/Epomaker F108 Pro (binario estático)
\tpkgver = {v}
\tpkgrel = 1
\turl = https://github.com/{REPO}
\tarch = x86_64
\tarch = aarch64
\tarch = armv7h
\tarch = i686
\tlicense = MIT
\tconflicts = openaula
\tprovides = openaula
\toptions = !strip
"""
    for asset, (arch, _) in ARCH_ASSETS.items():
        bininfo += (f"\tsource_{arch} = openaula-bin-{v}-{asset}.tar.gz::"
                    f"https://github.com/{REPO}/releases/download/v{v}/openaula_linux_{asset}.tar.gz\n")
        bininfo += f"\tsha256sums_{arch} = {bin_sha[asset]}\n"
    bininfo += "\npkgname = openaula-bin\n"
    open(os.path.join(AUR_DIR, "openaula-bin", ".SRCINFO"), "w").write(bininfo)

    print("archivos actualizados en packaging/aur")

    # ---- clones ----
    for name in ("openaula", "openaula-bin"):
        clone = os.path.join(CLONES, name)
        if not os.path.isdir(os.path.join(clone, ".git")):
            print(f"  (no hay clon en {clone}; copiá a mano)")
            continue
        for f in ("PKGBUILD", ".SRCINFO"):
            subprocess.run(["cp", os.path.join(AUR_DIR, name, f), os.path.join(clone, f)], check=True)
        subprocess.run(["git", "-C", clone, "add", "PKGBUILD", ".SRCINFO"], check=True)
        subprocess.run(["git", "-C", clone, "commit", "-m", f"{name} {v}"], check=False)
        if args.push:
            subprocess.run(["git", "-C", clone, "push"], check=True)
            print(f"  pusheado {name}")
        else:
            print(f"  {name}: commit local hecho, falta push")
    return 0


if __name__ == "__main__":
    sys.exit(main())
