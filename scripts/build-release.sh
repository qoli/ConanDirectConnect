#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
VERSION="${VERSION:-dev}"
DIST="${ROOT}/dist"
PKG="${DIST}/package"
ZIP_NAME="ConanDirectConnect-${VERSION}-windows-amd64.zip"
ZIP_PATH="${DIST}/${ZIP_NAME}"

rm -rf "${PKG}"
mkdir -p "${PKG}"

GOOS=windows GOARCH=amd64 go build -trimpath -ldflags="-s -w" \
  -o "${PKG}/ConanDirectConnect.exe" "${ROOT}"

cp "${ROOT}/README.md" "${ROOT}/LICENSE" "${PKG}/"
mkdir -p "${PKG}/docs/images"
cp "${ROOT}/docs/images/steam-shortcut-launch-options.jpg" "${PKG}/docs/images/"

rm -f "${ZIP_PATH}" "${ZIP_PATH}.sha256"
(
  cd "${PKG}"
  zip -q -9 "${ZIP_PATH}" ConanDirectConnect.exe README.md LICENSE docs/images/steam-shortcut-launch-options.jpg
)

unzip -t "${ZIP_PATH}"
shasum -a 256 "${ZIP_PATH}" > "${ZIP_PATH}.sha256"

echo "${ZIP_PATH}"
