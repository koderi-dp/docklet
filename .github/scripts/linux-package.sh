#!/usr/bin/env bash
# Builds the Linux release packages (AppImage and .deb) inside the image from .github/ci/linux.Dockerfile.
# Usage: linux-package.sh <version> <arch-label>   e.g. linux-package.sh 0.5.0 amd64
# The repository is mounted read-only at /src and the results are written to /out.
set -euo pipefail

VERSION="$1"
ARCH="$2"

mkdir -p /work /out
tar -C /src --exclude=./.git --exclude=./bin --exclude=./frontend/node_modules --exclude=./frontend/dist \
  --exclude=./frontend/bindings -cf - . | tar -C /work -xf -
cd /work

(cd frontend && npm ci)
wails3 generate bindings -clean=true -ts -i
(cd frontend && npm run build)

# AppImage tools are AppImages themselves; a container has no FUSE, so they unpack and run instead.
export APPIMAGE_EXTRACT_AND_RUN=1
export GIT_COMMITTER_NAME=Danijel

wails3 task linux:create:appimage "VERSION=$VERSION"
wails3 task linux:create:deb "VERSION=$VERSION"

# The profile that lets WebKit's sandbox start on Ubuntu 24.04 must be in the package.
deb_contents="$(dpkg-deb -c bin/*.deb)"
grep -q ' ./etc/apparmor.d/docklet$' <<< "$deb_contents"

reported="$(./bin/docklet --version)"
echo "$reported"
echo "$reported" | grep -qx "Docklet $VERSION"

appimage=$(ls bin/*.AppImage | head -n 1)
deb=$(ls bin/*.deb | head -n 1)
cp "$appimage" "/out/docklet-$VERSION-linux-$ARCH.AppImage"
cp "$deb" "/out/docklet-$VERSION-linux-$ARCH.deb"
ls -l /out
