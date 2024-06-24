---
authors: Jeff Anderson <jeff@goteleport.com>
state: draft
---

# RFD 0092 - Error Message Guidelines

As more companies deploy Teleport, more and more less-technical users whose eyes glaze over at highly technical error messages and stacktraces will be using our products.

Teleport can adopt some opinions on how its error messages and stacktraces are communicated to both the Teleport administrators and end users.

While much of this document is hopefully describing things that may already be goals, I wrote it based on the list of "case studies" further down. These are actual stacktraces and error messages that customers have asked about through support channels. I've tried to include at least one reference for each one, but I didn't do an exhaustive search to enumerate all occurrences.

## Suggested Opinions to adopt

After going through several cases studies, the following guidelines should be taken into account when determining error, logging, or stacktrace behavior.

### Unify the logging strategy

There are currently subtle differences in what logs and error messages look like. Sometimes, the stack trace is escaped and prints a literal `\n` instead of a newline. Sometimes it prints a multiline stacktrace. Sometimes html escaped entities are used, like `&gt;` or `&#39;`, which clog up the readibility.

This isn't a huge concern, but the subtle variations that we do can get in the way from time to time.

### prefer short messages instead of multiline messages

Full Stacktraces should be unexpected and rare. Any error message that is lower than debug should be a shorter version of what might be a multiline stacktrace or error message. Many shorter messages already exist in the product today. This is an example of one of the longer single line messages, but it does a good job getting relevant information across:

```
Aug 10 20:48:46 ip-172-33-11-148 teleport[11843]: 2022-08-10T20:48:46Z ERRO [PROC:1]    Node failed to establish connection to cluster: Post "https://telepport.example.com:443/v1/webapi/host/credentials": x509: certificate specifies an incompatible key usage, invalid character '<' looking for beginning of value. pid:11843.1 service/connect.go:113
```

There is no embedded/escaped stacktrace. Just a user message, and the name of the go file and line number. This is a good amount of information.

### full stacktraces should appear only at the DEBUG level

A non-debug level short message can appear at INFO/WARN/ERROR, immediately followed by a DEBUG level message with the stacktrace.

See the [network error when writing to tls client times out] case study for an example.

### Error messages should play an advisory role

This mostly a suggestion for the CLI utilities. It may also make sense for actual teleport service log messages in some cases.

A message that is simply stating the existence of a problem isn't always enough of a hint as to what should be done next. This is especially true for beginners and non-technical users. A shift in the tone of error messages to actively instruct what possible actions to take can help reduce frustration and support load.

This could include a suggestion, a link to a doc, or another piece of situational advice.

For example:

* an error message about a client getting a malformed response from an http:// endpoint could advise `check whether {{hostname}}:{{port}} is something other than an http endpoint.`
* receiving a 403 response from teleport when joining with a token should advise the user that it is an expired token rather than just tell the user it got an unexpected response. (see the [missing or expired token] case study)
* tsh having an error like `listen tcp 127.0.0.1:14432: bind: address already in use.` could include a link to the docs for how to deal with this type of problem, even though it is kind of a "beginner" level problem. Not all tsh users will be technical.
* The [but I am already an admin?] case study suggests a small edit along this theme.

### A stacktrace from a User-facing CLI interface should be considered a bug

This will help accomodate non-technical users. Having robust error handling and messaging will contribute to the general feeling of maturity of the teleport software.

Even for a technical end user, it can be mildly jarring seeing a full blown multiline stacktrace from a CLI, especially if it's something like a mundane network error. As an anecdote, I don't recall seeing an error along the lines of a stacktrace from mature C/C++ utilities that have been around for years.

### Network Errors should almost never need a stacktrace

Things like connection refused, no route to host, dns failures, etc generally don't need a stacktrace. They might need some indicator as to where the value is coming from, such as endpoints value discovered via `/webapi/ping`.

When troubleshooting a network-related error, you only need to know what part of teleport is making the connection, where it is trying to connect to, and what the nature of the error is. Enriching these typical errors with clues or advisory statements will smooth things out when the unexpected happens.

### Eliminate generic error messages 

Specifically, with golang projects, `context deadline exceeded` pops up and doesn't really give information on what is really going on. It just means that something took too long. There's a whole section of case studies for this specific error message.

The Docker project ended up working to actively catch every one of these errors to pass along better information to users. It is always frustrating for  a user who searches that common phrase and finds loads of causes that are unrelated to their actual problem.

The same policy of should be applied to any other vague thing that has more to do with the programming language rather than whatever the task at hand from the user perspective.

## Case Studies

These are all examples of stack traces that Teleport users have run into.

## but I am already an admin?

