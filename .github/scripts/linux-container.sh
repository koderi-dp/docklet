#!/usr/bin/env bash
# Runs inside the image built from .github/ci/linux.Dockerfile. The repository is mounted read-only at /src;
# everything is done on a copy inside the container, because a Windows bind mount is slow and must not be
# written to. Same checks as the GitHub-hosted Linux job, without the .deb and AppArmor smoke tests (those
# need a real Ubuntu machine with systemd and AppArmor).
set -euo pipefail

mkdir -p /work
tar -C /src --exclude=./.git --exclude=./bin --exclude=./frontend/node_modules --exclude=./frontend/dist \
  --exclude=./frontend/bindings -cf - . | tar -C /work -xf -
cd /work

echo "== go $(go version | awk '{print $3}'), node $(node --version), $(wails3 version 2>/dev/null | head -n 1 || true)"

(cd frontend && npm ci)
wails3 generate bindings -clean=true -ts -i
(cd frontend && npm run build)

go vet . ./internal/...
go test ./internal/...

wails3 build
reported="$(./bin/docklet --version)"
echo "$reported"
echo "$reported" | grep -q "^Docklet "
