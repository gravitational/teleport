---
authors: Isaiah Becker-Mayer (isaiah@goteleport.com)
state: implmented (v10.2.0)
---

The next major feature for Teleport Desktop Access is referred to as "file
transfer" - by which we mean some ability to get files both:

- From a user's workstation to a remote Windows desktop
- From a remote Windows desktop to the user's workstation

There are a variety of technical approaches that could provide this feature,
each coming with it's own pros and cons as well as lending itself to a
particular user experience. The goal of this RFD is to discuss the various
options and select a path forward.

## Required Approvers

- Engineering: @zmb3 && (@probakowski || @LKozlowski)
- Product: (@klizhentas || @xinding33)

## Overview

In Teleport Desktop Access, data travels from the user's browser, to a Teleport
proxy (over websockets) where it is then passed to a Windows Desktop Service
(over mTLS). The Windows Desktop Service initiates the RDP connection to the
Windows Desktop.

```

                         Teleport Desktop Protocol                         RDP
                ------------------------------------------        ---------------------
                |                                        |        |                   |
+----------------------+     +------------------+  +------------------+     +------------------+
|                      |     |                  |  |                  |     |                  |
|                      |     |                  |  |    Teleport      |     |                  |
|  User's Web Browser  ------|  Teleport Proxy -----  Windows Desktop ------|  Windows Desktop |
|                      |     |                  |  |     Service      |     |                  |
+----------------------+     +------------------+  +------------------+     +------------------+
```

In order to transfer files across this path, there are two major considerations.

