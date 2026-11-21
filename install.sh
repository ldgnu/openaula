#!/bin/sh
# openaula installer: builds or downloads a static binary and installs the udev
# rule. Works on any Linux distribution with a POSIX shell.
#
#   curl -fsSL https://raw.githubusercontent.com/ldgnu/openaula/main/install.sh | sh
#
# Environment variables:
#   VERSION      release tag to install (default: latest)
#   PREFIX       install prefix (default: /usr/local)
#   FROM_SOURCE  set to 1 to always build with go
#   NO_SUDO      set to 1 to skip privileged steps (udev rule + install)
set -eu

REPO="ldgnu/openaula"
BIN="openaula"
PREFIX="${PREFIX:-/usr/local}"
VERSION="${VERSION:-latest}"
FROM_SOURCE="${FROM_SOURCE:-0}"

say()  { printf '\033[1;36m%s\033[0m\n' "$*"; }
warn() { printf '\033[1;33m%s\033[0m\n' "$*" >&2; }
die()  { printf '\033[1;31m%s\033[0m\n' "$*" >&2; exit 1; }

need() { command -v "$1" >/dev/null 2>&1 || die "falta '$1'"; }

# --- architecture -----------------------------------------------------------
arch="$(uname -m)"
case "$arch" in
  x86_64|amd64) goarch=amd64 ;;
  aarch64|arm64) goarch=arm64 ;;
  armv7l|armv6l) goarch=arm ;;
  i386|i686) goarch=386 ;;
  *) die "arquitectura no soportada: $arch" ;;
esac

say "openaula · $arch"

# --- helpers ----------------------------------------------------------------
have_sudo=0
if [ "$(id -u)" -eq 0 ]; then
  SUDO=""
elif command -v sudo >/dev/null 2>&1; then
  SUDO="sudo"
  have_sudo=1
elif command -v doas >/dev/null 2>&1; then
  SUDO="doas"
  have_sudo=1
else
  SUDO=""
fi

if [ "${NO_SUDO:-0}" = "1" ]; then
  SUDO=""
  have_sudo=0
fi

# Group used for the udev rule: pick the first that exists on this system.
pick_group() {
  for g in usb plugdev input; do
    if getent group "$g" >/dev/null 2>&1; then
      echo "$g"
      return
    fi
  done
  echo ""
}

# --- obtain the binary ------------------------------------------------------
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

if [ "$FROM_SOURCE" = "0" ]; then
  if [ "$VERSION" = "latest" ]; then
    url="https://github.com/$REPO/releases/latest/download/openaula_linux_${goarch}.tar.gz"
  else
    url="https://github.com/$REPO/releases/download/$VERSION/openaula_linux_${goarch}.tar.gz"
  fi
  say "descargando $url"
  if command -v curl >/dev/null 2>&1; then
    curl -fL --retry 3 -o "$tmp/openaula.tar.gz" "$url" || FROM_SOURCE=1
  elif command -v wget >/dev/null 2>&1; then
    wget -qO "$tmp/openaula.tar.gz" "$url" || FROM_SOURCE=1
  else
    warn "no hay curl ni wget"
    FROM_SOURCE=1
  fi
  if [ "$FROM_SOURCE" = "0" ]; then
    mkdir -p "$tmp/extract"
    tar -xzf "$tmp/openaula.tar.gz" -C "$tmp/extract"
    if [ -f "$tmp/extract/openaula" ]; then
      mv "$tmp/extract/openaula" "$tmp/openaula.bin"
    else
      # Older releases named the binary openaula_linux_<arch>.
      found="$(find "$tmp/extract" -maxdepth 1 -type f -name 'openaula*' ! -name '*.tar.gz' | head -n1)"
      [ -n "$found" ] || die "el paquete no contiene el binario"
      mv "$found" "$tmp/openaula.bin"
    fi
  fi
fi

if [ "$FROM_SOURCE" = "1" ]; then
  need go
  say "compilando desde el código fuente"
  src="$tmp/src"
  if command -v git >/dev/null 2>&1; then
    git clone --depth 1 "https://github.com/$REPO.git" "$src"
  else
    need curl
    curl -fL -o "$tmp/src.tar.gz" "https://github.com/$REPO/archive/refs/heads/main.tar.gz"
    mkdir -p "$src"
    tar -xzf "$tmp/src.tar.gz" -C "$src" --strip-components=1
  fi
  ( cd "$src" && CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o "$tmp/openaula.bin" ./cmd/openaula )
