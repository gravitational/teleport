# Systemd Service

Sample configuration of `systemd` service file for Teleport
To use it:

```bash
cp teleport.service /etc/systemd/system/teleport.service
systemctl daemon-reload
systemctl enable teleport
systemctl start teleport
```

To check on Teleport daemon status:

```bash
systemctl status teleport
```

To take a look at Teleport system log:

```bash
journalctl -fu teleport
```

