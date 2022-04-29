# Systemd Service

Create and fill out a configuration file for Machine ID at `/etc/tbot.yaml`.

```yaml
auth_server: "auth.example.com:3025"
onboarding:
  join_method: "token"
  token: "00000000000000000000000000000000"
  ca_pins:
  - "sha256:1111111111111111111111111111111111111111111111111111111111111111"
storage:
  directory: /var/lib/teleport/bot
destinations:
  - directory: /opt/machine-id
```

Next initialize ownership of the short-lived certificate directory. In this
example, ownership will belong to the user:group `jenkins:jenkins`. Make sure
the `jenkins` user exists on your system.

```bash
$ sudo tbot init \
    --destination-dir=/opt/machine-id \
    --owner=jenkins:jenkins
```

Finally, run the following commands to start Machine ID.

```bash
$ sudo cp machine-id.service /etc/systemd/system/machine-id.service
$ sudo systemctl daemon-reload
$ sudo systemctl start machine-id
$ sudo systemctl status machine-id
```

## Next Steps

More information and guides on Machine ID are available at goteleport.com.

* [Machine ID Getting Started](https://goteleport.com/docs/machine-id/getting-started/)
* [Machine ID with Ansible](https://goteleport.com/docs/machine-id/guides/ansible/)
