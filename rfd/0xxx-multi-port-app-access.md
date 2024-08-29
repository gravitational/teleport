### UX

#### Setup

There's a made up app called waldo which accepts connections on multiple ports. Alice wants to make
the app available in the teleport.example.com cluster. To do so, she creates a new app and includes
a new field called `extra_ports` in the app definition. This field is supported in both the app
service config and in the app resource spec.

```yaml
apps:
- name: "waldo"
  uri: "tcp://localhost:4080"
  extra_ports: [4081-4090, 5095]
  labels:
    env: prod
```

#### Usage

TODO: Is it possible to have a situation in which we have a multi-port app

TODO: What should happen in a situation where there's a single-port app and someone connects to it
with VNet using an arbitrary port? We should probably continue to support the current behavior where
all local ports end up being forwarded to the port defined in the app spec.

##### Happy path

###### Regular usage

Bob wants to connect from his device to waldo using a CLI client called waldo-client. Bob sees in
the UI of Teleport Connect that the app waldo.teleport.example.com supports ports 4080 and 4081,
which are the default ports over which waldo-client connects. Bob starts VNet, opens his terminal of
choice and points waldo-client at the app in the cluster.

```
waldo-client waldo.teleport.example.com
```

###### Local proxy

Charlie needs to connect to waldo's debug service. Its API is typically available over the port
5095. Charlie uses Linux, where VNet is not yet available. Through `tsh apps ls` they see that the
waldo app supports the port 5095, so they have to start a local proxy that targets that port:

```
tsh proxy app waldo --target-port 5095 --port 5095
waldo-debug localhost # In a separate shell session.
```

##### Error path

###### Incorrect port

Dave wants to connect to the debug port through VNet, but he makes a typo and provides a port
number that's not in the app spec.

```
waldo-debug waldo.teleport.example.com:5096
```

The connection reaches the app service where it's terminated. waldo-debug observes the connection
being unexpectedly closed. Dave doesn't see any immediate information about an incorrect port – the
app service has no way of passing this information as it does not know about the protocol used by
waldo. Dave has to ask Alice to look into the logs of the app agent. She notices that around the
time when the connection was made, the app service logged a warning saying that port 5096 is not
defined in the app spec.

```
2024-08-23T16:47:02+02:00 WARN [APP:SERVI] Failed to handle client connection. error:[
ERROR REPORT:
Original Error: *trace.BadParameterError port 5095 is not found in neither uri nor extra_ports fields of the app "waldo"
Stack Trace: …
```

###### Invalid app spec

Alice wants to extend the app spec with another port, 6060, but she makes a mistake and forgets a
comma:

```yaml
apps:
- name: "waldo"
  uri: "tcp://localhost:4080"
  extra_ports: [4081-4090, 50956060]
  labels:
    env: prod
```

The relevant UI (`teleport start`, `tctl edit`, the Web UI) returns an error about an incorrect port
in the app spec and prevents Alice from saving such app spec.

### Passing the port number from the client to the app agent

In order to pass the port number from the client to the app service, the underlying local proxy is
going to include the port number as a part of a special subdomain in the SNI, e.g.
`app-teleport-proxy-target-port-1337.teleport.cluster.local`. The ALPN proxy is then able to extract
the port number out of the SNI and pass it to the app service through a new field in
`sshutils.DialReq`.

The app service already fetches the app from the auth service using the public address of the app.
When the port makes its way to the app service, the app service is going to check the port against
ports defined in the app spec before deciding whether to proxy the connection or not.

At this point we don't plan to introduce RBAC for port numbers, but this is something we can
consider in the future. Port numbers on an individual app are akin to database names within a single
database server. The customers might want to treat them as such, by e.g. limiting access to
different ports.

#### Advantages

* A single cert per app.
* With per-session MFA enabled, the user needs to tap an MFA device only once per app.
* Backwards-compatible and single-port-compatible out of the box.

#### Disadvantages

* It's another instance of misusing ALPN to pass some metadata about the connection.
* It's not going to work for HTTP apps.

#### Alternative approaches

##### Embedding the port within an app cert (`RouteToApp`)

The easiest way to implement multi-port app access would be to extend `RouteToApp` to include the
port number. The only drawback here is that the client would need to request a cert for each port.
With per-session MFA in use, every connection over a separate port would require a separate tap of
an MFA device.

##### Embedding the port within an ALPN protocol

