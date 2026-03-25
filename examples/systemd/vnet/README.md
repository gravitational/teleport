# Teleport VNet Linux Files

This directory contains files needed for VNet to work on Linux.
Teleport Connect ships these files in its package.

## Files

- `teleport-vnet.service`: systemd unit for the privileged VNet daemon.
- `dbus/org.teleport.vnet1.conf`: D-Bus system bus policy for `org.teleport.vnet1`.
- `dbus/org.teleport.vnet1.service`: D-Bus service activation entry for `org.teleport.vnet1`.
- `polkit/org.teleport.vnet1.policy`: polkit policy used to authorize starting and stopping the privileged VNet daemon.

## Install locations (package defaults)

- `teleport-vnet.service` -> `/usr/lib/systemd/system/teleport-vnet.service`
- `dbus/org.teleport.vnet1.conf` -> `/usr/share/dbus-1/system.d/org.teleport.vnet1.conf`
- `dbus/org.teleport.vnet1.service` -> `/usr/share/dbus-1/system-services/org.teleport.vnet1.service`
- `polkit/org.teleport.vnet1.policy` -> `/usr/share/polkit-1/actions/org.teleport.vnet1.policy`

Notes:
- For packaged vendor files, `/usr/share/...` is the standard location.
- `/etc/dbus-1/system.d/` is typically for local admin overrides, not vendor package files.

## Manual install example

```bash
sudo cp teleport-vnet.service /usr/lib/systemd/system/teleport-vnet.service
sudo cp dbus/org.teleport.vnet1.conf /usr/share/dbus-1/system.d/org.teleport.vnet1.conf
sudo cp dbus/org.teleport.vnet1.service /usr/share/dbus-1/system-services/org.teleport.vnet1.service
sudo cp polkit/org.teleport.vnet1.policy /usr/share/polkit-1/actions/org.teleport.vnet1.policy
sudo systemctl daemon-reload
sudo dbus-send --print-reply --system --dest=org.freedesktop.DBus /org/freedesktop/DBus org.freedesktop.DBus.ReloadConfig
```