```
user@macbook ~ % tsh status
> Profile URL:        https://teleport.example.com:443
  Logged in as:       admin
  Cluster:            example
  Roles:              admin, contractor
  Logins:             ubuntu, ec2-user, -teleport-internal-join
  Kubernetes:         enabled
  Kubernetes groups:  system:masters
  Valid until:        2022-08-11 01:36:00 -0400 EDT [valid for 11h53m0s]
  Extensions:         permit-X11-forwarding, permit-agent-forwarding, permit-port-forwarding, permit-pty

user@macbook ~ % tc
user@macbook ~ % tctl get all --with-secrets
ERROR: this request can be only executed by an admin

user@macbook ~ % tctl get user/admin --with-secrets
ERROR: this request can be only executed by an admin
```

* https://github.com/gravitational/teleport/blob/v10.1.2/lib/auth/auth_with_roles.go#L1988
* https://github.com/gravitational/teleport/blob/v10.1.2/lib/auth/auth_with_roles.go#L2019

---
> 📝 **Suggested change:**
> 
> "this request can be only executed from an auth node"

---

Other similar messages:

* [this request can be only executed by a proxy](https://github.com/gravitational/teleport/blob/v10.1.2/lib/auth/auth_with_roles.go#L394)
* [this request can be only executed by a teleport built-in server](https://github.com/gravitational/teleport/blob/v10.1.2/lib/auth/auth_with_roles.go#L2903)
* [this request can be only executed by proxy, node or auth](https://github.com/gravitational/teleport/blob/v10.1.2/lib/auth/auth_with_roles.go#L2925)

## Context Deadline Exceeded

This generic go terminology plagued Docker early on when swarm mode first came
out. Most users don't know what it means, and it seems to pop up in many places
across several components. It would be best to consider this phrase appearing
in any logs as a bug, and even better if we considered it appearing in a CLI
tool as a major bug (both from a user experience point of view). We should make
efforts to catch and pass along a user-meaningful message that is actionable.

---
> 📝 **Suggestion**
> 
> Strive to eliminate "context deadline exceeded" appearing to users.
> 
---

### Context Deadline Exceeded: Credentials expired (plugins)

https://github.com/gravitational/teleport-plugins/issues/628

At least half a dozen folks have run into this and contacted support.

### Context Deadline Exceeded: Connect to 0.0.0.0

```
2022-07-28T01:46:21Z ERRO [PROC:1]    Failed to resolve tunnel address context deadline exceeded pid:2002.1 reversetunnel/transport.go:90
2022-07-28T01:46:21Z DEBU [PROC:1]    Failed to connect to Auth Server directly. auth-addrs:[0.0.0.0:3025] error:[
ERROR REPORT:
Original Error: *trace.ConnectionProblemError connection error: desc = &#34;transport: Error while dialing failed to dial: dial tcp 0.0.0.0:3025: i/o timeout&#34;
Stack Trace:
        /go/src/github.com/gravitational/teleport/api/client/client.go:2758 github.com/gravitational/teleport/api/client.(*Client).GetDomainName
        /go/src/github.com/gravitational/teleport/lib/auth/httpfallback.go:35 github.com/gravitational/teleport/lib/auth.(*Client).GetDomainName
        /go/src/github.com/gravitational/teleport/lib/service/connect.go:1160 github.com/gravitational/teleport/lib/service.(*TeleportProcess).newClientDirect
        /go/src/github.com/gravitational/teleport/lib/service/connect.go:1053 github.com/gravitational/teleport/lib/service.(*TeleportProcess).newClient
        /go/src/github.com/gravitational/teleport/lib/service/connect.go:626 github.com/gravitational/teleport/lib/service.(*TeleportProcess).firstTimeConnect
        /go/src/github.com/gravitational/teleport/lib/service/connect.go:235 github.com/gravitational/teleport/lib/service.(*TeleportProcess).connect
        /go/src/github.com/gravitational/teleport/lib/service/connect.go:198 github.com/gravitational/teleport/lib/service.(*TeleportProcess).connectToAuthService
        /go/src/github.com/gravitational/teleport/lib/service/connect.go:70 github.com/gravitational/teleport/lib/service.(*TeleportProcess).reconnectToAuthService
        /go/src/github.com/gravitational/teleport/lib/service/service.go:2412 github.com/gravitational/teleport/lib/service.(*TeleportProcess).registerWithAuthServer.func1
        /go/src/github.com/gravitational/teleport/lib/service/supervisor.go:521 github.com/gravitational/teleport/lib/service.(*LocalService).Serve
        /go/src/github.com/gravitational/teleport/lib/service/supervisor.go:269 github.com/gravitational/teleport/lib/service.(*LocalSupervisor).serve.func1
        /opt/go/src/runtime/asm_arm64.s:1263 runtime.goexit
User Message: connection error: desc = &#34;transport: Error while dialing failed to dial: dial tcp 0.0.0.0:3025: i/o timeout&#34;] pid:2002.1 service/connect.go:1076
```

The auth service had an advertise addr of 0.0.0.0.

Either the auth service itself or the proxy service could recognize that 0.0.0.0 is probably not what is correct.

---
> 📝 **Suggestion**
> 
> Craft a single line error message that indicates the source of the endpoint being used, include the network error, and include the go source file and line.
> 
> `Error while dialing to the advertise_ip for the auth_service: failed to dial: dial tcp 0.0.0.0:3025: i/o timeout pid:2002.1 service/connect.go:1076`
> 
---


### Context Deadline Exceeded: proxy server hits maximum allowed connections

to test, set a proxy's max connections pretty low and then exhaust them:

connection_limits.max_connections: 5

once exhausted, "context deadline exceeded" can be seen from tsh

```
$ tsh ssh user@host
connection error: desc = "transport: authentication handshake failed: context deadline exceeded"
```

or

```
$ tsh login --proxy=teleport.example.com:443
If browser window does not open automatically, open it by clicking on the link:
 http://127.0.0.1:49982/1446b*******0d6de1
ERROR: rpc error: code = Unavailable desc = connection error: desc = "transport: authentication handshake failed: context deadline exceeded"
```

### Context Deadline Exceeded: etcd initialization timeout

```
Jun 13 19:50:51 teleportauth0 teleport[84595]: ERROR: initialization failed
Jun 13 19:50:51 teleportauth0 teleport[84595]: context deadline exceeded
Jun 13 19:50:51 teleportauth0 systemd[1]: teleport.service: Main process exited, code=exited, status=1/FAILURE
```

This is an extremely vague message that doesn't give any clues about the actual cause. In debug mode, there's a little more. This happens with an expired etcd client certificate.

## remote TLS errors (a TLS client disconnects for some reason)

These often cause confusion and folks feel like they really have something to fix. These are very verbose full blown stacktraces for situations that just need to convey "hey some client connected but then disconnected because it didn't like the cert we are presenting to it."

---
> 📝 **Suggested changes**
> 
> * These could arguably be more of a DEBUG setting than a WARN.
> * These could be wrapped in a message that is more concise message instead of a full stacktrace.

---

```
User Message: remote error: tls: bad certificate] alpnproxy/proxy.go:322
2022-08-02T07:39:42Z WARN [ALPN:PROX] Failed to handle client connection. error:[
ERROR REPORT:
Original Error: *net.OpError read tcp 10.222.43.68:443-&gt;10.222.43.155:8250: read: connection reset by peer
Stack Trace:
	/go/src/github.com/gravitational/teleport/lib/srv/alpnproxy/proxy.go:376 github.com/gravitational/teleport/lib/srv/alpnproxy.(*Proxy).handleConn
	/go/src/github.com/gravitational/teleport/lib/srv/alpnproxy/proxy.go:314 github.com/gravitational/teleport/lib/srv/alpnproxy.(*Proxy).Serve.func1
	/opt/go/src/runtime/asm_amd64.s:1581 runtime.goexit
```

and

```
User Message: remote error: tls: bad certificate] alpnproxy/proxy.go:322
2022-08-02T07:43:44Z WARN [ALPN:PROX] Failed to handle client connection. error:[
ERROR REPORT:
Original Error: *tls.permanentError remote error: tls: bad certificate
Stack Trace:
	/go/src/github.com/gravitational/teleport/lib/srv/alpnproxy/proxy.go:376 github.com/gravitational/teleport/lib/srv/alpnproxy.(*Proxy).handleConn
	/go/src/github.com/gravitational/teleport/lib/srv/alpnproxy/proxy.go:314 github.com/gravitational/teleport/lib/srv/alpnproxy.(*Proxy).Serve.func1
	/opt/go/src/runtime/asm_amd64.s:1581 runtime.goexit
User Message: remote error: tls: bad certificate] alpnproxy/proxy.go:322
```


## Connection reset by peer

---
> 📝 **Suggested changes**
> * Network related errors do not necessarily need a full stacktrace unless DEBUG is on.
> * WARN should relay enough information to communicate which component was active when the network error occurred
> * Information about what the remote connection is should be included

---

```
2021-11-30T08:48:11Z DEBU [MX:PROXY:] "backoff on accept error: 
ERROR REPORT:
Original Error: *trace.ConnectionProblemError listener is closed
Stack Trace:
\t/go/src/github.com/gravitational/teleport/lib/srv/alpnproxy/listener.go:76 github.com/gravitational/teleport/lib/srv/alpnproxy.(*ListenerMuxWrapper).Accept
\t/go/src/github.com/gravitational/teleport/lib/multiplexer/multiplexer.go:178 github.com/gravitational/teleport/lib/multiplexer.(*Mux).Serve
\t/opt/go/src/runtime/asm_amd64.s:1581 runtime.goexit
User Message: listener is closed" multiplexer/multiplexer.go:192
```
 
## proxy held by other agent

```
Aug 02 02:55:43 ip-10-193-18-196 teleport[641153]: 2022-08-02T02:55:43Z DEBU [NODE:PROX] Proxy already held by other agent: [d51df1ed-9afc-4336-94b1-cdf9083933af.us-east-1.prod d51df1ed-9afc-4336-94b1-cdf9083933af teleport-proxy-cd7555cb8-twclw.us-east-1.prod teleport-proxy-cd7555cb8-twclw localhost 127.0.0.1 ::1 teleport.example.com 10.193.18.120 remote.kube.proxy.teleport.cluster.local], releasing. leaseID:18 target:teleport.example.com:443 reversetunnel/agent.go:453
Aug 02 02:55:43 ip-10-193-18-196 teleport[641153]: 2022-08-02T02:55:43Z DEBU [NODE:PROX] Changing state connecting -> disconnected. leaseID:18 target:teleport.example.com:443 reversetunnel/agent.go:213
Aug 02 02:55:43 ip-10-193-18-196 teleport[641153]: 2022-08-02T02:55:43Z DEBU [NODE:PROX] Pool is closing agent. leaseID:18 target:teleport.example.com:443 reversetunnel/agentpool.go:241
Aug 02 02:55:48 ip-10-193-18-196 teleport[641153]: 2022-08-02T02:55:48Z DEBU             Attempting GET teleport.example.com:443/webapi/find webclient/webclient.go:118
Aug 02 02:55:48 ip-10-193-18-196 teleport[641153]: 2022-08-02T02:55:48Z DEBU [PROXY:AGE] Adding agent(leaseID=19,state=connecting) -> us-east-1.prod:teleport.example.com:443. cluster:us-east-1.prod reversetunnel/agentpool.go:312
Aug 02 02:55:48 ip-10-193-18-196 teleport[641153]: 2022-08-02T02:55:48Z DEBU [HTTP:PROX] No proxy set in environment, returning direct dialer. proxy/proxy.go:276
Aug 02 02:55:48 ip-10-193-18-196 teleport[641153]: 2022-08-02T02:55:48Z INFO [NODE:PROX] Connected. addr:10.193.18.196:46772 remote-addr:10.193.18.88:443 leaseID:19 target:teleport.example.com:443 reversetunnel/agent.go:421 
```

Users investigating why a particular node will fail to connect don't kunderstand the debug messages. This is just an agent trying to establish a connection to every proxy, but the phrase "Proxy held by other agent" is not easy to understand. agent in this context is referring to another worker/thread running inside this teleport instance. To an end user, this node _is_ the agent, so saying that a proxy is held by another agent suggests that a completely different ssh node is interfering with this ssh node.

---
> 📝 **Suggested changes**
> * There should be some more user-friendly messages about the status of tunnels in general.
> 

---

## TSH and other client errors

stacktraces to end users nearly always add some confusion.

If tsh exits with an error message and perhaps a suggested course of action, it's _much_ more user friendly.

### psql not found in path with tsh db connect

A beginner question:

I'm trying to connect to a self-hosted postgresql, after adding the database I get the following error:

```
root@bastion ~ % tsh db connect --db-user=user --db-name=dbname dbname
ERROR: exec: "psql": executable file not found in $PATH

02:59:23 root@bastion ~ % tsh db ls
Name                                        Description Allowed Users Labels Connect                  
------------------------------------------- ----------- ------------- ------ ------------------------ 
> myexample (user: user, db: dbname)             (none)               tsh db connect dbname
```

This user didn't know that `tsh db connect` expects the psql binary to be available.

Thoughts:
* the teleport package could list postgres client as an optional dependency
* `tsh db connect` could catch this and explain that it is trying to run the `psql` command for the user and it doesn't appear to be installed.

### tsh stacktrace on duplicate port forwarding

```
$ tsh ssh -L 127.0.0.1:14432:127.0.0.1:12345 user@host
ERROR REPORT: 
Original Error: *errors.errorString Failed to bind to 127.0.0.1:14432: listen tcp 127.0.0.1:14432: bind: address already in use. 
Stack Trace: 
       /go/src/github.com/gravitational/teleport/lib/client/api.go:1920 github.com/gravitational/teleport/lib/client.(*TeleportClien
t).startPortForwarding 
       /go/src/github.com/gravitational/teleport/lib/client/api.go:1872 github.com/gravitational/teleport/lib/client.(*TeleportClien
t).SSH 
       /go/src/github.com/gravitational/teleport/tool/tsh/tsh.go:2535 main.onSSH.func1.1 
       /go/src/github.com/gravitational/teleport/lib/client/api.go:719 github.com/gravitational/teleport/lib/client.RetryWithRelogin 
       /go/src/github.com/gravitational/teleport/tool/tsh/tsh.go:2534 main.onSSH.func1 
       /go/src/github.com/gravitational/teleport/tool/tsh/tsh.go:2447 main.retryWithAccessRequest 
       /go/src/github.com/gravitational/teleport/tool/tsh/tsh.go:2533 main.onSSH 
       /go/src/github.com/gravitational/teleport/tool/tsh/tsh.go:877 main.Run 
       /go/src/github.com/gravitational/teleport/tool/tsh/tsh.go:401 main.main 
       /opt/go/src/runtime/proc.go:250 runtime.main 
       /opt/go/src/runtime/asm_amd64.s:1571 runtime.goexit 
User Message: Failed to bind to 127.0.0.1:14432: listen tcp 127.0.0.1:14432: bind: address already in use.
```

The stack should be suppressed unless debug mode is on. Simply share the message itself:

```
$ tsh ssh -L 127.0.0.1:14432:127.0.0.1:12345 user@host
Failed to bind to 127.0.0.1:14432: listen tcp 127.0.0.1:14432: bind: address already in use.
```

This is more in line with what end users expect from an ssh client.

### 

```
root@tp:/# tsh login --proxy=teleport.example.com --user=user --debug
DEBU [CLIENT]    open /root/.tsh/teleport.example.com.yaml: no such file or directory client/api.go:1056
DEBU [TSH]       Web proxy port was not set. Attempting to detect port number to use. tsh/tsh.go:3059
DEBU [TSH]       Resolving default proxy port (insecure: false) tsh/resolve_default_addr.go:108
DEBU [TSH]       Trying teleport.example.com:3080... tsh/resolve_default_addr.go:96
DEBU [TSH]       Race request failed error:[Get "https://teleport.example.com:3080/webapi/ping": dial tcp 10.222.0.8:3080: connect: connection refused] tsh/resolve_default_addr.go:60
DEBU [TSH]       Trying teleport.example.com:443... tsh/resolve_default_addr.go:96
DEBU [TSH]       Address teleport.example.com:443 succeeded. Selected as canonical proxy address tsh/resolve_default_addr.go:182
DEBU [TSH]       Waiting for all in-flight racers to finish tsh/resolve_default_addr.go:131
INFO [CLIENT]    no host login given. defaulting to root client/api.go:1418
ERRO [CLIENT]    [KEY AGENT] Unable to connect to SSH agent on socket: "". client/api.go:3951
DEBU [CLIENT]    not using loopback pool for remote proxy addr: teleport.example.com:443 client/api.go:3910
DEBU             Attempting GET teleport.example.com:443/webapi/ping webclient/webclient.go:115
Enter password for Teleport user yser:
Enter your OTP token:
DEBU [CLIENT]    not using loopback pool for remote proxy addr: teleport.example.com:443 client/api.go:3910
DEBU [CLIENT]    HTTPS client init(proxyAddr=teleport.example.com:443, insecure=false) client/weblogin.go:233
DEBU [KEYAGENT]  Adding CA key for teleport.example.com client/keyagent.go:324
DEBU [KEYSTORE]  Adding known host teleport.example.com with proxy teleport.example.com and key: SHA256:xRD83mbuMJxoldAbNg7wRpwljXF462udDZIUcbxw/qY client/keystore.go:595
DEBU [KEYSTORE]  Returning Teleport TLS certificate "/root/.tsh/keys/teleport.example.com/yser-x509.pem" valid until "2022-08-05 01:39:23 +0000 UTC". client/keystore.go:319
DEBU [KEYAGENT]  Deleting obsolete stored key with index {ProxyHost:teleport.example.com Username:yser ClusterName:teleport.example.com}. client/keyagent.go:525
INFO [KEYAGENT]  Loading SSH key for user "yser" and cluster "teleport.example.com". client/keyagent.go:202
INFO [CLIENT]    Connecting to proxy=teleport.example.com:3023 login="-teleport-nologin-b18335e2-c03a-4a40-8781-79e3981321ee" client/api.go:2989
DEBU [HTTP:PROX] No proxy set in environment, returning direct dialer. proxy/proxy.go:297

ERROR REPORT:
Original Error: *net.OpError dial tcp 10.222.0.8:3023: connect: connection refused
Stack Trace:
	/go/src/github.com/gravitational/teleport/lib/utils/proxy/proxy.go:124 github.com/gravitational/teleport/lib/utils/proxy.directDial.Dial
	/go/src/github.com/gravitational/teleport/lib/client/api.go:3000 github.com/gravitational/teleport/lib/client.makeProxySSHClientDirect
	/go/src/github.com/gravitational/teleport/lib/client/api.go:2990 github.com/gravitational/teleport/lib/client.makeProxySSHClient
	/go/src/github.com/gravitational/teleport/lib/client/api.go:2933 github.com/gravitational/teleport/lib/client.(*TeleportClient).connectToProxy
	/go/src/github.com/gravitational/teleport/lib/client/api.go:2853 github.com/gravitational/teleport/lib/client.(*TeleportClient).ConnectToProxy.func1
	/opt/go/src/runtime/asm_amd64.s:1571 runtime.goexit
User Message: failed to authenticate with proxy teleport.example.com:3023
	dial tcp 10.222.0.8:3023: connect: connection refused
root@tp:/# 
```

connection refused error wrapped in stacktrace is unexpected. also, newer user doesn't know there port 3023 comes from.

## S3 bucket specified is not globally unique and is in another region:

Another unexpected error message from an end-user:

```
ERROR REPORT:
Original Error: *trace.BadParameterError AuthorizationHeaderMalformed: The authorization header is malformed; the region &#39;us-east-1&#39; is wrong; expecting &#39;ap-southeast-1&#39;
        status code: 400, request id: PNXMPYRZ6YK0DABR, host id: gw&#43;Q/lDEGpyZmkLJYD1paVVNnO8t4sq94mgny7UjSw4jbWag0M6drcFa9yF70mR&#43;Vvf2mL3z0bw=
Stack Trace:
        /go/src/github.com/gravitational/teleport/lib/events/s3sessions/s3handler.go:432 github.com/gravitational/teleport/lib/events/s3sessions.ConvertS3Error
        /go/src/github.com/gravitational/teleport/lib/events/s3sessions/s3stream.go:193 github.com/gravitational/teleport/lib/events/s3sessions.(*Handler).ListUploads
        /go/src/github.com/gravitational/teleport/lib/events/complete.go:144 github.com/gravitational/teleport/lib/events.(*UploadCompleter).checkUploads
        /go/src/github.com/gravitational/teleport/lib/events/complete.go:131 github.com/gravitational/teleport/lib/events.(*UploadCompleter).Serve
        /opt/go/src/runtime/asm_amd64.s:1571 runtime.goexit
User Message: AuthorizationHeaderMalformed: The authorization header is malformed; the region &#39;us-east-1&#39; is wrong; expecting &#39;ap-southeast-1&#39;
        status code: 400, request id: PNXMPYRZ6YK0DABR, host id: gw&#43;Q/lDEGpyZmkLJYD1paVVNnO8t4sq94mgny7UjSw4jbWag0M6drcFa9yF70mR&#43;Vvf2mL3z0bw=] events/complete.go:132
```

## newer node connecting to an older version

```
Aug 05 14:03:40 status-service-postgresql teleport[24240]: 2022-08-05T14:03:40+02:00 INFO [PROC:1]    Joining the cluster with a secure token. pid:24240.1 service/connect.go:576
Aug 05 14:03:40 status-service-postgresql teleport[24240]: 2022-08-05T14:03:40+02:00 INFO [AUTH]      Attempting registration via proxy server. auth/register.go:185
Aug 05 14:03:40 status-service-postgresql teleport[24240]: 2022-08-05T14:03:40+02:00 INFO [AUTH]      Successfully registered via proxy server. auth/register.go:192
Aug 05 14:03:40 status-service-postgresql teleport[24240]: 2022-08-05T14:03:40+02:00 ERRO [PROC:1]    Node failed to establish connection to cluster: Cert: missing parameter. pid:24240.1 service/connect.go:113
Aug 05 14:04:02 status-service-postgresql teleport[24240]: 2022-08-05T14:04:02+02:00 INFO [PROC:1]    Joining the cluster with a secure token. pid:24240.1 service/connect.go:576
Aug 05 14:04:02 status-service-postgresql teleport[24240]: 2022-08-05T14:04:02+02:00 INFO [AUTH]      Attempting registration via proxy server. auth/register.go:185
Aug 05 14:04:02 status-service-postgresql teleport[24240]: 2022-08-05T14:04:02+02:00 INFO [AUTH]      Attempting registration with auth server. auth/register.go:185
Aug 05 14:04:02 status-service-postgresql teleport[24240]: 2022-08-05T14:04:02+02:00 ERRO [PROC:1]    Instance failed to establish connection to cluster: role Instance is not registered, connection error: desc = "transport: authenticatio>
```

Teleport node version is 10.1.2
Teleport cluster version is 9.2.3

a newer node should be able to gracefully handle connecting to an unsupported cluster version, and call that out specifically in the error message.

## multiplexer receives http traffic instead of https traffic

```
WARN [MX:PROXY:] "\nERROR REPORT:\nOriginal Error: *trace.BadParameterError multiplexer failed to detect connection protocol, first few bytes were: []byte{0x47, 0x45, 0x54, 0x20, 0x2f, 0x74, 0x65, 0x6c}\nStack Trace:\n\t/go/src/github.com/gravitational/teleport/lib/multiplexer/multiplexer.go:423 github.com/gravitational/teleport/lib/multiplexer.detectProto\n\t/go/src/github.com/gravitational/teleport/lib/multiplexer/multiplexer.go:259 github.com/gravitational/teleport/lib/multiplexer.detect\n\t/go/src/github.com/gravitational/teleport/lib/multiplexer/multiplexer.go:221 github.com/gravitational/teleport/lib/multiplexer.(*Mux).detectAndForward\n\t/opt/go/src/runtime/asm_amd64.s:1571 runtime.goexit\nUser Message: multiplexer failed to detect connection protocol, first few bytes were: []byte{0x47, 0x45, 0x54, 0x20, 0x2f, 0x74, 0x65, 0x6c}" multiplexer/multiplexer.go:224
```

These "first few bytes" decode to: `GET /tel`

The multiplexer should be able to detect and advise that it received an HTTP request on its TLS endpoint, and have a user-friendly error message pointing that out.

I could even include the same first few characters as "proof", but decoded to the appropriate text encoding for the http request in question. Additionally, this error doesn't give any information about the source ip of the request. Because of this, the teleport admin doesn't know if there's something wrong with the cluster itself, or if there's simply a misconfigured client or load balancer somewhere, or just a port scanner sending http requests to open ports.

## missing or expired token

The "invalid character '<'" comes from the html body of the 403 response from teleport. The teleport server should not return an html response if an agent expects yaml/json instead. Additionally, the agent should not include this unnecessary info. The 'can not join cluster with role "Node", token expired or not found' message is mostly sufficient. Perhaps it could advise the user to check the roles associated with the token.

```
ERRO [PROC:1]    Node failed to establish connection to cluster: "ip-10-110-10-50.ad.xxx.cloud" [809a7fd6-3a84-4f6b-83ce-22da7e52f617] can not join the cluster with role "Node", token expired or not found, invalid character '<' looking for beginning of value. pid:25770.1 service/connect.go:113
```

## Incompatible Key Usage

```
-- Logs begin at Mon 2022-08-01 07:18:15 UTC. --
Aug 10 20:48:46 ip-172-44-11-148 teleport[11843]: 2022-08-10T20:48:46Z INFO [PROC:1]    Joining the cluster with a secure token. pid:11843.1 service/connect.go:583
Aug 10 20:48:46 ip-172-44-11-148 teleport[11843]: 2022-08-10T20:48:46Z INFO [AUTH]      Attempting registration via proxy server. auth/register.go:185
Aug 10 20:48:46 ip-172-44-11-148 teleport[11843]: 2022-08-10T20:48:46Z INFO [AUTH]      Attempting registration with auth server. auth/register.go:185
Aug 10 20:48:46 ip-172-44-11-148 teleport[11843]: 2022-08-10T20:48:46Z WARN [AUTH]      Joining cluster without validating the identity of the Auth Server. This may open you up to a Man-In-The-Middle (MITM) attack if an attacker can gain privileged network access. To remedy this, use the CA pin value provided when join token was generated to validate the identity of the Auth Server. auth/register.go:341
Aug 10 20:48:46 ip-172-44-11-148 teleport[11843]: 2022-08-10T20:48:46Z ERRO [PROC:1]    Node failed to establish connection to cluster: Post "https://infra-teleport.cloudlockng.com:443/v1/webapi/host/credentials": x509: certificate specifies an incompatible key usage, invalid character '<' looking for beginning of value. pid:11843.1 service/connect.go:113
```

This is a less common TLS error that folks may not be familiar with. An advisory action (visit this specific doc page URL) could be helpful.

## Network Error when renewing LDAP cert for windows desktop

```
2022-08-11T14:02:28-04:00 ERRO [WINDOWS_D] couldn’t renew certificate for LDAP auth error:[
  ERROR REPORT:
  Original Error: *ldap.Error LDAP Result Code 200 &#34;Network Error&#34;: read tcp 10.222.13.20:53280-&gt;10.222.10.3:636: read: connection reset by peer
  Stack Trace:
          /go/src/github.com/gravitational/teleport/lib/srv/desktop/windows_server.go:518 github.com/gravitational/teleport/lib/srv/desktop.(*WindowsService).initializeLDAP
          /go/src/github.com/gravitational/teleport/lib/srv/desktop/windows_server.go:557 github.com/gravitational/teleport/lib/srv/desktop.(*WindowsService).scheduleNextLDAPCertRenewalLocked.func1
          /opt/go/src/runtime/asm_amd64.s:1581 runtime.goexit
  User Message: dial
          LDAP Result Code 200 &#34;Network Error&#34;: read tcp 10.222.13.20:53280-&gt;10.222.10.3:636: read: connection reset by peer] desktop/windows_server.go:558
```

This error could simply be:

```
2022-08-11T14:02:28-04:00 ERRO [WINDOWS_D] couldn’t renew certificate for LDAP auth: LDAP Result Code 200 "Network Error": read tcp 10.222.13.20:53280->10.222.10.3:636: read: connection reset by peer
```

DEBUG could include the full stacktrace.

HTML escaping in logs gets in the way and should not be escaped.

(this stacktrace resulting in a request for a call before they realized it was an internal network error.)

## network error when writing to tls client times out

```
2022-08-03T13:58:40-04:00 WARN [MX:PROXY:] "\nERROR REPORT:\nOriginal Error: *net.OpError read tcp 10.117.150.15:443-&gt;10.115.55.160:38712: i/o timeout\nStack Trace:\n\t/go/src/github.com/gravitational/teleport/lib/multiplexer/multiplexer.go:261 github.com/gravitational/teleport/lib/multiplexer.detect\n\t/go/src/github.com/gravitational/teleport/lib/multiplexer/multiplexer.go:221 github.com/gravitational/teleport/lib/multiplexer.(*Mux).detectAndForward\n\t/opt/go/src/runtime/asm_amd64.s:1581 runtime.goexit\nUser Message: failed to peek connection\n\tread tcp 10.117.150.15:443-&gt;10.115.55.160:38712: i/o timeout" multiplexer/multiplexer.go:224
```

WARN should be more concise:


```
2022-08-03T13:58:40-04:00 WARN [MX:PROXY:] *net.OpError read tcp 10.117.150.15:443->10.115.55.160:38712: i/o timeout multiplexer/multiplexer.go:224
```

Stacktrace can still appear in debug level, unescaped

```
2022-08-03T13:58:40-04:00 WARN [MX:PROXY:] *net.OpError read tcp 10.117.150.15:443->10.115.55.160:38712: i/o timeout multiplexer/multiplexer.go:224
2022-08-03T13:58:40-04:00 DEBU [MX:PROXY:] Stack Trace:
	/go/src/github.com/gravitational/teleport/lib/multiplexer/multiplexer.go:261 github.com/gravitational/teleport/lib/multiplexer.detect
	/go/src/github.com/gravitational/teleport/lib/multiplexer/multiplexer.go:221 github.com/gravitational/teleport/lib/multiplexer.(*Mux).detectAndForward
	/opt/go/src/runtime/asm_amd64.s:1581 runtime.goexit
User Message: failed to peek connection
	read tcp 10.117.150.15:443->10.115.55.160:38712: i/o timeout
```


## endpoint required

I believe the user was setting the S3 url as something like `https://your-tenancy.compat.objectstorage.your-region.oraclecloud.com/mybucket` or `s3://your-tenancy.compat.objectstorage.your-region.oraclecloud.com/mybucket` instead of `s3://mybucket?endpoint=your-tenancy.compat.objectstorage.your-region.oraclecloud.com`.

The stacktrace could have given more specific advice about what "this service" is, and what an Endpoint configuration is. It could have linked to the docs about configuring S3.

```
2022-08-15T14:25:28Z [AUTH:COMP] WARN Failed to check uploads. error:[
ERROR REPORT:
Original Error: *trace.BadParameterError MissingEndpoint: &#39;Endpoint&#39; configuration is required for this service
Stack Trace:
        /go/src/github.com/gravitational/teleport/lib/events/s3sessions/s3handler.go:450 github.com/gravitational/teleport/lib/events/s3sessions.ConvertS3Error
        /go/src/github.com/gravitational/teleport/lib/events/s3sessions/s3stream.go:192 github.com/gravitational/teleport/lib/events/s3sessions.(*Handler).ListUploads
        /go/src/github.com/gravitational/teleport/lib/events/complete.go:144 github.com/gravitational/teleport/lib/events.(*UploadCompleter).checkUploads
        /go/src/github.com/gravitational/teleport/lib/events/complete.go:131 github.com/gravitational/teleport/lib/events.(*UploadCompleter).Serve
        /opt/go/src/runtime/asm_amd64.s:1571 runtime.goexit
User Message: MissingEndpoint: &#39;Endpoint&#39; configuration is required for this service] events/complete.go:132
```

## Failed to handle connection.

<https://github.com/gravitational/teleport/blob/8394f4fb487b095dca5ea8a584c547322a909f77/lib/srv/alpnproxy/local_proxy.go#L137>

When running a mongo query, `database_service` logs "Failed to handle connection." and the mongosh reports an unexpected connection close.

See <https://github.com/gravitational/teleport/issues/16348> for details.

## Client Connection log spam

Adjust log levels to make server WARN level less noisy. clients doing the wrong thing isn't terribly actionable for a cluster admin, so consider bumping these to INFO rather than WARN.

```
2022-09-20T15:08:54Z WARN [SSH:PROXY] Error occurred in handshake for new SSH conn error:[ssh: overflow reading version string] pid:7.1 remote_addr:10.252.28.35:24724 sshutils/server.go:446
```

This can happen due to a TLS client trying to connect to the teleport ssh proxy port. This can be INFO. Perhaps some introspection can happen to better advise if it's a recognize non-ssh connection attempt rather than simply passing the ssh protocol error through?

```
2022-09-20T15:09:37Z WARN [PROXY:1]   Failed to authenticate client, err: ssh: cert has expired. pid:7.1 remote:10.252.28.35:28592 user:-teleport-nologin-9753ed77-7916-4226-9c38-7db9544ab900 reversetunnel/srv.go:768
```