We already have [`ProtocolAuth`](https://github.com/gravitational/teleport/blob/8495398fa164aaa70236f6d7abd55238b2925cb2/lib/srv/alpnproxy/common/protocols.go#L99-L100)
in the form of `"teleport-auth@"` that's used to [pass the cluster name](https://github.com/gravitational/teleport/blob/8495398fa164aaa70236f6d7abd55238b2925cb2/lib/srv/alpnproxy/auth/auth_proxy.go#L96-L97)
when dialing the auth server.

In a similar vein, we could introduce `"teleport-tcp@"` which includes the port number after `@`.
It is yet another method of abusing ALPN to pass additional data. But the protocol is already being
used to select an appropriate ALPN handler. Since the handler for multi-port apps and non-multi-port
apps is going to be the same, it seems that passing this info through the SNI is a slightly better
choice.

##### Implementing a custom protocol

Instead of abusing ALPN, we could actually make use of it by implementing a custom protocol, say
`teleport-tcp-multi-port`. The clients speaking this protocol (local proxies) would be expected to
send the port number in the first few bytes of the connection and then proxy the rest of the
downstream connection (of whatever client that wants to connect to a TCP app over a local proxy).

The server speaking the custom protocol on the other end of the connection, the app agent, would
read the port number and proxy the rest of the connection to the app itself on the given port.

This all seems fine until we consider how we could support both multi-port apps and regular apps
with a single port. The ALPN proxy, after recognizing that the client wants to use
`teleport-tcp-multi-port`, would need to forward the connection to an app agent through the reverse
tunnel. For both types of TCP apps, the code that handles the connection would be pretty much the
same, with the exception of reading the port first. Unless we changed the setup so that two kinds of
app services register themselves in the reverse tunnel, the ALPN proxy would need to pass the
information about the multi-port protocol out of band to the app service, which seems to defeat the
purpose of using a custom protocol. The SNI and ALPN protocol solutions already pass the port number
out of band, without the overhead of a custom protocol.

##### Multiple apps with the same URI but different ports

TODO: Move this to the configuration section.

Instead of including port numbers in the definition of a single app, the admin could add multiple
apps to the same app agent where the main difference between the apps would be a different port in
the URI. Then VNet could somehow wrap those apps so that from the perspective of the user, they
wouldn't need to connect to something like `app-1337.teleport.example.com` but rather just
`app.teleport.example.com:1337`.

While this could technically also work for web apps in the future, implementing this means adjusting
every single user-facing tool to support multi-port apps (the Web UI, Connect, tsh, tctl, …). Old
clients that do not support multi-port would see almost identical app repeated multiple times.
Supporting wide port ranges would also be significantly harder, as each port would require its own
app resource. Per-session MFA would require creating a separate cert for each port.

### Configuration

The idea behind adding a new field is to keep backwards compatibility with old clients who are not
aware of multi-port apps. TODO

TODO: Limitations of adding an extra field to an app spec.
Unable to serve different ports of the same app from different app agents.

### Security

SNI is sent over plain text, so the information about the port is both visible and can be rewritten
by a malicious agent (in security terms, not Teleport terms) between the device and the proxy
service.

By using wide ranges of allowed ports, cluster admins can mistakenly grant access to ports that
shouldn't be accessible within Teleport. As there's no RBAC for ports, a user with a valid cert for
an app can access any of the ports specified in the app spec.

### Privacy

N/A

### UI

`tsh apps ls` and `tsh apps ls -R` include a column called Target Ports if any of the returned apps
supports multiple target ports. If a TCP app has only a single port, its port is not shown in Target
Ports – VNet always routes connections from any port to the same single target port.

```
$ tsh apps ls
Application      Description Type Public Address                      Labels   Target Ports
---------------- ----------- ---- ----------------------------------- -------- ---------------------
dumper                       HTTP dumper.teleport-local.dev
example1         Example app HTTP example1.teleport-local.dev         env=test
simplehttpserver             HTTP simplehttpserver.teleport-local.dev
tcp-postgres                 TCP  tcp-postgres.teleport-local.dev
waldo                        TCP  waldo.teleport-local.dev            env=prod 4080, 4081-4090, 5095
```

The Web UI and Connect show a list of available target ports through a three dots button next to the
main button in the resource card. Ports are clickable and result in the hostname + port being copied
to the clipboard. Port ranges are clickable too and result in the hostname + port range being
copied.

In Connect, clicking on a port starts VNet if it's not already running and shows a notification
about the text being copied to the clipboard, similar to how clicking "Connect" next to a TCP app
works today.

TODO: Screenshot.

The user can also select "Connect without VNet" which starts a regular local proxy. From there, the
user is still able to select the target port. This field is not shown if the app doesn't support
multiple ports. The default port is the port from the URI of the app. There will be a basic
validation to make sure that the user does not set the target port to one that the app doesn't
support.

### Proto Specification

### Backward Compatibility

#### VNet with single-port app

### Audit Events

The way audit events work for local proxies with single-port TCP apps is as follows. When the app
service agent proxies a connection to the actual URI of an app, the agent creates an
`app.session.start` event. Each new connection made through the proxy creates another
`app.session.start` event. Each of those events shares the same session ID (`sid`), because [session
ID comes from the cert](https://github.com/gravitational/teleport/blob/d1c7f26e70544879c04f184d88b9fbc07626be6b/lib/srv/app/common/audit.go#L94)
and the cert is the same for all connections coming from a single local proxy.

The only thing that changes for multi-port apps is that `app_uri` includes the actual port to which
the connection was forwarded to. The session ID stays the same no matter which target port was
chosen. This means that two connections on two different ports for the same app are going to
generate two `app.session.start` events which are identical, with the exception of `app_uri`,
`time`, and `uid` fields.

### Observability

Whether the target port makes it all the way from the client to the app service can be observed by
the `app.session.start` audit event. It should include the correct port in the `app_uri` field.

We don't expect the implementation to impact performance. The port number is transmitted between the
client and the proxy service through SNI which is already sent. The proxy service is going to pass
it to the app agent through `sshutils.DialReq`. That struct is serialized to JSON with empty fields
completely skipped, so only connections to TCP apps are going to pay the price for an extra field.

The app service already has access to the app spec to verify whether the target port is valid, so it
doesn't need to perform any additional network calls.

### Product Usage

### Test Plan
