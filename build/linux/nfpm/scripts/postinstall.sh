#!/bin/sh

# Update desktop database for .desktop file changes
# This makes the application appear in application menus and registers its capabilities.
if command -v update-desktop-database >/dev/null 2>&1; then
  echo "Updating desktop database..."
  update-desktop-database -q /usr/share/applications
else
  echo "Warning: update-desktop-database command not found. Desktop file may not be immediately recognized." >&2
fi

# Update MIME database for custom URL schemes (x-scheme-handler)
# This ensures the system knows how to handle your custom protocols.
if command -v update-mime-database >/dev/null 2>&1; then
  echo "Updating MIME database..."
  update-mime-database -n /usr/share/mime
else
  echo "Warning: update-mime-database command not found. Custom URL schemes may not be immediately recognized." >&2
fi

# Load the AppArmor profile that lets WebKitGTK's bubblewrap sandbox create user namespaces
# (Ubuntu 24.04+ restricts them). Only when AppArmor is running; never fails the install.
profile=/etc/apparmor.d/docklet
if [ -f "$profile" ]; then
  if [ "$(cat /sys/module/apparmor/parameters/enabled 2>/dev/null)" = "Y" ] && command -v apparmor_parser >/dev/null 2>&1; then
    # Releases whose parser does not know "abi <abi/4.0>" or "userns" (22.04) cannot load it and do not
    # need it: drop the profile so a failing file is not left behind for the AppArmor service.
    if ! apparmor_parser -r -W "$profile" >/dev/null 2>&1; then
      rm -f "$profile"
    fi
  fi
fi

exit 0
