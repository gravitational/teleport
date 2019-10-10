# YAML Configuration

You should use a [configuration file](#configuration-file) to configure the `teleport` daemon.
For simple experimentation, you can use command line flags with the `teleport start`
command:

```bsh
$ teleport start --help
usage: teleport start [<flags>]

Starts the Teleport service.

Flags:
      --insecure-no-tls  Disable TLS for the web socket
  -r, --roles            Comma-separated list of roles to start with [proxy,node,auth]
      --pid-file         Full path to the PID file. By default no PID file will be created
      --advertise-ip     IP to advertise to clients if running behind NAT
  -l, --listen-ip        IP address to bind to [0.0.0.0]
      --auth-server      Address of the auth server [127.0.0.1:3025]
      --token            Invitation token to register with an auth server [none]
      --ca-pin           CA pin to validate the Auth Server
      --nodename         Name of this node, defaults to hostname
  -c, --config           Path to a configuration file [/etc/teleport.yaml]
      --labels           List of labels for this node
      --insecure         Insecure mode disables certificate validation
      --fips             Start Teleport in FedRAMP/FIPS 140-2 mode.
```

### Configuration Flags

Let's cover some of these flags in more detail:

* `--insecure-no-tls` flag tells Teleport proxy to not generate default self-signed TLS
  certificates. This is useful when running Teleport on kubernetes (behind reverse
  proxy) or behind things like AWS ELBs, GCP LBs or Azure Load Balancers where SSL termination 
  is provided externally.
  The possible values are `true` or  `false`. The default value is `false`.

* `--roles` flag tells Teleport which services to start. It is a comma-separated
  list of roles. The possible values are `auth`, `node` and `proxy`. The default
  value is `auth,node,proxy`. These roles are explained in the
  [Teleport Architecture](architecture.md) document.

* `--advertise-ip` flag can be used when Teleport nodes are running behind NAT and
  their externally routable IP cannot be automatically determined.
  For example, assume that a host "foo" can be reached via `10.0.0.10` but there is
  no `A` DNS record for "foo", so you cannot connect to it via `tsh ssh foo`. If
  you start teleport on "foo" with `--advertise-ip=10.0.0.10`, it will automatically
  tell Teleport proxy to use that IP when someone tries to connect
  to "foo". This is also useful when connecting to Teleport nodes using their labels.

* `--nodename` flag lets you assign an alternative name for the node which can be used
  by clients to login. By default it's equal to the value returned by `hostname`
  command.

* `--listen-ip` should be used to tell `teleport` daemon to bind to a specific network
  interface. By default it listens on all.

* `--labels` flag assigns a set of labels to a node. See the explanation
  of labeling mechanism in the [Labeling Nodes](#labeling-nodes) section below.

* `--pid-file` flag creates a PID file if a path is given.

* `--permit-user-env` flag reads in environment variables from `~/.tsh/environment`
  when creating a session.

### Configuration File

Teleport uses the YAML file format for configuration. A sample configuration file is shown below. By default, it is stored in `/etc/teleport.yaml`

!!! note "IMPORTANT":
    When editing YAML configuration, please pay attention to how your editor
    handles white space. YAML requires consistent handling of tab characters.

```yaml
# By default, this file should be stored in /etc/teleport.yaml

# This section of the configuration file applies to all teleport
# services.
teleport:
    # nodename allows to assign an alternative name this node can be reached by.
    # by default it's equal to hostname
    nodename: graviton

    # Data directory where Teleport daemon keeps its data.
    # See "Filesystem Layout" section above for more details.
    data_dir: /var/lib/teleport

    # Invitation token used to join a cluster. it is not used on
    # subsequent starts
    auth_token: xxxx-token-xxxx

    # Optional CA pin of the auth server. This enables more secure way of adding new
    # nodes to a cluster. See "Adding Nodes" section above.
    ca_pin: "sha256:7e12c17c20d9cb504bbcb3f0236be3f446861f1396dcbb44425fe28ec1c108f1"

    # When running in multi-homed or NATed environments Teleport nodes need
    # to know which IP it will be reachable at by other nodes
    #
    # This value can be specified as FQDN e.g. host.example.com
    advertise_ip: 10.1.0.5

    # list of auth servers in a cluster. you will have more than one auth server
    # if you configure teleport auth to run in HA configuration.
    # If adding a node located behind NAT, use the Proxy URL. e.g. 
    #  auth_servers:
    #     - teleport-proxy.example.com:3080
    auth_servers:
        - 10.1.0.5:3025
        - 10.1.0.6:3025

    # Teleport throttles all connections to avoid abuse. These settings allow
    # you to adjust the default limits
    connection_limits:
        max_connections: 1000
        max_users: 250

    # Logging configuration. Possible output values are 'stdout', 'stderr' and
    # 'syslog'. Possible severity values are INFO, WARN and ERROR (default).
    log:
        output: stderr
        severity: ERROR

    # Configuration for the storage back-end used for the cluster state and the
    # audit log. Several back-end types are supported. See "High Availability"
    # section of this Admin Manual below to learn how to configure DynamoDB, 
    # S3, etcd and other highly available back-ends. 
    storage:
        # By default teleport uses the `data_dir` directory on a local filesystem
        type: dir

        # Array of locations where the audit log events will be stored. by
        # default they are stored in `/var/lib/teleport/log`
        audit_events_uri: ['file:///var/lib/teleport/log', 'dynamodb://events_table_name', 'stdout://']

        # Use this setting to configure teleport to store the recorded sessions in
        # an AWS S3 bucket. see "Using Amazon S3" chapter for more information.
        audit_sessions_uri: 's3://example.com/path/to/bucket?region=us-east-1'

    # Cipher algorithms that the server supports. This section only needs to be
    # set if you want to override the defaults.
    ciphers:
      - aes128-ctr
      - aes192-ctr
      - aes256-ctr
      - aes128-gcm@openssh.com
      - chacha20-poly1305@openssh.com

    # Key exchange algorithms that the server supports. This section only needs
    # to be set if you want to override the defaults.
    kex_algos:
      - curve25519-sha256@libssh.org
      - ecdh-sha2-nistp256
      - ecdh-sha2-nistp384
      - ecdh-sha2-nistp521

    # Message authentication code (MAC) algorithms that the server supports.
    # This section only needs to be set if you want to override the defaults.
    mac_algos:
      - hmac-sha2-256-etm@openssh.com
      - hmac-sha2-256

    # List of the supported ciphersuites. If this section is not specified,
    # only the default ciphersuites are enabled.
    ciphersuites:
       - tls-rsa-with-aes-128-gcm-sha256
       - tls-rsa-with-aes-256-gcm-sha384
       - tls-ecdhe-rsa-with-aes-128-gcm-sha256
       - tls-ecdhe-ecdsa-with-aes-128-gcm-sha256
       - tls-ecdhe-rsa-with-aes-256-gcm-sha384
       - tls-ecdhe-ecdsa-with-aes-256-gcm-sha384
       - tls-ecdhe-rsa-with-chacha20-poly1305
       - tls-ecdhe-ecdsa-with-chacha20-poly1305


# This section configures the 'auth service':
auth_service:
    # Turns 'auth' role on. Default is 'yes'
    enabled: yes

    # A cluster name is used as part of a signature in certificates
    # generated by this CA.
    #
    # We strongly recommend to explicitly set it to something meaningful as it
    # becomes important when configuring trust between multiple clusters.
    #
    # By default an automatically generated name is used (not recommended)
    #
    # IMPORTANT: if you change cluster_name, it will invalidate all generated
    # certificates and keys (may need to wipe out /var/lib/teleport directory)
    cluster_name: "main"

    authentication:
        # default authentication type. possible values are 'local', 'oidc' and 'saml'
        # only local authentication (Teleport's own user DB) is supported in the open
        # source version
        type: local
        # second_factor can be off, otp, or u2f
        second_factor: otp
        # this section is used if second_factor is set to 'u2f'
        u2f:
            # app_id must point to the URL of the Teleport Web UI (proxy) accessible
            # by the end users
            app_id: https://localhost:3080
            # facets must list all proxy servers if there are more than one deployed
            facets:
            - https://localhost:3080

    # IP and the port to bind to. Other Teleport nodes will be connecting to
    # this port (AKA "Auth API" or "Cluster API") to validate client
    # certificates
    listen_addr: 0.0.0.0:3025

    # The optional DNS name the auth server if located behind a load balancer.
    # (see public_addr section below)
    public_addr: auth.example.com:3025

    # Pre-defined tokens for adding new nodes to a cluster. Each token specifies
    # the role a new node will be allowed to assume. The more secure way to
    # add nodes is to use `ttl node add --ttl` command to generate auto-expiring
    # tokens.
    #
    # We recommend to use tools like `pwgen` to generate sufficiently random
    # tokens of 32+ byte length.
    tokens:
        - "proxy,node:xxxxx"
        - "auth:yyyy"

    # Optional setting for configuring session recording. Possible values are:
    #    "node"  : sessions will be recorded on the node level  (the default)
    #    "proxy" : recording on the proxy level, see "recording proxy mode" section.
    #    "off"   : session recording is turned off
    session_recording: "node"

    # This setting determines if a Teleport proxy performs strict host key checks.
    # Only applicable if session_recording=proxy, see "recording proxy mode" for details.
    proxy_checks_host_keys: yes

    # Determines if SSH sessions to cluster nodes are forcefully terminated
    # after no activity from a client (idle client).
    # Examples: "30m", "1h" or "1h30m"
    client_idle_timeout: never

    # Determines if the clients will be forcefully disconnected when their
    # certificates expire in the middle of an active SSH session. (default is 'no')
    disconnect_expired_cert: no

    # Determines the interval at which Teleport will send keep-alive messages. The 
    # default value mirrors sshd at 15 minutes.  keep_alive_count_max is the number 
    # of missed keep-alive messages before the server tears down the connection to the 
    # client. 
    keep_alive_interval: 15
    keep_alive_count_max: 3

    # License file to start auth server with. Note that this setting is ignored
    # in open-source Teleport and is required only for Teleport Pro, Business
    # and Enterprise subscription plans.
    #
    # The path can be either absolute or relative to the configured `data_dir`
    # and should point to the license file obtained from Teleport Download Portal.
    #
    # If not set, by default Teleport will look for the `license.pem` file in
    # the configured `data_dir`.
    license_file: /var/lib/teleport/license.pem

    # DEPRECATED in Teleport 3.2 (moved to proxy_service section)
    kubeconfig_file: /path/to/kubeconfig

# This section configures the 'node service':
ssh_service:
    # Turns 'ssh' role on. Default is 'yes'
    enabled: yes

    # IP and the port for SSH service to bind to.
    listen_addr: 0.0.0.0:3022

    # The optional public address the SSH service. This is useful if administrators
    # want to allow users to connect to nodes directly, bypassing a Teleport proxy
    # (see public_addr section below)
    public_addr: node.example.com:3022

    # See explanation of labels in "Labeling Nodes" section below
    labels:
        role: master
        type: postgres

    # List of the commands to periodically execute. Their output will be used as node labels.
    # See "Labeling Nodes" section below for more information and more examples.
    commands:
    # this command will add a label 'arch=x86_64' to a node
    - name: arch
      command: ['/bin/uname', '-p']
      period: 1h0m0s

    # enables reading ~/.tsh/environment before creating a session. by default
    # set to false, can be set true here or as a command line flag.
    permit_user_env: false

    # configures PAM integration. see below for more details.
    pam:
        enabled: no
        service_name: teleport

# This section configures the 'proxy service'
proxy_service:
    # Turns 'proxy' role on. Default is 'yes'
    enabled: yes

    # SSH forwarding/proxy address. Command line (CLI) clients always begin their
    # SSH sessions by connecting to this port
    listen_addr: 0.0.0.0:3023

    # Reverse tunnel listening address. An auth server (CA) can establish an
    # outbound (from behind the firewall) connection to this address.
    # This will allow users of the outside CA to connect to behind-the-firewall
    # nodes.
    tunnel_listen_addr: 0.0.0.0:3024

    # The HTTPS listen address to serve the Web UI and also to authenticate the
    # command line (CLI) users via password+HOTP
    web_listen_addr: 0.0.0.0:3080

    # The DNS name the proxy HTTPS endpoint as accessible by cluster users.
    # Defaults to the proxy's hostname if not specified. If running multiple
    # proxies behind a load balancer, this name must point to the load balancer
    # (see public_addr section below)
    public_addr: proxy.example.com:3080

    # The DNS name of the proxy SSH endpoint as accessible by cluster clients.
    # Defaults to the proxy's hostname if not specified. If running multiple proxies 
    # behind a load balancer, this name must point to the load balancer. 
    # Use a TCP load balancer because this port uses SSH protocol.
    ssh_public_addr: proxy.example.com:3023

    # TLS certificate for the HTTPS connection. Configuring these properly is
    # critical for Teleport security.
    https_key_file: /var/lib/teleport/webproxy_key.pem
    https_cert_file: /var/lib/teleport/webproxy_cert.pem

    # This section configures the Kubernetes proxy service
    kubernetes:
        # Turns 'kubernetes' proxy on. Default is 'no'
        enabled: yes

        # Kubernetes proxy listen address.
        listen_addr: 0.0.0.0:3026

        # The DNS name of the Kubernetes proxy server that is accessible by cluster clients.
        # If running multiple proxies behind  a load balancer, this name must point to the 
        # load balancer.
        public_addr: ['kube.example.com:3026']

        # This setting is not required if the Teleport proxy service is 
        # deployed inside a Kubernetes cluster. Otherwise, Teleport proxy 
        # will use the credentials from this file:
        kubeconfig_file: /path/to/kube/config
```

#### Public Addr

Notice that all three Teleport services (proxy, auth, node) have an optional
`public_addr` property. The public address can take an IP or a DNS name.
It can also be a list of values:

```yaml
public_addr: ["proxy-one.example.com", "proxy-two.example.com"]
```

Specifying a public address for a Teleport service may be useful in the following use cases:

* You have multiple identical services, like proxies, behind a load balancer.
* You want Teleport to issue SSH certificate for the service with the
  additional principals, e.g. host names.

## Authentication

Teleport uses the concept of "authentication connectors" to authenticate users when
they execute `tsh login` command. There are three types of authentication connectors:

### Local Connector

Local authentication is used to authenticate against a local Teleport user database. This database
is managed by `tctl users` command. Teleport also supports second factor authentication
(2FA) for the local connector. There are three possible values (types) of 2FA:

  * `otp` is the default. It implements [TOTP](https://en.wikipedia.org/wiki/Time-based_One-time_Password_Algorithm)
     standard. You can use [Google Authenticator](https://en.wikipedia.org/wiki/Google_Authenticator) or
    [Authy](https://www.authy.com/) or any other TOTP client.
  * `u2f` implements [U2F](https://en.wikipedia.org/wiki/Universal_2nd_Factor) standard for utilizing hardware (USB)
    keys for second factor.
  * `off` turns off second factor authentication.

Here is an example of this setting in the `teleport.yaml`:

```yaml
auth_service:
  authentication:
    type: local
    second_factor: off
```

### Github OAuth 2.0 Connector

This connector implements Github OAuth 2.0 authentication flow. Please refer
to Github documentation on [Creating an OAuth App](https://developer.github.com/apps/building-oauth-apps/creating-an-oauth-app/)
to learn how to create and register an OAuth app.

Here is an example of this setting in the `teleport.yaml`:

```yaml
auth_service:
  authentication:
    type: github
```

See [Github OAuth 2.0](#github-oauth-20) for details on how to configure it.

### SAML

This connector type implements SAML authentication. It can be configured
against any external identity manager like Okta or Auth0. This feature is
only available for Teleport Enterprise.

Here is an example of this setting in the `teleport.yaml`:

```yaml
auth_service:
  authentication:
    type: saml
```

### OIDC

Teleport implements OpenID Connect (OIDC) authentication, which is similar to
SAML in principle. This feature is only available for Teleport Enterprise.

Here is an example of this setting in the `teleport.yaml`:

```yaml
auth_service:
  authentication:
    type: oidc
```


### FIDO U2F

Teleport supports [FIDO U2F](https://www.yubico.com/about/background/fido/)
hardware keys as a second authentication factor. By default U2F is disabled. To start using U2F:

* Enable U2F in Teleport configuration `/etc/teleport.yaml`.
* For CLI-based logins you have to install [u2f-host](https://developers.yubico.com/libu2f-host/) utility.
* For web-based logins you have to use Google Chrome, as it is the only browser supporting U2F at this time.

```yaml
# snippet from /etc/teleport.yaml to show an example configuration of U2F:
auth_service:
  authentication:
    type: local
    second_factor: u2f
    # this section is needed only if second_factor is set to 'u2f'
    u2f:
       # app_id must point to the URL of the Teleport Web UI (proxy) accessible
       # by the end users
       app_id: https://localhost:3080
       # facets must list all proxy servers if there are more than one deployed
       facets:
       - https://localhost:3080
```

For single-proxy setups, the `app_id` setting can be equal to the domain name of the
proxy, but this will prevent you from adding more proxies without changing the
`app_id`. For multi-proxy setups, the `app_id` should be an HTTPS URL pointing to
a JSON file that mirrors `facets` in the auth config.

!!! warning "Warning":
    The `app_id` must never change in the lifetime of the cluster. If the App ID
    changes, all existing U2F key registrations will become invalid and all users
    who use U2F as the second factor will need to re-register.
	When adding a new proxy server, make sure to add it to the list of "facets"
	in the configuration file, but also to the JSON file referenced by `app_id`

 
**Logging in with U2F**

For logging in via the CLI, you must first install [u2f-host](https://developers.yubico.com/libu2f-host/).
Installing:

```yaml
# OSX:
$ brew install libu2f-host

# Ubuntu 16.04 LTS:
$ apt-get install u2f-host
```

Then invoke `tsh ssh` as usual to authenticate:

```
tsh --proxy <proxy-addr> ssh <hostname>
```

!!! tip "Version Warning":
    External user identities are only supported in [Teleport Enterprise](/enterprise/). Please reach
    out to `sales@gravitational.com` for more information.



## Adding Nodes to the Cluster

Teleport is a "clustered" system, meaning it only allows
access to nodes (servers) that had been previously granted cluster membership.

A cluster membership means that a node receives its own host certificate signed
by the cluster's auth server. To receive a host certificate upon joining a cluster,
a new Teleport host must present an "invite token". An invite token also defines
which role a new host can assume within a cluster: `auth`, `proxy` or `node`.

There are two ways to create invitation tokens:

* **Static Tokens** are easy to use and somewhat less secure.
* **Dynamic Tokens** are more secure but require more planning.

### Static Tokens

Static tokens are defined ahead of time by an administrator and stored
in the auth server's config file:

```yaml
# Config section in `/etc/teleport.yaml` file for the auth server
auth_service:
    enabled: true
    tokens:
    # This static token allows new hosts to join the cluster as "proxy" or "node"
    - "proxy,node:secret-token-value"
    # A token can also be stored in a file. In this example the token for adding
    # new auth servers is stored in /path/to/tokenfile
    - "auth:/path/to/tokenfile"
```