# Ansible-like OpenSSH sessions load test

This setup is designed to be ran from the home directory of a VM (the default working directory for a systemd user service) and the xltenant.teleport.sh service; the proxy public address and cluster name should be changed in `gen_inventory.sh`, `proxy_templates.yaml` and `tbot.yaml` for use in a different cluster. It requires openssh, jq, xargs, and dumb-init.

This setup assumes that nodes are being ran by the `node-agent` Helm chart, and proxy templates are applied to do predicate-based dialing on the NODENAME label, as the chart sets up. Commenting or blanking the `proxy_templates.yaml` file (and restarting tbot) will change it to hostname-based dialing.

Bot and token can be created with `tctl -f loadtest-bot.yaml`. Token-based joining with tbot is incredibly annoying, so IAM joining or some other ambient-based joining method is recommended. Running the `node-agent` chart is left as an exercise for the reader.

## Usage

- Run `tbot_install.sh` to set up tbot (it will install a specific Teleport version as listed in the script, tweak it as required), or `systemctl --user restart tbot.service` if tbot is already set up.
- Run the `gen_inventory.sh` script to produce a list of hosts in random order in the `inventory` file, check that it matches the expected list of hosts.
- Choose a random host in the inventory and confirm that the setup is working with `ssh -F tbot_destdir_mux/ssh_config root@host`.
- Run `run.sh >/dev/null` (in tmux, probably). In a different terminal or tab, check how many sockets are being opened in the ssh controlmaster directory with `ls -1 /run/user/1000/ssh-control | wc -l` to confirm that connections are being established and muxed by ssh. Logs for tbot can be viewed with `journalctl --user-unit tbot --follow`.
