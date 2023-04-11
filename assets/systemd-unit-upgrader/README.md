# Systemd Unit Upgrader

This directory contains a systemd unit based updater for teleport.

## Quickstart

note: most of the commands here need to be run as root.

The upgrader relies on two external resources.  First, an agent upgrade window must
be defined on the auth server. Ex:

```bash
$ cat > mw.yaml <<EOF
kind: cluster-maintenance-config
spec:
  agent_upgrades:
    utc_start_hour: 2
    weekdays:
      - Mon
      - Wed
      - Fri
EOF
$ tctl create -f mw.yaml
```

Second, a TLS endpoint must be set up to serve the "target version" for the updater (i.e.
what version the updater should be trying to install).  For local testing, this can be mocked
like so:

```bash
$ echo "localhost:8000" > /etc/teleport-upgrade.d/endpoint
$ echo "yes" > /etc/teleport-upgrade.d/insecure # http only works for localhost.
$ echo "1.2.3" > version # replace 1.2.3 with desired version.
$ echo "no" > critical # optional. set to yes to force install outside of maintenance window.
$ python3 -m http.server 8000
```

Once these assets are in place, you can install the updater like so:

```bash
$ ./install.sh apt|yum|nop
```

Prefer installing the `nop` variant to begin with so that you can confirm that everything is
working as intended. You can test your configuration by invoking `teleport-upgrade` directly:

```bash
$ teleport-upgrade 
[!] fetching localhost:8000/version in insecure mode (not safe for production use). [ 186 ]
[i] an upgrade is available (1.2.3 -> 1.2.4) [ 274 ]
[!] fetching localhost:8000/critical in insecure mode (not safe for production use). [ 186 ]
[i] agent may be unhealthy. marking for potential future upgrade. [ 290 ]
```

Note that log details will differ based on whether or not your agent has exported a maintenance window
schedule yet and wether or not a previous run noted the need for an upgrade, but they should
be free of errors.


Once you are confident that your configuration is correct, you can amend the updater's configuration
to use the `apt` or `yum` installer variants:

```bash
$ echo apt|yum > /etc/teleport-upgrade.d/installer
```
