# Empaquetado y distribución

openaula se distribuye de cuatro formas:

1. **Releases de GitHub**: binarios estáticos para linux amd64, arm64, arm y 386.
2. **install.sh**: instalador universal para cualquier distro.
3. **AUR**: `openaula` (compila) y `openaula-bin` (binario).
4. **Gentoo**: ebuild de GURU u overlay propio.

Todo el empaquetado vive en `packaging/`.

---

## Releases

El workflow `.github/workflows/release.yml` se dispara con un tag `v*`:

```bash
git tag -a v0.1.5 -m "v0.1.5"
git push origin v0.1.5
```

Genera:

```
openaula_linux_amd64.tar.gz
openaula_linux_arm64.tar.gz
openaula_linux_arm.tar.gz
openaula_linux_386.tar.gz
checksums.txt
```

Cada tarball contiene un único archivo `openaula`, que es lo que espera tanto
`install.sh` como el `PKGBUILD` de `openaula-bin`.

---

## Arch Linux (AUR)

Los archivos están listos en:

```
packaging/aur/openaula/PKGBUILD        # compila desde el tag
packaging/aur/openaula-bin/PKGBUILD    # baja el binario del release
```

Antes de publicar un release nuevo, actualizar en ambos `PKGBUILD` y en los
`.SRCINFO`:

- `pkgver`
- `sha256sums` (tarball de fuentes y de cada binario)
- `source`

### Crear los paquetes en AUR

1. Crear cuenta en <https://aur.archlinux.org> y subir una clave SSH pública:

   ```bash
   ssh-keygen -t ed25519 -f ~/.ssh/aur
   cat ~/.ssh/aur.pub   # pegar en AUR → My Account → SSH Public Key
   ```

   Y configurar el host:

   ```
   Host aur.archlinux.org
     User aur
     IdentityFile ~/.ssh/aur
   ```

2. Clonar el repo del paquete vacío y copiar los archivos:

   ```bash
   git clone ssh://aur@aur.archlinux.org/openaula.git
   cp packaging/aur/openaula/{PKGBUILD,.SRCINFO} openaula/
   cd openaula
   makepkg --printsrcinfo > .SRCINFO   # regenerar siempre antes de subir
   makepkg -si                          # probar el build local
   git add PKGBUILD .SRCINFO
   git commit -m "openaula 0.1.5"
   git push
   ```

   Igual para `openaula-bin`.

3. Verificación local sin publicar:

   ```bash
   cd packaging/aur/openaula
   makepkg -f
   namcap openaula-*.pkg.tar.zst
   ```

No se puede pushear a AUR sin cuenta y clave; el contenido ya está preparado
para copiar tal cual.

---

## Gentoo

El ebuild está en:

```
packaging/gentoo/app-misc/openaula/openaula-0.1.5.ebuild
packaging/gentoo/app-misc/openaula/metadata.xml
```

Usa `go-module` con `EGO_SUM` generado desde `go.sum`, así que compila sin red
durante el build y sin depender de libusb.

### Probar en un overlay local

```bash
sudo mkdir -p /var/db/repos/local/app-misc
sudo cp -r packaging/gentoo/app-misc/openaula /var/db/repos/local/app-misc/
cd /var/db/repos/local/app-misc/openaula
sudo ebuild openaula-0.1.5.ebuild manifest
sudo emerge -av app-misc/openaula
```

Si el overlay no está declarado, agregalo en
`/etc/portage/repos.conf/local.conf`:

```ini
[local]
location = /var/db/repos/local
masters = gentoo
```

### Publicar en GURU

GURU es el overlay oficial de la comunidad:

1. Fork de <https://github.com/gentoo/guru>.
2. Copiar `app-misc/openaula/` dentro del fork.
3. Generar el `Manifest` con `pkgdev manifest` o `ebuild ... manifest`.
4. Pasar `pkgcheck scan --net` y `pkgdev style`.
5. Abrir el PR con el template de GURU.

### Compartir el binario en Gentoo

Gentoo no usa paquetes binarios sueltos, pero hay tres vías:

- **install.sh** en la máquina destino (lo más simple).
- **Binhost propio**: `BINPKG_FORMAT=gpkg`, configurar `binhost` en
  `make.conf`, y publicar `Packages` + `*.gpkg` por HTTP. Sirve para varias
  máquinas.
- **Overlay con ebuild** y compilar en cada máquina; es lo que espera Portage.

---

## Mantenimiento del release

Checklist al cortar una versión:

1. Actualizar `README.md` si cambió el comportamiento.
2. `make test && make vet`.
3. Tag `vX.Y.Z` y push.
4. Esperar el workflow de release.
5. Actualizar `PKGBUILD`/`.SRCINFO` de AUR y el ebuild de Gentoo con la versión
   y los hashes nuevos.
6. Probar `install.sh` en una máquina limpia.
