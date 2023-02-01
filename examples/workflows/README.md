## Example workflows plugin

This example plugin demonstrates how to create a workflows plugin to automatically approve/deny new access requests based on a simple Allow List.

## Demo

This demo can be used to quickly get the example plugin up and running. If you are
connecting to your Teleport Cluster from your machine you must have impersonation rights in your role
such as below to issue plugin credentials.

```yaml
kind: role
metadata:
  name: example-role
spec:
  allow:
    impersonate:
      users: ['access-plugin-auto-approve']
      roles: ['access-plugin-auto-approve']
```

### Create resources

```console
# create the access-plugin-auto-approve user and role
$ tctl create -f access-plugin.yaml
# generate an identity file for the access-plugin
$ tctl auth sign --ttl=8760h --format=file --user=access-plugin-auto-approve --out=access-plugin-identity
```

### Edit the config file

Open `config.toml` and replace the `addr` with your own Auth or Proxy server address.

### Run the plugin

Start up the plugin and keep it running.

```console
$ go run main.go
```

### Make an access request

Open another terminal and execute the following commands to make a new access request.

```console
# create the requester role
$ tctl create -f requester.yaml
# create a new user named alice using this role
$ tctl users add alice --roles=requester
# login as alice
$ tsh --proxy=proxy.example.com login --user=alice
# request the editor role as alice
$ tsh request new --roles=editor
```

Since `alice` is on the Allow List in `config.toml` with allowed role `requester`, the request should be automatically approved by the plugin.

Sample plugin output.

```console
2023/02/01 16:39:54 watcher initialized...
2023/02/01 16:44:29 Handling request: AccessRequest(user=alice,roles=[editor])
2023/02/01 16:44:29 User "alice" in Allow List, approving request...
2023/02/01 16:44:29 Request state set: APPROVED.
```
