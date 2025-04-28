# oslog

The oslog package provides access to the unified logging system on macOS.

Useful resources:

* [Your Friend the System Log](https://developer.apple.com/forums/thread/705868)
* [Logging | Apple Developer Documentation](https://developer.apple.com/documentation/os/logging?language=objc)

## Generating messages

os_log supports [five log
types](https://developer.apple.com/documentation/os/oslogtype?language=objc): debug, info, default,
error, fault. The documentation and os_log(5) state that debug level logging is disabled by default,
but as of macOS 15.4 this doesn't seem to be the case – debug messages are stored in memory. Info
messages are not peristed to disk by default, every other level is logged and persisted to disk. In
our codebase, default is functionally equivalent to warn.

os_log_t can be distinguished by subsystem and category. Think of subsystem as a program (though
multiple programs can share the same subsystem and category) and of category as teleport.ComponentKey.
Unlike teleport.ComponentKey, subsystem and category needs to be set ahead of time when creating a
logger and cannot be specified per message.

Messages have a 1024-byte encoded size limit. Messages over this size will be truncated. The limit
can be increased to 32 kilobytes on a per-subsystem or per-category level through
`Enable-Oversize-Messages`, see os_log(5) and [Customizing logging
behavior](#customizing-logging-behavior). Apple does not recommended enabling this for
performance-sensitive code paths. Since at Teleport any component can log long stack traces at any
code path, it's generally recommend to leave it turned off.

All messages are logged as [public messages](https://developer.apple.com/documentation/os/generating-log-messages-from-your-code?language=objc#Redact-Sensitive-User-Data-from-a-Log-Message)
since the Teleport codebase doesn't have the notion of public and private log messages.

## Reading messages

There are two main ways to consume logs. There's Console.app which is a convenient GUI for searching
and filtering logs. Just make sure to include info and debug messages since they're not shown by
default (options for including them are under the Action menu). When viewing message details,
Console.app shows "Volatile" next to the message level if the message is stored only in memory.

The other way to read logs is the `log` CLI tool. It can dump logs to a file, it can show them in a
pager, it can stream them. It supports predicate filtering.

```
# Stream logs from tsh process.
log stream --level debug --style syslog --predicate 'process == "tsh"'

# Show logs from a specific subsystem.
log show --info --debug --style syslog --predicate 'subsystem == "com.goteleport.tshdev.vnetd"'
```

## Customizing logging behavior

There are at least two ways in which logging behavior can be customized:

* `OSLogPreferences` dictionary in `Info.plist` of an app (see os_log(5) for available options).
* `sudo log config` with `--subsystem` and `--mode` flags.

In theory there's also `/Library/Preferences/Logging/Subsystems/` which is mentioned by the Apple
Developer Documentation, but we haven't tested it yet.

In theory, this enables developers to customize the behavior without modifying the code. In
practice, we found that configuring logging through `sudo log config` doesn't seem to have an effect
on an app with specific options set in `OSLogPreferences`, almost as if `OSLogPreferences` from
`Info.plist` always took precedence over what's set through `sudo log config`.

* [Customizing Logging Behavior While Debugging | Apple Developer Documentation](https://developer.apple.com/documentation/os/customizing-logging-behavior-while-debugging?language=objc)
