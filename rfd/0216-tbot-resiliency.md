---
authors: Dan Upton (daniel.upton@goteleport.com)
state: draft
---

# RFD 216 - Improving `tbot`'s Resiliency for Production Workloads

## What

Making `tbot` more resilient to auth server availability issues, with graceful
degradation.

## Why

As customers move from using Machine and Workload Identity in CI/CD pipelines to
putting us in the critical path of their "online" systems (e.g. by using SPIFFE to
secure service-to-service communication), the risk and blast radius of a Teleport
outage increases dramatically.

Wherever it is practical and safe to do so, `tbot` should gracefully handle
transient issues, and fall back to using stale or cached data rather than
failing completely - which could impact the availability of customer applications.

This RFD is the product of several conversations around a
[customer request](https://github.com/gravitational/teleport/issues/53711)
to cache the X509-SVID that `tbot` exchanges for AWS Credentials in our Roles
Anywhere integration.

From a purely technical perspective, the requested change is straightforward,
and there are a number of possible approaches detailed below. However, there are
also broader UX implications and trade-offs that we want to consider more
holistically in this document.

## How

At this stage, we don't have a clear decision on the right path forward, so this
section enumerates our possible options (both from a UX and implementation
perspective).

Let's begin with a brief overview of how `tbot` operates today.

### Background: Error handling

When `tbot` starts up, it dials a connection to an auth server and renews its
identity. If this fails, the whole process will exit, and if it's running as a
daemon (rather than in one-shot mode) it will be restarted by SystemD or the
user's supervisor of choice.

If initialization succeeds, `tbot` will start running the user's configured
services/outputs, and spawn a goroutine to periodically renew its identity in
the background. If identity renewal fails 7 times in a row, `tbot` will consider
this to be an irrecoverable error and the entire process will exit.

As well as being relatively easy to understand, the current behavior allows us
to surface misconfigurations to the user quickly. If they've accidentally given
the wrong port or they do not have network connectivity to a proxy or auth
server, the process will exit with an error - although it's worth noting that
our current error messages can be a little hard to interpret.

The downside is that a healthy connection to the auth server is a pre-requisite
for **all** of `tbot`'s functionality, there is simply no possibility of
graceful degradation.

### Background: Pathway selection

`tbot` supports connecting to an auth server either directly or via an SSH
tunnel through a proxy. These are the `auth_server` and `proxy_server`
configuration options, respectively.

Confusingly, for backward-compatibility reasons, we allow you to provide the
address of a proxy as the `auth_server`. The inverse may also work but we don't
make any promises about it.

To support this, `tbot` first tries to connect to the configured address as if
it were an auth server and makes a "ping" RPC to test the connection. If this
fails, it treats the address as a proxy instead.

This behavior is implemented by the `lib/auth/authclient` package's `Connect`
method. In effect, it makes creating an API client a *blocking* operation, and
makes it impossible to create a client without having a working connection to
the auth server; which differs from gRPC's standard behavior of establishing
connections on-demand - this difference will become relevant when evaluating our
implementation options below.

### UX Question: How should we handle connectivity issues on-startup?

`tbot`'s current behavior of refusing to start if the auth server is unavailable
makes it impossible to implement any kind of graceful degradation.

However, if we proceed without a working connection, we cannot as effectively
surface configuration issues to users.

So the question becomes: what is the right balance? If a proxy address
is syntactically valid but plainly wrong, is it best to exit loudly or retry
indefinitely? Is the answer different for one-shot and daemon mode?

### UX / Implementation Question: Can we remove the path selection logic?

It would also greatly simplify things if we could take the `proxy_server` and
`auth_server` configuration options at face value, rather than trying both
connection methods regardless.

If we knew up-front whether the given address is an auth server or a proxy, we
wouldn't need to test the connection to find out. Instead, we could just create
the client and rely on gRPC to manage retrying failed connections asynchronously.

Theoretically, we might be able to achieve this and keep the path selection
logic by implementing a custom gRPC dialer, resolver, etc. (see below) but this
is a lot of complexity with dubious benefit.

### UX Option 1: Just keep running

Perhaps the most obvious option if connection fails on-startup is to just keep
running, logging errors, and retrying the connection in the background.

The `tbot` process itself would appear healthy, so as an operator you'd need to
look at the logs to find out something is wrong. Or ideally, we'd have a useful
health-check endpoint scraped regularly by your monitoring solution of choice.

Another benefit of having the process actually exit is that it might reset any
state that is causing the connection to fail, which we'd lose with this option.

### UX Option 1a: Keep running... for a while

An extension of Option 1 would be to keep running but "time out" and terminate
the process if a connection to the auth server could not be established within a
chosen timeframe.

This feels like it would be a bit of a ticking timebomb, though.

### UX Option 1b: Expose a way to explicitly test the connection

Many tools provide a way to "dry run" or test your configuration ahead of time,
for example: `nginx -t`. Perhaps we could add a similar option to the `tbot` CLI?
Or perhaps one-shot mode already achieves this?

### UX Option 1c: Make "keep running" opt-in

To avoid breaking things for existing users, we could add a CLI flag which
opts-in to the new behavior. This has the downside of adding yet another
configuration option and offloading even more complexity onto our users.

### UX Option 2: Test the connection on first run only

Given `tbot` already stores state (e.g. identity certificates) for use between
runs, we could test the connection on the first run only. If the connection
fails, `tbot` would exit as it does today, otherwise it would store a hash of
the configuration file to record its validity.

On subsequent runs, if the configuration file has not changed, we know that any
connection issues must be transient rather than a misconfiguration so we would
keep running and retry the connection.

This could also be used as a way to maintain the path selection logic. We could
probe the address as an auth server and proxy on the first run and "cache" which
it was on-disk so that on subsequent runs we just connect using the known method.

Alternatively, we could treat the presence of an existing identity as proof that
the configuration was valid at one point (although this doesn't handle the case
of changing configuration).

### Implementation Option 1: Make the client optional

In terms of implementation, the simplest option conceptually might be to make
the API client "optional" (i.e. check for `nil` or similar before using it).

In practice, this isn't a good option because the client is used in all of
`tbot`'s services, so not only would the diff be large, but depending on how it's
implemented it could create quite a serious foot-gun.

### Implementation Option 2: Wrap the client in a proxy type

A better approach would be to make the optionality of the client invisible to
its callers by wrapping it in a struct which delegates to the real client, or
returns a canned error if the client has not been initialized.

This approach was explored in [#55070](https://github.com/gravitational/teleport/pull/55070).

### Implementation Option 3: Wrap the gRPC connection

The major downside of Option 2 is that you need to maintain wrapper methods for
every RPC by hand. A more scalable approach is to implement `grpc.ClientConnInterface`
and inject the real gRPC connection once it is initialized.

This approach was explored in [#55170](https://github.com/gravitational/teleport/pull/55170)
and [#55202](https://github.com/gravitational/teleport/pull/55202).

### Implementation Option 3a: Make it possible to create an `api/client.Client` from an existing connection

The downside of Option 3 is that our `api/client` package does not provide a way
to wrap the `grpc.ClientConn`, so we must use the gRPC client stubs directly
rather than the client package's higher-level wrapper methods.

In [#55293](https://github.com/gravitational/teleport/pull/55293), we add
separate methods to dial a gRPC connection and to create a client, which makes
it possible to wrap client's connection.

### Implementation Option 4: Drop support for path selection

As mentioned before, if we know up-front which is the correct connection method,
we could rely on gRPC's "normal" asynchronous connection behavior. Rather than
needing to do any injection or wrapping, we could simply call `grpc.NewClient`
which would return a virtual connection that could be used by the API client
even if the real underlying TCP connection isn't ready yet.

### Implementation Option 5: Custom gRPC resolver, dialer, etc.

If we truly did want to keep the path selection behavior without wrapping the
connection, one relatively complex option would be to implement our own gRPC
resolver and dialer.

Here's roughly how it would work:

1. Our resolver would try the different connection pathways and return a set of
   "addresses" - string identifiers, rather than actual IP addresses - for the
   ones that work (e.g. `["strategy://direct", "strategy://proxy"]`).

2. gRPC would call our dialer with the resolved "address" and we'd re-dial using
   the chosen method. We'd need to re-dial because gRPC offers no way to re-use
   an existing TCP connection between clients (e.g. it starts by sending the
   HTTP/2 preface).

Complexity aside, these APIs are also marked as "experimental" which might make
upgrading gRPC difficult in the future.
