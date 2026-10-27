#!/bin/sh
# Installs Haepage as a systemd service on Linux (Debian, Ubuntu, Fedora,
# and other systemd distributions; amd64, arm64 or armv7).
#
#   curl -fsSL https://raw.githubusercontent.com/haelahpx/haepage/main/packaging/install.sh | sudo sh
#
# Environment:
#   HAEPAGE_VERSION   release tag to install (default: the latest release)
#   HAEPAGE_ARCHIVE   install from a local release archive instead of downloading
#   HAEPAGE_PORT      port to listen on (default 8080)
set -eu

REPO="haelahpx/haepage"
PREFIX="/usr/local/bin"
PORT="${HAEPAGE_PORT:-8080}"

say() { printf '\033[1m==>\033[0m %s\n' "$*"; }
die() { printf 'error: %s\n' "$*" >&2; exit 1; }

[ "$(id -u)" -eq 0 ] || die "run as root, for example: curl -fsSL ... | sudo sh"
command -v systemctl >/dev/null 2>&1 || die "systemd is required"

case "$(uname -m)" in
  x86_64 | amd64) ARCH=amd64 ;;
  aarch64 | arm64) ARCH=arm64 ;;
  armv7l | armv7) ARCH=armv7 ;;
  *) die "unsupported CPU: $(uname -m)" ;;
esac

WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT

if [ -n "${HAEPAGE_ARCHIVE:-}" ]; then
  say "Using local archive $HAEPAGE_ARCHIVE"
  cp "$HAEPAGE_ARCHIVE" "$WORK/haepage.tar.gz"
else
  command -v curl >/dev/null 2>&1 || die "curl is required"
  TAG="${HAEPAGE_VERSION:-}"
  if [ -z "$TAG" ]; then
    TAG="$(curl -fsSL "https://api.github.com/repos/$REPO/releases/latest" | sed -n 's/.*"tag_name": *"\([^"]*\)".*/\1/p' | head -n1)"
    [ -n "$TAG" ] || die "could not find the latest release"
  fi
  VERSION="${TAG#v}"
  FILE="haepage_${VERSION}_linux_${ARCH}.tar.gz"
  BASE="https://github.com/$REPO/releases/download/$TAG"
  say "Downloading Haepage $TAG for linux/$ARCH"
  curl -fsSL -o "$WORK/haepage.tar.gz" "$BASE/$FILE"
  curl -fsSL -o "$WORK/checksums.txt" "$BASE/checksums.txt"
  say "Verifying checksum"
  WANT="$(grep " $FILE\$" "$WORK/checksums.txt" | cut -d' ' -f1)"
  GOT="$(sha256sum "$WORK/haepage.tar.gz" | cut -d' ' -f1)"
  [ -n "$WANT" ] && [ "$WANT" = "$GOT" ] || die "checksum mismatch for $FILE"
fi

tar -xzf "$WORK/haepage.tar.gz" -C "$WORK"
[ -x "$WORK/haepage" ] || die "archive has no haepage binary"

say "Installing $PREFIX/haepage"
install -m 0755 "$WORK/haepage" "$PREFIX/haepage"

say "Creating the haepage user"
if [ -f "$WORK/packaging/systemd/haepage.sysusers" ] && command -v systemd-sysusers >/dev/null 2>&1; then
  install -m 0644 "$WORK/packaging/systemd/haepage.sysusers" /usr/lib/sysusers.d/haepage.conf
  systemd-sysusers /usr/lib/sysusers.d/haepage.conf
elif ! id haepage >/dev/null 2>&1; then
  useradd --system --home-dir /var/lib/haepage --shell /usr/sbin/nologin haepage
fi

# Docker discovery reads the Docker socket. Membership of the docker group
# gives that access; leave it out to run without Docker discovery.
if getent group docker >/dev/null 2>&1; then
  say "Adding haepage to the docker group (for Docker discovery)"
  usermod -aG docker haepage
fi

say "Installing the systemd service"
install -m 0644 "$WORK/packaging/systemd/haepage.service" /etc/systemd/system/haepage.service
mkdir -p /etc/haepage
if [ ! -f /etc/haepage/env ]; then
  printf 'HAEPAGE_ADDR=:%s\n' "$PORT" > /etc/haepage/env
fi
systemctl daemon-reload
systemctl enable --now haepage
systemctl restart haepage

HOST="$(hostname -I 2>/dev/null | cut -d' ' -f1)"
say "Haepage is running: http://${HOST:-localhost}:$PORT"
echo "    Config: /var/lib/haepage/haepage.json   Settings: /etc/haepage/env"
echo "    Logs:   journalctl -u haepage -f"
