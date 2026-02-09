---
authors: Tim Buckley (tim@goteleport.com)
state: draft
---

# RFD 0240 - Dynamic Services in `tbot`

## Required Approvers

- Engineering: @strideynet && @boxofrad
- Product: @klizhentas

## What

This RFD discusses new features in the `tbot` client that would allow users to
ask an already-running bot to start additional services on demand. This involves
a new bot API service to accept API requests, plus various changes in the `tbot`
client to facilitate spawning new services at runtime.

## Why

At a high level, this aims to provide a UX and performance improvement, and
could enable new dynamic and interactive use cases in the future.

In short, dynamic services would enable:
1. Improved performance: A single bot process can be shared between `tbot`
   invocations or GitHub Actions steps. The bot only joins with a Teleport
   cluster once, and only creates one bot instance.
2. Improved CLI and GitHub Actions UX: Users can easily start more than one
   service using only the CLI or pre-made GitHub Actions.
3. Improved flexibility: Users can start additional services on-the-fly at any
   point, including as a result of API calls or other user input.

### What's wrong with tbot's CLI today?

`tbot`'s CLI today has a UX cliff. Simple use cases, like running one service,
are relatively simple. These can be done entirely through the CLI, like
`tbot start identity ...`, or config files can be automatically generated, like
`tbot configure app-tunnel ...`.

However, when multiple services are required, this breaks. The CLI cannot
feasibly support running multiple services in a single invocation, so users will
need to use a YAML config file instead.

In CI/CD environments, things are further complicated: the UX for spawning
services in the background can be arcane, so users need to solve proper
background execution and coordination for each Teleport resource they want to
access.

### Why improve the CLI?

`tbot`'s CLI has a number of UX benefits over YAML configuration files:
- It is self documenting. Users can use `--help` to see all supported service
  types and flags, and all flags are validated. Contrast this with YAML which is
  schemaless, requires browsing to our docs page, and unexpected fields are
  ignored, which leads to frequent support issues.

- It encourages tighter iterations. Users can very easily Ctrl+C, adjust a flag,
  and run `tbot` again to try out a change with minimal effort.

- The full bot configuration is visible without a context switch, and no
  separate config file needs to be managed.

- As always, users can easily convert their CLI to a YAML config once they are
  happy with it. (It is not easy to do the reverse.)

Dynamic services would provide the flexibility of running arbitrary bot services
with all these CLI-centric benefits.

### Improving the CLI UX improves CI/CD

GitHub Actions faces many of the same UX constraints as the CLI: it's hard to
encode a complex configuration with many services into one command, which is why
our pre-made GitHub Actions are single purpose and can only run a single bot
service. This allows users to get started quickly, but does not adequately allow
for more complex use cases.