fi

chmod +x "$tmp/openaula.bin"

# --- install binary ---------------------------------------------------------
if [ -n "$SUDO" ] || [ "$(id -u)" -eq 0 ]; then
  say "instalando en $PREFIX/bin/$BIN"
  $SUDO install -Dm755 "$tmp/openaula.bin" "$PREFIX/bin/$BIN"
else
  dest="$HOME/.local/bin"
  mkdir -p "$dest"
  say "instalando en $dest/$BIN (sin root)"
  install -m755 "$tmp/openaula.bin" "$dest/$BIN"
  case ":$PATH:" in
    *":$dest:"*) ;;
    *) warn "agregá $dest al PATH" ;;
  esac
fi

# --- udev rule --------------------------------------------------------------
write_rule() {
  group="$1"
  target="$2"
  {
    echo '# openaula: AULA/Epomaker F108 Pro (cable y dongle 2.4 GHz).'
    if [ -n "$group" ]; then
      echo "SUBSYSTEM==\"usb\", ATTR{idVendor}==\"0c45\", ATTR{idProduct}==\"800a\", MODE=\"0660\", GROUP=\"$group\", TAG+=\"uaccess\""
      echo "SUBSYSTEM==\"hidraw\", ATTRS{idVendor}==\"0c45\", ATTRS{idProduct}==\"800a\", MODE=\"0660\", GROUP=\"$group\", TAG+=\"uaccess\""
      echo "SUBSYSTEM==\"usb\", ATTR{idVendor}==\"05ac\", ATTR{idProduct}==\"024f\", MODE=\"0660\", GROUP=\"$group\", TAG+=\"uaccess\""
      echo "SUBSYSTEM==\"hidraw\", ATTRS{idVendor}==\"05ac\", ATTRS{idProduct}==\"024f\", MODE=\"0660\", GROUP=\"$group\", TAG+=\"uaccess\""
    else
      echo "SUBSYSTEM==\"usb\", ATTR{idVendor}==\"0c45\", ATTR{idProduct}==\"800a\", MODE=\"0666\", TAG+=\"uaccess\""
      echo "SUBSYSTEM==\"hidraw\", ATTRS{idVendor}==\"0c45\", ATTRS{idProduct}==\"800a\", MODE=\"0666\", TAG+=\"uaccess\""
      echo "SUBSYSTEM==\"usb\", ATTR{idVendor}==\"05ac\", ATTR{idProduct}==\"024f\", MODE=\"0666\", TAG+=\"uaccess\""
      echo "SUBSYSTEM==\"hidraw\", ATTRS{idVendor}==\"05ac\", ATTRS{idProduct}==\"024f\", MODE=\"0666\", TAG+=\"uaccess\""
    fi
  } > "$target"
}

if [ -n "$SUDO" ] || [ "$(id -u)" -eq 0 ]; then
  group="$(pick_group)"
  say "instalando regla udev (grupo: ${group:-todos})"
  write_rule "$group" "$tmp/99-openaula.rules"
  $SUDO mkdir -p /etc/udev/rules.d
  $SUDO install -m644 "$tmp/99-openaula.rules" /etc/udev/rules.d/99-openaula.rules
  if command -v udevadm >/dev/null 2>&1; then
    $SUDO udevadm control --reload-rules 2>/dev/null || true
    $SUDO udevadm trigger 2>/dev/null || true
  fi
  if [ -n "$group" ] && [ "$(id -u)" -ne 0 ]; then
    if ! id -nG "$USER" 2>/dev/null | tr ' ' '\n' | grep -qx "$group"; then
      warn "tu usuario no está en el grupo $group; agregalo con:"
      warn "  sudo usermod -aG $group $USER"
      warn "y volvé a iniciar sesión"
    fi
  fi
  if [ -n "$group" ] && [ "$(id -u)" -ne 0 ]; then
    if $SUDO -n true 2>/dev/null && command -v usermod >/dev/null 2>&1; then
      $SUDO usermod -aG "$group" "$USER" 2>/dev/null || true
    fi
  fi
else
  warn "sin root: no instalé la regla udev. Copiá packaging/udev/99-openaula.rules"
  warn "a /etc/udev/rules.d/ manualmente."
fi

say "listo"
"$PREFIX/bin/$BIN" modes >/dev/null 2>&1 || true
printf '\nProbá:  %s status\n        %s\n' "$BIN" "$BIN"
