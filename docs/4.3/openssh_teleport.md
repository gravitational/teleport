# Using Teleport with OpenSSH

Teleport is fully compatible with OpenSSH and can be quickly setup to record and
audit all SSH activity. Using Teleport and OpenSSH has a few pros / cons, and long
term we would recommend replacing `sshd` with `teleport`.  We've oulined these in
[OpenSSH vs Teleport SSH for Servers?](https://gravitational.com/blog/openssh-vs-teleport/)

Existing fleets of OpenSSH servers can be configured to accept SSH certificates
dynamically issued by a Teleport CA. This makes it easier and quicker to adopt
Teleport and often is used as the first step. We'll outline how to set it up here.

## Architecture

~ blah ~
~ fix these
~ https://gravitational.com/blog/how-to-record-ssh-sessions/


## Setting up OpenSSH Recording Proxy Mode

To enable session recording for `sshd` nodes, the cluster must be switched to
["recording proxy" mode](architecture/teleport_proxy.md#recording-proxy-mode).
In this mode, the recording will be done on the proxy level:

``` yaml
# snippet from /etc/teleport.yaml
auth_service:
   # Session Recording must be set to Proxy to work with OpenSSH
   session_recording: "proxy"  # can also be "off" and "node" (default)
```

Next, `sshd` must be told to allow users to log in with certificates generated
by the Teleport User CA. Start by exporting the Teleport CA public key:

Export the Teleport CA certificate into a file:

``` bash
# tctl needs to be ran on the auth server.
$ tctl auth export --type=user > teleport_user_ca.pub
```

To allow access per-user, append the contents of `teleport_user_ca.pub` to
`~/.ssh/authorized_keys` .

To allow access for all users:

  + Edit `teleport_user_ca.pub` and remove `cert-authority` from the start of
    line.
  + Copy `teleport_user_ca.pub` to `/etc/ssh/teleport-user-ca.pub`
  + Update `sshd` configuration (usually `/etc/ssh/sshd_config` ) to point to
    this file: `TrustedUserCAKeys /etc/ssh/teleport_user_ca.pub`

Add the following line to `/etc/ssh/sshd_config` :

``` yaml
TrustedUserCAKeys /etc/ssh/teleport_user_ca.pub
```

Now `sshd` will trust users who present a Teleport-issued certificate. The next
step is to configure host authentication.

When in recording mode, Teleport will check that the host certificate of the
node a user connects to is signed by a Teleport CA. By default this is a strict
check. If the node presents just a key, or a certificate signed by a different
CA, Teleport will reject this connection with the error message saying _"ssh:
handshake failed: remote host presented a public key, expected a host
certificate"_

You can disable strict host checks as shown below. However, this opens the
possibility for Man-in-the-Middle (MITM) attacks and is not recommended.

``` yaml
# snippet from /etc/teleport.yaml
auth_service:
  proxy_checks_host_keys: no
```

The recommended solution is to ask Teleport to issue valid host certificates for
all OpenSSH nodes. To generate a host certificate run this on your auth server:

``` bash
$ tctl auth sign \
      --host=node.example.com \
      --format=openssh
```

Then add the following lines to `/etc/ssh/sshd_config` and restart sshd.

``` yaml
HostKey /etc/ssh/node.example.com
HostCertificate /etc/ssh/node.example.com-cert.pub
```

Now you can use [ `tsh ssh --port=22 user@host.example.com` ](cli-docs.md#tsh) to login
into any `sshd` node in the cluster and the session will be recorded. If you
want to use OpenSSH `ssh` client for logging into `sshd` servers behind a proxy
in "recording mode", you have to tell the `ssh` client to use the jump host and
enable the agent forwarding, otherwise a recording proxy will not be able to
terminate the SSH connection to record it:

``` bash
# Note that agent forwarding is enabled twice: one from a client to a proxy
# (mandatory if using a recording proxy), and then optionally from a proxy
# to the end server if you want your agent running on the end server or not
$ ssh -o "ForwardAgent yes" \
    -o "ProxyCommand ssh -o 'ForwardAgent yes' -p 3023 %r@p.example.com -s proxy:%h:%p" \
    user@host.example.com
```

!!! tip "Tip"

    To avoid typing all this and use the usual `ssh
    user@host.example.com `, users can update their ` ~/.ssh/config` file.

**IMPORTANT**

It's important to remember that SSH agent forwarding must be enabled on the
client. Verify that a Teleport certificate is loaded into the agent after
logging in:

``` bsh
# Login as Joe
$ tsh login --proxy=proxy.example.com --user=joe
# see if the certificate is present (look for "teleport:joe") at the end of the cert
$ ssh-add -L
```

!!! warning "GNOME Keyring SSH Agent and GPG Agent"

    It is well-known that Gnome Keyring SSH agent, used by many popular Linux
    desktops like Ubuntu, and gpg-agent from GnuPG do not support SSH
    certificates. We recommend using the `ssh-agent` from OpenSSH.
    Alternatively, you can disable SSH agent integration entirely using
    `--no-use-local-ssh-agent` flag or `TELEPORT_USE_LOCAL_SSH_AGENT=false`
    environment variable with `tsh`.


### Using OpenSSH Client

It is possible to use the OpenSSH client `ssh` to connect to nodes within a
Teleport cluster. Teleport supports SSH subsystems and includes a `proxy`
subsystem that can be used like `netcat` is with `ProxyCommand` to connect
through a jump host.

On your client machine, you need to import these keys. It will allow your
OpenSSH client to verify that host's certificates are signed by the trusted CA
key:

``` yaml
$ cat teleport_user_ca.pub >> ~/.ssh/known_hosts
```

If you have multiple Teleport clusters, you have to export and set up these
certificate authorities for each cluster individually.

!!! tip "OpenSSH and Trusted Clusters"

    If you use [recording proxy mode](#recording-proxy-mode) and [trusted
    clusters](#trusted-clusters), you need to set up certificate authority from
    the _root_ cluster to match **all** nodes, even those that belong to _leaf_
    clusters. For example, if your node naming scheme is `*.root.example.com`,
    `*.leaf1.example.com`, `*.leaf2.example.com`, then the
    `@certificate-authority` entry should match `*.example.com` and use the CA
    from the root auth server only.

Make sure you are running OpenSSH's `ssh-agent` , and have logged in to the
Teleport proxy:

``` bash
$ eval `ssh-agent`
$ tsh --proxy=work.example.com login
```

`ssh-agent` will print environment variables into the console. Either `eval` the
output as in the example above, or copy and paste the output into the shell you
will be using to connect to a Teleport node. The output exports the
`SSH_AUTH_SOCK` and `SSH_AGENT_PID` environment variables that allow OpenSSH
clients to find the SSH agent.

Lastly, configure the OpenSSH client to use the Teleport proxy when connecting
to nodes with matching names. Edit `~/.ssh/config` for your user or
`/etc/ssh/ssh_config` for global changes:

``` bash
# work.example.com is the jump host (proxy). credentials will be obtained from the
# openssh agent.
Host work.example.com
    HostName 192.168.1.2
    Port 3023

# connect to nodes in the work.example.com cluster through the jump
# host (proxy) using the same. credentials will be obtained from the
# openssh agent.
Host *.work.example.com
    HostName %h
    Port 3022
    ProxyCommand ssh -p 3023 %r@work.example.com -s proxy:%h:%p

# when connecting to a node within a trusted cluster with name "remote-cluster",
# add the name of the cluster to the invocation of the proxy subsystem.
Host *.remote-cluster.example.com
   HostName %h
   Port 3022
   ProxyCommand ssh -p 3023 %r@work.example.com -s proxy:%h:%p@remote-cluster
```

When everything is configured properly, you can use ssh to connect to any node
behind `work.example.com` :

``` bash
$ ssh root@database.work.example.com
```

!!! tip "Note"

    Teleport uses OpenSSH certificates instead of keys which means
    you can not connect to a Teleport node by IP address. You have to connect by
    DNS name. This is because OpenSSH ensures the DNS name of the node you are
    connecting is listed under the `Principals` section of the OpenSSH
    certificate to verify you are connecting to the correct node.


To connect to the OpenSSH server via `tsh`, add `--port=<ssh port>` with the `tsh ssh` command:

Example ssh to `database.work.example.com` as `root` with a OpenSSH server on port 22 via `tsh`:
   tsh ssh --port=22 root@database.work.example.com

!!! warning "Warning"

    The principal (username) being used to connect must be listed in the Teleport user/role configuration.

### OpenSSH Rate Limiting

When using a Teleport proxy in "recording mode", be aware of OpenSSH built-in
rate limiting. On large number of proxy connections you may encounter errors
like:

``` bash
channel 0: open failed: connect failed: ssh: handshake failed: EOF
```

See `MaxStartups` setting in `man sshd_config` . This setting means that by
default OpenSSH only allows 10 unauthenticated connections at a time and starts
dropping connections 30% of the time when the number of connections goes over 10
and when it hits 100 authentication connections, all new connections are
dropped.

To increase the concurrency level, increase the value to something like
MaxStartups 50:30:100. This allows 50 concurrent connections and a max of 100.

Teleport is a standards-compliant SSH proxy and it can work in environments with
existing SSH implementations, such as OpenSSH. This section will cover:

* Configuring OpenSSH client `ssh` to login into nodes inside a Teleport
  cluster.
* Configuring OpenSSH server `sshd` to join a Teleport cluster.
