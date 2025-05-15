# eventlog

The eventlog package provides access to Event Log on Windows. There's
[golang.org/x/sys/windows/svc/eventlog](https://golang.org/x/sys/windows/svc/eventlog) already,
however that package is hardcoded to always install an event source in the Application log which is
shared among any application. This package builds on top of eventlog from x/sys/windows to enable
adding event sources to custom logs.

Useful resources:

* [Event Logging](https://learn.microsoft.com/en-us/windows/win32/eventlog/event-logging)

## Concepts

First, there's **a log**. There is a bunch of built-in logs, such as Application, Security, or System.
Those are available in the Event Viewer app under "Windows Logs". Then there are custom logs. **A
custom log** is nothing more than [an entry in the
registry](https://learn.microsoft.com/en-us/windows/win32/eventlog/eventlog-key) under
`SYSTEM\CurrentControlSet\Services\EventLog\<log name>` for which Windows creates a custom log. Each
log has its own size and retention policy. Logs created through the eventlog package have their size
set to 20 MB and they overwrite events as needed (oldest events first).

**An event source** can be thought of as a label that's automatically applied to all log messages
written to a single logger. An event source is also [a key in the
registry](https://learn.microsoft.com/en-us/windows/win32/eventlog/event-sources) under
`SYSTEM\CurrentControlSet\Services\EventLog\<log name>\<event source name>`. Each event source
should specify a path to its message file. [**A message
file**](https://learn.microsoft.com/en-us/windows/win32/eventlog/message-files) defines all event
IDs used by an event source. When a program opens a logger, the logger is identified just by the
event source, without a specific log, so event sources live in a kind of global namespace.

Each log message has to have [an event
ID](https://learn.microsoft.com/en-us/windows/win32/eventlog/event-identifiers) assigned. Unlike in
syslog and os_log, Event Log expects event producers to define all possible events upfront. This is
because:

1) It makes localization easier.
2) Event Log is not really like syslog or os_log where the program is expected to write absolutely
everything.

This goes against the approach to logging that we use at Teleport. To work around this, we define a
custom message file whose entire body is just an interpolated string that was sent with the message.
For more details, see [Message file](#message-file).

## Generating logs

Event Log supports just three levels: info, warning, error. Logs can also be distinguished by an
event source and a category. You can think of an event source as a program, though multiple programs
can share the same event source and categories.
[Categories](https://learn.microsoft.com/en-us/windows/win32/eventlog/event-categories) are like our
components set with teleport.ComponentKey. Unfortunately, categories, just like event IDs, must be
set up ahead of time, so we don't use them.

## Reading logs

Logs are primarily consumed through Event Viewer. Custom logs are available under "Applications and
Services Logs" section in the sidebar where you can find the custom log called Teleport that we
typically use. Logs can be exported as .evtx files that can later be opened in Event Viewer or as
text files.

Logs can also be extracted using PowerShell, where `vnet` should be replaced with the event source
name:

```
Get-WinEvent -LogName Teleport -FilterXPath "*[System[Provider[@Name='vnet']]]" -Oldest | Format-Table -Property TimeCreated,LevelDisplayName,Message -Wrap | Out-File event.log
```

## Message file

The source of the message file lives in `msgfile.mc`. It needs to be distributed as a DLL.

To compile it to a DLL, you need to have [the message
compiler](https://learn.microsoft.com/en-us/windows/win32/wes/message-compiler--mc-exe-) (mc.exe)
and the resource compiler (rc.exe). They're both available in Windows 11 SDK. This SDK is available
as an individual component in Visual Studio Installer. If you followed [the Teleport Connect Windows
build process instructions](/web/packages/teleterm/README.md#native-dependencies-on-windows), then
you most likely have installed it already.

To compile `msgfile.dll`, execute the following commands from the root of the repo. They should
result in `msgfile.dll` being created in the `msgfile` directory.

```
. .\build.assets\windows\build.ps1
Compile-Message-File -MessageFile "$PWD\lib\utils\log\eventlog\msgfile.mc" -CompileDir "$PWD\msgfile"
```

In Teleport Connect we distribute this file next to `tsh.exe`. The path to the DLL can be specified
as `CONNECT_MSGFILE_DLL_PATH` during `pnpm package-term`.

Useful resources:

* [Creating your very own event message DLL](https://www.eventsentry.com/blog/2010/11/creating-your-very-own-event-m.html)

### Custom event ID

We use an event ID of 10000 for our single custom event. The event ID can be arbitrary, though the
problem is that after the user removes the app (and thus the message file), the events generated by
the program thus far will "inherit" descriptions from a built-in message file. This would result
in a confusing experience. For example, if we used event ID 100, the summary for each event would
read "Cannot create another system semaphore".

We found that the built-in message file has event IDs going into thousands. ID 10000 was the first
one we could find through trial and error that wasn't occupied. If the message file is removed,
Event Viewer will instead show "The description for Event ID 10000 from source vnet cannot be
found". If the user reinstalls the app, Event Viewer will start showing the messages again. The
messages can be extracted even after the message file is removed, but this involves writing custom
PowerShell scripts.
