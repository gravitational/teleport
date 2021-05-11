## Example workflows plugin

This example plugin demonstrates how to create a workflows plugin to automatically approve/deny new access requests based on a simple whitelist.

## Demo

This demo can be used to quickly get the example plugin up and running.

##### Create resources

```bash
# create the access-plugin user and role
$ tctl create -f access-plugin.yaml
# generate an identity file for the access-plugin
$ tctl auth sign --ttl=8760h --format=file --user=access-plugin --out=access-plugin-identity
```

##### Edit the config file

Open `config.toml` and replace the `addr` with your own Auth or Proxy server address.

##### Run the plugin

Start up the plugin and keep it running.

```bash
$ go run main.go
```

##### Make an access request

Open another terminal and execute the following commands to make a new access request.

```bash
# create the requester role
$ tctl create -f requester.yaml
# create a new user named alice using this role
$ tctl users add alice --roles=requester
# login as alice
$ tsh --proxy=proxy.example.com login --user=alice
# request the admin role as alice
$ tsh --proxy=proxy.example.com request new --roles=admin
```

Since `alice` is on the whitelist in `cofig.toml`, the request should be automatically approved by the plugin.