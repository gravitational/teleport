# Teleport Enterprise Quick Start

Welcome to the Quick Start Guide for Teleport Enterprise.

The goal of this document is to show off the basic capabilities of Teleport.
There are three types of services Teleport nodes can run: `nodes`, `proxies`
and `auth servers`.

- **Auth servers** store user accounts and
  provide authentication and authorization services for every node and every
  user in a cluster.
- **Proxy servers** route client connection requests to the appropriate node and serve a
  Web UI which can also be used to log into SSH nodes. Every client-to-node
  connection in Teleport must be routed via a proxy.
- **Nodes** are regular SSH servers, similar to the `sshd` daemon you may be familiar
  with. When a node receives a connection request, the request is authenticated
  through the cluster's auth server.

The `teleport` daemon runs all three of these services by default. This Quick
Start Guide will be using this default behavior to create a cluster and
interact with it using Teleport's client-side tools:

| Tool       | Description
|------------|------------
| tctl       | Cluster administration tool used to invite nodes to a cluster and manage user accounts.
| tsh        | Similar in principle to OpenSSH's `ssh`. Used to login into remote SSH nodes, list and search for nodes in a cluster, securely upload/download files, etc.
| browser    | You can use your web browser to login into any Teleport node by opening `https://<proxy-host>:3080`.

## Prerequisites

