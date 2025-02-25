# Ansible-like OpenSSH sessions load test

This setup is designed to be ran from the home directory of a VM (the default working directory for a systemd user service); the proxy public address and cluster name should be changed in `gen_inventory.sh`, `proxy_templates.yaml` and `tbot.yaml` from `PROXYHOST` and `CLUSTERNAME` respectively. It requires openssh, jq, xargs, and dumb-init, as well as tbot and fdpass-teleport.

This setup assumes that nodes are being ran by the `node-agent` Helm chart, and proxy templates are applied to do predicate-based dialing on the NODENAME label, as the chart sets up. Commenting or blanking the `proxy_templates.yaml` file (and restarting tbot) will change it to hostname-based dialing. Changing the `proxy_templates.yaml` file (and restarting tbot) can also be used to test a simpler predicate, or to test search-based dialing rather than predicate-based dialing.

Bot and token can be created with `tctl -f loadtest-bot.yaml`, after editing the IAM account and role in it. Token-based joining with tbot is incredibly annoying, so IAM joining or some other ambient-based joining method is recommended. Running the `node-agent` chart is left as an exercise for the reader.

The machine running the client should be scaled depending on how many nodes are targeted in the inventory; for 60000 nodes (i.e. 60k shell scripts and 120k ssh processes running at peak) the memory usage with Teleport 15 seems to be ~20GiB for tbot and ~200 for the scripts and SSH, so something like an AWS 32xlarge or 48xlarge might be necessary (maybe the compute-optimized variants, as memory isn't really a problem). Depending on the scale of the test and the runner machine, tuning GOMAXPROCS and GOMEMLIMIT in tbot.service might be useful.

## Usage

- Run `tbot_install.sh` to set up tbot (it will install a specific Teleport version as listed in the script, tweak it as required), or `systemctl --user restart tbot.service` if tbot is already set up.
- Run the `gen_inventory.sh` script to produce a list of hosts in random order in the `inventory` file, check that it matches the expected list of hosts.
- Choose a random host in the inventory and confirm that the setup is working with `ssh -F tbot_destdir_mux/ssh_config root@host`.
- Run `run.sh >/dev/null` (in tmux, probably). In a different terminal or tab, check how many sockets are being opened in the ssh controlmaster directory with `ls -1 /run/user/1000/ssh-control | wc -l` to confirm that connections are being established and muxed by ssh. Logs for tbot can be viewed with `journalctl --user-unit tbot --follow`.
