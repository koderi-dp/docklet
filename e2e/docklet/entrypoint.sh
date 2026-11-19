#!/bin/sh
# Installs the SSH key and the engine's host key of this test run, then starts Docklet.
set -eu

mkdir -p /root/.ssh
chmod 700 /root/.ssh
install -m 600 /keys/client_ed25519 /root/.ssh/id_ed25519
install -m 644 /keys/known_hosts /root/.ssh/known_hosts

exec dbus-run-session -- docklet-server
