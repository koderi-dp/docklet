#!/bin/sh
# Runs inside the test engine before every test run: removes what earlier runs left and creates the
# containers, the registry image and the Compose project the tests expect. The names are in support/names.ts.
set -eu

base=busybox:1.36
registry_image=registry:5000/e2e/busybox:smoke

ids=$(docker ps -aq)
[ -z "$ids" ] || docker rm -f $ids >/dev/null
docker network prune -f >/dev/null

docker image inspect "$base" >/dev/null 2>&1 || docker pull -q "$base" >/dev/null

# The pull test fetches this one from the registry, so it must not be in the engine yet.
docker tag "$base" "$registry_image"
docker push -q "$registry_image" >/dev/null
docker rmi "$registry_image" >/dev/null

# --init: the containers stop at once instead of after the engine's 10 second grace period.
docker run -d --init --name e2e-ticker "$base" sh -c 'i=0; while true; do i=$((i+1)); echo "tick $i"; sleep 1; done' >/dev/null
docker run -d --init --name e2e-toggle "$base" sleep 86400 >/dev/null
docker run -d --init --name e2e-doomed "$base" sleep 86400 >/dev/null

# Created but not started: Docklet lists a Compose project once its containers exist.
cd /projects/demo
docker compose -p e2e-demo create >/dev/null 2>&1
