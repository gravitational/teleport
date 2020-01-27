# launchd

Sample configuration of launchd for Teleport.

## Install

```
sudo cp teleport.plist /Library/LaunchDaemons/
sudo launchctl load /Library/LaunchDaemons/teleport.plist
```

## Status

```
launchctl list | grep -i teleport
```

## Logs

```
sudo tail -f /var/log/teleport-stderr.log
```

## Restart

```
sudo launchctl unload /Library/LaunchDaemons/teleport.plist && \
sudo launchctl load /Library/LaunchDaemons/teleport.plist
```
