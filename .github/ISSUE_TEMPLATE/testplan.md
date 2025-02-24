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
  - [ ] [Resource-based Labels](https://goteleport.com/docs/admin-guides/management/admin/labels/#apply-resource-based-labels) using `server_info`

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
    - [ ] Removing the last MFA device on the user fails
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

- [ ] External Audit Storage

    External Audit Storage must be tested on an Enterprise Cloud tenant.
    Instructions for deploying a custom release to a cloud staging tenant: https://github.com/gravitational/teleport.e/blob/master/dev-deploy.md

  - [ ] Discover flow works to configure External Audit Storage https://goteleport.com/docs/choose-an-edition/teleport-cloud/external-audit-storage/
    - [ ] Docs (including screenshots) are up to date
    - [ ] Discover flow works with or without an existing AWS OIDC integration
    - [ ] Draft configuration can be resumed after navigating away
    - [ ] Bootstrap step (oneoff command pasted into CloudShell) works to create infra
    - [ ] Created IAM policy (attached to AWS OIDC integration role) matches docs example
    - [ ] Audit Events and Session Recordings (created after EAS enabled) are stored in configured S3 buckets
    - [ ] Audit Events and Session Recordings (created after EAS enabled) can be queried and played in the web UI
    - [ ] `tsh play <session-id>` works
  - [ ] Existing EAS configuration can be replaced with a new one via Discover flow
  - [ ] Existing EAS configuration can be deleted (disabling EAS)

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
    - [ ] (Linux) `tsh ssh -X server01 "echo Hello World | xclip -sel c && xclip -sel c -o"` should fail with "BadAccess" X error
      - This test doesn't work with XQuartz as it doesn't seem to enable the X Security Extension.

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

### Kubernetes exec via WebSockets/SPDY

To control usage of websockets on kubectl side environment variable `KUBECTL_REMOTE_COMMAND_WEBSOCKETS` can be used:
`KUBECTL_REMOTE_COMMAND_WEBSOCKETS=true kubectl -v 8 exec -n namespace podName -- /bin/bash --version`. With `-v 8` logging level
you should be able to see `X-Stream-Protocol-Version: v5.channel.k8s.io` in case kubectl is connected over websockets to Teleport.
To do tests you'll need kubectl version at least 1.29, Kubernetes cluster v1.29 or less (doesn't support websockets stream protocol v5)
and cluster v1.30 (does support it by default) and to access them both through kube agent and kubeconfig each.

* [ ] Check that you can exec into a cluster with `KUBECTL_REMOTE_COMMAND_WEBSOCKETS=false`
  * [ ] Cluster v1.29 in agent mode
  * [ ] Cluster v1.29 in kubeconfig mode
  * [ ] Cluster v1.30 in agent mode
  * [ ] Cluster v1.30 in kubeconfig mode
* [ ] Check that you can exec into a cluster with `KUBECTL_REMOTE_COMMAND_WEBSOCKETS=true`
  * [ ] Cluster v1.29 in agent mode
  * [ ] Cluster v1.29 in kubeconfig mode
  * [ ] Cluster v1.30 in agent mode (should see `X-Stream-Protocol-Version: v5.channel.k8s.io`)
  * [ ] Cluster v1.30 in kubeconfig mode (should see `X-Stream-Protocol-Version: v5.channel.k8s.io`)