The first is for transferring the file between the Windows Desktop and the
Teleport Windows Desktop Service over the RDP connection, for which we consider
the following options (we'll call them 1 and 2):

- (1) using the clipboard virtual channel extension
- (2) using the file system virtual channel extension

The second is how to transfer the files between the user's browser and Teleport,
for which we consider:

- (a) using native browser upload/download functionality
- (b) using the file system access API

This RFD will outline the various options and select one from each group for
implementation. In other words, the full set of possible outcomes are 1a, 1b,
2a, or 2b.

## RDP File Transfer

RDP supports two means of transferring files between systems:

1. The clipboard virtual channel extension
   ([[MS-RDPECLIP]](https://docs.microsoft.com/en-us/openspecs/windows_protocols/ms-rdpeclip)).
2. The file system virtual channel extension
   ([[MS-RDPEFS]](https://docs.microsoft.com/en-us/openspecs/windows_protocols/ms-rdpefs)).

### Clipboard Virtual Channel Extension (RDP Option 1)

This approach relies on the clipboard to transfer files, which lends itself to a
user-experience that involves using copy/paste functionality as a way to
initiate file transfers.

The data flow when using the clipboard virtual channel extension (henceforth
referred to as RDP Option 1) is similar to any clipboard data, and described in
[Data Flow and Delayed Rendering](https://github.com/gravitational/teleport/blob/master/rfd/0049-desktop-clipboard.md#data-flow-and-delayed-rendering)
in RFD 0049.

Like with clipboard data, RDP's "delayed rendering" approach notifies the RDP
client when a file (or files) is copied to the clipboard, but doesn't send the
contents of the file until the client requests them. This was designed for
network efficiency - it avoids the need to transfer a copied file until it is
actually pasted. This approach doesn't work well with Teleport's Desktop
Protocol, as we don't implement delayed rendering - the notification that was
copied includes the copied data. While this is acceptable for small amounts of
text, it will be inefficient for large files.

Additionally, files transferred via this mechanism are limited to a maximum size
of 4GB unless "huge file support" is enabled. This is a protocol extension that
would require extra implementation to support.

Lastly, this approach is best suited to one-time transfers of a specific file
(copy file, paste on other machine, make some edits, copy and paste the updated
version back). It is not well suited to sharing an entire directory or making
many files available to the remote system simultaneously.

### File System Virtual Channel Extension (RDP Option 2)

The other option for transferring files over an RDP connection is the file system
virtual channel extension. This is a feature supported in many native RDP
clients. After the connection is established, the RDP client announces a
directory it wishes to share. This announcement includes basic metadata about the
directory contents, but not the files themselves.

From this point on, the RDP server makes the directory appear as if it were any
other directory on the Windows filesystem. When the user attempts to perform any
sort of file access, the RDP server initiates a request to the client and the
client is able to respond with the file data.

From a user's perspective, there is no distinct "file transfer" - the files in
the shared directory simply appear on the Windows machine, where they can be
copied, opened directly, or even edited. As long as the RDP client responds to
the server's various requests, the RDP server takes care of making the remote
file behave like any other file.

From an implementation perspective, this approach is simpler because the client does
not need to monitor the directory for changes or alert the server.

From an efficiency standpoint, this approach saves on network bandwidth as the RDP
server is smart enough to ask only for what it needs. For example, when viewing the
shared directory in Windows Explorer, the RDP server will only ask for file metadata
until the user actually tries to open a file (at which point it requests file contents).
Lastly, the 4GB size limit of the clipboard-based approach does not apply here.

With this approach we can either elect one of two subcases:

1. Advertise a shared folder that lives on the host running the Windows Desktop Service.
   Windows Desktop Service would be responsible for responding to any requests from the RDP
   server, and it would be up to us to figure out a separate means for transferring files
   between the user's browser and the Windows Desktop Service.
2. Advertise a shared folder that lives on the user's workstation. In this case, we would
   define TDP equivalents for the RDP messages related to file changes and requests, and
   Teleport would forward them all the way to the user. It would ultimately be up to the
   Teleport client running in the user's browser to respond to them.

## Client Side Possibilities

Once we've decided how we will transfer files between the Windows desktop and
Teleport, we still need to determine how we will initiate / complete file transfers
from the user's browser.

For these options, we need to determine both initialization (or how the user
will start a file send operation) and finalization (how the user will receive a
file from the remote desktop).

### Clipboard API (not selected)

Since RDP supports file transfer via the clipboard, one option would be to use
the same mechanism to transfer files between the browser and Teleport's backend.
Unfortunately, browsers are
[constrained](https://w3c.github.io/clipboard-apis/#reading-from-clipboard) to
the `text/plain` `text/html` and `image/png` MIME types when using the clipboard
API.

This option is not suitable for us, as we want users to be able to transfer any type of file.

### Browser Upload and Download (Client Option a)

This approach uses traditional browser operations for upload and download

#### Initialization: `<input type="file">`

All browsers support initializing a file transfer from the client side using an
[`<input type="file">`](https://developer.mozilla.org/en-US/docs/Web/HTML/Element/input/file)
button (and/or drag and drop). With this approach, the user would click some
sort of "browse" button, and use a native dialog to select a local file for
upload. It's difficult to find canonical information on the size limitations of
such transfers, and there may be
[a 2GB file size limit for Firefox](https://www.motobit.com/help/scptutl/pa98.htm).

Once a file (or set of files) is uploaded to the browser, it is typically
submitted to the server via a POST request. In our case, this approach winds up
posing a difficult technical problem due to Teleport's architecture. If we POST
the file data, it will need to be handled by an HTTP endpoint listening on the
`proxy_service`. There are several issues with this approach:

1. It occurs "out of band" - data is transferred over HTTP and not over the existing
   websocket connection that is servicing the desktop session.
2. The proxy that receives this data will need to get it to the Windows Desktop Service
   that is handling the RDP connection. There may be multiple Windows Desktop Services,
   so Teleport will need additional logic to identify the right service.

We could avoid these issues by grabbing the files directly from the input
element and sending them to the Teleport proxy over the existing websocket
connection (in a new TDP message). This workaround is suitable for uploading
files to the remote desktop, but not for downloading files from the remote
desktop.

#### Finalization: Browser Download

When a file made ready for transfer on the server side (either by copying it to
the system clipboard or by moving it into some shared drive, depending on which
RDP Option we go with), it can be displayed as available to the user in a UI
widget and they can request to download it. Upon the user selecting "download",
the client will need to hit an HTTP endpoint which must respond with the
[appropriate headers](https://stackoverflow.com/questions/20508788/do-i-need-content-type-application-octet-stream-for-file-download)
to initiate a browser download dialog.

The user experience of this approach would involve some sort of visual notification
that a new [remote] file is available for download, and the user would have the option
of clicking to downloaded.

This operation poses a similar problem to the one described above, where we will
need to develop some mechanism for getting the file from the RDP client on the
`windows_desktop_service` to the `proxy_service` serving the websocket
connection.

To summarize, any sort of native browser download requires that the file is
streamed over HTTP from the Teleport proxy, and this out of band approach does
not fit well with Teleport's architecture. This approach is also less convenient
for a user, as they have to perform manual actions outside the remote desktop
interface to perform file transfers.

### TDP Upload and Download via the File System Access API (Client Option b)

The alternative to native browser upload/download functions (which operate over
HTTP), is to implement the file transfer in TDP messages over the existing
websocket connection and use the
[File System Access API](https://developer.mozilla.org/en-US/docs/Web/API/File_System_Access_API)
to read and write to files on the local disk. This API allows us to ask the user
for permission to access a specific directory on their local file system and
exposes CRUD functionality for that directory:

- https://web.dev/file-system-access/
- https://web.dev/file-system-access/#opening-a-directory-and-enumerating-its-contents
- https://web.dev/file-system-access/#creating-or-accessing-files-and-folders-in-a-directory
- https://web.dev/file-system-access/#read-file
- https://web.dev/file-system-access/#write-file
- https://web.dev/file-system-access/#deleting-files-and-folders-in-a-directory

This approach pairs best with RDP's virtual file system channel extension. Since
the RDP server initiates all actions, the browser can respond by usign the file
system access API. From a user-experience perspective, the user never has to
think about a file "transfer" - they just select a directory and then interact
with it from the remote machine.

The biggest drawback with this approach is that this API is currently only
[available](https://caniuse.com/?search=File%20System%20Access) in some Chromium
browsers, the most popular being Chrome and Edge (enabling it in Brave requires
modifying the setting at brave://flags/#file-system-access-api). However, this
seems acceptable since some desktop access features (clipboard sharing) already
require a Chromium based browser.

## Implementation Options

In this section, we'll summarize the combinations of options available to us.

- Option 1a: Pair RDP clipboard with native browser upload and download
- Option 1b: Pair RDP clipboard with browser's file system access API
- Option 2a: Pair RDP virtual file system extension with native browser upload/download
- Option 2b: Pair RDP virtual file system extension with browser's file system access API

### Option 1a: Pair RDP clipboard with native browser upload and download (rejected)

With this option, the user uploads a file by:

- using drag and drop or clicking an "open" button in browser to initiate the transfer
- "pasting" in the remote desktop

The user downloads a file by:

- copying a file in the remote desktop
- then clicking download in a UI widget that displays available files

This approach should work in all browsers.

In both cases, the file transfer is a two step operation, and the steps are
seemingly unrelated/asymmetric. This is a poor user experience, and suffers from
the "out of band" issues on the technical implementation.

This option is **NOT** selected.

### Option 1b: Pair RDP clipboard with browser's file system access API (rejected)

With this option, the user uploads a file by:

- moving the file into a selected directory on their local machine
- "pasting" in the remote desktop

The user downloads a file by:

- copying a file in the remote desktop
- then clicking download in a UI widget that displays available files

This is identical to option 1a, except the upload is initiated by dropping the
file in a local directory rather than via drag/drop. It suffers from the same UX
issues.

This option only works in Chromium-based browsers that support the file system
access API.

This option is **NOT** selected.

### Option 2a: Pair RDP virtual file system extension with native browser upload/download (rejected)

With this option, the user uploads a file by:

- drag and drop or upload dialog
- accessing the file in a shared directory on the windows desktop

The user downloads a file by:

- moving a file to a shared directory on the windows desktop
- then clicking download in a UI widget that displays available files

This option is the one most comparable to the
[file transfer UX in Guacamole](https://guacamole.apache.org/doc/gug/using-guacamole.html#the-rdp-virtual-drive).
The UX is still relatively asymmetrical in that the user is using browser
upload/drag-and-drop and download on the client side, while accessing the native
file system on the server side, but the two sides here are more similar as
compared to either of the Clipboard-Based options.

This option should work in all browsers, but it does not solve the "out of band"
issue. Additionally, it gives the impression of a true "shared folder" but does
not provide persistence. Since the shared folder actually lives on the Windows
Desktop Service, there are no guarantees that future sessions route through the
same service where those files are present. Lastly, care must be taken to ensure
that shared files do not exceed the disk on the Windows Desktop Service and are
appropriately cleaned up.

### Option 2b: Pair RDP virtual file system extension with browser's file system access API (accepted)

With this option, the user uploads a file by:

- move the file to a shared directory on the local machine (if it isn't already there)
- accessing the file in a shared directory on the windows desktop

The user downloads a file by:

- moving a file to a shared directory on the windows desktop
- accessing the file in a shared directory on their local machine

This option provides the best UX, as the conceptual model of a "shared
directory" holds. Users can think of this directory as a place to drop files
that need to be accessed by both ends, and don't need to think about explicit
steps to transfer a file. It is also consistent with native RDP clients, which
current RDP users will appreciate.

This approach avoids the "out of band" problem because file transfer would occur
over the existing TDP connection. Additionally, since the files only ever live
on the user's workstation or on the Windows Desktop, Teleport doesn't need to
worry about cleaning up files or running out of disk space.

The primary disadvantages of this approach are:

- it only works in Chromium-based browsers (a concession we've previously accepted)
- it makes the TDP protocol more complex, as we need a 1-1 mapping of the TDP messages
  to the RDP messages that implement Drive Redirection.

https://docs.microsoft.com/en-us/openspecs/windows_protocols/ms-rdpefs/77b4e4ae-c25a-4aad-bd93-8c9b1f35291b

This option is the **accepted approach** and will be implemented in an upcoming Teleport release.
