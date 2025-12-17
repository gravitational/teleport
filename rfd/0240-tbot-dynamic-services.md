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
a new bot API service to accept API requests and various changes in the `tbot`
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

`tbot`'s CLI today has a UX cliff:
- Simple use cases, like running one service, are relatively simple. These can
  be done entirely through the CLI, like `tbot start identity ...`, or config
  files can be automatically generated, like `tbot configure app-tunnel ...`.

- If multiple services are required, this breaks. The CLI cannot feasibly
  support running multiple services, so users will need to use a YAML config
  file instead.

- In non-systemd environments, like CI/CD providers, things are further
  complicated: the UX for spawning services in the background can be arcane, so
  users need to solve proper background execution and coordination for each
  Teleport resource they want to access.

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
- Report and/or ensuring service readiness
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
background reliably. For example:

```yaml
steps:
- name: Start a tunnel for app 'foo'
  id: tunnel-foo
  uses: teleport-actions/application-tunnel@v1
  with:
    proxy: example.teleport.sh:443
    token: example-bot
    app: foo
- name: Start a tunnel for app 'bar'
  id: tunnel-bar
  uses: teleport-actions/application-tunnel@v1
  with:
    proxy: example.teleport.sh:443
    token: example-bot
    app: bar
```

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
the ability to ask an existing bot to start a new service on demand.

This features involves running an API server on a local Unix socket. Clients can
request a new service via a "spawn" endpoint and provide service configuration.

Importantly, this endpoint takes advantage of an enhanced version of the same
"wait" ability discussed above, and waits for the service to become ready.
However, it additionally responds with a full error trace if the service fails
outright before reporting its status.

#### Usage example: GitHub Actions

```yaml
steps:
- name: Start a bot API in the background
  id: bot-api
  uses: teleport-actions/bot-api@v1
  with:
    proxy: example.teleport.sh:443
    token: example-bot

- name: Open a database tunnel
  uses: teleport-actions/start@v1
  with:
    kind: database-tunnel
    config: |
      listen: tcp://127.0.0.1:25432
      service: postgres
      database: postgres

- name: Open a application tunnel
  uses: teleport-actions/start@v1
  with:
    kind: application-tunnel
    config: |
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
$ tbot start bot-api --listen unix:///path/to/api.sock ...
$ tbot api spawn db-tunnel --listen unix:///path/to/api.sock --service postgres --database postgres
$ tbot api spawn app-tunnel --listen unix:///path/to/api.sock --app example
```

A minimal [proof of concept implementation][poc] was made to validate this
concept. If desired, you can build the `tbot` client from the linked PR to try
out the CLI flow described here. A [recorded demo][demo] is also available
internally.

#### The bot API

```proto3
syntax = "proto3";

import "google/protobuf/duration.proto";

service BotAPIService {
  // Spawns a new service.
  rpc SpawnService(SpawnServiceRequest) returns (SpawnServiceResponse);
  // Returns the current effective bot configuration.
  rpc GetConfig(GetConfigRequest) returns (GetConfigResponse);
}

message SpawnServiceRequest {
  //
  string configuration = 1;
  // If set, don't wait for the service to become healthy.
  bool skip_wait = 2;
}

// Response when a service has successfully spawned
message SpawnServiceResponse {}

// Requests the current bot configuration
message GetConfigRequest {}

message GetConfigResponse {
  // The effective bot configuration in YAML format.
  string configuration = 1;
}
```

We have two obvious implementation paths for the API:
1. gRPC: Used broadly throughout Teleport, but challenging to use in other
   applications. This challenge can be mitigated with a high-quality CLI.
2. Plain HTTP: Unusual in Teleport, but substantially simpler for others to use
   within their own apps. Arguably harder to maintain.

The initial API will have just two RPCs or endpoints: `spawn` and `config`. It
will be exposed over an arbitrary listening socket; we'll strongly recommend
a Unix socket, but will allow use of TCP sockets.

Regardless of protocol, the v1 bot API endpoints will accept and return
configuration in YAML format. This allows use of our existing config parsing
library without additional refactoring. If we decide to further develop the
local bot API, we should consider revisiting this and switching to a protobuf
source of truth for bot configuration.

##### Option: gRPC API (Preferred)

gRPC does meaningfully increase the cost of integrating with the bot API, and is
less beneficial in this context (local, explicitly intended for end user
integration, etc) than in other Teleport components. However, it should ease the
maintenance burden over time and will simplify the process of adding new
functionality should we develop the bot API further.

The integration burden will be at least somewhat mitigated by providing a CLI
entrypoint with effectively 100% API coverage, which shouldn't be too difficult
given the minimal

Additionally, we can consider adopting a library like
[`connectrpc`](https://github.com/connectrpc/connect-go) to make the API
available to HTTP clients, which is used in Teleport already to allow API access
from the web UI.

##### Option: HTTP API (Alternative)

**`POST /spawn`**: Accepts an array of service definitions in YAML format. This
exercises our existing YAML config parser. Waits for the service to either
report its first status or exit before returning.

**`GET /config`**: Returns the current bot configuration in YAML format.

The [proof of concept implementation][poc] used an HTTP API.

#### The CLI

We will introduce a new family of CLI commands: `tbot api`:

* `tbot api spawn`: Spawns a service. Fully reuses our existing CLI and inherits
  all flags and service subcommands, exactly like `tbot start` and
  `tbot configure`
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

#### Required changes in `tbot`

TODO

#### GitHub Actions changes

We'll introduce 2 new GitHub Actions in the [`teleport-actions`] organization:

- `teleport-actions/bot-api`: starts a bot running only the `bot-api` service,
  joining Teleport through the usual process. It will redirect logs to a file,
  detach, and wait for general bot readiness before continuing. It will
  additionally configure an exit hook to dump logs to the GHA run at the end of
  the job.

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

#### Additional API endpoints

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

#### Agent use

We could feasibly allow agents to interact with the `tbot` client and request
access to new services on demand, however an API needs to exist first to be
called.

[poc]: https://github.com/gravitational/teleport/pull/62020
[demo]: https://goteleport.zoom.us/clips/share/b6Qniz7qSjmeXviwZPSFYg