* [ ] Test Kubernetes exec via javascript client - [client](https://github.com/kubernetes-client/javascript/blob/45b68c98e62b6cc4152189b9fd4a27ad32781bc4/examples/typescript/exec/exec-example.ts)


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

* [ ] Migrate trusted clusters
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
- [ ] Create a GCP instance with [the required permissions]((https://goteleport.com/docs/management/guides/gcp-tags/))
and with [label](https://cloud.google.com/compute/docs/labeling-resources)
`foo`: `bar` and [tag](https://cloud.google.com/resource-manager/docs/tags/tags-overview)
`baz`: `quux`. Verify that a node running on the instance has labels
`gcp/label/foo=bar` and `gcp/tag/baz=quux`.

### Passwordless

This feature has additional build requirements, so it should be tested with a
pre-release build (eg: `https://cdn.teleport.dev/tsh-v16.0.0-alpha.2.pkg`).

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
pre-release build (eg: `https://cdn.teleport.dev/teleport-ent-v16.0.0-alpha.2-linux-amd64-bin.tar.gz`).

Client-side enrollment requires a signed `tsh` for macOS, make sure to use the
`tsh` binary from `tsh.app`.

Additionally, Device Trust Web requires Teleport Connect to be installed (device
authentication for the Web is handled by Connect).

A simple formula for testing device authorization is:

```shell
# Before enrollment.
# Replace with other kinds of access, as appropriate (db, kube, etc)
tsh ssh node-that-requires-device-trust
> ERROR: ssh: rejected: administratively prohibited (unauthorized device)

# Register/enroll the device.
tsh device enroll --current-device
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

- [ ] Device authentication
  - [ ] tsh or Connect
    - [ ] SSH
    - [ ] DB Access
    - [ ] K8s Access
  - [ ] Web UI (requires Connect)
    - [ ] SSH
    - [ ] App Access
    - [ ] Desktop Access
    - [ ] GitHub user
    - [ ] OIDC user
    - [ ] SAML user

    Confirm that it works by failing first. Most protocols can be tested using
    device_trust.mode="required". App Access and Desktop Access require a custom
    role (see [enforcing device trust](https://goteleport.com/docs/access-controls/device-trust/enforcing-device-trust/#app-access-support)).

    For SSO users confirm that device web authentication happens successfully.

- [ ] Device authorization
  - [ ] device_trust.mode other than "off" or "" not allowed (OSS)
  - [ ] device_trust.mode="off" doesn't impede access (Enterprise and OSS)
  - [ ] device_trust.mode="optional" doesn't impede access, but issues device
        extensions on login
  - [ ] device_trust.mode="required" enforces enrolled devices
    - [ ] SSH
    - [ ] DB Access
    - [ ] K8s Access
    - [ ] App Access NOT enforced in global mode
    - [ ] Desktop Access NOT enforced in global mode
  - [ ] Role-based authz enforces enrolled devices
        (device_trust.mode="optional" and role.spec.options.device_trust_mode="required")
    - [ ] SSH
    - [ ] DB Access
    - [ ] K8s Access
    - [ ] App Access
    - [ ] Desktop Access
  - [ ] Device authentication works correctly for both require_session_mfa=false
        and require_session_mfa=true
    - [ ] SSH
    - [ ] DB Access
    - [ ] K8s Access
    - [ ] Desktop Access
  - [ ] Device authorization applies to Trusted Clusters
        (root with mode="optional" and leaf with mode="required")

- [ ] Device audit (see [lib/events/codes.go][device_event_codes])
  - [ ] Inventory management actions issue events (success only)
  - [ ] Device enrollment issues device event (any outcomes)
  - [ ] Device authentication issues device event (any outcomes)
  - [ ] Device web authentication issues "Device Web Token Created" and "Device
        Web Authentication Confirmed" events
  - [ ] Device web authentication events have web_authentication_id set.
        Corresponding "Device Authenticated" events have both
        web_authentication=true and web_authentication_id set.
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

This feature has additional build requirements, so it should be tested with a pre-release build (eg: `https://cdn.teleport.dev/teleport-ent-v16.0.0-alpha.2-linux-amd64-bin.tar.gz`).

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
- [ ] AWS KMS Support
  - [ ] Make sure docs/links are up to date
  - [ ] New cluster with AWS KMS CA works
  - [ ] Migrating a software cluster to AWS KMS works
  - [ ] CA rotation works
- [ ] GCP KMS Support
  - [ ] Make sure docs/links are up to date
  - [ ] New cluster with GCP KMS CA works
  - [ ] Migrating a software cluster to GCP KMS works
  - [ ] CA rotation works

Run the full test suite with each HSM/KMS:

```shell
$ # test YubiHSM
$ yubihsm-connector -d # in a background shell
$ cat /etc/yubihsm_pkcs11.conf
# /etc/yubihsm_pkcs11.conf
connector = http://127.0.0.1:12345
debug
$ TELEPORT_TEST_YUBIHSM_PKCS11_PATH=/usr/local/lib/pkcs11/yubihsm_pkcs11.dylib TELEPORT_TEST_YUBIHSM_PIN=0001password YUBIHSM_PKCS11_CONF=/etc/yubihsm_pkcs11.conf go test ./lib/auth/keystore -v --count 1
$ TELEPORT_TEST_YUBIHSM_PKCS11_PATH=/usr/local/lib/pkcs11/yubihsm_pkcs11.dylib TELEPORT_TEST_YUBIHSM_PIN=0001password YUBIHSM_PKCS11_CONF=/etc/yubihsm_pkcs11.conf go test ./integration/hsm -v --count 1 --timeout 20m # this takes ~12 minutes
$
$ # test AWS KMS
$ # login in to AWS locally
$ AWS_ACCOUNT="$(aws sts get-caller-identity | jq -r '.Account')"
$ TELEPORT_TEST_AWS_KMS_ACCOUNT="${AWS_ACCOUNT}" TELEPORT_TEST_AWS_KMS_REGION=us-west-2 go test ./lib/auth/keystore -v --count 1
$ TELEPORT_TEST_AWS_KMS_ACCOUNT="${AWS_ACCOUNT}" TELEPORT_TEST_AWS_KMS_REGION=us-west-2 go test ./integration/hsm -v --count 1
$
$ # test AWS CloudHSM
$ # set up the CloudHSM cluster and run this on an EC2 that can reach it
$ TELEPORT_TEST_CLOUDHSM_PIN="<CU_username>:<CU_password>" go test ./lib/auth/keystore -v --count 1
$ TELEPORT_TEST_CLOUDHSM_PIN="<CU_username>:<CU_password>" go test ./integration/hsm -v --count 1
$
$ # test GCP KMS
$ # login in to GCP locally
$ TELEPORT_TEST_GCP_KMS_KEYRING=projects/<account>/locations/us-west3/keyRings/<keyring> go test ./lib/auth/keystore -v --count 1
$ TELEPORT_TEST_GCP_KMS_KEYRING=projects/<account>/locations/us-west3/keyRings/<keyring> go test ./integration/hsm -v --count 1
```

## Moderated session

Create two Teleport users, a moderator and a user. Configure Teleport roles to require that the moderator moderate the user's sessions. Use `TELEPORT_HOME` to `tsh login` as the user in one terminal, and the moderator in another.

Ensure the default `terminationPolicy` of `terminate` has not been changed.

For each of the following cases, create a moderated session with the user using `tsh ssh` and join this session with the moderator using `tsh join --role moderator`:
 - [ ] Ensure that `Ctrl+C` in the user terminal disconnects the moderator as the session has ended.
 - [ ] Ensure that `Ctrl+C` in the moderator terminal disconnects the moderator and terminates the user's session as the session no longer has a moderator.
 - [ ] Ensure that `t` in the moderator terminal terminates the session for all participants.

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
Some tests are marked with "coverved by E2E test" and automatically completed
by default. In cases the E2E test is flaky or disabled, deselect the task for
manualy testing.

- [ ] Connect to a database within a local cluster.
  - [ ] Self-hosted Postgres.
    - [ ] verify that cancelling a Postgres request works. (`select pg_sleep(10)` followed by ctrl-c is a good query to test.)
  - [ ] Self-hosted MySQL.
    - [ ] MySQL server version reported by Teleport is correct.
  - [ ] Self-hosted MariaDB.
  - [ ] Self-hosted MongoDB.
  - [ ] Self-hosted CockroachDB.
  - [ ] Self-hosted Redis/Valkey.
  - [ ] Self-hosted Redis Cluster.
  - [ ] Self-hosted MSSQL.
  - [ ] Self-hosted MSSQL with PKINIT authentication.
  - [ ] Self-hosted Elasticsearch.
  - [ ] Self-hosted Cassandra/ScyllaDB.
  - [ ] Self-hosted Oracle.
  - [ ] Self-hosted ClickHouse.
  - [ ] AWS Aurora Postgres.
  - [ ] AWS Aurora MySQL.
    - [ ] MySQL server version reported by Teleport is correct.
  - [ ] AWS RDS Proxy (MySQL, Postgres, MariaDB, or SQL Server)
  - [ ] AWS Redshift.
  - [ ] AWS Redshift Serverless.
    - [ ] Verify connection to external AWS account works with `assume_role_arn: ""` and `external_id: "<id>"`
  - [ ] AWS ElastiCache.
  - [ ] AWS MemoryDB.
  - [ ] AWS OpenSearch.
  - [ ] AWS Dynamodb.
    - [ ] Verify connection to external AWS account works with `assume_role_arn: ""` and `external_id: "<id>"`
  - [ ] AWS DocumentDB
  - [ ] AWS Keyspaces
    - [ ] Verify connection to external AWS account works with `assume_role_arn: ""` and `external_id: "<id>"`
  - [ ] GCP Cloud SQL Postgres.
  - [ ] GCP Cloud SQL MySQL.
  - [ ] GCP Cloud Spanner.
  - [ ] Azure Cache for Redis.
  - [x] Azure single-server MySQL and Postgres (EOL Sep 2024 and Mar 2025, skip)
  - [ ] Azure flexible-server MySQL
  - [ ] Azure flexible-server Postgres
  - [ ] Azure SQL Server.
  - [ ] Snowflake.
  - [ ] MongoDB Atlas.
- [ ] Connect to a database within a remote cluster via a trusted cluster.
  - [ ] Self-hosted Postgres.
  - [ ] Self-hosted MySQL.
  - [ ] Self-hosted MariaDB.
  - [ ] Self-hosted MongoDB.
  - [ ] Self-hosted CockroachDB.
  - [ ] Self-hosted Redis/Valkey.
  - [ ] Self-hosted Redis Cluster.
  - [ ] Self-hosted MSSQL.
  - [ ] Self-hosted MSSQL with PKINIT authentication.
  - [ ] Self-hosted Elasticsearch.
  - [ ] Self-hosted Cassandra/ScyllaDB.
  - [ ] Self-hosted Oracle.
  - [ ] Self-hosted ClickHouse.
  - [ ] AWS Aurora Postgres.
  - [ ] AWS Aurora MySQL.
  - [ ] AWS RDS Proxy (MySQL, Postgres, MariaDB, or SQL Server)
  - [ ] AWS Redshift.
  - [ ] AWS Redshift Serverless.
  - [ ] AWS ElastiCache.
  - [ ] AWS MemoryDB.
  - [ ] AWS OpenSearch.
  - [ ] AWS Dynamodb.
  - [ ] AWS DocumentDB
  - [ ] AWS Keyspaces
  - [ ] GCP Cloud SQL Postgres.
  - [ ] GCP Cloud SQL MySQL.
  - [ ] GCP Cloud Spanner.
  - [ ] Azure Cache for Redis.
  - [x] Azure single-server MySQL and Postgres (EOL Sep 2024 and Mar 2025, skip)
  - [ ] Azure flexible-server MySQL
  - [ ] Azure flexible-server Postgres
  - [ ] Azure SQL Server.
  - [ ] Snowflake.
  - [ ] MongoDB Atlas.
- [ ] Verify auto user provisioning.
  Verify all supported modes: `keep`, `best_effort_drop`
  - [ ] Self-hosted Postgres.
  - [ ] Self-hosted MySQL.
  - [ ] Self-hosted MariaDB.
  - [ ] Self-hosted MongoDB.
  - [x] AWS RDS Postgres. (covered by E2E test)
  - [x] AWS RDS MySQL. (coverved by E2E test)
  - [ ] AWS RDS MariaDB.
  - [x] AWS Redshift (coverved by E2E test).
- [ ] Verify Database Access Control
  - [ ] Postgres (tables)
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
      - [x] Can detect and register RDS instances. (covered by E2E test)
        - [x] Can detect and register RDS instances in an external AWS account when `assume_role_arn` and `external_id` is set.
      - [ ] Can detect and register RDS proxies, and their custom endpoints.
        - [ ] Can detect and register RDS instances in an external AWS account when `assume_role_arn` and `external_id` is set.
      - [ ] Can detect and register Aurora clusters, and their reader and custom endpoints.
      - [ ] Can detect and register RDS proxies, and their custom endpoints.
      - [x] Can detect and register Redshift clusters. (covered by E2E test)
      - [x] Can detect and register Redshift serverless workgroups, and their VPC endpoints. (covered by E2E test)
      - [ ] Can detect and register ElastiCache Redis clusters.
      - [ ] Can detect and register MemoryDB clusters.
      - [ ] Can detect and register OpenSearch domains.
      - [ ] Can detect and register DocumentDB clusters.
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
- [ ] `tsh bench` load tests (instructions on Notion -> Database Access -> Load test)
- [ ] Verify database session player
  - [ ] Web UI
    - [ ] Postgres
  - [ ] `tsh play`
    - [ ] Postgres

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
  - [ ] Can connect to AD desktop defined in static `static_hosts` section.
  - [ ] Can connect to non-AD desktop defined in static `static_hosts` section.
  - [ ] Can connect to desktop discovered via LDAP
- IoT mode (reverse tunnel through proxy):
  - [ ] Can connect to AD desktop defined in static `static_hosts` section.
  - [ ] Can connect to non-AD desktop defined in static `static_hosts` section.
  - [ ] Can connect to desktop discovered via LDAP
- [ ] Connect multiple `windows_desktop_service`s to the same Teleport cluster,
  verify that connections to desktops on different AD domains works. (Attempt to
  connect several times to verify that you are routed to the correct
  `windows_desktop_service`)
- [ ] Set `client_idle_timeout` to a small value and verify that idle sessions
  are terminated (the session should end and an audit event will confirm it
  was due to idle connection)
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
  - [ ] All desktops have `teleport.dev/origin` label.
  - [ ] Desktops discovered via LDAP have additional `teleport.dev` labels for OS, OS
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
    - [ ] Directory sharing option is not available in the dropdown
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
    - [ ] Give the user one role that explicitly disables directory sharing (`desktop_directory_sharing: false`)
      and confirm that the option to share a directory doesn't appear in the menu and  that the directory sharing
      icon is in a disabled state.
- Per-Session MFA
  - [ ] Attempting to start a session with no keys registered shows an error message
  - [ ] Attempting to start a session with a webauthn registered pops up the MFA dialog
    - [ ] Canceling this dialog (clicking the X in the corner) shows an error
    - [ ] Hitting "Passkey or MFA Device" causes your browser to prompt you for MFA
    - [ ] Cancelling that browser MFA prompt shows an error
    - [ ] Successful MFA verification allows you to connect
- Session Recording
  - [ ] Verify sessions are not recorded if *all* of a user's roles disable recording
  - [ ] Verify sync recording (`mode: node-sync` or `mode: proxy-sync`)
  - [ ] Verify async recording (`mode: node` or `mode: proxy`)
  - [ ] Sessions show up in session recordings UI with desktop icon
  - [ ] Sessions can be played back, including play/pause functionality
  - [ ] Sessions playback speed can be toggled while it's playing
  - [ ] Sessions playback speed can be toggled while it's paused
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
    start (due to RBAC, or a desktop lock, for example)
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
  - [ ] Induce the backend to send a TDP Error, confirm that session is terminated and error popup is shown. Confirm that the error is
        shown at the end of the playback of this session (confirms backwards compatibility w/ recordings from older w_d_s pre Teleport 12).
- Trusted Cluster / Tunneling
  - Set up Teleport in a trusted cluster configuration where the root and leaf cluster has a w_d_s connected via tunnel (w_d_s running as a separate process)
    - [ ] Confirm that windows desktop sessions can be made on root cluster
    - [ ] Confirm that windows desktop sessions can be made on leaf cluster
- Screen size/resize
  - resize
    - [ ] Screen can be resized during an active session
    - [ ] Screen can be resized during login (meaning before resize dvc is opened).
          The screen won't resize immediately, but it should resize when the dvc is opened (about when login completes).
    - [ ] Screen can be resized during mfa dialog without losing the session
    - [ ] Screen can be resized during "Active Session" dialog without losing the session
  - `screen_size`
    - [ ] Desktops that specify a fixed `screen_size` in their spec always use the same screen size.
    - [ ] Desktops sessions for desktops which specify a fixed `screen_size` do not resize automatically.
    - [ ] Attempting to register a desktop with a `screen_size` dimension larger than 8192 fails.
- Non-AD setup
  - [ ] Installer in GUI mode finishes successfully on instance that is not part of domain
  - [ ] Installer works correctly invoked from command line
  - [ ] Non-AD instance has label `teleport.dev/ad: false`
  - [ ] Connecting to non-AD instance works with OSS if there are no more than 5 non-AD desktops
  - [ ] Connecting to non-AD instance fails with OSS if there are more than 5 non-AD desktops
  - [ ] Connecting to non-AD instance works with Enterprise license always
  - [ ] In OSS version, if there are more than 5 non-AD desktops banner shows up telling you to upgrade (check occurs every 5 minutes so you may need to wait to confirm)
  - [ ] Banner goes away if you reduce number of non-AD desktops to less or equal 5 (check occurs every 5 minutes so you may need to wait to confirm)
  - [ ] Installer in GUI mode successfully uninstalls Authentication Package (logging in is not possible)
  - [ ] Installer successfully uninstalls Authentication Package (logging in is not possible) when invoked from command line
- Dynamic registration
  - [ ] Dynamic Windows desktop resources can be added, removed, and updated using `tctl`
  - [ ] `tctl get dynamic_windows_desktop` works with all supported formats
  - [ ] Adding dynamic Windows desktop that doesn't match labels for any Windows Desktop Service does not create any
      Windows desktop
  - [ ] Adding dynamic Windows desktop that matches some `windows_desktop_service`s creates Windows desktops for each
      matching WDS
  - [ ] Updating dynamic Windows desktop updates corresponding Windows desktops
  - [ ] Updating dynamic Windows desktop's labels so it no longer matches `windows_desktop_services` deletes
      corresponding Windows desktops
  - [ ] Deleting dynamic Windows desktop deletes corresponding Windows desktops

## Binaries / OS compatibility

Verify that our software runs on the minimum supported OS versions as per
https://goteleport.com/docs/installation/#operating-system-support

### Windows

- [ ] `tsh` runs on the minimum supported Windows version
- [ ] Teleport Connect runs on the minimum supported Windows version

Azure offers virtual machines with the Windows 10 2016 LTSB image. This image runs on Windows 10
rev. 1607, which is the exact minimum Windows version that we support.

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
- [ ] Verify you are able to connect to the SSH node using the SSH multiplexer service

With a Postgres DB registered to the Teleport cluster:

- [ ] Verify you are able to interact with a database using `tbot db connect` with a database output
- [ ] Verify you are able to connect to the database using `tbot proxy db` with a database output
- [ ] Verify you are able to produce an authenticated tunnel using `tbot proxy db --tunnel` with a database output and then able to connect to the database through the tunnel without credentials

With a Kubernetes cluster registered to the Teleport cluster:

- [ ] Verify the `kubeconfig` produced by a Kubernetes output can be used to run basic commands (e.g `kubectl get pods`)
  - [ ] With ALPN routing
  - [ ] Without ALPN routing

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
Host users are considered "managed" when they belong to one of the teleport system groups: `teleport-system`, `teleport-keep`. Users outside of these groups are considered "unmanaged". Any users in the `teleport-static` group are
also managed, but not considered for role-based host user creation.

- Verify host users creation functionality
  - [ ] non-existing users are created automatically
  - [ ] non-existing configured groups are created automatically
  - [ ] users are added to groups
    - [ ] created and/or managed users are added to the `teleport-system` group when `create_host_user_mode: "insecure-drop"`
    - [ ] created and/or managed users are added to the `teleport-keep` group when `create_host_user_mode: "keep"`
  - [ ] managed users have their groups reconciled to reflect any `host_groups` changes (additions and removals)
  - [ ] failure to create or update host users does not bail out of SSH connections when host user already exists (can be forced by setting `create_host_user_mode: "off"`)
  - [ ] users belonging to `teleport-system` are cleaned up after their session ends
    - [ ] cleanup occurs if a program was left running after session ends
  - [ ] users belonging to `teleport-keep` are not cleaned up after their session ends
  - [ ] sudoers file creation is successful
    - [ ] invalid sudoers files are _not_ created
    - [ ] failure to write sudoers file, such as for invalid entries, does not bail out of SSH connections
  - [ ] unmanaged host users are accessible over SSH
    - [ ] unmanaged host users are not modified when `teleport-keep` is not included in `host_groups`
    - [ ] unmanaged host users are modified when `teleport-keep` is included in `host_groups`
  - [ ] setting `disable_create_host_user: true` stops user creation from occurring
  - [ ] setting `create_host_user_default_shell: <bash, zsh, fish, etc.>` should set the default shell for a newly created host user to the chosen shell (validated by confirming shell path has been written to the end of the user's record in `/etc/passwd`)

## CA rotations

- Verify the CA rotation functionality itself (by checking in the backend or with `tctl get cert_authority`)
  - [ ] `standby` phase: only `active_keys`, no `additional_trusted_keys`
  - [ ] `init` phase: `active_keys` and `additional_trusted_keys`
  - [ ] `update_clients` and `update_servers` phases: the certs from the `init` phase are swapped
  - [ ] `standby` phase: only the new certs remain in `active_keys`, nothing in `additional_trusted_keys`
  - [ ] `rollback` phase (second pass, after completing a regular rotation): same content as in the `init` phase
  - [ ] `standby` phase after `rollback`: same content as in the previous `standby` phase
  - [ ] Changing `signature_algorithm_suite` should change the algorithm used by new CA issuers when entering `init` - only issued certificates change algorithm if the suite is changed at other times
  - [ ] Even after changing `signature_algorithm_suite`, entering the `rollback` phase correctly restores the original issuer, no matter the algorithm
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

[Azure Discovery docs](https://goteleport.com/docs/enroll-resources/auto-discovery/servers/azure-discovery/)
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

## IGS:
- [ ] Access Monitoring
  - [ ] Verify that users can run custom audit queries.
  - [ ] Verify that the Privileged Access Report is generated and periodically refreshed.

- [ ] Access Requests
  - [ ] Verify when role.spec.allow.request.reason.mode: "required":
    - [ ] CLI fails to create Access Request displaying a message that reason is required.
    - [ ] Web UI fails to create Access Request displaying a message that reason is required.
    - [ ] Other roles allowing requesting the same resources/roles without reason.mode set or with reason.mode: "optional" don't affect the behaviour.
    - [ ] Non-affected resources/roles don't require reason.
    - [ ] When there is a role with spec.options.request_access: always it effectively becomes role.spec.options.request_access: reason (i.e.) requires reason:
      - [ ] For CLI.
      - [ ] For Web UI.

- [ ] Access Lists
  - [ ] Verify Access List membership/ownership/expiration date.
  - [ ] Verify permissions granted by Access List membership.
  - [ ] Verify permissions granted by Access List ownership.
  - [ ] Verify Access List Review.
  - [ ] Verify Access List Promotion.
  - [ ] Verify that owners can only add/remove members and not change other properties.
  - [ ] Nested Access Lists
    - [ ] Verify that Access Lists can be added as members or owners of other Access Lists.
    - [ ] Verify that member grants from ancestor lists are inherited by members of nested Access Lists added as members.
    - [ ] Verify that owner grants from ancestor lists are inherited by members of nested Access Lists added as owners.
    - [ ] Verify that Access List Review and Promotion work with nested Access Lists.
    - [ ] Verify that manually deleting a nested Access List used as a member or owner does not break UserLoginState generation or listing Access Lists.
    - [ ] Verify that an Access List can be added as a member or owner of another Access List using `tctl`.
    - [ ] Verify that Access Lists added as members or owners of other Access Lists using `tctl` are validated (no circular references, no nesting > 10 levels).

- [ ] Verify Okta Sync Service
  - [ ] Verify Okta Plugin configuration.
    - [ ] Verify that the Okta Plugin can be configured.
    - [ ] Verify the Single Sign-On (SSO) connector created by the Okta Plugin.
  - [ ] Verify Okta users/apps/groups sync.
    - [ ] Verify that users/apps/groups are synced from Okta to Teleport.
    - [ ] Verify the custom `okta_import_rule` rule configuration.
    - [ ] Verify that users/apps/groups are displayed in the Teleport Web UI.
    - [ ] Verify that users/groups are flattened on import, and are not duplicated on sync when their membership is inherited via nested Access Lists.
  - [ ] Verify that a user is locked/removed from Teleport when the user is Suspended/Deactivated in Okta.
  - [ ] Verify access to Okta apps granted by access_list/access_request.

## Teleport SAML Identity Provider
Verify SAML IdP service provider resource management.

### Docs:
- [ ] Verify SAML IdP guide instructions work.

### Manage Service Provider (SP)
- [ ] `saml_idp_service_provider` resource can be created, updated and deleted with `tctl create/update/delete sp.yaml` command.
  - [ ] SP can be created with `name` and `entity descriptor`.
  - [ ] SP can be created with `name`, `entity_id`, `acs_url`.
    - [ ] Verify Entity descriptor is generated.
  - [ ] Verify attribute mapping configuration works.
  - [ ] Verify test attribute mapping command. `$ tctl idp saml test-attribute-mapping --users <usernames or name of file containing user spec> --sp <name of file containing user spec> --format <json/yaml/defaults to text>`

### SAML service provider catalog
- [ ] GCP Workforce Identity Federation
  - [ ] Verify guided flow works end-to-end, signing into GCP web console from Teleport resource page.
  - [ ] Verify that when a SAML resource is created with preset value `preset: gcp-workforce`, Teleport adds
        relay state `relay_state: https://console.cloud.google/` value in the resulting resource spec.

## SSO MFA

Verify SSO MFA core functionality. The tests below should be performed once
with OIDC and once with SAML.

Configure both an OIDC connector and a SAML connector following the [Quick GitHub/SAML/OIDC Setup Tips]
and [enable MFA on them](https://goteleport.com/docs/ver/17.x/admin-guides/access-controls/sso/#configuring-sso-for-mfa-checks).

For simplicity, you can use the same IdP App (client id/secret or entity descriptor)
for both login and MFA. This way, each Teleport MFA check will make you re-login via SSO.

Ensure [SSO is allowed as a second factor](https://goteleport.com/docs/ver/17.x/admin-guides/access-controls/sso/#allowing-sso-as-an-mfa-method-in-your-cluster).
e.g. `cap.second_factors: ['webauthn', 'sso']`.

The following should work with SSO MFA, automatically opening the SSO MFA redirect URL:

- [ ] `tsh mfa ls` should display the SSO MFA device.
  - [ ] SSO MFA device cannot be deleted or added
- [ ] Add another MFA device (`tsh mfa add`)
- [ ] Delete the other MFA device (`tsh --mfa-mode=sso mfa rm`)
- [ ] Moderated Sessions
- [ ] Admin Actions (e.g. `tctl tokens ls`)
- [ ] Per-session MFA
  - [ ] Server Access
  - [ ] File Transfers
  - [ ] Kubernetes Access
  - [ ] App Access
  - [ ] Database Access
  - [ ] Desktop Access

## Resources

[Quick GitHub/SAML/OIDC Setup Tips]

<!---
reference style links
-->
[Quick GitHub/SAML/OIDC Setup Tips]: https://www.notion.so/goteleport/Quick-SSO-setup-fb1a64504115414ca50a965390105bee
