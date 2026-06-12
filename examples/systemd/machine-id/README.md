# Systemd Service

> **Warning**  
> This is a rough guide for using the provided systemd file in this directory. 
> For detailed instructions on configuring Machine ID, see
> https://goteleport.com/docs/machine-id/introduction/

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

Create a user for `tbot` to run as. In our example, this will be `teleport`.
Ensure that this user is able to read and write to `/var/lib/teleport/bot`.

Determine the user that you want to be able to read the certificates that `tbot`
produces. In our example, this will be `jenkins`. Initialize ownership of the 
destination directory so that this user is able to read from it and that the
`teleport` user is able to write to it by using `tbot init`:

```bash
$ sudo tbot init \
    --destination-dir=/opt/machine-id \
    --owner=teleport:teleport \
    --bot-user=teleport \
    --reader-user=jenkins
```

Finally, run the following commands to start Machine ID.

```bash
$ sudo cp machine-id.service /etc/systemd/system/machine-id.service
$ sudo systemctl daemon-reload
$ sudo systemctl start machine-id
$ sudo systemctl status machine-id
```
