# ansible-like openssh loadtest

This setup is intended to generate fake ansible-like load by spawning very massive numbers
of sessions against a large number of teleport nodes. It uses tbot/machineid with ssh multiplexing
to support the needed volume of sessions (as one would do with an ansible master that manages
a massive number of servers via teleport).

This setup is designed to be run on a fresh VM instance, and will perform various
installs and system configuration actions.

It expects the following to already be installed on the system:

- `openssh`
- `jq`
- `xargs`

It will perform installation of the following:

- all default teleport binaries (namely, `tbot`)
- `fdpass-teleport`
- `dumb-init`

By default, this test setup assumes that the `node-agents` loadtest helm chart is being
used.  The proxy templates generated rely on labels set by that helm chart. After setup
is run, it is possible to customize the proxy template used by editing `/etc/tbot/proxy-templates.yaml`.

Given the extreme scale of tests run with this setup, it is typically necessary to use a
very large VM. For example, 60k agent tests are typically run from a 32xlarge or 48xlarge
instance, either general purpose of compute optimized.

## Usage

- Copy `example.vars.env` to `vars.env` and edit the copy. The `PROXY_HOST` variable
and `BOT_TOKEN` variable *must* be changed.

- Run `install.sh` once to install `tbot`, `fdpass-teleport`, and `dumb-init`. This only need
ever be run once.

- Run `init.sh` to set up tbot directories/configuration and start the `bot.service`. If this needs
to be re-run (e.g. if proxy host or token need to be changed), it may be necessary to first manually
halt the tbot service.

- Run `journalctl -u tbot.service` to verify that `tbot` has successfully authenticated with the cluster.

- Run `gen-inventory.sh` to generate a list of all target teleport nodes. This only needs to be re-run
if/when the set of agents changes.

- Verify that the setup is functional by selecting a random host from `state/inventory` and attempting to
access it via `ssh -F /opt/machine-id/ssh_config root@host`

- Run `run.sh` to run the actual test scenario. This will invoke `run-node.sh` for each member of
the generated inventory and report success/failure of individual attempted sessions. Note that for
large scale tests the output of this script is enormous and may need to be piped to `/dev/null`. Long
running invocations should be performed within a `tmux` session or similar.

- Verify that ssh connections are being established and multiplexed by monitoring the control master
directory with `ls -1 /run/user/1000/ssh-control | wc -l`.
