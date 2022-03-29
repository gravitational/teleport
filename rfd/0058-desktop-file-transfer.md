---
authors: Isaiah Becker-Mayer (isaiah@goteleport.com)
state: draft
---

# RFD 58 - Desktop Access - File Transfer

File transfer for Teleport Desktop Access is a feature whose implementation includes navigating the constraints imposed by the RDP protocol, the limitations of the browser, Teleport's architecture, and storage and bandwidth implications. The purpose of this document is to lay out the approaches available to us given these constraints, and compare them in terms of user experience, difficulty of technical implementation, and other relevant criteria in order determine which approach we should take. Therefore, some of the finer technical details of each approach will be left in the abstract in favor of keeping this document concise and related to its core purpose.

## RDP (Server Side) Possibilities and Data Flow

RDP supports two means of transferring files between systems:

1. The clipboard virtual channel extension ([[MS-RDPECLIP]](https://docs.microsoft.com/en-us/openspecs/windows_protocols/ms-rdpeclip)).
2. The file system virtual channel extension ([[MS-RDPEFS]](https://docs.microsoft.com/en-us/openspecs/windows_protocols/ms-rdpefs)).

### Clipboard Virtual Channel Extension (RDP Option 1)

The data flow when using the clipboard virtual channel extension (henceforth referred to as RDP Option 1) includes the one described in [Data Flow and Delayed Rendering](https://github.com/gravitational/teleport/blob/master/rfd/0049-desktop-clipboard.md#data-flow-and-delayed-rendering) in RFD 0049, but with extra steps appended. In the case that a file, directory, or multiple files/directories have been cut or copied onto the shared clipboard, the `Format Data Response PDU` doesn't respond with the file data itself, but instead sends a [`Packed File List`](https://docs.microsoft.com/en-us/openspecs/windows_protocols/ms-rdpeclip/3570c2e4-cdd7-4460-8a7e-1a4595f5ebdc) which contains a list of filenames and associated metadata of the files on the clipboard. The files themselves aren't transferred until the shared clipboard owner receives a [`File Contents Request PDU`](https://docs.microsoft.com/en-us/openspecs/windows_protocols/ms-rdpeclip/cbc851d3-4e68-45f4-9292-26872a9209f2), at which point it sends back the file data in the form of [`File Contents Response PDU`(`s`)](https://docs.microsoft.com/en-us/openspecs/windows_protocols/ms-rdpeclip/df87c178-ab02-471a-acde-bb921aa1af85).

File transfer's are limited to files less than 4GB unless huge file support is enabled (`CB_HUGE_FILE_SUPPORT_ENABLED`).

### File System Virtual Channel Extension (RDP Option 2)

From a high level, the file system virtual channel extension kicks off by the RDP client announcing a directory it wishes to share. From then on out, all functionality on that directory is instigated by the RDP server, carried out by the RDP client, and then if the operation on the client was successful, it is communicated back to and carried out on the server.

For example, if a program on the server wants to read a shared file, it sends a [`Device Read Request`](https://docs.microsoft.com/en-us/openspecs/windows_protocols/ms-rdpefs/3192516d-36a6-47c5-987a-55c214aa0441), and the client reads the requested data and sends it back with a [`Device Read Response`](https://docs.microsoft.com/en-us/openspecs/windows_protocols/ms-rdpefs/d35d3f91-fc5b-492b-80be-47f483ad1dc9). If instead the server-side program wants to write to a shared file, it sends a [`Device Write Request`](https://docs.microsoft.com/en-us/openspecs/windows_protocols/ms-rdpefs/2e25f0aa-a4ce-4ff3-ad62-ab6098280a3a) specifying where and what data it wants to write. In the happy path, the client writes that data to disk and sends back a [`Device Write Response`](https://docs.microsoft.com/en-us/openspecs/windows_protocols/ms-rdpefs/58160a47-2379-4c4a-a99d-24a1a666c02a) with the number of bytes written. If the client-side write fails, it sends back the `Device Write Response` with an `IoStatus` field set to something besides `STATUS_SUCCESS` (such as `STATUS_UNSUCCESSFUL`), which alerts the server make the write operation fail on its end as well.

This clever system makes client implementations easier, because the client isn't responsible for monitoring and alerting the server of changes to the shared files. For example, if a user edits and writes to a shared file on the client side in another program that the RDP client doesn't control (like a text editor), the client doesn't need to recognize the change and communicate it back to the server. Instead, it just sends whatever file data is on disk if/when the server asks for it, and so those sorts of situations are handled without such low level control. Additionally, this system minimizes bandwidth by only sending whatever data is actually needed, when its needed (from client to server). For example, when a directory is first shared, the server only asks for metadata like the file names and sizes in the directory, which it can then display to the user as if those files are living on the remote Windows system. Only when a user actually opens one of those files does it request the file data itself. Also, if the file is a gigantic text file that the user opens in a well optimized text editor, it needn't request the entire file at once. Instead, it can just request the section of the file needed to be displayed at that time. There is no file size limit with this option.

For our purposes, the file system virtual channel extension option (RDP Option 2) can be broken down into two sub options, RDP Option 2a and RDP Option 2b. RDP Option 2a is that we share a directory on the machine running the RDP client (`windows_desktop_service`), and RDP Option 2b is sharing a directory that lives on the Teleport user's client itself (via the File System Access API, discussed in more detail below).

## Client Side Possibilities

Irrespective of which option we choose, we will need some means of initiating and completing file transfers on the client side (from the browser).

### Clipboard API

At first glance, it seemed like using the `Clipboard.read` and `Clipboard.write` API's might be a possibile mechanism for us to make use of. However while some files can be transferred to the client's clipboard with this API, browsers are [constrained](https://w3c.github.io/clipboard-apis/#reading-from-clipboard) to the `text/plain` `text/html` and `image/png` mime types. Ergo unless we want to limit our users to a very narrow set of file transfers, this option is not available to us.

### Browser Upload and Download (Client Option 1)

#### Initialization: `<input type="file">`

All browsers support us initializing a file transfer from the client side using an [`<input type="file">`](https://developer.mozilla.org/en-US/docs/Web/HTML/Element/input/file) button (and/or drag and drop). It's difficult to find canonical information on the size limitations of such transfers, and there may be [a 2GB file size limit for Firefox](https://www.motobit.com/help/scptutl/pa98.htm).

Once a file (or set of files) is uploaded to the browser, it is typically submitted to the server via a POST request. In our case, this approach winds up posing a difficult technical problem due to Teleport's architecture. If we POST the file data, it will need to be handled by an endpoint listening on the `proxy_service`. However that file data will ultimately need to be sent to the Windows server via our RDP client, which is running on the `windows_desktop_service`. Complicating things further, Teleport might have multiple `windows_desktop_service`'s set up, and we would need to develop a mechanism for sending the file to the correct one. (From here on out this problem will be referred to as the "`proxy_service` <--> `windows_desktop_service` problem").

#### Finalization: Browser Download

When a file made ready for transfer on the server side (either by copying it to the system clipboard or by moving it into the shared drive, depending on which RDP Option we go with), it can be displayed as available to the user in a UI widget and they can request to download it. Upon the user selecting "download", the client will need to hit an HTTP endpoint which must respond with the [appropriate headers](https://stackoverflow.com/questions/20508788/do-i-need-content-type-application-octet-stream-for-file-download) to instigate a browser download dialog.

This operation poses a similar problem to the one described above, where we will need to develop some mechanism for getting the file from the rdp client on the `windows_desktop_service` to the `proxy_service`.

#### TDP Upload/Download via the Standard Browser API (not viable for files >50mb)

One option I considered for navigating the `proxy_service` <--> `windows_desktop_service` problem is to avoid it entirely by [grabbing the `FileList`](https://developer.mozilla.org/en-US/docs/Web/HTML/Element/input/file#getting_information_on_selected_files) (from the `<input type="file">` element) and sending the file data to the server in a new TDP message. This would be relatively easy because we already have a TDP connection wrapped in a websocket which the `proxy_service` is proxying to the appropriate `windows_desktop_service`.

Unfortunately it doesn't work as well in the opposite direction -- we could use the same new TDP message to communicate file data back from the server, convert it to a base64 encoded string, and add that string as the `href` property of a link (like [`<a href="data:<data_type>;base64,<base64_encoded_file_content>">`](https://stackoverflow.com/a/38203812/6277051)). However this would limit downloads to small files only, since with this approach the entire file needs to be loaded into memory.

### TDP Upload and Download via the File System Access API (Client Option 2)

There is another option at our disposal which _would_ allow us to both upload and download file data over the websocket without ever needing to hold an entire file in memory, an approach which could be orchestrated by the [File System Access API](https://developer.mozilla.org/en-US/docs/Web/API/File_System_Access_API) (also see [here](https://web.dev/file-system-access/)). The File System Access API gives us the ability to [ask the user for access to a specific directory on their local filesystem](https://web.dev/file-system-access/#opening-a-directory-and-enumerating-its-contents), at which point we obtain [C](https://web.dev/file-system-access/#creating-or-accessing-files-and-folders-in-a-directory)[R](https://web.dev/file-system-access/#read-file)[U](https://web.dev/file-system-access/#write-file)[D](https://web.dev/file-system-access/#deleting-files-and-folders-in-a-directory) capabilities in that section of their filesystem.

Of note is that this API is currently only [available](https://caniuse.com/?search=File%20System%20Access) in some Chromium browsers, the most popular being Chrome and Edge (enabling it in Brave requires modifying the setting at brave://flags/#file-system-access-api).

#### Initialization and Finalization

The initialization and finalization UX and technical details of this option depend on which RDP Option its paired with; that discussion is deferred here in favor of further discussion in the combined option analyses below.

## Implementation Options

The combination of our RDP and Client Options result in the overall set of options available. Not all combinations of the RDP and Client Options make sense UX-wise and/or technically, and so I've only included the ones which I deemed worthy of exploring in greater detail.

### RDP Option 1 + Client Option 1 (Clipboard-Based Option 1)

With this option, the user initializes server-to-client file transfers by copying a file to clipboard and finalizes client-to-server file transfers by pasting the file into the Windows desktop. They initialize client-to-server file transfers by uploading or drag-and-dropping a file in browser, and finalize server-to-client transfers by initiating a browser download by clicking a UI widget that displays the files available on the clipboard.

#### Discussion

This option comes with the tricky `proxy_service` <--> `windows_desktop_service` problem inherent to Client Option 1. TDP would need to be extended, but minimally compared to some of the other options: we would need a `files available` message to list the files available on the server's clipboard and populate the UI widget, and to alert the server that files are available for pasting when uploaded/drag-and-dropped. The rest of the upload and download process would be handled by HTTP endpoints and RDP.

While there is an obvious advantage to keeping TDP as simple as possible, this option would undermine the integrity of the TDP protocol. By pushing so much of the actual file sharing implementation outside of the protocol, TDP is left with an awkward `files available` message for sharing files, but no further messages for actually transferring the available fiels.

This option also carries an integrity problem for the UX. Cut/copy/paste is used only on the Windows side of the equation whereas browser file upload/download is used on the client side, which makes the overall system relatively assymetrical for no reason easily discernible to the user. Additionally, although cut/copy/paste is not used on the client side, the user must still conceptualize the files they upload to the browser as being in the clipboard on the client side.

This option works in all browsers.

### RDP Option 1 + Client Option 2 (Clipboard-Based Option 2)

With this option, the user initializes server-to-client file transfers by copying a file to clipboard and finalizes client-to-server file transfers by pasting the file into the Windows desktop (the same as Clipboard-Based Option 1). They initialize client-to-server file transfers by moving the file to a selected directory on their machine, and finalize server-to-client transfers by initiating a browser download by clicking a UI widget that displays the files available on the clipboard, which then downloads the file into the selected directory (note that this is not a standard browser download, we are instead manually implementing the download with the File System Access API).

In theory another option for here would be to automatically download all files that are cut/copied onto the server's clipboard into the selected client side directory, but such a choice isn't practical due to the bandwidth implications of downloading entire files that the user might not actually even want downloaded.

#### Discussion

This option mitigates the `proxy_service` <--> `windows_desktop_service` problem of Clipboard-Based Option 1, and exchanges greater TDP integrity for somewhat greater TDP complexity. The upload and download of files into the selected client side directory would be done by adding TDP messages similar to RDP's `File Contents Request` and `File Contents Response` PDU's.

The UX of this option has the same clipboard assymetry problem described in Clipboard-Based Option 1. The browser upload/download is swapped out for manipulating files with the File System Access API, but cut/copy/paste is still only used only on the server side.

An idiosyncracy of this option is that we would need to define some semantics for initializing a cut/copy on the client side. For example, we could say that any file sitting in the File System Access API controlled client directory should be be considered "copied" to the clipboard. If we do that, though, then the user will need to more actively manage that directory during their sessions. For example, if they downloaded a file from the remote server (which would necessarily be downloaded into the File System Access API controlled directory on the client side), then it would immediately be "added to the clipboard", and the user would need to move it out of that shared directory right away. Another option is that we could allow files to sit in that directory, and only consider them "copied" when the user hits a "copy" button in the UI, though obviously this increases the complexity of the UI. Regardless of which of these we choose, the UX will be relatively convoluted.

This option would only work in Chromium based browsers that implement the File system Access API.

### RDP Option 2a + Client Option 1 (Shared-Directory-Based Option 1)

With this option, the user initializes server-to-client file transfers by moving a file to a shared directory and finalizes client-to-server file transfers by accessing the file from the shared directory. They initialize client-to-server file transfers by uploading or drag-and-dropping a file in browser, and finalize server-to-client transfers by initiating a browser download by clicking a UI widget that displays the files available on the clipboard.

#### Discussion

This option is the one most comparable to the [file transfer UX in Guacamole](https://guacamole.apache.org/doc/gug/using-guacamole.html#the-rdp-virtual-drive). A notable difference is that as I've envisioned it in Client Option 1, the user will select a file to download from a UI widget, whereas in Guacamole the user initiates a file download by dropping it into the `Download/` directory that is automatically created in the shared drive by Guacamole (this is an option for us as well).

This option has the disadvantages of Clipboard-Based Option 1, minus the lopsided UX of only using cut/copy/paste on the server side. The UX is still relatively asymmetrical in that the user is using browser upload/drag-and-drop and download on the client side, while accessing the native file system on the server side, but the two sides here are more similar as compared with Total Options 1 and 2. Because this option uses Client Option 1, the `proxy_service` <--> `windows_desktop_service` problem remains. Note that Guacamole doesn't have a corollary problem, because their web app (client) is served from the same machine as their server (in other words their "`proxy_service`" equivalent is guaranteed to run on the same machine as their "`windows_desktop_service`", which is among the core reasons this implementation is easier for them).

The `proxy_service` <--> `windows_desktop_service` problem also poses an additional problem here that's not present in Clipboard-Based Option 1 -- because file transfers will appear as a file in a directory on the Windows server, the user will likely have some expectation of that directory persisting between sessions. However because the files are in reality in a directory sitting on the `windows_desktop_service` (see RDP Option 2a), and the Teleport cluster can have multiple `windows_desktop_service`'s at once, there would need to be some mechanism to ensure that the user was reconnected with the same `windows_desktop_service` in subsequent sessions in order for the same files to remain accessible, or else find some other way to share files between `windows_desktop_service`'s.

This option would use the same `files available` TDP extension as Clipboard-Based Option 1, with the same simplicity vs integrity tradeoff.

This option could be made to work in all browsers.

### RDP Option 2b + Client Option 2 (Shared-Directory-Based Option 2)

With this option, the user initializes server-to-client file transfers by moving a file to a shared directory on the server and finalizes client-to-server file transfers by accessing the file from the shared directory on the server. They initialize and finalize client-to-server file transfers the same way, except with a directory on the client.

#### Discussion

This option eliminates the `proxy_service` <--> `windows_desktop_service` problem and has the clearest UX. The user is essentially mounting a piece of their local filesystem as a shared drive on the remote Windows machine.

The primary disadvantage of this option is that it would make TDP substantially more complex. TDP would need to add messages that are functionally equivalent to all the messages needed for shared directory initialization and CRUD operations, which would include most if not all of the messages listed under [3.3.5.2 Drive Redirection Messages](https://docs.microsoft.com/en-us/openspecs/windows_protocols/ms-rdpefs/77b4e4ae-c25a-4aad-bd93-8c9b1f35291b). Our RDP client would receive these RDP messages and pass their parameters into our TDP server, which would then send them to the browser client.

This option would currently only work in Chrome and Edge.

## Discussion

In my opinion the clear winner here is the last option presented, Shared-Directory-Based Option 2. The most important variable from a product standpoint is the UX, and Shared-Directory-Based Option 2 eliminates all the browser clunkiness and clipboard asymmetry of the other options. If the user wants to share a directory they will simply click a button in the UI, select the directory to share from an OS managed window, and that directory would show up on their remote desktop and work precisely as one would expect a shared drive to work.

On top of having the best UX, this option is also arguably the easiest to implement technically. Barring some solution that I haven't seen, its simply much easier for us to pipe all data through the websocket over TDP than to mess around with punching new HTTP endpoints on the `proxy_service` that need to talk to the `windows_desktop_service`, and potentially persist data between sessions, etc. (the `proxy_service` <--> `windows_desktop_service` problem). TDP's increased complexity in this case is just necessary complexity. In order for TDP to be a fully featured and integrated protocol, it will need such functionality. From that angle, the added complexity is not a problem.
Because we are already requesting that users use Chromium based browsers for clipboard sharing, it seems obviously acceptable to limit them to the same for file sharing.

Because we are already requesting that users use Chromium based browsers for clipboard sharing, it seems obviously acceptable to limit them to the same for file sharing.
