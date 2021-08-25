# Systemd Service

Sample configuration of `systemd` service file for Teleport
To use it:

```bash
sudo cp teleport.service /etc/systemd/system/teleport.service
sudo systemctl daemon-reload
sudo systemctl enable teleport
sudo systemctl start teleport
```

To check on Teleport daemon status:

```bash
systemctl status teleport
```

To take a look at Teleport system log:

```bash
journalctl -fu teleport
```

