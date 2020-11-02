---
authors: Adam Enger (adamenger@gmail.com)
state: discussion
---

# RFD 6 - Reverse Tunneling

## What

Implementation of [RFC4254 Section 7.1, Reverse Port Forwarding](https://tools.ietf.org/html/rfc4254#section-7.1)

## Why

Teleport supports Local Port Forwarding but does not support Reverse Port Forwarding. SSH supports the ability to route remote connections to a local listener. Implementing Reverse Port Forwarding will expand the possible use cases for `tsh` and bridge the gap between `ssh` and `tsh` functionality.

### Use Cases

#### Sharing development web server with remote parties

Sharing development servers with remote parties is useful when testing or exploring new features. For example, one could set up a development server on their workstation listening on port 8080. In this example, users are seperated by seperate home networks so attempting to view the web server over the private LAN will not work. By starting the local dev server and then establishing a reverse tunnel, users are able to connect to port 8080 on the remote server. When connections are made to the remote server, they are proxied back to the local workstation, allowing for remote parties to view the development server.

**Local dev server setup**
```
$ go run server.go
[INFO] listening on port 8080
```

**Reverse Port forward setup**
```
$ tsh ssh -R 8080:localhost:8080 user@node
```

#### XDebug

XDebug is a profiler and debugger for PHP. Using reverse port forwarding, we are able to set up a XDebug daemon on a local workstation on port 9089 and connect the workstation to a remote web server.  This workflow enables faster development and debugging for PHP developers.

On the remote server, we need to configure PHP to connect to XDebug on localhost. Modify the contents of `/etc/php7/php.d/xdebug.ini` to have the following settings:

```
xdebug.remote_connect_back = 0
xdebug.remote_enable = 1
xdebug.remote_host = 127.0.0.1
xdebug.remote_port = 9089
```

Once that's done, restart Apache. When Apache is finished restarting, use:

```
$ tsh ssh -R 9089:127.0.0.1:9089 user@web-server
```

This command establishes a reverse tunnel from your workstation to the remote server. When a request is made to the remote web server, XDebug will connect to the `remote_host` which in this case is a reverse tunnel. Each request sends stack traces, variable dumps and other useful information back to the IDE to assist the engineer through development.

## Server Details

Unlike Local Tunnels, a few things need to be modified on the server side in order to handle incoming ssh forwarding requests.

### Global Requests

Reverse Tunneling requires that the SSH server handles requests of type `tcpip-forward` and `cancel-tcpip-forward`. We will need to expand the global request handler to handle both of these request types and take the appropriate actions when they are encountered.

#### tcpip-forward requests

These requests are sent by the client when the client asks the remote end to open a port. The server should validate whether or not the user has permissions to port forward first. If the user has permission, a listening socket will be opened on the Node and an SSH channel of type `forwarded-tcpip` from the Node to the Client is opened. 

#### cancel-tcpip-forward requests

These requests are sent when the client requests port forwarding to be canceled. The server should respond to these messages with TRUE which will close the remote channel. *NOTE:* channel open requests may be received until a reply to this message is received.

## TSH Client Details

Some modifications are required to the client to support Reverse Tunneling. The syntax for Remote Forwarding with `tsh ssh -R` is similar to local tunneling. Since Reverse and Local Tunnels are so similar, the work for adding support for parsing the Reverse Tunnel cli syntax is minimal. Below is an example of how a party could set up a Reverse Port Forward with `tsh ssh -R`.

```
$ tsh ssh -R 9090:127.0.0.1:9090 user@node
```

This command would dial port 9090 on the client side and set up a remote listener on the node bound to 127.0.0.1:9090. Any connections established on the remote side will be proxied down to the client.

### Tunnel Setup

To establish a reverse tunnel, the tsh client issues a Global Request `tcpip-forward` which the server can permit or deny. If the request is permitted, the client listens for new `forwarded-tcpip` channels. The request to open a channel follows the following format:

```
byte      SSH_MSG_GLOBAL_REQUEST
string    "tcpip-forward"
boolean   want reply
string    address to bind (e.g., "0.0.0.0")
uint32    port number to bind
```

In order to proceed with reverse port forwarding set up, the client expects a response from the server in the following format. If the client passes 0 as port number to bind and has 'want reply' as TRUE, the server should allocate the next available unprivileged port number and reply with the following message; otherwise, there should be no response-specific data.

```
byte     SSH_MSG_REQUEST_SUCCESS
uint32   port that was bound on the server
```

If the server permits the tunnel, a channel will be opened from the remote end. Once the channel is established, the client sets up a net.Dial connection to the specified local socket and bidirectionally copies data between the remote end and client.

When the client shuts down or wants to close the reverse tunnel, it should send a `cancel-tcpip-forward` request in the following format:

```
byte      SSH_MSG_GLOBAL_REQUEST
string    "cancel-tcpip-forward"
boolean   want reply
string    address_to_bind (e.g., "127.0.0.1")
uint32    port number to bind
```
