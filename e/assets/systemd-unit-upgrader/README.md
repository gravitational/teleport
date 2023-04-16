# Systemd Unit Upgrader

This directory contains a systemd unit based updater for teleport.

## Quickstart

The upgrader relies on two external resources.  First, an agent upgrade window must
be defined on the auth server. Ex:

```bash
$ cat > cmc.yaml <<EOF
kind: cluster_maintenance_config
spec:
  agent_upgrades:
    utc_start_hour: 2
    weekdays:
      - Mon
      - Wed
      - Fri
EOF
$ tctl create -f cmc.yaml
```

Second, a TLS endpoint must be set up to serve the "target version" for the updater (i.e.
what version the updater should be trying to install).  For local testing, this can be mocked
like so:

```bash
$ # configure the upgrader to use local http endpoint
$ echo "localhost:8000" | sudo tee /etc/teleport-upgrade.d/endpoint
$ echo "yes" | sudo tee /etc/teleport-upgrade.d/insecure # http only works for localhost.
$ # set up the mock version endpoint
$ echo "1.2.3" > version # replace 1.2.3 with desired version.
$ echo "no" > critical # optional. set to yes to force install outside of maintenance window.
$ python3 -m http.server 8000
```

Once these assets are in place, you can install the updater from apt/yum. Ex:

```bash
$ sudo apt install teleport-ent-updater
```

In order to verify the correct functioning of the upgrader, perform a `dry-run`:

```bash
$ sudo teleport-upgrade dry-run
[!] fetching localhost:8000/version in insecure mode (not safe for production use). [ 186 ]
[i] an upgrade is available (1.2.3 -> 1.2.4) [ 274 ]
[!] fetching localhost:8000/critical in insecure mode (not safe for production use). [ 186 ]
[!] agent does not appear to have exported a valid upgrade schedule. [ 371 ]
[!] agent is new, or newly unhealthy. marking for potential future upgrade. [ 376 ]
```

Note that log details will differ based on whether or not your agent has exported a maintenance window
schedule yet and wether or not a previous run noted the need for an upgrade, but they should
be free of obvious errors.
