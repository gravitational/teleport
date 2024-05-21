---
name: Test Plan
about: Manual test plan for Teleport major releases
title: "Teleport X Test Plan"
labels: testplan
---

## Manual Testing Plan

Below are the items that should be manually tested with each release of Teleport.
These tests should be run on both a fresh installation of the version to be released
as well as an upgrade of the previous version of Teleport.

- [ ] Adding nodes to a cluster
  - [ ] Adding Nodes via Valid Static Token
  - [ ] Adding Nodes via Valid Short-lived Tokens
  - [ ] Adding Nodes via Invalid Token Fails
  - [ ] Revoking Node Invitation

- [ ] Labels
  - [ ] Static Labels
  - [ ] Dynamic Labels

- [ ] Trusted Clusters
  - [ ] Adding Trusted Cluster Valid Static Token
  - [ ] Adding Trusted Cluster Valid Short-lived Token
  - [ ] Adding Trusted Cluster Invalid Token
  - [ ] Removing Trusted Cluster
  - [ ] Changing role map of existing Trusted Cluster

- [ ] RBAC

  Make sure that invalid and valid attempts are reflected in audit log. Do this with both Teleport and [Agentless nodes](https://goteleport.com/docs/server-access/guides/openssh/).

  - [ ] Successfully connect to node with correct role
  - [ ] Unsuccessfully connect to a node in a role restricting access by label
  - [ ] Unsuccessfully connect to a node in a role restricting access by invalid SSH login
  - [ ] Allow/deny role option: SSH agent forwarding
  - [ ] Allow/deny role option: Port forwarding
  - [ ] Allow/deny role option: SSH file copying

- [ ] Verify that custom PAM environment variables are available as expected.

- [ ] Users

    With every user combination, try to login and signup with invalid second
    factor, invalid password to see how the system reacts.

    WebAuthn in the release `tsh` binary is implemented using libfido2 for
    linux/macOS. Ask for a statically built pre-release binary for realistic
    tests. (`tsh fido2 diag` should work in our binary.) Webauthn in Windows
    build is implemented using `webauthn.dll`. (`tsh webauthn diag` with
    security key selected in dialog should work.)

    Touch ID requires a signed `tsh`, ask for a signed pre-release binary so you
    may run the tests.

    Windows Webauthn requires Windows 10 19H1 and device capable of Windows
    Hello.

  - [ ] Adding Users Password Only
  - [ ] Adding Users OTP
  - [ ] Adding Users WebAuthn
    - [ ] macOS/Linux
    - [ ] Windows
  - [ ] Adding Users via platform authenticator
    - [ ] Touch ID
    - [ ] Windows Hello
  - [ ] Managing MFA devices
    - [ ] Add an OTP device with `tsh mfa add`
    - [ ] Add a WebAuthn device with `tsh mfa add`
      - [ ] macOS/Linux
      - [ ] Windows
    - [ ] Add platform authenticator device with `tsh mfa add`
      - [ ] Touch ID
      - [ ] Windows Hello
    - [ ] List MFA devices with `tsh mfa ls`
    - [ ] Remove an OTP device with `tsh mfa rm`
    - [ ] Remove a WebAuthn device with `tsh mfa rm`
    - [ ] Attempt removing the last MFA device on the user
      - [ ] with `second_factor: on` in `auth_service`, should fail
      - [ ] with `second_factor: optional` in `auth_service`, should succeed
  - [ ] Login Password Only
  - [ ] Login with MFA
    - [ ] Add an OTP, a WebAuthn and a Touch ID/Windows Hello device with `tsh mfa add`
    - [ ] Login via OTP
    - [ ] Login via WebAuthn
      - [ ] macOS/Linux
      - [ ] Windows
    - [ ] Login via platform authenticator
      - [ ] Touch ID
      - [ ] Windows Hello
    - [ ] Login via WebAuthn using an U2F/CTAP1 device

  - [ ] Login OIDC
  - [ ] Login SAML
  - [ ] Login GitHub
  - [ ] Deleting Users

- [ ] Backends
  - [ ] Teleport runs with etcd
  - [ ] Teleport runs with DynamoDB
    - [ ] AWS integration tests are passing
  - [ ] Teleport runs with SQLite
  - [ ] Teleport runs with Firestore
    - [ ] GCP integration tests are passing
  - [ ] Teleport runs with Postgres

- [ ] Session Recording
  - [ ] Session recording can be disabled
  - [ ] Sessions can be recorded at the node
    - [ ] Sessions in remote clusters are recorded in remote clusters
  - [ ] [Sessions can be recorded at the proxy](https://goteleport.com/docs/server-access/guides/recording-proxy-mode/)
    - [ ] Sessions on remote clusters are recorded in the local cluster
    - [ ] With an OpenSSH server without a Teleport CA signed host certificate:
      - [ ] Host key checking enabled rejects connection
      - [ ] Host key checking disabled allows connection

- [ ] Enhanced Session Recording
  - [ ] `disk`, `command` and `network` events are being logged.
  - [ ] Recorded events can be enforced by the `enhanced_recording` role option.
  - [ ] Enhanced session recording can be enabled on CentOS 7 with kernel 5.8+.

- [ ] Auditd
  - [ ] When auditd is enabled, audit events are recorded — https://github.com/gravitational/teleport/blob/7744f72c6eb631791434b648ba41083b5f6d2278/lib/auditd/common.go#L25-L34
    - [ ] SSH session start — user login event
    - [ ] SSH session end
    - [ ] SSH Login failures — SSH auth error
    - [ ] SSH Login failures — unknown OS user
    - [ ] Session ID is correct (only true when Teleport runs as systemd service)
    - [ ] Teleport user is recorded as an auditd event field

- [ ] Audit Log
  - [ ] Audit log with dynamodb
    - [ ] AWS integration tests are passing
  - [ ] Audit log with Firestore
    - [ ] GCP integration tests are passing
  - [ ] Failed login attempts are recorded
  - [ ] Interactive sessions have the correct Server ID
    - [ ] `server_id` is the ID of the node in "session_recording: node" mode
    - [ ] `server_id` is the ID of the node in "session_recording: proxy" mode
    - [ ] `forwarded_by` is the ID of the proxy in "session_recording: proxy" mode

    Node/Proxy ID may be found at `/var/lib/teleport/host_uuid` in the
    corresponding machine.

    Node IDs may also be queried via `tctl nodes ls`.

  - [ ] Exec commands are recorded
  - [ ] `scp` commands are recorded
  - [ ] Subsystem results are recorded

    Subsystem testing may be achieved using both
    [Recording Proxy mode](
    https://goteleport.com/teleport/docs/architecture/proxy/#recording-proxy-mode)
    and
    [OpenSSH integration](
    https://goteleport.com/docs/server-access/guides/openssh/).

    Assuming the proxy is `proxy.example.com:3023` and `node1` is a node running
    OpenSSH/sshd, you may use the following command to trigger a subsystem audit
    log:

    ```shell
    sftp -o "ProxyCommand ssh -o 'ForwardAgent yes' -p 3023 %r@proxy.example.com -s proxy:%h:%p" root@node1
    ```

- [ ] Interact with a cluster using `tsh`

   These commands should ideally be tested for recording and non-recording modes as they are implemented in a different ways.

  - [ ] tsh ssh \<regular-node\>
  - [ ] tsh ssh \<node-remote-cluster\>
  - [ ] tsh ssh \<agentless-node\>
  - [ ] tsh ssh \<agentless-node-remote-cluster\>
  - [ ] tsh ssh -A \<regular-node\>
  - [ ] tsh ssh -A \<node-remote-cluster\>
  - [ ] tsh ssh -A \<agentless-node\>
  - [ ] tsh ssh -A \<agentless-node-remote-cluster\>
  - [ ] tsh ssh \<regular-node\> ls
  - [ ] tsh ssh \<node-remote-cluster\> ls
  - [ ] tsh ssh \<agentless-node\> ls
  - [ ] tsh ssh \<agentless-node-remote-cluster\> ls
  - [ ] tsh join \<regular-node\>
  - [ ] tsh join \<node-remote-cluster\>
  - [ ] tsh play \<regular-node\>
  - [ ] tsh play \<node-remote-cluster\>
  - [ ] tsh play \<agentless-node\>
  - [ ] tsh play \<agentless-node-remote-cluster\>
  - [ ] tsh scp \<regular-node\>
  - [ ] tsh scp \<node-remote-cluster\>
  - [ ] tsh scp \<agentless-node\>
  - [ ] tsh scp \<agentless-node-remote-cluster\>
  - [ ] tsh ssh -L \<regular-node\>
  - [ ] tsh ssh -L \<node-remote-cluster\>
  - [ ] tsh ssh -L \<agentless-node\>
  - [ ] tsh ssh -L \<agentless-node-remote-cluster\>
  - [ ] tsh ssh -R \<regular-node\>
  - [ ] tsh ssh -R \<node-remote-cluster\>
  - [ ] tsh ssh -R \<agentless-node\>
  - [ ] tsh ssh -R \<agentless-node-remote-cluster\>
  - [ ] tsh ls
  - [ ] tsh clusters

- [ ] Interact with a cluster using `ssh`
   Make sure to test both recording and regular proxy modes.
  - [ ] ssh \<regular-node\>
  - [ ] ssh \<node-remote-cluster\>
  - [ ] ssh \<agentless-node\>
  - [ ] ssh \<agentless-node-remote-cluster\>
  - [ ] ssh -A \<regular-node\>
  - [ ] ssh -A \<node-remote-cluster\>
  - [ ] ssh -A \<agentless-node\>
  - [ ] ssh -A \<agentless-node-remote-cluster\>
  - [ ] ssh \<regular-node\> ls
  - [ ] ssh \<node-remote-cluster\> ls
  - [ ] ssh \<agentless-node\> ls
  - [ ] ssh \<agentless-node-remote-cluster\> ls
  - [ ] scp \<regular-node\>
  - [ ] scp \<node-remote-cluster\>
  - [ ] scp \<agentless-node\>
  - [ ] scp \<agentless-node-remote-cluster\>
  - [ ] ssh -L \<regular-node\>
  - [ ] ssh -L \<node-remote-cluster\>
  - [ ] ssh -L \<agentless-node\>
  - [ ] ssh -L \<agentless-node-remote-cluster\>
  - [ ] ssh -R \<regular-node\>
  - [ ] ssh -R \<node-remote-cluster\>
  - [ ] ssh -R \<agentless-node\>
  - [ ] ssh -R \<agentless-node-remote-cluster\>

- [ ] Verify proxy jump functionality
  Log into leaf cluster via root, shut down the root proxy and verify proxy jump works.
  - [ ] tls routing disabled
    - [ ] tsh ssh -J \<leaf.proxy.example.com:3023\>
    - [ ] ssh -J \<leaf.proxy.example.com:3023\>
  - [ ] tls routing enabled
    - [ ] tsh ssh -J \<leaf.proxy.example.com:3080\>
    - [ ] tsh proxy ssh -J \<leaf.proxy.example.com:3080\>

- [ ] Interact with a cluster using the Web UI
  - [ ] Connect to a Teleport node
  - [ ] Connect to a OpenSSH node
  - [ ] Connect to a Agentless node
  - [ ] Check agent forwarding is correct based on role and proxy mode.

- [ ] X11 Forwarding
  - Install `xeyes` and `xclip`:
    - Linux: `apt install x11-apps xclip`
    - Mac: Install and launch [XQuartz](https://www.xquartz.org/) which comes with `xeyes`. Then `brew install xclip`.
  - Enable X11 forwarding for a Node running as root: `ssh_service.x11.enabled = yes`
  - [ ] Successfully X11 forward as both root and non-root user
    - [ ] `tsh ssh -X user@node xeyes`
    - [ ] `tsh ssh -X root@node xeyes`
  - [ ] Test untrusted vs trusted forwarding
    - [ ] `tsh ssh -Y server01 "echo Hello World | xclip -sel c && xclip -sel c -o"` should print "Hello World"
    - [ ] `tsh ssh -X server01 "echo Hello World | xclip -sel c && xclip -sel c -o"` should fail with "BadAccess" X error

### User accounting

- [ ] Verify that active interactive sessions are tracked in `/var/run/utmp` on Linux.
- [ ] Verify that interactive sessions are logged in `/var/log/wtmp` on Linux.

### Combinations

For some manual testing, many combinations need to be tested. For example, for
interactive sessions the 12 combinations are below.

- Add an agentless Node in a local cluster.
  - [ ] Connect using OpenSSH.
  - [ ] Connect using Teleport.
  - [ ] Connect using the Web UI.
  - Remove the Node (but keep its custom CA in sshd config).
    - [ ] Verify that it fails to connect when using OpenSSH.
    - [ ] Verify that it fails to connect when using Teleport.
    - [ ] Verify that it fails to connect when using the Web UI.
- Add a Teleport Node in a local cluster.
  - [ ] Connect using OpenSSH.
  - [ ] Connect using Teleport.
  - [ ] Connect using the Web UI.

- Add an agentless Node in a remote (leaf) cluster.
  - [ ] Connect using OpenSSH from root cluster.
  - [ ] Connect using Teleport from root cluster.
  - [ ] Connect using the Web UI from root cluster.
  - Remove the Node (but keep its custom CA in sshd config).
    - [ ] Verify that it fails to connect when using OpenSSH from root cluster.
    - [ ] Verify that it fails to connect when using Teleport from root cluster.
    - [ ] Verify that it fails to connect when using the Web UI from root cluster.
- Add a Teleport Node in a remote (leaf) cluster.
  - [ ] Connect using OpenSSH from root cluster.
  - [ ] Connect using Teleport from root cluster.
  - [ ] Connect using the Web UI from root cluster.

### Teleport with EKS/GKE

* [ ] Deploy Teleport on a single EKS cluster
* [ ] Deploy Teleport on two EKS clusters and connect them via trusted cluster feature
* [ ] Deploy Teleport Proxy outside GKE cluster fronting connections to it (use [this script](https://github.com/gravitational/teleport/blob/master/examples/k8s-auth/get-kubeconfig.sh) to generate a kubeconfig)
* [ ] Deploy Teleport Proxy outside EKS cluster fronting connections to it (use [this script](https://github.com/gravitational/teleport/blob/master/examples/k8s-auth/get-kubeconfig.sh) to generate a kubeconfig)

### Teleport with multiple Kubernetes clusters

Note: you can use GKE or EKS or minikube to run Kubernetes clusters.
Minikube is the only caveat - it's not reachable publicly so don't run a proxy there.

* [ ] Deploy combo auth/proxy/kubernetes_service outside a Kubernetes cluster, using a kubeconfig
  * [ ] Login with `tsh login`, check that `tsh kube ls` has your cluster
  * [ ] Run `kubectl get nodes`, `kubectl exec -it $SOME_POD -- sh`
  * [ ] Verify that the audit log recorded the above request and session
* [ ] Deploy combo auth/proxy/kubernetes_service inside a Kubernetes cluster
  * [ ] Login with `tsh login`, check that `tsh kube ls` has your cluster
  * [ ] Run `kubectl get nodes`, `kubectl exec -it $SOME_POD -- sh`
  * [ ] Verify that the audit log recorded the above request and session
* [ ] Deploy combo auth/proxy_service outside the Kubernetes cluster and kubernetes_service inside of a Kubernetes cluster, connected over a reverse tunnel
  * [ ] Login with `tsh login`, check that `tsh kube ls` has your cluster
  * [ ] Run `kubectl get nodes`, `kubectl exec -it $SOME_POD -- sh`
  * [ ] Verify that the audit log recorded the above request and session
* [ ] Deploy a second kubernetes_service inside another Kubernetes cluster, connected over a reverse tunnel
  * [ ] Login with `tsh login`, check that `tsh kube ls` has both clusters
  * [ ] Switch to a second cluster using `tsh kube login`
  * [ ] Run `kubectl get nodes`, `kubectl exec -it $SOME_POD -- sh` on the new cluster
  * [ ] Verify that the audit log recorded the above request and session
* [ ] Deploy combo auth/proxy/kubernetes_service outside a Kubernetes cluster, using a kubeconfig with multiple clusters in it
  * [ ] Login with `tsh login`, check that `tsh kube ls` has all clusters
* [ ] Test Kubernetes screen in the web UI (tab is located on left side nav on dashboard):
  * [ ] Verify that all kubes registered are shown with correct `name` and `labels`
  * [ ] Verify that clicking on a rows connect button renders a dialogue on manual instructions with `Step 2` login value matching the rows `name` column
  * [ ] Verify searching for `name` or `labels` in the search bar works
  * [ ] Verify you can sort by `name` colum
* [ ] Test Kubernetes exec via WebSockets - [client](https://github.com/kubernetes-client/javascript/blob/45b68c98e62b6cc4152189b9fd4a27ad32781bc4/examples/typescript/exec/exec-example.ts)

### Kubernetes auto-discovery

* [ ] Test Kubernetes auto-discovery:
  * [ ] Verify that Azure AKS clusters are discovered and enrolled for different Azure Auth configs:
    * [ ] Local Accounts only
    * [ ] Azure AD
    * [ ] Azure RBAC
  * [ ] Verify that AWS EKS clusters are discovered and enrolled
  * [ ] Verify that GCP GKE clusters are discovered and enrolled
* [ ] Verify dynamic registration.
  * [ ] Can register a new Kubernetes cluster using `tctl create`.
  * [ ] Can update registered Kubernetes cluster using `tctl create -f`.
  * [ ] Can delete registered Kubernetes cluster using `tctl rm`.

### Kubernetes Secret Storage

* [ ] Kubernetes Secret storage for Agent's Identity
    * [ ] Install Teleport agent with a short-lived token
      * [ ] Validate if the Teleport is installed as a Kubernetes `Statefulset`
      * [ ] Restart the agent after token TTL expires to see if it reuses the same identity.
    * [ ] Force cluster CA rotation

### Kubernetes Pod RBAC

* [ ] Verify the following scenarios for `kubernetes_resources`:
    * [ ] `{"kind":"pod","name":"*","namespace":"*"}` - must allow access to every pod.
    * [ ] `{"kind":"pod","name":"<somename>","namespace":"*"}` - must allow access to pod `<somename>` in every namespace.
    * [ ] `{"kind":"pod","name":"*","namespace":"<somenamespace>"}` - must allow access to any pod in `<somenamespace>` namespace.
    * [ ] Verify support for  `*` wildcards - `<some-name>-*` and regex for `name` and `namespace` fields.
    * [ ] Verify support for delete pods collection - must use `go-client`.
* [ ] Verify scenarios with multiple roles defining `kubernetes_resources`:
    * [ ] Validate that the returned list of pods is the union of every role.
    * [ ] Validate that access to other pods is denied by RBAC.
    * [ ] Validate that the Kubernetes Groups/Users are correctly selected depending on the role that applies to the pod.
        * [ ] Test with a `kubernetes_groups` that denies exec into a pod
* [ ] Verify the following scenarios for Resource Access Requests to Pods:
    * [ ] Create a valid resource access request and validate if access to other pods is denied.
    * [ ] Validate if creating a resource access request with Kubernetes resources denied by `search_as_roles` is not allowed.

### Teleport with FIPS mode

* [ ] Perform trusted clusters, Web and SSH sanity check with all teleport components deployed in FIPS mode.

### ACME

- [ ] Teleport can fetch TLS certificate automatically using ACME protocol.

### Migrations

* [ ] Migrate trusted clusters from 2.4.0 to 2.5.0
  * [ ] Migrate auth server on main cluster, then rest of the servers on main cluster
        SSH should work for both main and old clusters
  * [ ] Migrate auth server on remote cluster, then rest of the remote cluster
       SSH should work

### Command Templates

When interacting with a cluster, the following command templates are useful:

#### OpenSSH

```
# when connecting to the recording proxy, `-o 'ForwardAgent yes'` is required.
ssh -o "ProxyCommand ssh -o 'ForwardAgent yes' -p 3023 %r@proxy.example.com -s proxy:%h:%p" \
  node.example.com

# the above command only forwards the agent to the proxy, to forward the agent
# to the target node, `-o 'ForwardAgent yes'` needs to be passed twice.
ssh -o "ForwardAgent yes" \
  -o "ProxyCommand ssh -o 'ForwardAgent yes' -p 3023 %r@proxy.example.com -s proxy:%h:%p" \
  node.example.com

# when connecting to a remote cluster using OpenSSH, the subsystem request is
# updated with the name of the remote cluster.
ssh -o "ProxyCommand ssh -o 'ForwardAgent yes' -p 3023 %r@proxy.example.com -s proxy:%h:%p@foo.com" \
  node.foo.com
```

#### Teleport

```
# when connecting to a OpenSSH node, remember `-p 22` needs to be passed.
tsh --proxy=proxy.example.com --user=<username> --insecure ssh -p 22 node.example.com

# an agent can be forwarded to the target node with `-A`
tsh --proxy=proxy.example.com --user=<username> --insecure ssh -A -p 22 node.example.com

# the --cluster flag is used to connect to a node in a remote cluster.
tsh --proxy=proxy.example.com --user=<username> --insecure ssh --cluster=foo.com -p 22 node.foo.com
```


### Teleport with SSO Providers

- [ ] G Suite install instructions work
    - [ ] G Suite Screenshots are up-to-date
- [ ] Azure Active Directory (AD) install instructions work
    - [ ] Azure Active Directory (AD) Screenshots are up-to-date
- [ ] ActiveDirectory (ADFS) install instructions work
    - [ ] Active Directory (ADFS) Screenshots are up-to-date
- [ ] Okta install instructions work
    - [ ] Okta Screenshots are up-to-date
- [ ] OneLogin install instructions work
    - [ ] OneLogin Screenshots are up-to-date
- [ ] GitLab install instructions work
    - [ ] GitLab Screenshots are up-to-date
- [ ] OIDC install instructions work
    - [ ] OIDC Screenshots are up-to-date
- [ ] All providers with guides in docs are covered in this test plan
- [ ] Login Rules work to transform traits from SSO provider
- [ ] SAML IdP guide instructions work
    - [ ] SAML IdP screenshots are up to date

### GitHub External SSO

- [ ] Teleport OSS
    - [ ] GitHub organization without external SSO succeeds
    - [ ] GitHub organization with external SSO fails
- [ ] Teleport Enterprise
    - [ ] GitHub organization without external SSO succeeds
    - [ ] GitHub organization with external SSO succeeds

### `tctl sso` family of commands

For help with setting up sso connectors, check out the [Quick GitHub/SAML/OIDC Setup Tips]

`tctl sso configure` helps to construct a valid connector definition:

- [ ] `tctl sso configure github ...` creates valid connector definitions
- [ ] `tctl sso configure oidc ...` creates valid connector definitions
- [ ] `tctl sso configure saml ...` creates valid connector definitions

`tctl sso test` test a provided connector definition, which can be loaded from
file or piped in with `tctl sso configure` or `tctl get --with-secrets`. Valid
connectors are accepted, invalid are rejected with sensible error messages.

- [ ] Connectors can be tested with `tctl sso test`.
    - [ ] GitHub
    - [ ] SAML
    - [ ] OIDC
        - [ ] Google Workspace
        - [ ] Non-Google IdP

### SSO login on remote host

- [ ] SSO login on a remote host

`tsh` should be running on a remote host (e.g. over an SSH session) and use the
local browser to complete and SSO login. Run
`tsh login --callback <remote.host>:<port> --bind-addr localhost:<port> --auth <auth>`
on the remote host. Note that the `--callback` URL must be able to resolve to the
`--bind-addr` over HTTPS.

### Teleport Plugins

- [ ] Test receiving a message via Teleport Slackbot
- [ ] Test receiving a new Jira Ticket via Teleport Jira

### Teleport Operator

- [ ] Test deploying a Teleport cluster with the `teleport-cluster` Helm chart and the operator enabled
- [ ] Test deploying a standalone operator against Teleport Cloud
- [ ] Test that operator can reconcile
  - [ ] TeleportUser
  - [ ] TeleportRole
  - [ ] TeleportProvisionToken

### AWS Node Joining
[Docs](https://goteleport.com/docs/setup/guides/joining-nodes-aws/)
- [ ] On EC2 instance with `ec2:DescribeInstances` permissions for local account:
  `TELEPORT_TEST_EC2=1 go test ./integration -run TestEC2NodeJoin`
- [ ] On EC2 instance with any attached role:
  `TELEPORT_TEST_EC2=1 go test ./integration -run TestIAMNodeJoin`
- [ ] EC2 Join method in IoT mode with node and auth in different AWS accounts
- [ ] IAM Join method in IoT mode with node and auth in different AWS accounts

### Kubernetes Node Joining
- [ ] Join a Teleport node running in the same Kubernetes cluster via a Kubernetes in-cluster ProvisionToken
- [ ] Join a tbot instance running in a different Kubernetes cluster as Teleport with a Kubernetes JWKS ProvisionToken

### Azure Node Joining
[Docs](https://goteleport.com/docs/agents/join-services-to-your-cluster/azure/)
- [ ] Join a Teleport node running in an Azure VM

### GCP Node Joining
[Docs](https://goteleport.com/docs/agents/join-services-to-your-cluster/gcp/)
- [ ] Join a Teleport node running in a GCP VM.

### Cloud Labels
- [ ] Create an EC2 instance with [tags in instance metadata enabled](https://goteleport.com/docs/management/guides/ec2-tags/)
and with tag `foo`: `bar`. Verify that a node running on the instance has label
`aws/foo=bar`.
- [ ] Create an Azure VM with tag `foo`: `bar`. Verify that a node running on the
instance has label `azure/foo=bar`.

### Passwordless

This feature has additional build requirements, so it should be tested with a
pre-release build from Drone (eg:
`https://get.gravitational.com/tsh-v10.0.0-alpha.2.pkg`).

This sections complements "Users -> Managing MFA devices". `tsh` binaries for
each operating system (Linux, macOS and Windows) must be tested separately for
FIDO2 items.

- [ ] Diagnostics

    Commands should pass all tests.

  - [ ] `tsh fido2 diag` (macOS/Linux)
  - [ ] `tsh touchid diag` (macOS only)
  - [ ] `tsh webauthnwin diag` (Windows only)

- [ ] Registration
  - [ ] Register a passworldess FIDO2 key (`tsh mfa add`, choose WEBAUTHN and
        passwordless)
    - [ ] macOS/Linux
    - [ ] Windows
  - [ ] Register a platform authenticator
    - [ ] Touch ID credential (`tsh mfa add`, choose TOUCHID)
    - [ ] Windows hello credential (`tsh mfa add`, choose WEBAUTHN and
          passwordless)

- [ ] Login
  - [ ] Passwordless login using FIDO2 (`tsh login --auth=passwordless`)
    - [ ] macOS/Linux
    - [ ] Windows
  - [ ] Passwordless login using platform authenticator (`tsh login --auth=passwordless`)
    - [ ] Touch ID
    - [ ] Windows Hello
  - [ ] `tsh login --auth=passwordless --mfa-mode=cross-platform` uses FIDO2
    - [ ] macOS/Linux
    - [ ] Windows
  - [ ] `tsh login --auth=passwordless --mfa-mode=platform` uses platform authenticator
    - [ ] Touch ID
    - [ ] Windows Hello
  - [ ] `tsh login --auth=passwordless --mfa-mode=auto` prefers platform authenticator
    - [ ] Touch ID
    - [ ] Windows Hello
  - [ ] Exercise credential picker (register credentials for multiple users in
        the same device)
    - [ ] FIDO2 macOS/Linux
    - [ ] Touch ID
    - [ ] Windows
  - [ ] Passwordless disable switch works
        (`auth_service.authentication.passwordless = false`)
  - [ ] Cluster in passwordless mode defaults to passwordless
        (`auth_service.authentication.connector_name = passwordless`)
  - [ ] Cluster in passwordless mode allows MFA login
        (`tsh login --auth=local`)

- [ ] Touch ID support commands
  - [ ] `tsh touchid ls` works
  - [ ] `tsh touchid rm` works (careful, may lock you out!)

### Device Trust

Device Trust requires Teleport Enterprise.

This feature has additional build requirements, so it should be tested with a
pre-release build from Drone (eg:
`https://get.gravitational.com/teleport-ent-v10.0.0-alpha.2-linux-amd64-bin.tar.gz`).

Client-side enrollment requires a signed `tsh` for macOS, make sure to use the
`tsh` binary from `tsh.app`.

A simple formula for testing device authorization is:

```shell
# Before enrollment.
# Replace with other kinds of access, as appropriate (db, kube, etc)
tsh ssh node-that-requires-device-trust
> ERROR: ssh: rejected: administratively prohibited (unauthorized device)

# Register the device.
# Get the serial number from `tsh device asset-tag`.
tctl devices add --os=macos --asset-tag=<SERIAL_NUMBER> --enroll

# Enroll the device.
tsh device enroll --token=<TOKEN_FROM_COMMAND_ABOVE>
tsh logout; tsh login

# After enrollment
tsh ssh node-that-requires-device-trust
> $
```

- [ ] Inventory management
  - [ ] Add device (`tctl devices add`)
  - [ ] Add device and create enrollment token (`tctl devices add --enroll`)
  - [ ] List devices (`tctl devices ls`)
  - [ ] Remove device using device ID (`tctl devices rm`)
  - [ ] Remove device using asset tag (`tctl devices rm`)
  - [ ] Create enrollment token using device ID (`tctl devices enroll`)
  - [ ] Create enrollment token using asset tag (`tctl devices enroll`)

- [ ] Device enrollment
  - [ ] Enroll/authn device on macOS (`tsh device enroll`)
  - [ ] Enroll/authn device on Windows (`tsh device enroll`)
  - [ ] Enroll/authn device on Linux (`tsh device enroll`)

    Linux users need read/write permissions to /dev/tpmrm0. The simplest way is
    to assign yourself to the `tss` group. See
    https://goteleport.com/docs/access-controls/device-trust/device-management/#troubleshooting.

  - [ ] Verify device extensions on TLS certificate

    Note that different accesses have different certificates (Database, Kube,
    etc).

    ```shell
    $ openssl x509 -noout -in ~/.tsh/keys/zarquon/llama-x509.pem -nameopt sep_multiline -subject | grep 1.3.9999.3
    > 1.3.9999.3.1=6e60b9fd-1e3e-473d-b148-27b4f158c2a7
    > 1.3.9999.3.2=AAAAAAAAAAAA
    > 1.3.9999.3.3=661c9340-81b0-4a1a-a671-7b1304d28600
    ```

  - [ ] Verify device extensions on SSH certificate

    ```shell
    ssh-keygen -L -f ~/.tsh/keys/zarquon/llama-ssh/zarquon-cert.pub | grep teleport-device-
    teleport-device-asset-tag ...
    teleport-device-credential-id ...
    teleport-device-id ...
    ```

- [ ] Device authorization
  - [ ] device_trust.mode other than "off" or "" not allowed (OSS)
  - [ ] device_trust.mode="off" doesn't impede access (Enterprise and OSS)
  - [ ] device_trust.mode="optional" doesn't impede access, but issues device
        extensions on login
  - [ ] device_trust.mode="required" enforces enrolled devices
  - [ ] device_trust.mode="required" is enforced by processes, and not only by
        Auth APIs

    Testing this requires issuing a certificate without device extensions
    (mode="off"), then changing the cluster configuration to mode="required" and
    attempting to access a process directly, without a login attempt.

  - [ ] Role-based authz enforces enrolled devices
        (device_trust.mode="off" or "optional",
        role.spec.options.device_trust_mode="required")
  - [ ] Device authorization works correctly for both require_session_mfa=false
        and require_session_mfa=true

  - [ ] Device authorization applies to SSH access (all items above)
  - [ ] Device authorization applies to Trusted Clusters (root with
        mode="optional" and leaf with mode="required")
  - [ ] Device authorization applies to Database access (all items above)
  - [ ] Device authorization applies to Kubernetes access (all items above)

  - [ ] Cluster-wide device authorization __does not apply__ to App access
  - [ ] Role-based device authorization __applies__ to App access

  - [ ] Device authorization __does not apply__ to Windows Desktop access
        (both cluster-wide and role)

- [ ] Device audit (see [lib/events/codes.go][device_event_codes])
  - [ ] Inventory management actions issue events (success only)
  - [ ] Device enrollment issues device event (any outcomes)
  - [ ] Device authorization issues device event (any outcomes)
  - [ ] Events with [UserMetadata][event_trusted_device] contain TrustedDevice
        data (for certificates with device extensions)

- [ ] Binary support
  - [ ] Non-signed and/or non-notarized `tsh` for macOS gives a sane error
        message for `tsh device enroll` attempts.

- [ ] Device support commands
  - [ ] `tsh device collect`   (macOS)
  - [ ] `tsh device asset-tag` (macOS)
  - [ ] `tsh device collect`   (Windows)
  - [ ] `tsh device asset-tag` (Windows)
  - [ ] `tsh device collect`   (Linux)
  - [ ] `tsh device asset-tag` (Linux)

[device_event_codes]: https://github.com/gravitational/teleport/blob/473969a700c3c4f981e956fae8a0d14c65c88abe/lib/events/codes.go#L389-L400
[event_trusted_device]: https://github.com/gravitational/teleport/blob/473969a700c3c4f981e956fae8a0d14c65c88abe/api/proto/teleport/legacy/types/events/events.proto#L88-L90

### Hardware Key Support

Hardware Key Support is an Enterprise feature and is not available for OSS.

You will need a YubiKey 4.3+ to test this feature.

This feature has additional build requirements, so it should be tested with a pre-release build from Drone (eg: `https://get.gravitational.com/teleport-ent-v11.0.0-alpha.2-linux-amd64-bin.tar.gz`).

#### Server Access

This test should be carried out on Linux, MacOS, and Windows.

Set `auth_service.authentication.require_session_mfa: hardware_key_touch` in your cluster auth settings and login.
- [ ] `tsh login`
  - [ ] Prompts for Yubikey touch with message "Tap your YubiKey" (separate from normal MFA prompt).
- [ ] Server Access `tsh ssh`
  - [ ] Requires yubikey to be connected
  - [ ] Prompts for touch (if not cached)
- [ ] Database Access: `tsh proxy db --tunnel`
  - [ ] Requires yubikey to be connected
  - [ ] Prompts for touch (if not cached)

### HSM Support

[Docs](https://goteleport.com/docs/choose-an-edition/teleport-enterprise/hsm/)

- [ ] YubiHSM2 Support (@nklaassen has hardware)
  - [ ] Make sure docs/links are up to date
  - [ ] New cluster with YubiHSM2 CA works
  - [ ] Migrating a software cluster to YubiHSM2 works
  - [ ] CA rotation works
- [ ] AWS CloudHSM Support
  - [ ] Make sure docs/links are up to date
  - [ ] New cluster with CloudHSM CA works
  - [ ] Migrating a software cluster to CloudHSM works
  - [ ] CA rotation works
- [ ] GCP KMS Support
  - [ ] Make sure docs/links are up to date
  - [ ] New cluster with GCP KMS CA works
  - [ ] Migrating a software cluster to GCP KMS works
  - [ ] CA rotation works

Run the full test suite with each HSM/KMS:

```shell
$ make run-etcd # in background shell
$
$ # test YubiHSM
$ yubihsm-connector -d # in a background shell
$ cat /etc/yubihsm_pkcs11.conf
# /etc/yubihsm_pkcs11.conf
connector = http://127.0.0.1:12345
debug
$ TELEPORT_TEST_YUBIHSM_PKCS11_PATH=/usr/local/lib/pkcs11/yubihsm_pkcs11.dylib TELEPORT_TEST_YUBIHSM_PIN=0001password YUBIHSM_PKCS11_CONF=/etc/yubihsm_pkcs11.conf go test ./lib/auth/keystore -v --count 1
$ TELEPORT_TEST_YUBIHSM_PKCS11_PATH=/usr/local/lib/pkcs11/yubihsm_pkcs11.dylib TELEPORT_TEST_YUBIHSM_PIN=0001password YUBIHSM_PKCS11_CONF=/etc/yubihsm_pkcs11.conf TELEPORT_ETCD_TEST=1 go test ./integration/hsm -v --count 1 --timeout 20m # this takes ~12 minutes
$
$ # test AWS KMS
$ # login in to AWS locally
$ AWS_ACCOUNT="$(aws sts get-caller-identity | jq -r '.Account')"
$ TELEPORT_TEST_AWS_KMS_ACCOUNT="${AWS_ACCOUNT}" TELEPORT_TEST_AWS_REGION=us-west-2 go test ./lib/auth/keystore -v --count 1
$ TELEPORT_TEST_AWS_KMS_ACCOUNT="${AWS_ACCOUNT}" TELEPORT_TEST_AWS_REGION=us-west-2 TELEPORT_ETCD_TEST=1 go test ./integration/hsm -v --count 1
$
$ # test AWS CloudHSM
$ # set up the CloudHSM cluster and run this on an EC2 that can reach it
$ TELEPORT_TEST_CLOUDHSM_PIN="<CU_username>:<CU_password>" go test ./lib/auth/keystore -v --count 1
$ TELEPORT_TEST_CLOUDHSM_PIN="<CU_username>:<CU_password>" TELEPORT_ETCD_TEST=1 go test ./integration/hsm -v --count 1
$
$ # test GCP KMS
$ # login in to GCP locally
$ TELEPORT_TEST_GCP_KMS_KEYRING=projects/<account>/locations/us-west3/keyRings/<keyring> go test ./lib/auth/keystore -v --count 1
$ TELEPORT_TEST_GCP_KMS_KEYRING=projects/<account>/locations/us-west3/keyRings/<keyring> TELEPORT_ETCD_TEST=1 go test ./integration/hsm -v --count 1
```

## Moderated session

Using `tsh` join an SSH session as two moderators (two separate terminals, role requires one moderator).
 - [ ] `Ctrl+C` in the #1 terminal should disconnect the moderator.
 - [ ] `Ctrl+C` in the #2 terminal should disconnect the moderator and terminate the session as session has no moderator.

Using `tsh` join an SSH session as two moderators (two separate terminals, role requires one moderator).
- [ ] `t` in any terminal should terminate the session for all participants.

## Performance

### Scaling Test
Scale up the number of nodes/clusters a few times for each configuration below.

 1) Verify that there are no memory/goroutine/file descriptor leaks
 2) Compare the baseline metrics with the previous release to determine if resource usage has increased
 3) Restart all Auth instances and verify that all nodes/clusters reconnect

 Perform reverse tunnel node scaling tests for all backend configurations:
  - [ ] etcd - 10k
  - [ ] DynamoDB - 10k
  - [ ] Firestore - 10k
  - [ ] Postgres - 10k

  Perform the following additional scaling tests on DynamoDB:
 - [ ] 10k direct dial nodes.
 - [ ] 500 trusted clusters.

### Soak Test

Run 30 minute soak test directly against direct and tunnel nodes
and via label based matching. Tests should be run against a Cloud
tenant.

```shell
tsh bench ssh --duration=30m user@direct-dial-node ls
tsh bench ssh --duration=30m user@reverse-tunnel-node ls
tsh bench ssh --duration=30m user@foo=bar ls
tsh bench ssh --duration=30m --random user@foo ls
```

### Concurrent Session Test

* Cluster with 1k reverse tunnel nodes

Run a concurrent session test that will spawn 5 interactive sessions per node in the cluster:

```shell
tsh bench web sessions --max=5000 user ls
tsh bench web sessions --max=5000 --web user ls
```

- [ ] Verify that all 5000 sessions are able to be established.
- [ ] Verify that tsh and the web UI are still functional.

### Robustness

* Connectivity Issues:

- [ ] Verify that a lack of connectivity to Auth does not prevent access to
  resources which do not require a moderated session and in async recording
  mode from an already issued certificate.
- [ ] Verify that a lack of connectivity to Auth prevents access to resources
  which require a moderated session and in async recording mode from an already
  issued certificate.
- [ ] Verify that an open session is not terminated when all Auth instances
  are restarted.

## Teleport with Cloud Providers

### AWS

- [ ] Deploy Teleport to AWS. Using DynamoDB & S3
- [ ] Deploy Teleport Enterprise to AWS. Using HA Setup https://goteleport.com/docs/deploy-a-cluster/deployments/aws-ha-autoscale-cluster-terraform/

### GCP

- [ ] Deploy Teleport to GCP. Using Cloud Firestore & Cloud Storage
- [ ] Deploy Teleport to GKE. Google Kubernetes engine.
- [ ] Deploy Teleport Enterprise to GCP.

### IBM

- [ ] Deploy Teleport to IBM Cloud. Using IBM Database for etcd & IBM Object Store
- [ ] Deploy Teleport to IBM Cloud Kubernetes.
- [ ] Deploy Teleport Enterprise to IBM Cloud.

## Application Access

- [ ] Run an application within local cluster.
  - [ ] Verify the debug application `debug_app: true` works.
  - [ ] Verify an application can be configured with command line flags.
  - [ ] Verify an application can be configured from file configuration.
  - [ ] Verify that applications are available at auto-generated addresses `name.rootProxyPublicAddr` and well as `publicAddr`.
- [ ] Run an application within a trusted cluster.
  - [ ] Verify that applications are available at auto-generated addresses `name.rootProxyPublicAddr`.
- [ ] Verify Audit Records.
  - [ ] `app.session.start` and `app.session.chunk` events are created in the Audit Log.
  - [ ] `app.session.chunk` points to a 5 minute session archive with multiple `app.session.request` events inside.
  - [ ] `tsh play <chunk-id>` can fetch and print a session chunk archive.
- [ ] Verify JWT using [verify-jwt.go](https://github.com/gravitational/teleport/blob/master/examples/jwt/verify-jwt.go).
- [ ] Verify RBAC.
- [ ] Verify [CLI access](https://goteleport.com/docs/application-access/guides/api-access/) with `tsh apps login`.
- [ ] Verify [AWS console access](https://goteleport.com/docs/application-access/cloud-apis/aws-console/).
  - [ ] Can log into AWS web console through the web UI.
  - [ ] Can interact with AWS using `tsh` commands.
    - [ ] `tsh aws`
    - [ ] `tsh aws --endpoint-url` (this is a hidden flag)
- [ ] Verify [Azure CLI access](https://goteleport.com/docs/application-access/cloud-apis/azure/) with `tsh apps login`.
  - [ ] Can interact with Azure using `tsh az` commands.
  - [ ] Can interact with Azure using a combination of `tsh proxy az` and `az` commands.
- [ ] Verify [GCP CLI access](https://goteleport.com/docs/application-access/cloud-apis/google-cloud/) with `tsh apps login`.
  - [ ] Can interact with GCP using `tsh gcloud` commands.
  - [ ] Can interact with Google Cloud Storage using `tsh gsutil` commands.
  - [ ] Can interact with GCP/GCS using a combination of `tsh proxy gcloud` and `gcloud`/`gsutil` commands.
- [ ] Verify dynamic registration.
  - [ ] Can register a new app using `tctl create`.
  - [ ] Can update registered app using `tctl create -f`.
  - [ ] Can delete registered app using `tctl rm`.
- [ ] Test Applications screen in the web UI (tab is located on left side nav on dashboard):
  - [ ] Verify that all apps registered are shown
  - [ ] Verify that clicking on the app icon takes you to another tab
  - [ ] Verify `Add Application` links to documentation.

## Database Access

- [ ] Connect to a database within a local cluster.
  - [ ] Self-hosted Postgres.
    - [ ] verify that cancelling a Postgres request works. (`select pg_sleep(10)` followed by ctrl-c is a good query to test.)
  - [ ] Self-hosted MySQL.
  - [ ] Self-hosted MariaDB.
  - [ ] Self-hosted MongoDB.
  - [ ] Self-hosted CockroachDB.
  - [ ] Self-hosted Redis.
  - [ ] Self-hosted Redis Cluster.
  - [ ] Self-hosted MSSQL.
  - [ ] Self-hosted MSSQL with PKINIT authentication.
  - [ ] AWS Aurora Postgres.
  - [ ] AWS Aurora MySQL.
  - [ ] AWS RDS Proxy (MySQL, Postgres, MariaDB, or SQL Server)
  - [ ] AWS Redshift.
  - [ ] AWS Redshift Serverless.
    - [ ] Verify connection to external AWS account works with `assume_role_arn: ""` and `external_id: "<id>"`
  - [ ] AWS ElastiCache.
  - [ ] AWS MemoryDB.
  - [ ] GCP Cloud SQL Postgres.
  - [ ] GCP Cloud SQL MySQL.
  - [ ] Snowflake.
  - [ ] Azure Cache for Redis.
  - [ ] Azure single-server MySQL and Postgres (EOL Sep 2024 and Mar 2025, use CLI to create)
  - [ ] Azure flexible-server MySQL and Postgres
  - [ ] Elasticsearch.
  - [ ] OpenSearch.
  - [ ] Cassandra/ScyllaDB.
    - [ ] Verify connection to external AWS account works with `assume_role_arn: ""` and `external_id: "<id>"`
  - [ ] Dynamodb.
    - [ ] Verify connection to external AWS account works with `assume_role_arn: ""` and `external_id: "<id>"`
  - [ ] Azure SQL Server.
  - [ ] Oracle.
  - [ ] ClickHouse.
- [ ] Connect to a database within a remote cluster via a trusted cluster.
  - [ ] Self-hosted Postgres.
  - [ ] Self-hosted MySQL.
  - [ ] Self-hosted MariaDB.
  - [ ] Self-hosted MongoDB.
  - [ ] Self-hosted CockroachDB.
  - [ ] Self-hosted Redis.
  - [ ] Self-hosted Redis Cluster.
  - [ ] Self-hosted MSSQL.
  - [ ] Self-hosted MSSQL with PKINIT authentication.
  - [ ] AWS Aurora Postgres.
  - [ ] AWS Aurora MySQL.
  - [ ] AWS RDS Proxy (MySQL, Postgres, MariaDB, or SQL Server)
  - [ ] AWS Redshift.
  - [ ] AWS Redshift Serverless.
  - [ ] AWS ElastiCache.
  - [ ] AWS MemoryDB.
  - [ ] GCP Cloud SQL Postgres.
  - [ ] GCP Cloud SQL MySQL.
  - [ ] Snowflake.
  - [ ] Azure Cache for Redis.
  - [ ] Azure single-server MySQL and Postgres
  - [ ] Azure flexible-server MySQL and Postgres
  - [ ] Elasticsearch.
  - [ ] OpenSearch.
  - [ ] Cassandra/ScyllaDB.
  - [ ] Dynamodb.
  - [ ] Azure SQL Server.
  - [ ] Oracle.
  - [ ] ClickHouse.
- [ ] Verify auto user provisioning.
  Verify all supported modes: `keep`, `best_effort_drop`
  - [ ] Self-hosted Postgres.
  - [ ] Self-hosted MySQL.
  - [ ] Self-hosted MariaDB.
  - [ ] Self-hosted MongoDB.
  - [ ] AWS RDS Postgres.
  - [ ] AWS RDS MySQL.
  - [ ] AWS RDS MariaDB.
- [ ] Verify audit events.
  - [ ] `db.session.start` is emitted when you connect.
  - [ ] `db.session.end` is emitted when you disconnect.
  - [ ] `db.session.query` is emitted when you execute a SQL query.
- [ ] Verify RBAC.
  - [ ] `tsh db ls` shows only databases matching role's `db_labels`.
  - [ ] Can only connect as users from `db_users`.
  - [ ] Can only connect as Teleport username, for auto-user-provisioning-enabled databases.
  - [ ] _(Postgres only)_ Can only connect to databases from `db_names`.
    - [ ] `db.session.start` is emitted when connection attempt is denied.
  - [ ] _(MongoDB only)_ Can only execute commands in databases from `db_names`.
    - [ ] `db.session.query` is emitted when command fails due to permissions.
  - [ ] Can configure per-session MFA.
    - [ ] MFA tap is required on each `tsh db connect`.
- [ ] Verify dynamic registration.
  - [ ] Can register a new database using `tctl create`.
  - [ ] Can update registered database using `tctl create -f`.
  - [ ] Can delete registered database using `tctl rm`.
  - [ ] Can register a database using Teleport's terraform provider.
- [ ] Verify discovery.
  Please configure discovery in Discovery Service instead of Database Service.
    - [ ] AWS
      - [ ] Can detect and register RDS instances.
        - [ ] Can detect and register RDS instances in an external AWS account when `assume_role_arn` and `external_id` is set.
      - [ ] Can detect and register RDS proxies, and their custom endpoints.
      - [ ] Can detect and register Aurora clusters, and their reader and custom endpoints.
      - [ ] Can detect and register RDS proxies, and their custom endpoints.
      - [ ] Can detect and register Redshift clusters.
      - [ ] Can detect and register Redshift serverless workgroups, and their VPC endpoints.
      - [ ] Can detect and register ElastiCache Redis clusters.
      - [ ] Can detect and register MemoryDB clusters.
      - [ ] Can detect and register OpenSearch domains.
    - [ ] Azure
      - [ ] Can detect and register MySQL and Postgres single-server instances.
      - [ ] Can detect and register MySQL and Postgres flexible-server instances.
      - [ ] Can detect and register Azure Cache for Redis servers.
      - [ ] Can detect and register Azure SQL Servers and Azure SQL Managed Instances.
- [ ] Verify Teleport managed users (password rotation, auto 'auth' on connection, etc.).
  - [ ] Can detect and manage ElastiCache users
  - [ ] Can detect and manage MemoryDB users
- [ ] Test Databases screen in the web UI (filter by "Database" type in unified view):
  - [ ] Verify that all dbs registered are shown with correct `name`, `description`, `type`, and `labels`
  - [ ] Verify that clicking on a rows connect button renders a dialogue on manual instructions with `Step 2` login value matching the rows `name` column
  - [ ] Verify searching for all columns in the search bar works
  - [ ] Verify you can sort by all columns except `labels`
- [ ] Other
  - [ ] MySQL server version reported by Teleport is correct.

## TLS Routing

- [ ] Verify that teleport proxy `v2` configuration starts only a single listener for proxy service, in contrast with `v1` configuration.
  Given configuration:
  ```
  version: v2
  proxy_service:
    enabled: "yes"
    public_addr: ['root.example.com']
    web_listen_addr: 0.0.0.0:3080
  ```
  There should be total of three listeners, with only `*:3080` for proxy service. Given the configuration above, 3022 and 3025 will be opened for other services.
  ```
  lsof -i -P | grep teleport | grep LISTEN
    teleport  ...  TCP *:3022 (LISTEN)
    teleport  ...  TCP *:3025 (LISTEN)
    teleport  ...  TCP *:3080 (LISTEN) # <-- proxy service
  ```
  In contrast for the same configuration with version `v1`, there should be additional ports 3023 and 3024.
  ```
  lsof -i -P | grep teleport | grep LISTEN
    teleport  ...  TCP *:3022 (LISTEN)
    teleport  ...  TCP *:3025 (LISTEN)
    teleport  ...  TCP *:3023 (LISTEN) # <-- extra proxy service port
    teleport  ...  TCP *:3024 (LISTEN) # <-- extra proxy service port
    teleport  ...  TCP *:3080 (LISTEN) # <-- proxy service
  ```
- [ ] Run Teleport Proxy in `multiplex` mode `auth_service.proxy_listener_mode: "multiplex"`
  - [ ] Trusted cluster
    - [ ] Setup trusted clusters using single port setup `web_proxy_addr == tunnel_addr`
    ```
    kind: trusted_cluster
    spec:
      ...
      web_proxy_addr: root.example.com:443
      tunnel_addr: root.example.com:443
      ...
    ```
- [ ] Database Access
  - [ ] Verify that `tsh db connect` works through proxy running in `multiplex` mode
    - [ ] Postgres
    - [ ] MySQL
    - [ ] MariaDB
    - [ ] MongoDB
    - [ ] CockroachDB
    - [ ] Redis
    - [ ] MSSQL
    - [ ] Snowflake
    - [ ] Elasticsearch.
    - [ ] OpenSearch.
    - [ ] Cassandra/ScyllaDB.
    - [ ] Oracle.
  - [ ] Verify connecting to a database through TLS ALPN SNI local proxy `tsh proxy db` with a GUI client.
  - [ ] Verify connecting to a database through Teleport Connect.
- [ ] Application Access
  - [ ] Verify app access through proxy running in `multiplex` mode
- [ ] SSH Access
  - [ ] Connect to a OpenSSH server through a local ssh proxy `ssh -o "ForwardAgent yes" -o "ProxyCommand tsh proxy ssh" user@host.example.com`
  - [ ] Connect to a OpenSSH server on leaf-cluster through a local ssh proxy`ssh -o "ForwardAgent yes" -o "ProxyCommand tsh proxy ssh --user=%r --cluster=leaf-cluster %h:%p" user@node.foo.com`
  - [ ] Verify `tsh ssh` access through proxy running in multiplex mode
- [ ] Kubernetes access:
  - [ ] Verify kubernetes access through proxy running in `multiplex` mode, using `tsh`
  - [ ] Verify kubernetes access through Teleport Connect
- [ ] Teleport Proxy single port `multiplex` mode behind L7 load balancer
  - [ ] Agent can join through Proxy and maintain reverse tunnel
  - [ ] `tsh login` and `tctl`
  - [ ] SSH Access: `tsh ssh` and `tsh config`
  - [ ] Database Access: `tsh proxy db` and `tsh db connect`
  - [ ] Application Access: `tsh proxy app` and `tsh aws`
  - [ ] Kubernetes Access: `tsh proxy kube`

## Desktop Access

- Direct mode (set `listen_addr`):
  - [ ] Can connect to AD desktop defined in static `hosts` section.
  - [ ] Can connect to AD desktop defined in static `static_hosts` section.
  - [ ] Can connect to non-AD desktop defined in static `static_hosts` section.
  - [ ] Can connect to non-AD desktop defined in static `non_ad_hosts` section.
  - [ ] Can connect to desktop discovered via LDAP
- IoT mode (reverse tunnel through proxy):
  - [ ] Can connect to AD desktop defined in static `hosts` section.
  - [ ] Can connect to AD desktop defined in static `static_hosts` section.
  - [ ] Can connect to non-AD desktop defined in static `static_hosts` section.
  - [ ] Can connect to non-AD desktop defined in static `non_ad_hosts` section.
  - [ ] Can connect to desktop discovered via LDAP
- [ ] Connect multiple `windows_desktop_service`s to the same Teleport cluster,
  verify that connections to desktops on different AD domains works. (Attempt to
  connect several times to verify that you are routed to the correct
  `windows_desktop_service`)
- Verify user input
  - [ ] Download [Keyboard Key Info](https://dennisbabkin.com/kbdkeyinfo/) and
    verify all keys are processed correctly in each supported browser. Known
    issues: F11 cannot be captured by the browser without
    [special configuration](https://social.technet.microsoft.com/Forums/en-US/784b2bbe-353f-412e-ac9a-193d81f306b6/remote-desktop-for-mac-f11-key-not-working-on-macbook-pro-touchbar?forum=winRDc)
    on MacOS.
  - [ ] Left click and right click register as Windows clicks. (Right click on
    the desktop should show a Windows menu, not a browser context menu)
  - [ ] Vertical and horizontal scroll work.
    [Horizontal Scroll Test](https://codepen.io/jaemskyle/pen/inbmB)
- [Locking](https://goteleport.com/docs/access-controls/guides/locking/#step-12-create-a-lock)
  - [ ] Verify that placing a user lock terminates an active desktop session.
  - [ ] Verify that placing a desktop lock terminates an active desktop session.
  - [ ] Verify that placing a role lock terminates an active desktop session.
- Labeling
  - [ ] Set `client_idle_timeout` to a small value and verify that idle sessions
    are terminated (the session should end and an audit event will confirm it
    was due to idle connection)
  - [ ] All desktops have `teleport.dev/origin` label.
  - [ ] Dynamic desktops have additional `teleport.dev` labels for OS, OS
    Version, DNS hostname.
  - [ ] Regexp-based host labeling applies across all desktops, regardless of
    origin.
  - [ ] Labels from `static_hosts` are applied to correct desktops
- RBAC
  - [ ] RBAC denies access to a Windows desktop due to labels
  - [ ] RBAC denies access to a Windows desktop with the wrong OS-login.
- Clipboard Support
  - When a user has a role with clipboard sharing enabled and is using a chromium based browser
    - [ ] Going to a desktop when clipboard permissions are in "Ask" mode (aka "prompt") causes the browser to show a prompt when you first click or press a key
    - [ ] The clipboard icon is highlighted in the top bar
    - [ ] After allowing clipboard permission, copy text from local workstation, paste into remote desktop
    - [ ] After allowing clipboard permission, copy text from remote desktop, paste into local workstation
    - [ ] After disallowing clipboard permission, confirm copying text from local workstation and pasting into remote desktop doesn't work
    - [ ] After disallowing clipboard permission, confirm copying text from remote desktop and pasting into local workstation doesn't work
  - When a user has a role with clipboard sharing enabled and is *not* using a chromium based browser
    - [ ] The clipboard icon is not highlighted in the top bar and copy/paste does not work
  - When a user has a role with clipboard sharing *disabled* and is using a chromium and non-chromium based browser (confirm both)
    - [ ] The clipboard icon is not highlighted in the top bar and copy/paste does not work
- Directory Sharing
  - On supported, non-chromium based browsers (Firefox/Safari)
    - [ ] Attempting to share directory logs a sensible warning in the warning dropdown
  - On supported, chromium based browsers (Chrome/Edge)
    - Begin sharing works
      - [ ] The shared directory icon in the top right of the screen is highlighted when directory sharing is initiated
      - [ ] The shared directory appears as a network drive named "<directory_name> on teleport"
      - [ ] The share directory menu option disappears from the menu
    - Navigation
      - [ ] The folders of the shared directory are navigable (move up and down the directory tree)
    - CRUD
      - [ ] A new text file can be created
      - [ ] The text file can be written to (saved)
      - [ ] The text file can be read (close it, check that it's saved on the local machine, then open it again on the remote)
      - [ ] The text file can be deleted
    - File/Folder movement
      - In to out (make at least one of these from a non-top-level-directory)
        - [ ] A file from inside the shared directory can be drag-and-dropped outside the shared directory
        - [ ] A folder from inside the shared directory can be drag-and-dropped outside the shared directory (and its contents retained)
        - [ ] A file from inside the shared directory can be cut-pasted outside the shared directory
        - [ ] A folder from inside the shared directory can be cut-pasted outside the shared directory
        - [ ] A file from inside the shared directory can be copy-pasted outside the shared directory
        - [ ] A folder from inside the shared directory can be copy-pasted outside the shared directory
      - Out to in (make at least one of these overwrite an existing file, and one go into a non-top-level directory)
        - [ ] A file from outside the shared directory can be drag-and-dropped into the shared directory
        - [ ] A folder from outside the shared directory can be drag-and-dropped into the shared directory (and its contents retained)
        - [ ] A file from outside the shared directory can be cut-pasted into the shared directory
        - [ ] A folder from outside the shared directory can be cut-pasted into the shared directory
        - [ ] A file from outside the shared directory can be copy-pasted into the shared directory
        - [ ] A folder from outside the shared directory can be copy-pasted into the shared directory
      - Within
        - [ ] A file from inside the shared directory cannot be drag-and-dropped to another folder inside the shared directory: a dismissible "Unsupported Action" dialog is shown
        - [ ] A folder from inside the shared directory cannot be drag-and-dropped to another folder inside the shared directory: a dismissible "Unsupported Action" dialog is shown
        - [ ] A file from inside the shared directory cannot be cut-pasted to another folder inside the shared directory: a dismissible "Unsupported Action" dialog is shown
        - [ ] A folder from inside the shared directory cannot be cut-pasted to another folder inside the shared directory: a dismissible "Unsupported Action" dialog is shown
        - [ ] A file from inside the shared directory can be copy-pasted to another folder inside the shared directory
        - [ ] A folder from inside the shared directory can be copy-pasted to another folder inside shared directory (and its contents retained)
  - RBAC
    - [ ] Give the user one role that explicitly disables directory sharing (`desktop_directory_sharing: false`) and confirm that the option to share a directory doesn't appear in the menu
- Per-Session MFA
  - [ ] Attempting to start a session no keys registered shows an error message
  - [ ] Attempting to start a session with a webauthn registered pops up the "Verify Your Identity" dialog
    - [ ] Hitting "Cancel" shows an error message
    - [ ] Hitting "Verify" causes your browser to prompt you for MFA
    - [ ] Cancelling that browser MFA prompt shows an error
    - [ ] Successful MFA verification allows you to connect
- Session Recording
  - [ ] Verify sessions are not recorded if *all* of a user's roles disable recording
  - [ ] Verify sync recording (`mode: node-sync` or `mode: proxy-sync`)
  - [ ] Verify async recording (`mode: node` or `mode: proxy`)
  - [ ] Sessions show up in session recordings UI with desktop icon
  - [ ] Sessions can be played back, including play/pause functionality
  - [ ] Sessions playback speed can be toggled while its playing
  - [ ] Sessions playback speed can be toggled while its paused
  - [ ] A session that ends with a TDP error message can be played back, ends by displaying the error message,
        and the progress bar progresses to the end.
  - [ ] Attempting to play back a session that doesn't exist (i.e. by entering a non-existing session id in the url) shows
        a relevant error message.
  - [ ] RBAC for sessions: ensure users can only see their own recordings when
    using the RBAC rule from our
    [docs](https://goteleport.com/docs/access-controls/reference/#rbac-for-sessions)
- Audit Events (check these after performing the above tests)
  - [ ] `windows.desktop.session.start` (`TDP00I`) emitted on start
  - [ ] `windows.desktop.session.start` (`TDP00W`) emitted when session fails to
    start (due to RBAC, for example)
  - [ ] `client.disconnect` (`T3006I`) emitted when session is terminated by or fails
    to start due to lock
  - [ ] `windows.desktop.session.end` (`TDP01I`) emitted on end
  - [ ] `desktop.clipboard.send` (`TDP02I`) emitted for local copy -> remote
    paste
  - [ ] `desktop.clipboard.receive` (`TDP03I`) emitted for remote copy -> local
    paste
  - [ ] `desktop.directory.share` (`TDP04I`) emitted when Teleport starts sharing a directory
  - [ ] `desktop.directory.read` (`TDP05I`) emitted when a file is read over the shared directory
  - [ ] `desktop.directory.write` (`TDP06I`) emitted when a file is written to over the shared directory
- Warnings/Errors (test by applying [this patch](https://gist.github.com/ibeckermayer/7591333275e87ad0d7afa028a7bb54cb))
  - [ ] Induce the backend to send a TDP Notification of severity warning (1), confirm that a warning is logged in the warning dropdown
  - [ ] Induce the backend to send a TDP Notification of severity error (2), confirm that session is terminated and error popup is shown
  - [ ] Induce the backend to send a TDP Error, confirm that session is terminated and error popup is shown (confirms backwards compatibility w/ older w_d_s starting in Teleport 12)
- Trusted Cluster / Tunneling
  - Set up Teleport in a trusted cluster configuration where the root and leaf cluster has a w_d_s connected via tunnel (w_d_s running as a separate process)
    - [ ] Confirm that windows desktop sessions can be made on root cluster
    - [ ] Confirm that windows desktop sessions can be made on leaf cluster
- Screen size
    - [ ] Desktops that specify a fixed `screen_size` in their spec always use the same screen size.
    - [ ] Desktops sessions for desktops which specify a fixed `screen_size` do not resize automatically.
    - [ ] Attempting to register a desktop with a `screen_size` dimension larger than 8192 fails.
- Non-AD setup
  - [ ] Installer in GUI mode finishes successfully on instance that is not part of domain
  - [ ] Installer works correctly invoked from command line
  - [ ] Non-AD instance can be added to `non_ad_hosts` section in config file and is visible in UI
  - [ ] Non-AD can be added as dynamic resource and is visible in UI
  - [ ] Non-AD instance has label `teleport.dev/ad: false`
  - [ ] Connecting to non-AD instance works with OSS if there are no more than 5 non-AD desktops
  - [ ] Connecting to non-AD instance fails with OSS if there are more than 5 non-AD desktops
  - [ ] Connecting to non-AD instance works with Enterprise license always
  - [ ] In OSS version, if there are more than 5 non-AD desktops banner shows up telling you to upgrade
  - [ ] Banner goes away if you reduce number of non-AD desktops to less or equal 5
  - [ ] Installer in GUI mode successfully uninstalls Authentication Package (logging in is not possible)
  - [ ] Installer successfully uninstalls Authentication Package (logging in is not possible) when invoked from command line

## Binaries / OS compatibility

Verify that our software runs on the minimum supported OS versions as per
https://goteleport.com/docs/installation/#operating-system-support

### Windows

- [ ] `tsh` runs on the minimum supported Windows version
- [ ] Teleport Connect runs on the minimum supported Windows version

### macOS

- [ ] `tsh` runs on the minimum supported macOS version
- [ ] `tctl` runs on the minimum supported macOS version
- [ ] `teleport` runs on the minimum supported macOS version
- [ ] `tbot` runs on the minimum supported macOS version
- [ ] Teleport Connect runs on the minimum supported macOS version

### Linux

- [ ] `tsh` runs on the minimum supported Linux version
- [ ] `tctl` runs on the minimum supported Linux version
- [ ] `teleport` runs on the minimum supported Linux version
- [ ] `tbot` runs on the minimum supported Linux version
- [ ] Teleport Connect runs on the minimum supported Linux version

## Machine ID

- [ ] Verify you are able to create a new bot user with `tctl bots add robot --roles=access`. Follow the instructions provided in the output to start `tbot`
  - [ ] Directly connecting to the auth server
  - [ ] Connecting to the auth server via the proxy reverse tunnel
- [ ] Verify that after the renewal period (default 20m, but this can be reduced via configuration), that newly generated certificates are placed in the destination directory
- [ ] Verify that sending both `SIGUSR1` and `SIGHUP` to a running tbot process causes a renewal and new certificates to be generated

With an SSH node registered to the Teleport cluster:

- [ ] Verify you are able to connect to the SSH node using openssh with the generated `ssh_config` in the destination directory
- [ ] Verify you are able to connect to the SSH node using `tsh` with the identity file in the destination directory

With a Postgres DB registered to the Teleport cluster:

- [ ] Verify you are able to interact with a database using `tbot db connect` with a database output
- [ ] Verify you are able to connect to the database using `tbot proxy db` with a database output
- [ ] Verify you are able to produce an authenticated tunnel using `tbot proxy db --tunnel` with a database output and then able to connect to the database through the tunnel without credentials

With a Kubernetes cluster registered to the Teleport cluster:

- [ ] Verify the `kubeconfig` produced by a Kubernetes output can be used to run basic commands (e.g `kubectl get pods`)

With a HTTP application registered to the Teleport cluster:

- [ ] Verify the certificates produced by an application output can be used directly against the proxy (e.g `curl --cert ./out/tlscert --key ./out/key https://httpbin.teleport.example.com/headers`)
- [ ] Verify you are able to produce an authenticated tunnel using `tbot proxy app httpbin` with an application output and then able to connect to the application through the tunnel without credentials `curl localhost:port/headers`

## Host users creation

[Host users creation docs](https://github.com/gravitational/teleport/pull/13056)
[Host users creation RFD](https://github.com/gravitational/teleport/pull/11077)
<!---
TODO(lxea): replace links with actual docs once merged

[Host users creation docs](../../docs/pages/server-access/guides/host-user-creation.mdx)
[Host users creation RFD](../../rfd/0057-automatic-user-provisioning.md)
-->

- Verify host users creation functionality
  - [ ] non-existing users are created automatically
  - [ ] users are added to groups
    - [ ] non-existing configured groups are created
	- [ ] created users are added to the `teleport-system` group
  - [ ] users are cleaned up after their session ends
	- [ ] cleanup occurs if a program was left running after session ends
  - [ ] sudoers file creation is successful
	- [ ] Invalid sudoers files are _not_ created
  - [ ] existing host users are not modified
  - [ ] setting `disable_create_host_user: true` stops user creation from occurring

## CA rotations

- Verify the CA rotation functionality itself (by checking in the backend or with `tctl get cert_authority`)
  - [ ] `standby` phase: only `active_keys`, no `additional_trusted_keys`
  - [ ] `init` phase: `active_keys` and `additional_trusted_keys`
  - [ ] `update_clients` and `update_servers` phases: the certs from the `init` phase are swapped
  - [ ] `standby` phase: only the new certs remain in `active_keys`, nothing in `additional_trusted_keys`
  - [ ] `rollback` phase (second pass, after completing a regular rotation): same content as in the `init` phase
  - [ ] `standby` phase after `rollback`: same content as in the previous `standby` phase
- Verify functionality in all phases (clients might have to log in again in lieu of waiting for credentials to expire between phases)
  - [ ] SSH session in tsh from a previous phase
  - [ ] SSH session in web UI from a previous phase
  - [ ] New SSH session with tsh
  - [ ] New SSH session with web UI
  - [ ] New SSH session in a child cluster on the same major version
  - [ ] New SSH session in a child cluster on the previous major version
  - [ ] New SSH session from a parent cluster
  - [ ] Application access through a browser
  - [ ] Application access through curl with `tsh apps login`
  - [ ] `kubectl get po` after `tsh kube login`
  - [ ] Database access (no configuration change should be necessary if the database CA isn't rotated, other Teleport functionality should not be affected if only the database CA is rotated)


## Proxy Peering

[Proxy Peering docs](https://goteleport.com/docs/architecture/proxy-peering/)

- Verify that Proxy Peering works for the following protocols:
  - [ ] SSH
  - [ ] Kubernetes
  - [ ] Database
  - [ ] Windows Desktop
  - [ ] App Access

## SSH Connection Resumption

Verify that SSH works, and that resumable SSH is not interrupted across a Teleport Cloud tenant upgrade.
|   | Standard node | Non-resuming node | Peered node | Agentless node |
|---|---|---|---|---|
| `tsh ssh` | <ul><li> [ ] </ul></li> | <ul><li> [ ] </ul></li> | <ul><li> [ ] </ul></li> | <ul><li> [ ] </ul></li> |
| `tsh ssh --no-resume` | <ul><li> [ ] </ul></li> | <ul><li> [ ] </ul></li> | <ul><li> [ ] </ul></li> | <ul><li> [ ] </ul></li> |
| Teleport Connect | <ul><li> [ ] </ul></li> | <ul><li> [ ] </ul></li> | <ul><li> [ ] </ul></li> | <ul><li> [ ] </ul></li> |
| Web UI (not resuming) | <ul><li> [ ] </ul></li> | <ul><li> [ ] </ul></li> | <ul><li> [ ] </ul></li> | <ul><li> [ ] </ul></li> |
| OpenSSH (standard `tsh config`) | <ul><li> [ ] </ul></li> | <ul><li> [ ] </ul></li> | <ul><li> [ ] </ul></li> | <ul><li> [ ] </ul></li> |
| OpenSSH (changing `ProxyCommand` to `tsh proxy ssh --no-resume`) | <ul><li> [ ] </ul></li> | <ul><li> [ ] </ul></li> | <ul><li> [ ] </ul></li> | <ul><li> [ ] </ul></li> |

Verify that SSH works, and that resumable SSH is not interrupted across a control plane restart (of either the root or the leaf cluster).

|   | Tunnel node | Direct dial node |
|---|---|---|
| `tsh ssh` | <ul><li> [ ] </ul></li> | <ul><li> [ ] </ul></li> |
| `tsh ssh --no-resume` | <ul><li> [ ] </ul></li> | <ul><li> [ ] </ul></li> |
| `tsh ssh` (from a root cluster) | <ul><li> [ ] </ul></li> | <ul><li> [ ] </ul></li> |
| `tsh ssh --no-resume` (from a root cluster) | <ul><li> [ ] </ul></li> | <ul><li> [ ] </ul></li> |
| OpenSSH (without `ProxyCommand`) | n/a | <ul><li> [ ] </ul></li> |
| OpenSSH's `ssh-keyscan` | n/a | <ul><li> [ ] </ul></li> |

## EC2 Discovery

[EC2 Discovery docs](https://goteleport.com/docs/server-access/guides/ec2-discovery/)

- Verify EC2 instance discovery
  - [ ]  Only EC2 instances matching given AWS tags have the installer executed on them
  - [ ]  Only the IAM permissions mentioned in the discovery docs are required for operation
  - [ ]  Custom scripts specified in different matchers are executed
  - [ ] Custom SSM documents specified in different matchers are executed
  - [ ] New EC2 instances with matching AWS tags are discovered and added to the teleport cluster
    - [ ] Large numbers of EC2 instances (51+) are all successfully added to the cluster
  - [ ] Nodes that have been discovered do not have the install script run on the node multiple times

## Azure Discovery

[Azure Discovery docs](https://goteleport.com/docs/server-access/guides/azure-discovery/)
- Verify Azure VM discovery
  - [ ] Only Azure VMs matching given Azure tags have the installer executed on them
  - [ ] Only the IAM permissions mentioned in the discovery docs are required for operation
  - [ ] Custom scripts specified in different matchers are executed
  - [ ] New Azure VMs with matching Azure tags are discovered and added to the teleport cluster
    - [ ] Large numbers of Azure VMs (51+) are all successfully added to the cluster
  - [ ] Nodes that have been discovered do not have the install script run on the node multiple times

## GCP Discovery

[GCP Discovery docs](https://goteleport.com/docs/server-access/guides/gcp-discovery/)

- Verify GCP instance discovery
  - [ ] Only GCP instances matching given GCP tags have the installer executed on them
  - [ ] Only the IAM permissions mentioned in the discovery docs are required for operation
  - [ ] Custom scripts specified in different matchers are executed
  - [ ] New GCP instances with matching GCP tags are discovered and added to the teleport cluster
    - [ ] Large numbers of GCP instances (51+) are all successfully added to the cluster
  - [ ] Nodes that have been discovered do not have the install script run on the node multiple times

## IP Pinning

Add a role with `pin_source_ip: true` (requires Enterprise) to test IP pinning.
Testing will require changing your IP (that Teleport Proxy sees).
Docs: [IP Pinning](https://goteleport.com/docs/access-controls/guides/ip-pinning/?scope=enterprise)

- Verify that it works for SSH Access
  - [ ] You can access tunnel node with `tsh ssh` on root cluster
  - [ ] You can access direct access node with `tsh ssh` on root cluster
  - [ ] You can access tunnel node from Web UI on root cluster
  - [ ] You can access direct access node from Web UI on root cluster
  - [ ] You can access tunnel node with `tsh ssh` on leaf cluster
  - [ ] You can access direct access node with `tsh ssh` on leaf cluster
  - [ ] You can access tunnel node from Web UI on leaf cluster
  - [ ] You can access direct access node from Web UI on leaf cluster
  - [ ] You can download files from nodes in Web UI (small arrows at top left corner)
  - [ ] If you change your IP you no longer can access nodes.
- Verify that it works for Kube Access
  - [ ] You can access Kubernetes cluster through standalone Kube service on root cluster
  - [ ] You can access Kubernetes cluster through agent inside Kubernetes on root cluster
  - [ ] You can access Kubernetes cluster through standalone Kube service on leaf cluster
  - [ ] You can access Kubernetes cluster through agent inside Kubernetes on leaf cluster
  - [ ] If you change your IP you no longer can access Kube clusters.
- Verify that it works for DB Access
  - [ ] You can access DB servers on root cluster
  - [ ] You can access DB servers on leaf cluster
  - [ ] If you change your IP you no longer can access DB servers.
- Verify that it works for App Access
  - [ ] You can access App service on root cluster
  - [ ] You can access App service on leaf cluster
  - [ ] If you change your IP you no longer can access App services.
- Verify that it works for Desktop Access
  - [ ] You can access Desktop service on root cluster
  - [ ] You can access Desktop service on leaf cluster
  - [ ] If you change your IP you no longer can access Desktop services.

## Assist

Assist is not supported by `tsh` and WebUI is the only way to use it.
Assist test plan is in the core section instead of WebUI as most functionality is implemented in the core.

- Configuration
  - [ ] Assist is disabled by default (OSS, Enterprise)
  - [ ] Assist can be enabled in the configuration file.
  - [ ] Assist is disabled in the Cloud.
  - [ ] Assist is enabled by default in the Cloud Team plan.
  - [ ] Assist is always disabled when etcd is used as a backend.

- Conversations
  - [ ] A new conversation can be started.
  - [ ] SSH command can be executed on one server.
  - [ ] SSH command can be executed on multiple servers.
  - [ ] SSH command can be executed on a node with per session MFA enabled.
  - [ ] Execution output is explained when it fits the context window.
  - [ ] Assist can list all nodes/execute a command on all nodes (using embeddings).
  - [ ] Access request can be created.
  - [ ] Access request is created when approved.
  - [ ] Conversation title is set after the first message.

- SSH integration
  - [ ] Assist icon is visible in WebUI's Terminal
  - [ ] A Bash command can be generated in the above window.
  - [ ] When an output is selected in the Terminal "Explain" option is available, and it generates the summary.

## Resources

[Quick GitHub/SAML/OIDC Setup Tips]

<!---
reference style links
-->
[Quick GitHub/SAML/OIDC Setup Tips]: https://gravitational.slab.com/posts/quick-git-hub-saml-oidc-setup-6dfp292a
