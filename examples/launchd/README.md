# launchd

Sample configuration of launchd for Teleport.

## Install

```
cp teleport.plist /Library/LaunchDaemons/
launchctl load /Library/LaunchDaemons/teleport.plist
```

## Status

```
launchctl list | grep -i teleport
```

## Logs

```
tail -f /var/log/teleport-stderr.log
```

## Restart

```
launchctl unload /Library/LaunchDaemons/teleport.plist && \
launchctl load /Library/LaunchDaemons/teleport.plist
```
