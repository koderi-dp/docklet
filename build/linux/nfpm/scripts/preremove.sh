#!/bin/sh

# Unload the AppArmor profile while its file still exists (dpkg and rpm remove files after this step).
# Upgrades keep it loaded: deb passes "upgrade", rpm passes a count of 1 or more.
case "$1" in
  remove|purge|0|"") ;;
  *) exit 0 ;;
esac

if [ -f /etc/apparmor.d/docklet ] && command -v apparmor_parser >/dev/null 2>&1; then
  apparmor_parser -R /etc/apparmor.d/docklet >/dev/null 2>&1 || true
fi

exit 0
