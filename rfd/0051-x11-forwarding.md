---
authors: Brian Joerger (bjoerger@goteleport.com)
state: implemented
---

# RFD 51 - X11 Forwarding

## What

X11 forwarding makes it possible to open a graphical application on a remote
machine and but forward its graphical (X) data to the client's local display.  
[RFC4254](https://www.rfc-editor.org/rfc/rfc4254#section-6.3) details the
protocol for supporting X11 forwarding in an SSH server/client.

## Why

X11 forwarding is used by some SSH users who are switching over to Teleport.
Currently, we only support X11 forwarding between an OpenSSH client and server
in proxy recording mode. As users move more of their fleet to Teleport, there
is a growing demand for supporting X11 between any combination of OpenSSH
server/client and Teleport server/client.

## Details

X11 forwarding is a sparsely documented protocol with many hidden intricacies.
In implementing X11 forwarding for Teleport I leaned heavily upon OpenSSH
implementation, which is (was) the only complete open source X11 forwarding
implementation.

This RFD will explain some key concepts and terms for X11 forwarding, the general
flow of the implementation, and the security details and decisions.

There are also two sections describing the main deviations made from the OpenSSH
implementation:
 - `Unix sockets instead of TCP sockets`
 - `Untrusted as default mode`

### X Server and Client

An X Server is an application on a machine which manages graphical displays, input
devices, etc. Linux systems ship with an XServer (XOrg), while other operating
systems require a standalone XServer (ex. XQuartz for Mac, XMing for Windows).

An X Client is a application which interacts with an X server to send/receive
graphical input and output. For Max and Linux, most terminals have an X Client
built in. On Windows, you have to install a standalone X Client such as MobaXTerm.

When an X Client makes an X request, it parses the set `$DISPLAY` to send an X request
to a specific display opened in the X Server. the first portion of this request is
an authorization packet used to authenticate and authorize the X Client.

The X Client gets the authorization packet from `$XAUTHORITY` or `~/.Xauthority`.
If there is no matching authorization data in the file, or it doesn't match what
the X Server expected, then the X request will be denied.

### X Security extension

The X Security extension was created about 10 years after the 11th version of X. The
extension introduced `untrusted` tokens which could be used to grant fewer X privileges
to the user of the token. It also introduced the ability to set a timeout for the token.

In contrast, normal `trusted` tokens give full unmitigated X privileges, allowing for action
such as screenshots, monitoring peripherals, and many other actions one would expect to
perform on local machines. In the case of X11 forwarding however, this means that the
client is providing the SSH server unmitigated access to its local X Server. If the server
becomes compromised or another user has access to your remote user, then the Client's
local XServer could be subject to attacks such as keystroke monitoring to capture a password.

Unfortunately, since the X Security extension was created as an afterthought, it is not
built into the X protocol and has some serious performance drawbacks. The performance 
drop is sometimes in the ballpark of 10x. Additional, on release many X Clients didn't even 
support the X Security Extension properly and would crash immediately or at some point while
running. For example, if an X Program reads the user's clipboard and doesn't handle failure,
then the program would crash when used with the security extension since clipboard is a
privileged action.

Despite these issues, `untrusted` forwarding is a very important addition to the x11
forwarding protocol and should not be overlooked by users, even in the face of performance
and compatibility issues. Additionally, these issues appear to have been improved in the
last two decades, though it is hard to ascertain the extent and breadth of the improvement.

For a more elaborate explanation of the potential dangers of trusted (and untrusted) x11
forwarding, give this [article](https://www.giac.org/paper/gcih/571/x11-forwarding-ssh-considered-harmful/104780)
a look.

### X11 Forwarding flow

1. Client opens an SSH session and then sends an `x11-req` ssh request to the server. 
   The `x11-req` includes an X authentication protocol and cookie, which will be detailed
   in the security sections below.
2. Server receives request and either approves or denies the request.
3. If the server approves, it will open an XServer proxy listener to catch any
   XServer requests within the SSH session.
4. Upon receiving an XServer request on the proxy listener, the server sends an
   `x11` channel request to the client.
5. If the client accepts, the client will start X11 forwarding between the x11
   channel and the client's local display - `$DISPLAY`.
6. Server begins X11 forwarding between the x11 channel and XServer proxy connection.
7. The XServer request is now connected to the client's local display. For example, if
   `xeyes` was run, then the program would be running on the server, but any graphical
   input/output would the client's local XServer.

### X Server Proxy

In step 3 above, the SSH server opens an X Server Proxy. More specifically, the server
starts listening on a unix socket so that it can accept requests to the corresponding
display. 

First, the server has to find an open display socket. A display's unix socket can be 
determined by its display number - `/tmp/.X11-unix/X<display_number>`. In order to avoid
overlapping with any local display sockets, which start from display number 0, we start
searching for an open display number between 10 and 1000. Both of these numbers can be
configured up to the largest display number supported by the X Protocol - the max 
Int32 (2147483647).

During session shell creation, we can set `$DISPLAY` and X authorization data for the
session. The `$DISPLAY` is set to `unix:<display_number>`, which will be translated
to the open socket during an X Client request. X Authorization data will be set in
the user's default xauthfile (`~/.Xauthority`) by calling 
`xauth add <$DISPLAY> <x11-req.proto> <x11-req.cookie>`. 

#### Unix sockets instead of TCP sockets

In the OpenSSH implementation, The X Server Proxies are opened as tcp sockets, from
`localhost:6010` to `localhost:7009`. This net listener created by the SSH session
child process on the server, and served by the parent process which holds the remote
connection.

Due to the re-execution model of Teleport SSH sessions, the parent and child processes
cannot share the listener easily like this. Instead, we've opted to use unix sockets
so that the listener can be opened and served by the parent process, while allowing 
the child process to perform a `chown` call to become the owner of the socket afterwards.

This decision also comes with a couple additional benefits:
 - There are many more unix display sockets available than tcp display sockets (65535 vs 2147483647)
   - For this reason Teleport also allows the max display supported to be configurable,
     while OpenSSH only allows 1000.
 - If a server is running both SSHD and Teleport with X11 forwarding, the sockets will not overlap.

### Security

There are four points of contact which concern security within the X11 forwarding flow
above:

1. Client sends `x11-req` to server to allow X11 forwarding during the Session.
2. Session user makes an X Client request to the SSH Server's proxy X Server.
3. Server requests to open an `x11` channel to client to forward X Client request.
4. Client forward X Client request to local X Server.

#### Server Authorization - X11 Forwarding Request

The initial X11 forwarding request made by the client will be denied by the server in
the following cases:

 - The Server does not have X11 forwarding enabled in its `teleport.yaml`.
 - The Server does not have an X Authority configured, specifically the `xauth` binary.
 - The Client's SSH certificate does not have the `permit-X11-forwarding` extension.

If the request is authorized, then the Server will read the X Authorization protocol and
cookie attached in the request and set `$DISPLAY` and `~/.Xauthority` as explained above.

#### Session Authorization - X Client Request

Once X11 forwarding is enabled for a session, the Session user can make an X Client
request and start X11 forwarding. As explained in the X Server Proxy section, the 
SSH Session's X Client requests will leverage the set `$DISPLAY` and the X Authorization
data set in `~/.Xauthority` to connect to the X Server Proxy.

If a user attempts to send an X Client request for the `$DISPLAY` with a different xauth
file, it will be denied. However it's important to note that any remote user that does
have access to `~/.Xauthority` will be able to perform X11 forwarding as if they were
in the session. This is where understanding `trusted` vs `untrusted` forwarding is 
very important, which have their own sections below.

#### Client Authentication - X11 Channel Request

When the SSH client receives an `x11` channel request, it must authenticate the server
before providing access to the client's local X Server. To do this, it will use the
X authorization data it sent in the original `x11-req`

Note: The X authorization data sent in the `x11-req` is fake X authorization data
created by the server. 

First, the client sends fake X authorization data to the server in the `x11-req` request.
As detailed in the SSH RFC, this data includes an X authorization protocol and a random
cookie of that protocol. We use the `MIT-MAGIC-COOKIE-1` protocol, which calls for a random
128 bit hexadecimal encoded cookie. This is the simplest and most common protocol.

When the client receives an `x11` SSH channel request from the server, it mimics and
X Server by scanning the connection for its X authorization packet, the first portion
of the request. If the X authorization packet includes the protocol and cookie sent by
the client, then the server will be trusted.

Note: The X authorization data used in this step is fake and does not actually provide
access to the client's XServer. It's essential that an XServer's X authorization data
is never shared outside of the device, or else the XServer can be compromised.

#### Client X Server Authorization

To complete X11 forwarding, the client must start forwarding between the `x11` channel
and the local X Server. However, the X authorization data in the `x11` channel request
was the fake data we created. So before forwarding the X request to the X Server, the
SSH client replaces the fake data with real X authorization data. This data will either
be trusted xauth data, or untrusted xauth data, depending on what the client requested.

##### Untrusted X11 Forwarding

Untrusted X authorization data must be generated through `xauth` in order to be recognized
by the X Server. To do this, the client runs `xauth generate <$DISPLAY> untrusted <timeout?>`.
Any X Client request using this untrusted token will be be subject to the X Security extension,
granting it fewer privileges than usual. 

##### Trusted X11 Forwarding

In trusted X11 forwarding, we can rely on the client's local environment. If we fill in 
the X authorization packet in the request with a random cookie, the X Server will just
ignore it and connect as if it received the request locally.

### tsh UX

New `tsh ssh` flags and ssh options:
 - `-X`/`--x11-untrusted`: requests `untrusted` X11 forwarding for the session.
 - `-Y`/`--x11-trusted`: requests `trusted` X11 forwarding for the session.
 - `--x11-untrusted-timeout=<duration>`: can be used with untrusted X11 forwarding to set
   a timeout, after which the xauth cookie will expire and the server will reject any new requests.

Additionally, we will support the following OpenSSH flags for user's who want the same UX as OpenSSH:
 - `-oForwardX11Trusted=<yes/no>`: sets the X11 forwarding trust mode.
   - When set to `yes`, X11 forwarding will always be in `trusted` mode, whether through the
     `-X`, `-Y`, or `-oForwardX11=yes` flag.
   - When set to `no`, normal behavior will be used, meaning `-X` and `-Y` will lead to `untrusted`
     and `trusted` forwarding respectively. `-oForwardX11=yes` will lead to `untrusted` forwarding.
 - `-oForwardX11=yes`: requests X11 forwarding for the session.
   - When used, `oForwardX11Trusted` will default to `yes` unless explicitly set.
 - `-oForwardX11Timeout=<int>`: same effect as `--x11-timeout`.

#### Untrusted as default mode

In OpenSSH, the X11 forwarding flags and options have very unintuitive UX. Essentially, in
many OpenSSH distributions, both `-X` and `-Y` will start X11 forwarding in `trusted` mode, 
unless the client explicitly sets `ForwardX11Trusted=no` in their ssh client config. In my
experience and research, this leads to many users becoming confused, not knowing if they are
using `trusted` or `untrusted`, and having no idea what the difference is.

The reason for this strange UX behavior is that the X Security Extension had quite a contentious 
release. On top of the security benefits being confusing and unclear to many users, using the
extension initially led to many X programs to perform poorly or even crash, as mentioned in the
previous section.

It seems that this UX decision is ultimately the result of trying to include an incompatible
yet important security fix without having any effect on existing users of X11 forwarding.
In other words the X Security Extension is an opt-in feature in OpenSSH X11 forwarding as a
result of its issues.

Two decades later, I believe we have no reason to follow the same UX and should instead learn
from the mistakes of OpenSSH. As a security centered product, the security implications of a 
user's actions should be as clear and unambiguous as possible. 

From the get-go, `tsh ssh -X` and `tsh ssh -Y` will always lead  to `untrusted` or `trusted`
forwarding respectively. If a user runs into an X program which still doesn't support the X 
Security Extension, I would prefer that they experience the crash, come to customer support
or other sources to discover the reason, and personally decide whether to switch to `trusted`
forwarding or keep their X Server secure.

This does leave us with the unfortunate fact that the UX of `tsh` and `ssh` will be different
and may lead to some confusion for customers who are making the switch from `ssh` to `tsh`.
However, the resulting UX of `tsh` should be intuitive and clear enough to guide the user
through this switch, or the user has the options to use the supported SSH flags to maintain
the familiar UX.