You will need to have access to the [customer portal](https://dashboard.gravitational.com)
to download the software. You will also need three computers: two servers and
one client (probably a laptop) to complete this tutorial. Let's assume the servers have
the following DNS names and IPs:

Server Name    |  IP Address    | Purpose
---------------|----------------|--------------
_"auth.example.com"_  | 10.1.1.10      | This server will be used to run all three Teleport services: auth, proxy and node.
_"node.example.com"_  | 10.1.1.11      | This server will only run the SSH service. The vast majority of servers in production will be nodes.

This Quick Start Guide assumes that both servers are running a [systemd-based](https://www.freedesktop.org/wiki/Software/systemd/)
Linux distribution such as Debian, Ubuntu or a RHEL derivative.

## Installing

To start using Teleport Enterprise, you will need to Download the binaries and the license file from the [customer portal](https://dashboard.gravitational.com).
After downloading the binary tarball, run:

```bsh
$ tar -xzf teleport-ent-v{{ teleport.version }}-linux-amd64-bin.tar.gz
$ cd teleport-ent
```

* Copy `teleport` and `tctl` binaries to a bin directory (we suggest `/usr/local/bin`) on the auth server.
* Copy `teleport` binary to a bin directory on the node server.
* Copy `tsh` binary to a bin directory on the client computer.

### License File

The Teleport license file contains a X.509 certificate and the corresponding
private key in [PEM format](https://en.wikipedia.org/wiki/Privacy-enhanced_Electronic_Mail).

Download the license file from the [customer portal](https://dashboard.gravitational.com)
and save it as `/var/lib/teleport/license.pem` on the auth server.


### Configuration File

Save the following configuration file as `/etc/teleport.yaml` on the _node.example.com_:

```yaml
teleport:
  auth_token: dogs-are-much-nicer-than-cats
  # you can also use auth server's IP, i.e. "10.1.1.10:3025"
  auth_servers: [ "auth.example.com:3025" ]

  # enable ssh service and disable auth and proxy:
ssh_service:
  enabled: true

auth_service:
  enabled: false
proxy_service:
  enabled: false
```

Now, save the following configuration file as `/etc/teleport.yaml` on the _auth.example.com_:

```yaml
teleport:
  auth_token: dogs-are-much-nicer-than-cats
  auth_servers: [ "localhost:3025" ]

auth_service:
  # enable the auth service:
  enabled: true

  tokens:
  # this static token is used for other nodes to join this Teleport cluster
  - proxy,node:dogs-are-much-nicer-than-cats
  # this token is used to establish trust with other Teleport clusters
  - trusted_cluster:trains-are-superior-to-cars

  # by default, local authentication will be used with 2FA
  authentication:
      second_factor: otp

  # SSH is also enabled on this node:
ssh_service:
  enabled: "yes"
```


### Systemd Unit File

Next, download the systemd service unit file from [examples directory](https://github.com/gravitational/teleport/tree/master/examples/systemd)
on Github and save it as `/etc/systemd/system/teleport.service` on both servers.

```bsh
# run this on both servers:
$ sudo systemctl daemon-reload
$ sudo systemctl enable teleport
```

## Starting

```bsh
# run this on both servers:
$ sudo systemctl start teleport
```

Teleport daemon should start and you can use `netstat -lptne` to make sure that
it's listening on [TCP/IP ports](../admin-guide.md#ports). On _auth.example.com_, it should
look something like this:

```bsh
$ auth.example.com ~: sudo netstat -lptne
Active Internet connections (only servers)
Proto Recv-Q Send-Q Local Address   State       User       PID/Program name
tcp6       0      0 :::3024         LISTEN      0          337/teleport
tcp6       0      0 :::3025         LISTEN      0          337/teleport
tcp6       0      0 :::3080         LISTEN      0          337/teleport
tcp6       0      0 :::3022         LISTEN      0          337/teleport
tcp6       0      0 :::3023         LISTEN      0          337/teleport
```

and _node.example.com_ should look something like this:

```bsh
$ node.example.com ~: sudo netstat -lptne
Active Internet connections (only servers)
Proto Recv-Q Send-Q Local Address   State       User       PID/Program name
tcp6       0      0 :::3022         LISTEN      0          337/teleport
```

See [troubleshooting](#troubleshooting) section at the bottom if something is not working.

## Adding Users

This portion of the Quick Start Guide should be performed on the auth server, i.e.
on _auth.example.com_

Every user in a Teleport cluster must be assigned at least one role. By
default, Teleport comes with one pre-configured role called "admin". You can
see it's definition by executing `sudo tctl get roles/admin > admin-role.yaml`.

The output will look like this (re-formatted here to use compact YAML
representation for brevity):

```yaml
kind: role
version: v3
metadata:
  name: admin
spec:
  options:
    cert_format: standard
    forward_agent: true
    max_session_ttl: 30h0m0s
    port_forwarding: true
  # allow rules:
  allow:
    logins:
    - '{% raw %}{{internal.logins}}{% endraw %}'
    - root
    node_labels:
      '*': '*'
    rules:
    - resources: [role]
      verbs: [list, create, read, update, delete]
    - resources: [auth_connector]
      verbs: [list, create, read, update, delete]
    - resources: [session]
      verbs: [list, read]
    - resources: [trusted_cluster]
      verbs: [list, create, read, update, delete]
  # no deny rules are present, the admin role must have access to everything)
  deny: {}
```

Pay attention to the _allow/logins_ field in the role definition: by default, this
role only allows SSH logins as `root@host`.

!!! note "Note"

    Ignore `{% raw %}{{internal.logins}}{% endraw %}` "allowed login" for now. It exists for
    compatibility purposes when upgrading existing open source Teleport
    clusters.

You probably want to replace "root" with something else. Let's assume there will
be a local UNIX account called "admin" on all hosts. In this case you can
dump the role definition YAML into _admin-role.yaml_ file and update "allow/logins"
to look like this:

```yaml
allow:
   logins: [admin]
```

Then send it back into Teleport:

```bsh
$ sudo tctl create -f admin-role.yaml
```

Now, lets create a new Teleport user "joe" with "admin" role:

```bsh
$ sudo tctl users add --roles=admin joe

Signup token has been created and is valid for 1 hours. Share this URL with the user:
https://auth.example.com:3080/web/newuser/22e3acb6a0c2cde22f13bdc879ff9d2a
```

Share the generated sign-up URL with Joe and let him pick a password and configure
the second factor authentication. We recommend [Google Authenticator](https://en.wikipedia.org/wiki/Google_Authenticator)
which is available for both Android and iPhone.

## Assigning Roles

To update user's roles, dump the user resource into a file:

```bsh
$ sudo tctl get users/joe > joe.yaml
```

Edit the YAML file and update the "roles" array.
Then, re-insert it back:

```bsh
$ sudo tctl create -f joe.yaml
```

## Logging In

Joe now has a local account on a Teleport cluster. The local account is good for
administrative purposes but regular users of Teleport Enterprise should be using
a Single Sign-On (SSO) mechanism.

But first, lets see how Joe can log into the Teleport cluster. He can do this
on his client laptop:

```bsh
$ tsh --proxy=auth.example.com --insecure login --user=joe
```

Note that "--user=joe" part can be omitted if `$USER` environment variable is "joe".

Notice that `tsh` client always needs `--proxy` flag because all client connections
in Teleport always must to go through an SSH proxy, sometimes called an "SSH bastion".

!!! warning "Warning"

    For the purposes of this quickstart we are using the `--insecure` flag which allows
    us to skip configuring the HTTP/TLS certificate for Teleport proxy. Your browser will
    throw a warning **Your connection is not private**. Click Advanced, and **Proceed to 0.0.0.0 (unsafe)**
    to preview the Teleport UI.

    Never use `--insecure` in production unless you terminate SSL at a load balancer. This will
    apply to most cloud providers (AWS, GCP and Azure). You must configure a HTTP/TLS certificate for the Proxy.
    This process has been made easier with Let's Encrypt. [We've instructions here](https://gravitational.com/blog/letsencrypt-teleport-ssh/).

If successful, `tsh login` command will receive Joe's user certificate and will
store it in `~/.tsh/keys/<proxy>` directory.

With a certificate in place, Joe can now interact with the Teleport cluster:

```bsh
# SSH into any host behind the proxy:
$ tsh ssh node.example.com

# See what hosts are available behind the proxy:
$ tsh ls

# Log out (this will remove the user certificate from ~/.tsh)
$ tsh logout
```

## Configuring SSO

The local account is good for administrative purposes but regular users of
Teleport Enterprise should be using a Single Sign-On (SSO) mechanism that use SAML or OIDC protocols.


Take a look at the [SSH via Single Sign-on](ssh_sso.md) chapter to learn the basics of
integrating Teleport with SSO providers. We have the following detailed guides for
configuring SSO providers:

* [Okta](sso/ssh_okta.md)
* [Active Directory](sso/ssh_adfs.md)
* [One Login](sso/ssh_one_login.md)
* [Github](../admin-guide.md#github-oauth-20)

Any SAML-compliant provider can be configured with Teleport by following the
same steps.  There are Teleport Enterprise customers who are using Oracle IDM,
SailPoint and others.

## Troubleshooting

If Teleport services do not start, take a look at the syslog:

```bsh
$ sudo journalctl -fu teleport
```

Usually the error will be reported there. Common reasons for failure are:

* Mismatched tokens, i.e. "auth_token" on the node does not match "tokens/node" value on the auth server.
* Network issues: port `3025` is closed via iptables.
* Network issues: ports `3025` or `3022` are occupied by another process.
* Disk issues: Teleport fails to create `/var/lib/teleport` because the volume is read-only or not accessible.

## Getting Help

If something is not working, please reach out to us by creating a ticket in your [customer portal](https://dashboard.gravitational.com/).
Customers who have purchased the premium support package can also ping us through
your Slack channel.
