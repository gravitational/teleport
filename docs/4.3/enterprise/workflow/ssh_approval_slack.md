## slackbot

This package implements a simple slackbot using the API provided in the
[`access`](../) package which allows Access Requests to be approved/denied
via interactive slack messages.

## Setup

Run `make access-slackbot && ./access/slackbot/build/teleport-slackbot configure` from the
repository root.  The `configure` command will produce an example
configuration file that looks something like this:

```toml
# example slackbot configuration file
[teleport]
auth-server = "example.com:3025"                  # Auth GRPC API address.  
client-key = "/var/lib/teleport/plugins/slackbot/auth.key"  # Teleport GRPC client secret key
client-crt = "/var/lib/teleport/plugins/slackbot/auth.crt"  # Teleport GRPC client certificate 
root-cas = "/var/lib/teleport/plugins/slackbot/auth.cas"    # Teleport cluster CA certs

[slack]
token = "api-token"         # Slack Bot OAuth token
secret = "secret-value"     # Slack API Signing Secret
channel = "channel-name"    # Message delivery channel

[http]
listen = ":8081" # Callback http server listen addr.
# host = "example.com" # Host name by which interaction callback is accessible publicly.
# https-key-file = "/var/lib/teleport/plugins/slackbot/server.key"  # TLS private key
# https-cert-file = "/var/lib/teleport/plugins/slackbot/server.crt" # TLS certificate

[log]
output = "stderr" # Logger output. Could be "stdout", "stderr" or "/var/lib/teleport/slackbot.log"
severity = "INFO" # Logger severity. Could be "INFO", "ERROR", "DEBUG" or "WARN".
```

Detailed install steps are provided within the [`install`](INSTALL.md) instructions.

### `[teleport]`

This configuration section ensures that the bot can talk to your teleport
auth server & manage access-requests.  Use `tctl auth sign --format=tls`
to generate the required PEM files, and make sure that the Auth Server's
GRPC API is accessible at the address indicated by `auth-server`.

*NOTE*: The slackbot must be given a teleport user identity with
appropriate permissions.  See the [acccess package README](../README.md#authentication)
for an example of how to configure an appropriate user & role.

### `[slack]`

In order to interact with slack, we need a valid bot OAuth token and we need
to be able to receive callbacks from slack when users interact with messages.

A token can be provisioned from [api.slack.com](https://api.slack.com) by
registering an App and associated Bot User for your workspace.

In order to receive interaction callbacks, make sure the `host` address is
publicly accessible and register it with your App under 
`Features > Interactive Components > Request URL`.

*NOTE*: For debug purposes, slack recommends using `ngrok http` to get a
public HTTPS endpoint for your interaction callback. You must also use
`--insecure-no-tls` option when running Slackbot under `ngrok`.

## Usage

Once your slackbot has been configured, you can verify that it is working
correctly by using `tctl request create <user> --roles=<roles>` to simulate
an access request.  If everything is working as intended, a message with
`Approve` and `Deny` buttons should appear in the channel specified under
`slack.channel`.  Select `Deny` and verify that the request was indeed
denied using `tctl request ls`.


## Security

Currently, this Bot does not make any distinction about *who* approves/denies
a request.  Any user with access to the specified channel will be able to
manage requests.  Therefore, it is important that access to the channel
be limited.