GitHub Actions has an additional constraint: background services are hard. We
are also working to [improve this UX directly](#already-pending-improvements),
but adapting a one-shot bot to provide a persistent tunnel is currently a fairly
large change, especially in CI where background services are hard.

Dynamic services solve both issues here: users can configure services across
several GitHub Actions steps, which makes it easy to add any number of services
with a single, repeatable verb. It also improves performance as the bot will
only need to authenticate with Teleport once. Finally, it nullifies the
complexity cost of adding a long-running service; a `tbot` is already running in
the background.

Beyond GitHub Actions, other providers mostly do not have an equivalent feature
that would let vendors provide an interface beyond app CLIs. As such, improving
the CLI does directly improve the UX for most CI/CD providers.

## Details

### Background

#### What works well today?

A minimal GHA example today might look like this:
```yaml
jobs:
  example:
    # ... snip ...
    steps:
    - name: Fetch application credentials
      id: auth
      uses: teleport-actions/auth-application@v2
      with:
        proxy: example.teleport.sh:443
        token: example-bot
        app: grafana
```

This authenticates to Teleport and fetches credentials for a single application.
Behind the scenes, it effectively runs `tbot` like so:

```code
$ tbot start application \
  --oneshot \
  --proxy-server example.teleport.sh:443 \
  --token example-bot \
  --app grafana \
  --destination /path/to/dest
```

This works fairly well:
- The example is fully self-contained and does not depend on additional config
  files
- `tbot`'s oneshot mode means we return synchronously only when certificates
  have been written to disk. The next step can use the credentials immediately.
- If there is an error, we return a sane exit code allowing the workflow to
  handle the error appropriately
- Errors are reported cleanly within GHA's UI

The UX is similarly good for simple access to single Kubernetes clusters, or
for non-multiplexed SSH access.

#### Where do things go wrong?

We'll demonstrate two problematic scenarios that real users have tried to
accomplish in GitHub Actions:
1. Accessing more than one resource
2. Access one or more resources whose names are not known at the time of writing
   the workflow

(Resource types that require a background service are currently also
troublesome, but are being improved separately; [see below](#already-pending-improvements).)

Accessing multiple resources is generally problematic today. Our GitHub Actions
only support running a single service, for largely the same reason that our CLI
can only run service at a time: it's difficult to encode multiple services worth
of configuration into a single flat namespace. To work around this limitation,
users need to to call `teleport-actions/auth-application` repeatedly:

```yaml
# ... snip ...
steps:
- name: Fetch credentials for app 'foo'
  id: auth
  uses: teleport-actions/auth-application@v2
  with:
    proxy: example.teleport.sh:443
    token: example-bot
    app: foo
- name: Fetch credentials for app 'bar'
  id: auth
  uses: teleport-actions/auth-application@v2
  with:
    proxy: example.teleport.sh:443
    token: example-bot
    app: bar
```

This spawns a new `tbot` client for each action, which authenticates to Teleport
from scratch repeatedly. It counts as multiple bot instances, creates
unnecessary audit log entries, and will perform poorly. It is not currently
possible to start any kind of tunnel or long-running service (SPIFFE socket,
etc) as our current GitHub Actions cannot continue to run in the background.

A more complicated use case is accessing resources on-the-fly: some users have
expressed a desire to access a set of resources that will be determined at
runtime. The `tbot` client has no equivalent to `tsh <kind> login`, so they'll
need to use the `tsh` client:

```yaml
# ... snip ...
steps:
- name: Authenticate using tbot
  id: auth
  uses: teleport-actions/auth@v2
  with:
    proxy: example.teleport.sh:443
    token: example-bot
    allow-reissue: true
- name: Login to a database
  run: |
    tsh database login foo # 'foo' can be determined dynamically if desired
    tsh database login bar
```

This can work, and users are generally already familiar with `tsh`. However, its
toolset is not purpose built for machine use and users will need to manually
solve a number of tasks `tbot` would normally do for them:
- Fetch certificates for each app
- Start the service or tunnel in the background appropriately
- Report and/or ensure service readiness
- Renew all certificates before expiry
- Restart tunnels when necessary (new certs or if otherwise interrupted)

Manual `tsh` use also impacts bot status reporting: users won't be able to see
realtime service health in Teleport's UI as `tbot` won't be running to report
it.

Ideally, we would like to give users the machine-friendly UX of
`tbot start database-tunnel ...` with the flexibility of `tsh`'s login and proxy
abilities.

[manual]: https://goteleport.com/docs/machine-workload-identity/deployment/github-actions/#example-manual-configuration

#### Already pending improvements

We are already planning to renovate our first-party GitHub Actions to improve
the UX wherever `tbot` needs to run in the background. These new GitHub Actions
will change the following:

- They will support running a detached `tbot` client in the background in the
  job
- They will take advantage of [an upcoming `/wait` HTTP API][wait] to ensure
  service readiness before allowing the next step to start running
- They will create post-job logging hooks to ensure logs are visible to end
  users

[wait]: https://github.com/gravitational/teleport/issues/62117

This will allow users to run individual tunnels for apps and databases in the
background reliably, with a similar UX to our current oneshot actions.

This change will not particularly simplify running multiple `tbot` services
(tunnels, etc), which will still require starting and authenticating new bots
for each service, unless users switch to manually running bots with a config
file:

```yaml
steps:
- name: Start tbot and wait for it to become ready
  run: |
    # Start a bot in the background
    nohup tbot start -c tbot.yaml --diag-addr 127.0.0.1:4444 > tbot.log 2>&1 &

    # Wait for it to become ready
    tbot wait --diag-addr 127.0.0.1:4444
```

### Dynamic services

Dynamic services complement and build on these background improvements, adding
the ability to ask an existing bot to start one or more new services on demand.

This feature involves running an API server on a local Unix socket. Clients can
request new services via a "spawn" endpoint and provide service configuration in
our existing YAML format.

This API server will be local to a particular `tbot` process, and will be
disabled unless explicitly requested. It will be designed with easy integration
in mind for third party apps and scripts, and will have full CLI coverage, plus
explicit support in our GitHub Actions.

Importantly, this endpoint takes advantage of an enhanced version of the same
"wait" ability discussed above, and waits for the service to become ready.
However, it additionally responds with a full error trace if the service fails
outright before reporting its status.

#### Usage example: GitHub Actions

```yaml
steps:
- name: Start a bot API in the background
  uses: teleport-actions/bot-api@v1
  with:
    proxy: example.teleport.sh:443
    token: example-bot

- name: Open a database and an app tunnel
  uses: teleport-actions/start-service@v1
  with:
    services: |
      - type: database-tunnel
        listen: tcp://127.0.0.1:25432
        service: postgres
        database: postgres
      - type: application-tunnel
        listen: tcp://127.0.0.1:1234
        app: dumper

- name: Start an app tunnel dynamically
  run: |
    tbot api spawn app-tunnel --app foo --listen tcp://127.0.0.1:2345
```

There are a few key improvements here:
- Only one `tbot` instance is started
- There is less config repetition: auth details (proxy, token, etc) are only
  needed once
- Users never need to provide a config file
- Each step waits for the requested service to become healthy, and immediately
  reports errors and fails (per GHA semantics) if the services fails to start.

#### Usage example: CLI

The GitHub Actions example maps well onto other CI providers, or any CLI:

```code
$ export TBOT_API_SOCKET=unix:///path/to/api.sock
$ tbot start bot-api ... &
$ tbot api spawn db-tunnel --service postgres --database postgres
$ tbot api spawn app-tunnel --app example
```

A minimal [proof of concept implementation][poc] was made to validate this
concept. If desired, you can build the `tbot` client from the linked PR to try
out the CLI flow described here. A [recorded demo][demo] is also available
internally.

This new CLI functionality can help users explore `tbot` interactively. Consider
this session:
```code
# Start the bot with a snippet explicitly provided by the Teleport UI
$ export TBOT_API_SOCKET=unix:///tmp/bot-api.sock
$ tbot start bot-api --join-uri=tbot+bound-keypair://...

# Full functionality can be seen with `--help`
$ tbot api --help
$ tbot api start --help

# With the bot running, the user can now try out some functionality
$ tbot api start identity --destination ./ssh ...
Started service: identity-1
$ ssh -F ./ssh/ssh_config user@host hostname

# Fallible services will report errors immediately; users can try again and fix
# the issue.
$ tbot api start app-tunnel --app invalid --listen tcp://127.0.0.1:1234
Error: [...snip...]
  app "invalid" not found

$ tbot api start app-tunnel --app dumper --listen tcp://127.0.0.1:1234
Started service: app-tunnel-1
$ curl http://localhost:1234

# Once happy with the bot's configuration, it can be saved to a config file
$ tbot api config > /etc/tbot.yaml
```

Note that in the MVP implementation we aren't guaranteeing the ability to stop
individual services (feasibility not yet determined; may require significant
refactoring beyond what spawning requires). This does mean that if a service is
started such that it starts successfully but with undesirable configuration
(e.g. wrong but writable output path, wrong but valid app name, etc), it can't
be stopped without stopping the outright. We will want to support killing
services eventually.

#### The bot API service

The primary new feature is a new bot API service that exposes a machine-friendly
API for interacting with a running `tbot` process. Users will start this new
bot API service using `tbot start bot-api ...` or by running `tbot` with a YAML
config file that starts it similar to any of our existing service types. This
service is entirely optional and available only when explicitly enabled by
users, either via `tbot.yaml` or `tbot start bot-api`.

The API itself will expose just two RPCs or endpoints for this MVP: `spawn` and
`config`. It will be exposed over a Unix socket and will be otherwise
unauthenticated. It is up to the user to enforce the security of the socket,
but as with `tbot`'s existing credential outputs today we will configure sane
default permissions and our existing tooling can be used to restrict access to
the socket via filesystem ACLs.

These V1 bot API endpoints will accept and return configuration in YAML format.
This allows use of our existing config parsing library without additional
refactoring, and makes it feasible for end users to use the API without
problematic amounts of boilerplate.

If we decide to further develop the local bot API, we should consider revisiting
this and switching to a protobuf source of truth for bot configuration. Future
API iterations could additionally accept fully structured input when we are
prepared to accept it.

##### Security considerations

The bot API service introduces new ways to interact with a running `tbot`
process, and by its nature can be used to gain access to Teleport resources that
were not explicitly configured by the user. It will be disabled by default and
must be explicitly enabled by users, after which the user is expected to ensure
the socket is adequately protected.

The bot API service should not be enabled in situations where access to the
socket cannot be adequately restricted, or where there is no tolerance for
changes in bot behavior at runtime.

As a quick reference, consider these examples:
| Use case | Good fit? |
|-|-|
| Traditional systemd deployment with read-only `/etc/tbot.yaml` | ❌ |
| New user onboarding and experimentation | ✅ |
| Ephemeral use cases including CI/CD workflows | ✅ |

The bot API should not meaningfully represent a change in effective permissions
given common ways in which bots are deployed today, particularly in ephemeral
environments. As a rough rule, any deployment where the bot configuration can be
modified or otherwise overridden by the bot's own Unix user will need to deal
with the same security pitfalls exposed by the bot API - at least assuming the
socket itself is secured properly.

By default, the bot API's Unix socket will be configured with restrictive (0600)
permissions to ensure only the bot's own Unix user can interact with it. We will
provide documentation stating that this socket should be adequately secured and
should not be e.g. shared via Docker mount.

If adequately protected, all an attacker with access to the API socket can
feasibly do is have the bot write credentials to the filesystem at arbitrary
paths to which its Unix user has access. If they have RCE as the bot's Unix
user, they could do that anyway.

And as always, Teleport RBAC permissions should also be limited such that bots
are only able to access the minimal set of resources needed to accomplish their
intended function.

##### The `SpawnService` RPC

The MVP makes relatively few guarantees about the state of a running bot after
spawning services except when it returns without error. Consumers, especially in
CI/CD environments, should consider the system to be in a failed state if the
API returns any error. In CI/CD environments, this should fail the run with an
error.

The endpoint only returns without error when all requested services have spawned
and reported that they are healthy, or returned outright without error. It
returns an error immediately without spawning a service if and only if the YAML
fails to parse and deserialize. Otherwise, it will attempt to spawn all
requested services.

Multiple services may be spawned at once. This is partially in service of our
existing config unmarshaling, but is useful to allow spawning multiple services
without unnecessary sequential waits. When waiting is enabled (the default), it
only returns when all services have either reported a readiness status (either
healthy or unhealthy; not initializing), or failed outright. If at least one
service has failed or reported an error, or if the optional timeout has elapsed,
the endpoint returns all aggregated errors.

Services should be expected to continue running (or trying to run) once started,
even if the spawn RPC returns an error. The `/readyz` endpoint can be used to
verify the state of services (and which were spawned, if any) after spawning.
This does mean the state of a bot will be uncertain if a failure occurs, but in
a CI/CD job, a failure is expected to end the job outright: the CLI will return
nonzero, and the job will terminate.

##### gRPC API

Minimal protobuf specification:

```protobuf
syntax = "proto3";

import "google/protobuf/duration.proto";

service BotAPIService {
  // Spawns a new service and optionally waits for it to become ready with a
  // configurable timeout.
  rpc SpawnService(SpawnServiceRequest) returns (SpawnServiceResponse);
  // Returns the current effective bot configuration.
  rpc GetConfig(GetConfigRequest) returns (GetConfigResponse);
  // Stretch goal: An additional endpoint to stop a service
  // rpc StopService(StopServiceRequest) returns (StopServiceResponse);
  // Stretch goal: Expose readyz and waiting functionality over this interface
  // alongside existing diag HTTP.
  // rpc GetServiceStatus(GetServiceStatusRequest) returns (GetServiceStatusResponse);
}

// Request for a new service to be started.
message SpawnServiceRequest {
  // A YAML document containing a list of services to spawn.
  string configuration = 1;
  // If set, don't wait for the service to become healthy.
  bool skip_wait = 2;
  // If set, return with an error if the service has not become ready before the
  // specified duration has elapsed. The service will continue trying to run.
  google.protobuf.Duration timeout = 3;
}

// Response when a service has successfully spawned
message SpawnServiceResponse {}

// Requests the current bot configuration
message GetConfigRequest {}

// Response containing current bot configuration
message GetConfigResponse {
  // The effective bot configuration in YAML format.
  string configuration = 1;
}
```

gRPC does meaningfully increase the cost of integrating with the bot API, and is
less beneficial in this context (local, explicitly intended for end user
integration, etc) than in other Teleport components. However, it should ease the
maintenance burden over time and will simplify the process of adding new
functionality should we develop the bot API further.

The integration burden will be mitigated by providing a CLI entrypoint with
effectively 100% API coverage for the two RPCs.

Additionally, we can consider adopting a library like
[`connectrpc`](https://github.com/connectrpc/connect-go) to make the API
available to HTTP clients, which is used in Teleport already to allow API access
from the web UI.

The [proof of concept implementation][poc] used a plain HTTP API, however a
gRPC API with `connectrpc` should provide a good balance of maintainability,
extensibility, and ease of integration requiring a native client.

#### The CLI

We will introduce a new family of CLI commands: `tbot api`

* `tbot api spawn`: Spawns a service. Fully reuses our existing CLI and inherits
  all flags and service subcommands, exactly like `tbot start` and
  `tbot configure`.
* `tbot api config`: Fetches configuration from a currently running bot to
  provide `tbot configure`-like functionality when building services
  sequentially.

  For example:
  ```
  $ tbot start bot-api ... &
  $ tbot api spawn identity ...
  $ tbot api spawn app-tunnel ...
  $ tbot api spawn ssh-multiplexer ...
  $ tbot api config > tbot.yaml

  # ...later, after stopping the first tbot process...
  $ tbot start -c tbot.yaml # resumes with the previous config
  ```

The `tbot api spawn ...` command will expose flags to disable waiting, and
optionally enforce a timeout.

An exit code of zero means all spawned services have been started and have
reported healthy. Failure to meet this condition within the specified timeout
period (if any) returns an unspecified nonzero exit code, and no guarantees are
made about which services (if any) may still be running or trying to run.

We could consider exiting with different status codes depending on failure
type (failed outright and no service was started, service started and is
failing, timeout reached), but for most use cases simply returning zero or
nonzero is sufficient. It should not be a breaking change if we later decide to
increase the specificity of the error code to indicate different failure types.

If waiting is disabled, an exit code of zero just means the service was accepted
and the configuration was parsed. Clients will need to use the existing
`/readyz` or `/wait` endpoints to properly coordinate.

##### Edge case: path locality

One issue worth considering in the CLI is that paths are not necessarily local
to the calling shell. If the user runs the bot API on a TCP socket, bind-mounts
it with Docker, or otherwise accesses the API from another environment, any
paths provided will be relative to the original `tbot` process's view of its
filesystem rather than the client's. Relative paths will also be relative to
the original `tbot`'s CWD rather than the client.

A robust solution to this problem is probably not in scope. We can expect most
users to use the bot API as intended - locally, via a Unix socket - which will
not have path locality problems. If they do encounter this issue, the API and
CLI should report errors as expected if the path is not writable. We may at
least normalize paths at the CLI to mitigate relative path issues, and only
communicate absolute paths to the API via the client CLI.

#### Additional required changes in `tbot`

The core changes are straightforward:
- Adding a new `bot-api` service
- Adding a new `tbot api` subcommand

However, actually spawning services dynamically is less straightforward than
simply spawning a goroutine. The following sections discuss a few dependencies
discovered in [the PoC][poc] that should be resolved.

##### Improve service management

`tbot` will need to be capable of spawning services dynamically. Today they're
all started immediately in an `errgroup`, which while simple, does not allow
additional tasks to be spawned at runtime.

We should refactor `tbot` to move away from a simple `errgroup` to a custom
mechanism that can spawn additional tasks over time. Ideally, this should
integrate with our `readyz` service to allow querying for service health.
We should evaluate cleanly stopping specific services and exposing an additional
RPC for this if possible.

##### Simplify service dependencies

Services in `tbot` require quite a few dependencies, both at the `lib/tbot`
level (when converting from the YAML into the runtime representation) and at the
`lib/tbot/bot` level just before the service actually begins execution.

It _might_ be good to flatten this hierarchy and keep all dependencies in one
place, but that might be too large of a refactor. Some sort of dependency
framework might be helpful but a wholesale refactor is out of scope for this
MVP.

##### Maintain configuration at runtime

Bots currently discard the YAML-compatible config representation after parsing
and converting to a runtime service struct. This means it isn't possible to
reverse the process, which the `/config` / `GetConfig` RPCs require to be able
to output the current bot config.

We should either preserve the YAML config representation alongside the service
handle, or build a facility to recreate it from a runtime service struct.

##### Enhance waiting on specific services

It needs to be possible to wait for specific service readiness, to allow the
spawn RPC to return only once the spawned service is ready for use.

This is already being built in [#62191], but may need minor enhancement to
cover early-exiting services as discovered in [the PoC][poc].

[#62191]: https://github.com/gravitational/teleport/pull/62191

##### Config parsing utility

Parsing service config was a bit problematic in [the PoC][poc] as it was very
easy to cause an import cycle given the current layout. A relatively simple
workaround to break the cycle was found (keeping config for the `bot-api`
service in a separate package) but we should evaluate cleaner alternatives.

#### GitHub Actions changes

We'll introduce 2 new GitHub Actions in the [`teleport-actions`] organization:

- `teleport-actions/bot-api`: starts a bot that initially runs only the
  `bot-api` service, joining Teleport through the usual process. It will
  redirect logs to a file, detach, and wait for general bot readiness before
  continuing. It will additionally configure an exit hook to dump logs to the
  GHA run at the end of the job.

  Example:
  ```yaml
  - name: Start Machine & Workload ID
    uses: teleport-actions/bot-api@v1
    with:
      proxy: example.teleport.sh:443
      token: some-token
  ```
- `teleport-actions/start-service`: starts a service defined in lightly nested
  YAML (unfortunately GHA Actions cannot accept arbitrary nested YAML).

  Example:
  ```yaml
  - name: Start an app tunnel
    uses: teleport-actions/start-service@v1
    with:
      # A list of services to start, in tbot's standard config format
      services: |
        - type: application-tunnel
          app_name: dumper
          listen: tcp://0.0.0.0:1234
      # The readiness wait can optionally be skipped
      skip_wait: false
  ```

The `bot-api` action will start a bot running the `bot-api` service at a unique
socket path. It will publish the socket path as a GHA "output" to be referenced
from future steps, but will also configure an environment variable containing
the socket path that future steps will automatically inherit. This means the
`start-service` action won't need to refer to a socket path at all unless there
are multiple bots running (e.g. the user has multiple Teleport clusters).

As a bonus, this will immediately support all service types in GitHub Actions
without us needing to implement individual Actions, as is our currently
established pattern.

[`teleport-actions`]: https://github.com/teleport-actions

### Future work

These are out of scope at this time, but may be worth exploring in the future.

#### Additional API functionality

The bot API service would allow for a lot of future expansion through new
endpoints or RPCs. For example:

- `tbot api query database --label foo=bar`: query for all databases that match
  a selector. The result can be passed back to `tbot` to spawn tunnels for
  everything matching a selector, for example:

  ```bash
  nodes=$(tbot api query databases --label foo=bar)
  for name in $nodes; do
    tbot api spawn database-tunnel "$name" --listen unix:///path/to/db-$name.sock
  done
  ```
- `tbot api query node --label foo=bar --format=ansible`: query for nodes and
  generate an Ansible inventory. We could also evaluate an inventory plugin to
  do this automatically.

Generally speaking, this allows us to provide a minimal Teleport API proxy for
certain machine use cases. There's a threshold at which it will always make more
sense to import the full API client and use `embeddedtbot`, but a few carefully
curated local-only APIs that are very easy to integrate into another app might
prove useful.

#### AI Agent use

We could feasibly allow AI agents to interact with the `tbot` client and request
access to new services on demand. We would want to establish some real use cases
before pursuing this properly, but some variety of machine-usable API is a
necessary prerequisite.

#### Remote configuration

We've discussed remote (or "server-driven") bot configuration internally as an
additional tool for admins to provision bots with IaC. For example, this could
let admins configure bot services via Teleport's Terraform provider and tweak
the behavior of running bots on the fly.

This isn't something we can realistically pursue without a very clear use case
in mind, or without customer interest. Nevertheless, the backend work to enable
dynamic service management in `tbot` is a necessary prerequisite if we ever did
decide to implement remote configuration.

#### Interactive onboarding

We could use this new functionality (bot API, dynamic services, or both) to
provide an interactive local onboarding experience. This could take many forms,
including a configuration TUI, an LLM-backed config wizard, or full integration
into Teleport's UI. Whichever path we pick (if any), this would walk users
interactively through starting all bot services and committing the configuration
when finished.

Alongside other recent improvements, we should have all the building blocks
needed to create a high-quality interactive configuration flow. We can provide
users with a command that requires zero parameters tweaks to start the
interactive flow on any given client system, provide live feedback as they
configure services, and then commit the configuration to disk, even creating a
systemd unit file automatically if needed.

[poc]: https://github.com/gravitational/teleport/pull/62020
[demo]: https://goteleport.zoom.us/clips/share/b6Qniz7qSjmeXviwZPSFYg
