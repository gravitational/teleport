---
authors: Adam Enger (adamenger@gmail.com)
state: discussion
---

# RFD 6 - Reverse Tunneling

## What

Implementation of [RFC4254 Section 7.1, Reverse Port Forwarding](https://tools.ietf.org/html/rfc4254#section-7.1)

## Why

Teleport supports Local Port Forwarding but does not support Reverse Port Forwarding. SSH supports the ability to route remote connections to a local listener. Implementing Reverse Port Forwarding will expand the possible use cases for `tsh` and bridge the gap between `ssh` and `tsh` functionality.

## Details

Unlike Local Tunnels, a few things need to be modified on the server side in order to handle incoming ssh forwarding requests.

### Global Requests

Reverse Tunneling requires that the SSH server handles requests of type `tcpip-forward` and `cancel-tcpip-forward`. We will need to expand the global request handler to handle both of these request types and take the appropriate actions when they are encountered.

#### tcpip-forward requests

These requests are sent by the client when the client asks the remote end to open a port. The server should validate whether or not the user has permissions to port forward first. If the user has permission, a listening socket will be opened on the Node and an SSH channel of type `forwarded-tcpip` from the Node to the Client is opened. 

#### cancel-tcpip-forward requests

These requests are sent when the client requests port forwarding to be canceled. The server should respond to these messages with TRUE which will close the remote channel. *NOTE:* channel open requests may be received until a reply to this message is received.

### tsh client

Minor modifications are required to the client to support Reverse Tunneling. The syntax for Remote Forwarding with `tsh ssh -R` is similar to local tunneling. Since Reverse and Local Tunnels are so similar, adding support for parsing Reverse Tunnels is trivial. Below is an example of how a party could set up a Reverse Port Forward with `tsh ssh -R`.

```
$ tsh ssh -R 9090:127.0.0.1:9090 user@node
```

This would set up a local listener on port 127.0.0.1:9090 and set up a remote listener on 127.0.0.1:9090. Any connections established on the remote side will be proxied down to the client.
