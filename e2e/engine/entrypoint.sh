#!/bin/sh
# Creates the SSH keys of this test run (shared with the Docklet container through /keys), starts sshd and
# then the Docker daemon.
set -eu

keys=/keys
mkdir -p "$keys" /root/.ssh
[ -f "$keys/host_ed25519" ] || ssh-keygen -q -t ed25519 -N '' -f "$keys/host_ed25519"
[ -f "$keys/client_ed25519" ] || ssh-keygen -q -t ed25519 -N '' -C docklet-e2e -f "$keys/client_ed25519"

# Docklet never accepts a host key on its own, so the Docklet container gets this known_hosts file.
echo "engine $(cut -d' ' -f1,2 "$keys/host_ed25519.pub")" > "$keys/known_hosts"

cp "$keys/client_ed25519.pub" /root/.ssh/authorized_keys
chmod 700 /root/.ssh
chmod 600 /root/.ssh/authorized_keys
install -m 600 "$keys/host_ed25519" /etc/ssh/ssh_host_ed25519_key

# A password makes the account usable for sshd; it is only reachable on the private Compose network.
echo 'root:docklet-e2e' | chpasswd >/dev/null 2>&1
/usr/sbin/sshd -h /etc/ssh/ssh_host_ed25519_key -o PermitRootLogin=yes -o PasswordAuthentication=yes

exec dockerd-entrypoint.sh --insecure-registry=registry:5000
