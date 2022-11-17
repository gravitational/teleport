---
name: Test Plan
about: Manual test plan for Teleport major releases
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

- [ ] RBAC

  Make sure that invalid and valid attempts are reflected in audit log.

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
    - [ ] Login via WebAuthn using an U2F device

    U2F devices must be registered in a previous version of Teleport.

    Using Teleport v9, set `auth_service.authentication.second_factor = u2f`,
    restart the server and then register an U2F device (`tsh mfa add`). Upgrade
    the installation to the current Teleport version (one major at a time) and try to
    log in using the U2F device as your second factor - it should work.

  - [ ] Login OIDC
  - [ ] Login SAML
  - [ ] Login GitHub
  - [ ] Deleting Users

- [ ] Backends
  - [ ] Teleport runs with etcd
  - [ ] Teleport runs with dynamodb
  - [ ] Teleport runs with SQLite
  - [ ] Teleport runs with Firestore

- [ ] Session Recording
  - [ ] Session recording can be disabled
  - [ ] Sessions can be recorded at the node
    - [ ] Sessions in remote clusters are recorded in remote clusters
  - [ ] [Sessions can be recorded at the proxy](https://goteleport.com/docs/server-access/guides/recording-proxy-mode/)
    - [ ] Sessions on remote clusters are recorded in the local cluster
    - [ ] With an OpenSSH server without a Teleport CA signed host certificate:
      - [ ] Host key checking enabled rejects connection
      - [ ] Host key checking disabled allows connection

- [ ] Audit Log
  - [ ] Failed login attempts are recorded
  - [ ] Interactive sessions have the correct Server ID
    - [ ] Server ID is the ID of the node in "session_recording: node" mode
    - [ ] Server ID is the ID of the proxy in "session_recording: proxy" mode

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
  - [ ] tsh ssh -A \<regular-node\>
  - [ ] tsh ssh -A \<node-remote-cluster\>
  - [ ] tsh ssh \<regular-node\> ls
  - [ ] tsh ssh \<node-remote-cluster\> ls
  - [ ] tsh join \<regular-node\>
  - [ ] tsh join \<node-remote-cluster\>
  - [ ] tsh play \<regular-node\>
  - [ ] tsh play \<node-remote-cluster\>
  - [ ] tsh scp \<regular-node\>
  - [ ] tsh scp \<node-remote-cluster\>
  - [ ] tsh ssh -L \<regular-node\>
  - [ ] tsh ssh -L \<node-remote-cluster\>
  - [ ] tsh ls
  - [ ] tsh clusters

- [ ] Interact with a cluster using `ssh`
   Make sure to test both recording and regular proxy modes.
  - [ ] ssh \<regular-node\>
  - [ ] ssh \<node-remote-cluster\>
  - [ ] ssh -A \<regular-node\>
  - [ ] ssh -A \<node-remote-cluster\>
  - [ ] ssh \<regular-node\> ls
  - [ ] ssh \<node-remote-cluster\> ls
  - [ ] scp \<regular-node\>
  - [ ] scp \<node-remote-cluster\>
  - [ ] ssh -L \<regular-node\>
  - [ ] ssh -L \<node-remote-cluster\>

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
  - [ ] Check agent forwarding is correct based on role and proxy mode.

- [ ] `tsh` CA loading

  Create a trusted cluster pair with a node in the leaf cluster. Log into the root cluster.
  - [ ] `load_all_cas` on the root auth server is `false` (default) -
  `tsh ssh leaf.node.example.com` results in access denied.
  - [ ] `load_all_cas` on the root auth server is `true` - `tsh ssh leaf.node.example.com`
  succeeds.

### User accounting

- [ ] Verify that active interactive sessions are tracked in `/var/run/utmp` on Linux.
- [ ] Verify that interactive sessions are logged in `/var/log/wtmp` on Linux.

### Combinations

For some manual testing, many combinations need to be tested. For example, for
interactive sessions the 12 combinations are below.

- [ ] Connect to a OpenSSH node in a local cluster using OpenSSH.
- [ ] Connect to a OpenSSH node in a local cluster using Teleport.
- [ ] Connect to a OpenSSH node in a local cluster using the Web UI.
- [ ] Connect to a Teleport node in a local cluster using OpenSSH.
- [ ] Connect to a Teleport node in a local cluster using Teleport.
- [ ] Connect to a Teleport node in a local cluster using the Web UI.
- [ ] Connect to a OpenSSH node in a remote cluster using OpenSSH.
- [ ] Connect to a OpenSSH node in a remote cluster using Teleport.
- [ ] Connect to a OpenSSH node in a remote cluster using the Web UI.
- [ ] Connect to a Teleport node in a remote cluster using OpenSSH.
- [ ] Connect to a Teleport node in a remote cluster using Teleport.
- [ ] Connect to a Teleport node in a remote cluster using the Web UI.

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

### Teleport Plugins

- [ ] Test receiving a message via Teleport Slackbot
- [ ] Test receiving a new Jira Ticket via Teleport Jira

### AWS Node Joining
[Docs](https://goteleport.com/docs/setup/guides/joining-nodes-aws/)
- [ ] On EC2 instance with `ec2:DescribeInstances` permissions for local account:
  `TELEPORT_TEST_EC2=1 go test ./integration -run TestEC2NodeJoin`
- [ ] On EC2 instance with any attached role:
  `TELEPORT_TEST_EC2=1 go test ./integration -run TestIAMNodeJoin`
- [ ] EC2 Join method in IoT mode with node and auth in different AWS accounts
- [ ] IAM Join method in IoT mode with node and auth in different AWS accounts

### Cloud Labels
- [ ] Create an EC2 instance with [tags in instance metadata enabled](https://goteleport.com/docs/management/guides/ec2-tags/)
and with tag `foo`: `bar`. Verify that a node running on the instance has label
`aws/foo=bar`.
- [ ] Create an Azure VM with tag `foo`: `bar`. Verify that a node running on the
instance has label `azure/foo=bar`.

### Passwordless

This feature has additional build requirements, so it should be tested with a pre-release build from Drone (eg: `https://get.gravitational.com/teleport-v10.0.0-alpha.2-linux-amd64-bin.tar.gz`).

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
  - [ ] Passwordless disable switch works
        (`auth_service.authentication.passwordless = false`)
  - [ ] Cluster in passwordless mode defaults to passwordless
        (`auth_service.authentication.connector_name = passwordless`)
  - [ ] Cluster in passwordless mode allows MFA login
        (`tsh login --auth=local`)

- [ ] Touch ID support commands
  - [ ] `tsh touchid ls` works
  - [ ] `tsh touchid rm` works (careful, may lock you out!)

### Hardware Key Support

Hardware Key Support is an Enterprise feature and is not available for OSS.

You will need a YubiKey 4.3+ to test this feature.

This feature has additional build requirements, so it should be tested with a pre-release build from Drone (eg: `https://get.gravitational.com/teleport-ent-v11.0.0-alpha.2-linux-amd64-bin.tar.gz`).

#### Server Access

These tests should be carried out sequentially. `tsh` tests should be carried out on Linux, MacOS, and Windows.

1. [ ] `tsh login` as user with [Webauthn](https://goteleport.com/docs/access-controls/guides/webauthn/) login and no hardware key requirement.
2. [ ] Request a role with `role.role_options.require_session_mfa: hardware_key` - `tsh login --request-roles=hardware_key_required`
  - [ ] Assuming the role should force automatic re-login with yubikey
  - [ ] `tsh ssh`
    - [ ] Requires yubikey to be connected for re-login
    - [ ] Prompts for per-session MFA
3. [ ] Request a role with `role.role_options.require_session_mfa: hardware_key_touch` - `tsh login --request-roles=hardware_key_touch_required`
  - [ ] Assuming the role should force automatic re-login with yubikey
    - [ ] Prompts for touch if not cached (last touch within 15 seconds)
  - [ ] `tsh ssh`
    - [ ] Requires yubikey to be connected for re-login
    - [ ] Prompts for touch if not cached
4. [ ] `tsh logout` and `tsh login` as the user with no hardware key requirement.
5. [ ] Upgrade auth settings to `auth_service.authentication.require_session_mfa: hardware_key`
  - [ ] Using the existing login session (`tsh ls`) should force automatic re-login with yubikey
  - [ ] `tsh ssh`
    - [ ] Requires yubikey to be connected for re-login
    - [ ] Prompts for per-session MFA
6. [ ] Upgrade auth settings to `auth_service.authentication.require_session_mfa: hardware_key_touch`
  - [ ] Using the existing login session (`tsh ls`) should force automatic re-login with yubikey
    - [ ] Prompts for touch if not cached
  - [ ] `tsh ssh`
    - [ ] Requires yubikey to be connected for re-login
    - [ ] Prompts for touch if not cached

#### Other

Set `auth_service.authentication.require_session_mfa: hardware_key_touch` in your cluster auth settings.

- [ ] Database Acces: `tsh proxy db`
- [ ] Application Access: `tsh login app && tsh proxy app`

## WEB UI

## Main
For main, test with a role that has access to all resources.

#### Top Nav
- [ ] Verify that cluster selector displays all (root + leaf) clusters
- [ ] Verify that user name is displayed
- [ ] Verify that user menu shows logout, help&support, and account settings (for local users)

#### Side Nav
- [ ] Verify that each item has an icon
- [ ] Verify that Collapse/Expand works and collapsed has icon `>`, and expand has icon `v`
- [ ] Verify that it automatically expands and highlights the item on page refresh

#### Servers aka Nodes
- [ ] Verify that "Servers" table shows all joined nodes
- [ ] Verify that "Connect" button shows a list of available logins
- [ ] Verify that "Hostname", "Address" and "Labels" columns show the current values
- [ ] Verify that "Search" by hostname, address, labels works
- [ ] Verify that terminal opens when clicking on one of the available logins
- [ ] Verify that clicking on `Add Server` button renders dialogue set to `Automatically` view
  - [ ] Verify clicking on `Regenerate Script` regenerates token value in the bash command
  - [ ] Verify using the bash command successfully adds the server (refresh server list)
  - [ ] Verify that clicking on `Manually` tab renders manual steps
  - [ ] Verify that clicking back to `Automatically` tab renders bash command

#### Applications
- [ ] Verify that clicking on `Add Application` button renders dialogue
  - [ ] Verify input validation (prevent empty value and invalid url)
  - [ ] Verify after input and clicking on `Generate Script`, bash command is rendered
  - [ ] Verify clicking on `Regenerate` button regenerates token value in bash command

#### Databases
- [ ] Verify that clicking on `Add Database` button renders dialogue for manual instructions:
  - [ ] Verify selecting different options on `Step 4` changes `Step 5` commands
#### Active Sessions
- [ ] Verify that "empty" state is handled
- [ ] Verify that it displays the session when session is active
- [ ] Verify that "Description", "Session ID", "Users", "Nodes" and "Duration" columns show correct values
- [ ] Verify that "OPTIONS" button allows to join a session

#### Audit log
- [ ] Verify that time range button is shown and works
- [ ] Verify that clicking on `Session Ended` event icon, takes user to session player
- [ ] Verify event detail dialogue renders when clicking on events `details` button
- [ ] Verify searching by type, description, created works


#### Users
- [ ] Verify that users are shown
- [ ] Verify that creating a new user works
- [ ] Verify that editing user roles works
- [ ] Verify that removing a user works
- [ ] Verify resetting a user's password works
- [ ] Verify search by username, roles, and type works

#### Auth Connectors
For help with setting up auth connectors, check out the [Quick GitHub/SAML/OIDC Setup Tips]

- [ ] Verify when there are no connectors, empty state renders
- [ ] Verify that creating OIDC/SAML/GITHUB connectors works
- [ ] Verify that editing  OIDC/SAML/GITHUB connectors works
- [ ] Verify that error is shown when saving an invalid YAML
- [ ] Verify that correct hint text is shown on the right side
- [ ] Verify that encrypted SAML assertions work with an identity provider that supports it (Azure).
- [ ] Verify that created GitHub, saml, oidc card has their icons
#### Roles
- [ ] Verify that roles are shown
- [ ] Verify that "Create New Role" dialog works
- [ ] Verify that deleting and editing works
- [ ] Verify that error is shown when saving an invalid YAML
- [ ] Verify that correct hint text is shown on the right side

#### Managed Clusters
- [ ] Verify that it displays a list of clusters (root + leaf)
- [ ] Verify that every menu item works: nodes, apps, audit events, session recordings, etc.

#### Help & Support
- [ ] Verify that all URLs work and correct (no 404)

## Access Requests

Access Request is a Enterprise feature and is not available for OSS.

### Creating Access Requests (Role Based)
Create a role with limited permissions `allow-roles-and-nodes`. This role allows you to see the Role screen and ssh into all nodes.

```
kind: role
metadata:
  name: allow-roles-and-nodes
spec:
  allow:
    logins:
    - root
    node_labels:
      '*': '*'
    rules:
    - resources:
      - role
      verbs:
      - list
      - read
  options:
    max_session_ttl: 8h0m0s
version: v5

```

Create another role with limited permissions `allow-users-with-short-ttl`. This role session expires in 4 minutes, allows you to see Users screen, and denies access to all nodes.

```
kind: role
metadata:
  name: allow-users-with-short-ttl
spec:
  allow:
    rules:
    - resources:
      - user
      verbs:
      - list
      - read
  deny:
    node_labels:
      '*': '*'
  options:
    max_session_ttl: 4m0s
version: v5
```

Create a user that has no access to anything but allows you to request roles:
```
kind: role
metadata:
  name: test-role-based-requests
spec:
  allow:
    request:
      roles:
      - allow-roles-and-nodes
      - allow-users-with-short-ttl
      suggested_reviewers:
      - random-user-1
      - random-user-2
version: v5
```

- [ ] Verify that under requestable roles, only `allow-roles-and-nodes` and `allow-users-with-short-ttl` are listed
- [ ] Verify you can select/input/modify reviewers
- [ ] Verify you can view the request you created from request list (should be in pending states)
- [ ] Verify there is list of reviewers you selected (empty list if none selected AND suggested_reviewers wasn't defined)
- [ ] Verify you can't review own requests

### Creating Access Requests (Search Based)
Create a role with access to searcheable resources (apps, db, kubes, nodes, desktops). The template `searcheable-resources` is below.

```
kind: role
metadata:
  name: searcheable-resources
spec:
  allow:
    app_labels:  # just example labels
      label1-key: label1-value
      env: [dev, staging] 
    db_labels:
      '*': '*'   # asteriks gives user access to everything
    kubernetes_labels:
      '*': '*' 
    node_labels:
      '*': '*'
    windows_desktop_labels:
      '*': '*'
version: v5
```

Create a user that has no access to resources, but allows you to search them:

```
kind: role
metadata:
  name: test-search-based-requests
spec:
  allow:
    request:
      search_as_roles:
      - searcheable resources
      suggested_reviewers:
      - random-user-1
      - random-user-2
version: v5
```

- [ ] Verify that a user can see resources based on the `searcheable-resources` rules
- [ ] Verify you can select/input/modify reviewers
- [ ] Verify you can view the request you created from request list (should be in pending states)
- [ ] Verify there is list of reviewers you selected (empty list if none selected AND suggested_reviewers wasn't defined)
- [ ] Verify you can't review own requests
- [ ] Verify that you can't mix adding resources from different clusters (there should be a warning dialogue that clears the selected list)

### Viewing & Approving/Denying Requests
Create a user with the role `reviewer` that allows you to review all requests, and delete them.
```
kind: role
version: v3
metadata:
  name: reviewer
spec:
  allow:
    review_requests:
      roles: ['*']
```
- [ ] Verify you can view access request from request list
- [ ] Verify you can approve a request with message, and immediately see updated state with your review stamp (green checkmark) and message box
- [ ] Verify you can deny a request, and immediately see updated state with your review stamp (red cross)
- [ ] Verify deleting the denied request is removed from list

### Assuming Approved Requests (Role Based)
- [ ] Verify that assuming `allow-roles-and-nodes` allows you to see roles screen and ssh into nodes
- [ ] After assuming `allow-roles-and-nodes`, verify that assuming `allow-users-short-ttl` allows you to see users screen, and denies access to nodes
  - [ ] Verify a switchback banner is rendered with roles assumed, and count down of when it expires
  - [ ] Verify `switching back` goes back to your default static role
  - [ ] Verify after re-assuming `allow-users-short-ttl` role, the user is automatically logged out after the expiry is met (4 minutes)

### Assuming Approved Requests (Search Based)
- [ ] Verify that assuming approved request, allows you to see the resources you've requested.
### Assuming Approved Requests (Both)
- [ ] Verify assume buttons are only present for approved request and for logged in user
- [ ] Verify that after clicking on the assume button, it is disabled in both the list and in viewing
- [ ] Verify that after re-login, requests that are not expired and are approved are assumable again

## Access Request Waiting Room
#### Strategy Reason
Create the following role:
```
kind: role
metadata:
  name: waiting-room
spec:
  allow:
    request:
      roles:
      - <some other role to assign user after approval>
  options:
    max_session_ttl: 8h0m0s
    request_access: reason
    request_prompt: <some custom prompt to show in reason dialogue>
version: v3
```
- [ ] Verify after login, reason dialogue is rendered with prompt set to `request_prompt` setting
- [ ] Verify after clicking `send request`, pending dialogue renders
- [ ] Verify after approving a request, dashboard is rendered
- [ ] Verify the correct role was assigned

#### Strategy Always
With the previous role you created from `Strategy Reason`, change `request_access` to `always`:
- [ ] Verify after login, pending dialogue is auto rendered
- [ ] Verify after approving a request, dashboard is rendered
- [ ] Verify after denying a request, access denied dialogue is rendered
- [ ] Verify a switchback banner is rendered with roles assumed, and count down of when it expires
- [ ] Verify switchback button says `Logout` and clicking goes back to the login screen

#### Strategy Optional
With the previous role you created from `Strategy Reason`, change `request_access` to `optional`:
- [ ] Verify after login, dashboard is rendered as normal

## Terminal
- [ ] Verify that top nav has a user menu (Main and Logout)
- [ ] Verify that switching between tabs works with `ctrl+[1...9]` (alt on linux/windows)

#### Node List Tab
- [ ] Verify that Cluster selector works (URL should change too)
- [ ] Verify that Quick launcher input works
- [ ] Verify that Quick launcher input handles input errors
- [ ] Verify that "Connect" button shows a list of available logins
- [ ] Verify that "Hostname", "Address" and "Labels" columns show the current values
- [ ] Verify that "Search" by hostname, address, labels work
- [ ] Verify that new tab is created when starting a session

#### Session Tab
- [ ] Verify that session and browser tabs both show the title with login and node name
- [ ] Verify that terminal resize works
    - Install midnight commander on the node you ssh into: `$ sudo apt-get install mc`
    - Run the program: `$ mc`
    - Resize the terminal to see if panels resize with it
- [ ] Verify that session tab shows/updates number of participants when a new user joins the session
- [ ] Verify that tab automatically closes on "$ exit" command
- [ ] Verify that SCP Upload works
- [ ] Verify that SCP Upload handles invalid paths and network errors
- [ ] Verify that SCP Download works
- [ ] Verify that SCP Download handles invalid paths and network errors

## Session Player
- [ ] Verify that it can replay a session
- [ ] Verify that when playing, scroller auto scrolls to bottom most content
- [ ] Verify when resizing player to a small screen, scroller appears and is working
- [ ] Verify that error message is displayed (enter an invalid SID in the URL)

## Invite and Reset Form
- [ ] Verify that input validates
- [ ] Verify that invite works with 2FA disabled
- [ ] Verify that invite works with OTP enabled
- [ ] Verify that invite works with U2F enabled
- [ ] Verify that invite works with WebAuthn enabled
- [ ] Verify that error message is shown if an invite is expired/invalid

## Login Form and Change Password
- [ ] Verify that input validates
- [ ] Verify that login works with 2FA disabled
- [ ] Verify that changing passwords works for 2FA disabled
- [ ] Verify that login works with OTP enabled
- [ ] Verify that changing passwords works for OTP enabled
- [ ] Verify that login works with U2F enabled
- [ ] Verify that changing passwords works for U2F enabled
- [ ] Verify that login works with WebAuthn enabled
- [ ] Verify that changing passwords works for WebAuthn enabled
- [ ] Verify that login works for Github/SAML/OIDC
- [ ] Verify that redirect to original URL works after successful login
- [ ] Verify that account is locked after several unsuccessful login attempts
- [ ] Verify that account is locked after several unsuccessful change password attempts

## Multi-factor Authentication (mfa)
Create/modify `teleport.yaml` and set the following authentication settings under `auth_service`

```yaml
authentication:
  type: local
  second_factor: optional
  require_session_mfa: yes
  webauthn:
    rp_id: example.com
```

#### MFA invite, login, password reset, change password
- [ ] Verify during invite/reset, second factor list all auth types: none, hardware key, and authenticator app
- [ ] Verify registration works with all option types
- [ ] Verify login with all option types
- [ ] Verify changing password with all option types
- [ ] Change `second_factor` type to `on` and verify that mfa is required (no option `none` in dropdown)

#### MFA require auth
Go to `Account Settings` > `Two-Factor Devices` and register a new device

Using the same user as above:
- [ ] Verify logging in with registered WebAuthn key works
- [ ] Verify connecting to a ssh node prompts you to tap your registered WebAuthn key
- [ ] Verify in the web terminal, you can scp upload/download files

#### MFA Management

- [ ] Verify adding first device works without requiring re-authentication
- [ ] Verify re-authenticating with a WebAuthn device works
- [ ] Verify re-authenticating with a U2F device works
- [ ] Verify re-authenticating with a OTP device works
- [ ] Verify adding a WebAuthn device works
- [ ] Verify adding a U2F device works
- [ ] Verify adding an OTP device works
- [ ] Verify removing a device works
- [ ] Verify `second_factor` set to `off` disables adding devices

#### Passwordless

- [ ] Pure passwordless registrations and resets are possible
- [ ] Verify adding a passwordless device (WebAuthn)
- [ ] Verify passwordless logins

## Cloud
From your cloud staging account, change the field `teleportVersion` to the test version.
```
$ kubectl -n <namespace> edit tenant
```

#### Recovery Code Management

- [ ] Verify generating recovery codes for local accounts with email usernames works
- [ ] Verify local accounts with non-email usernames are not able to generate recovery codes
- [ ] Verify SSO accounts are not able to generate recovery codes

#### Invite/Reset
- [ ] Verify email as usernames, renders recovery codes dialog
- [ ] Verify non email usernames, does not render recovery codes dialog

#### Recovery Flow: Add new mfa device
- [ ] Verify recovering (adding) a new hardware key device with password
- [ ] Verify recovering (adding) a new otp device with password
- [ ] Verify viewing and deleting any old device (but not the one just added)
- [ ] Verify new recovery codes are rendered at the end of flow

#### Recovery Flow: Change password
- [ ] Verify recovering password with any mfa device
- [ ] Verify new recovery codes are rendered at the end of flow

#### Recovery Email
- [ ] Verify receiving email for link to start recovery
- [ ] Verify receiving email for successfully recovering
- [ ] Verify email link is invalid after successful recovery
- [ ] Verify receiving email for locked account when max attempts reached

## RBAC
Create a role, with no `allow.rules` defined:
```
kind: role
metadata:
  name: rbac
spec:
  allow:
    app_labels:
      '*': '*'
    logins:
    - root
    node_labels:
      '*': '*'
  options:
    max_session_ttl: 8h0m0s
version: v3
```
- [ ] Verify that a user has access only to: "Servers", "Applications", "Databases", "Kubernetes", "Active Sessions", "Access Requests" and "Manage Clusters"
- [ ] Verify there is no `Add Server, Application, Databases, Kubernetes` button in each respective view
- [ ] Verify only `Servers`, `Apps`, `Databases`, and `Kubernetes` are listed under `options` button in `Manage Clusters`

Note: User has read/create access_request access to their own requests, despite resource settings

Add the following under `spec.allow.rules` to enable read access to the audit log:
```
  - resources:
      - event
      verbs:
      - list
```
- [ ] Verify that the `Audit Log` and `Session Recordings` is accessible
- [ ] Verify that playing a recorded session is denied

Add the following to enable read access to recorded sessions
```
  - resources:
      - session
      verbs:
      - read
```
- [ ] Verify that a user can re-play a session (session.end)

Add the following to enable read access to the roles

```
- resources:
      - role
      verbs:
      - list
      - read
```
- [ ] Verify that a user can see the roles
- [ ] Verify that a user cannot create/delete/update a role

Add the following to enable read access to the auth connectors

```
- resources:
      - auth_connector
      verbs:
      - list
      - read
```
- [ ] Verify that a user can see the list of auth connectors.
- [ ] Verify that a user cannot create/delete/update the connectors

Add the following to enable read access to users
```
  - resources:
      - user
      verbs:
      - list
      - read
```
- [ ] Verify that a user can access the "Users" screen
- [ ] Verify that a user cannot reset password and create/delete/update a user

Add the following to enable read access to trusted clusters
```
  - resources:
      - trusted_cluster
      verbs:
      - list
      - read
```
- [ ] Verify that a user can access the "Trust" screen
- [ ] Verify that a user cannot create/delete/update a trusted cluster.


## Performance

Perform all tests on the following configurations:

- [ ] With default networking configuration
- [ ] With Proxy Peering Enabled
- [ ] With TLS Routing Enabled

* Cluster with 10K direct dial nodes: 
 - [ ] etcd
 - [ ] DynamoDB
 - [ ] Firestore

* Cluster with 10K reverse tunnel nodes:
 - [ ] etcd
 - [ ] DynamoDB
 - [ ] Firestore

* Cluster with 500 trusted clusters:
- [ ] etcd
- [ ] DynamoDB
- [ ] Firestore

### Soak Test

Run 30 minute soak test with a mix of interactive/non-interactive sessions for both direct and reverse tunnel nodes:

```shell
tsh bench --duration=30m user@direct-dial-node ls
tsh bench -i --duration=30m user@direct-dial-node ps uax

tsh bench --duration=30m user@reverse-tunnel-node ls
tsh bench -i --duration=30m user@reverse-tunnel-node ps uax
```

Observe prometheus metrics for goroutines, open files, RAM, CPU, Timers and make sure there are no leaks

- [ ] Verify that prometheus metrics are accurate.

### Concurrent Session Test

* Cluster with 1k reverse tunnel nodes

Run a concurrent session test that will spawn 5 interactive sessions per node in the cluster:

```shell
tsh bench sessions --max=5000 user ls
tsh bench sessions --max=5000 --web user ls 
```

- [ ] Verify that all 5000 sessions are able to be established.
- [ ] Verify that tsh and the web UI are still functional.

## Teleport with Cloud Providers

### AWS

- [ ] Deploy Teleport to AWS. Using DynamoDB & S3
- [ ] Deploy Teleport Enterprise to AWS. Using HA Setup https://gravitational.com/teleport/docs/aws-terraform-guide/

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
- [ ] Verify [CLI access](https://goteleport.com/docs/application-access/guides/api-access/) with `tsh app login`.
- [ ] Verify AWS console access.
  - [ ] Can log into AWS web console through the web UI.
  - [ ] Can interact with AWS using `tsh aws` commands.
- [ ] Verify dynamic registration.
  - [ ] Can register a new app using `tctl create`.
  - [ ] Can update registered app using `tctl create -f`.
  - [ ] Can delete registered app using `tctl rm`.
- [ ] Test Applications screen in the web UI (tab is located on left side nav on dashboard):
  - [ ] Verify that all apps registered are shown
  - [ ] Verify that clicking on the app icon takes you to another tab
  - [ ] Verify using the bash command produced from `Add Application` dialogue works (refresh app screen to see it registered)

## Database Access

- [ ] Connect to a database within a local cluster.
  - [ ] Self-hosted Postgres.
  - [ ] Self-hosted MySQL.
  - [ ] Self-hosted MariaDB.
  - [ ] Self-hosted MongoDB.
  - [ ] Self-hosted CockroachDB.
  - [ ] Self-hosted Redis.
  - [ ] Self-hosted Redis Cluster.
  - [ ] Self-hosted MSSQL.
  - [ ] AWS Aurora Postgres.
  - [ ] AWS Aurora MySQL.
  - [ ] AWS Redshift.
  - [ ] AWS ElastiCache.
  - [ ] AWS MemoryDB.
  - [ ] GCP Cloud SQL Postgres.
  - [ ] GCP Cloud SQL MySQL.
  - [ ] Snowflake.
  - [ ] Azure Cache for Redis.
  - [ ] Elasticsearch.
  - [ ] Cassandra/ScyllaDB.
- [ ] Connect to a database within a remote cluster via a trusted cluster.
  - [ ] Self-hosted Postgres.
  - [ ] Self-hosted MySQL.
  - [ ] Self-hosted MariaDB.
  - [ ] Self-hosted MongoDB.
  - [ ] Self-hosted CockroachDB.
  - [ ] Self-hosted Redis.
  - [ ] Self-hosted Redis Cluster.
  - [ ] Self-hosted MSSQL.
  - [ ] AWS Aurora Postgres.
  - [ ] AWS Aurora MySQL.
  - [ ] AWS Redshift.
  - [ ] AWS ElastiCache.
  - [ ] AWS MemoryDB.
  - [ ] GCP Cloud SQL Postgres.
  - [ ] GCP Cloud SQL MySQL.
  - [ ] Snowflake.
  - [ ] Azure Cache for Redis.
  - [ ] Elasticsearch.
  - [ ] Cassandra/ScyllaDB.
- [ ] Verify audit events.
  - [ ] `db.session.start` is emitted when you connect.
  - [ ] `db.session.end` is emitted when you disconnect.
  - [ ] `db.session.query` is emitted when you execute a SQL query.
- [ ] Verify RBAC.
  - [ ] `tsh db ls` shows only databases matching role's `db_labels`.
  - [ ] Can only connect as users from `db_users`.
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
- [ ] Verify discovery.
    - [ ] AWS
      - [ ] Can detect and register RDS instances.
      - [ ] Can detect and register Aurora clusters, and their reader and custom endpoints.
      - [ ] Can detect and register Redshift clusters.
      - [ ] Can detect and register ElastiCache Redis clusters.
      - [ ] Can detect and register MemoryDB clusters.
    - [ ] Azure
      - [ ] Can detect and register MySQL and Postgres instances.
      - [ ] Can detect and register Azure Cache for Redis servers.
- [ ] Verify Teleport managed users (password rotation, auto 'auth' on connection, etc.).
  - [ ] Can detect and manage ElastiCache users
  - [ ] Can detect and manage MemoryDB users 
- [ ] Test Databases screen in the web UI (tab is located on left side nav on dashboard):
  - [ ] Verify that all dbs registered are shown with correct `name`, `description`, `type`, and `labels`
  - [ ] Verify that clicking on a rows connect button renders a dialogue on manual instructions with `Step 2` login value matching the rows `name` column
  - [ ] Verify searching for all columns in the search bar works
  - [ ] Verify you can sort by all columns except `labels`
- [ ] Other
  - [ ] MySQL server version reported by Teleport is correct.

## TLS Routing

- [ ] Verify that teleport proxy `v2` configuration starts only a single listener.
  ```
  version: v2
  teleport:
    proxy_service:
      enabled: "yes"
      public_addr: ['root.example.com']
      web_listen_addr: 0.0.0.0:3080
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
    - [ ] Cassandra/ScyllaDB.
  - [ ] Verify connecting to a database through TLS ALPN SNI local proxy `tsh db proxy` with a GUI client.
  - [ ] Verify tsh proxy db with teleport proxy behind ALB.
- [ ] Application Access
  - [ ] Verify app access through proxy running in `multiplex` mode
- [ ] SSH Access
  - [ ] Connect to a OpenSSH server through a local ssh proxy `ssh -o "ForwardAgent yes" -o "ProxyCommand tsh proxy ssh" user@host.example.com`
  - [ ] Connect to a OpenSSH server on leaf-cluster through a local ssh proxy`ssh -o "ForwardAgent yes" -o "ProxyCommand tsh proxy ssh --user=%r --cluster=leaf-cluster %h:%p" user@node.foo.com`
  - [ ] Verify `tsh ssh` access through proxy running in multiplex mode
- [ ] Kubernetes access:
  - [ ] Verify kubernetes access through proxy running in `multiplex` mode

## Desktop Access

- Direct mode (set `listen_addr`):
  - [ ] Can connect to desktop defined in static `hosts` section.
  - [ ] Can connect to desktop discovered via LDAP
- IoT mode (reverse tunnel through proxy):
  - [ ] Can connect to desktop defined in static `hosts` section.
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
- RBAC
  - [ ] RBAC denies access to a Windows desktop due to labels
  - [ ] RBAC denies access to a Windows desktop with the wrong OS-login.
- Clipboard Support
  - When a user has a role with clipboard sharing enabled and is using a chromium based browser
    - [ ] Going to a desktop when clipboard permissions are in "Ask" mode (aka "prompt") causes the browser to show a prompt when you first click or press a key
    - [ ] The clipboard icon is highlighted in the top bar
    - [ ] After allowing clipboard permission, copy text from local workstation, paste into remote desktop
    - [ ] After allowing clipboard permission, copy text from remote desktop, paste into local workstation
  - When a user has a role with clipboard sharing enabled and is *not* using a chromium based browser
    - [ ] The clipboard icon is not highlighted in the top bar and copy/paste does not work
  - When a user has a role with clipboard sharing *disabled* and is using a chromium and non-chromium based browser (confirm both)
    - [ ] The clipboard icon is not highlighted in the top bar and copy/paste does not work
- Directory Sharing
  - On supported, non-chromium based browsers (Firefox/Safari)
    - [ ] Attempting to share directory shows a dismissible "Unsupported Action" dialog
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
- Per-Session MFA (try webauthn on each of Chrome, Safari, and Firefox; u2f only works with Firefox)
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

## Binaries compatibility

- Verify `tsh` runs on:
  - [ ] Windows 10
  - [ ] MacOS

## Machine ID

### SSH

With a default Teleport instance configured with a SSH node:

- [ ] Verify you are able to create a new bot user with `tctl bots add robot --roles=access`. Follow the instructions provided in the output to start `tbot`
- [ ] Verify you are able to connect to the SSH node using openssh with the generated `ssh_config` in the destination directory
- [ ] Verify that after the renewal period (default 20m, but this can be reduced via configuration), that newly generated certificates are placed in the destination directory
- [ ] Verify that sending both `SIGUSR1` and `SIGHUP` to a running tbot process causes a renewal and new certificates to be generated
- [ ] Verify that you are able to make a connection to the SSH node using the `ssh_config` provided by `tbot` after each phase of a manual CA rotation.

Ensure the above tests are completed for both:

- [ ] Directly connecting to the auth server
- [ ] Connecting to the auth server via the proxy reverse tunnel

### DB Access

With a default Postgres DB instance, a Teleport instance configured with DB access and a bot user configured:

- [ ] Verify you are able to connect to and interact with a database using `tbot db` while `tbot start` is running

## Teleport Connect

- Auth methods
  - Verify that the app supports clusters using different auth settings
    (`auth_service.authentication` in the cluster config):
    - [ ] `type: local`, `second_factor: "off"`
    - [ ] `type: local`, `second_factor: "otp"`
    - [ ] `type: local`, `second_factor: "webauthn"`,
    - [ ] `type: local`, `second_factor: "webauthn"`, log in passwordlessly with hardware key
    - [ ] `type: local`, `second_factor: "webauthn"`, log in passwordlessly with touch ID
    - [ ] `type: local`, `second_factor: "optional"`, log in without MFA
    - [ ] `type: local`, `second_factor: "optional"`, log in with OTP
    - [ ] `type: local`, `second_factor: "optional"`, log in with hardware key
    - [ ] `type: local`, `second_factor: "on"`, log in with OTP
    - [ ] `type: local`, `second_factor: "on"`, log in with hardware key
    - [Authentication connectors](https://goteleport.com/docs/setup/reference/authentication/#authentication-connectors):
      - For those you might want to use clusters that are deployed on the web, specified in parens.
        Or set up the connectors on a local enterprise cluster following [the guide from our wiki](https://gravitational.slab.com/posts/quick-git-hub-saml-oidc-setup-6dfp292a).
      - [ ] GitHub (asteroid)
        - [ ] local login on a GitHub-enabled cluster
      - [ ] SAML (platform cluster)
      - [ ] OIDC (e-demo)
- Shell
  - [ ] Verify that the shell is pinned to the correct cluster (for root clusters and leaf clusters).
    - That is, opening new shell sessions in other workspaces or other clusters within the same
      workspace should have no impact on the original shell session.
  - [ ] Verify that the local shell is opened with correct env vars.
    - `TELEPORT_PROXY` and `TELEPORT_CLUSTER` should pin the session to the correct cluster.
    - `TELEPORT_HOME` should point to `~/Library/Application Support/Teleport Connect/tsh`.
    - `PATH` should include `/Applications/Teleport Connect.app/Contents/Resources/bin`.
  - [ ] Verify that the working directory in the tab title is updated when you change the directory
        (only for local terminals).
  - [ ] Verify that terminal resize works for both local and remote shells.
    - Install midnight commander on the node you ssh into: `$ sudo apt-get install mc`
    - Run the program: `$ mc`
    - Resize Teleport Connect to see if the panels resize with it
  - [ ] Verify that the tab automatically closes on `$ exit` command.
- State restoration
  - [ ] Verify that the app asks about restoring the previous tabs when launched and restores them
        properly.
  - [ ] Verify that the app opens with the cluster that was active when you closed the app.
  - [ ] Verify that the app remembers size & position after restart.
  - [ ] Verify that [reopening a cluster that has no workspace assigned](https://github.com/gravitational/webapps.e/issues/275#issuecomment-1131663575)
        works.
  - [ ] Verify that reopening the app after removing `~/Library/Application Support/Teleport Connect/tsh`
        doesn't crash the app.
  - [ ] Verify that reopening the app after removing `~/Library/Application Support/Teleport Connect/app_state.json`
        but not the `tsh` dir doesn't crash the app.
  - [ ] Verify that logging out of a cluster and then logging in to the same cluster doesn't
        remember previous tabs (they should be cleared on logout).
- Connections picker
  - [ ] Verify that the connections picker shows new connections when ssh & db tabs are opened.
  - [ ] Check if those connections are available after the app restart.
  - [ ] Check that those connections are removed after you log out of the root cluster that they
        belong to.
  - [ ] Verify that reopening a db connection from the connections picker remembers last used port.
- Cluster resources (servers/databases)
  - [ ] Verify that the app shows the same resources as the Web UI.
  - [ ] Verify that search is working for the resources lists.
  - [ ] Verify that you can connect to these resources.
  - [ ] Verify that clicking "Connect" shows available logins and db usernames.
    - Logins and db usernames are taken from the role, under `spec.allow.logins` and
      `spec.allow.db_users`.
  - [ ] Repeat the above steps for resources in leaf clusters.
  - [ ] Verify that tabs have correct titles set.
  - [ ] Verify that the port number remains the same for a db connection between app restarts.
  - [ ] Create a db connection, close the app, run `tsh proxy db` with the same port, start the app.
        Verify that the app doesn't crash and the db connection tab shows you the error (address in
        use) and offers a way to retry creating the connection.
- Shortcuts
  - [ ] Verify that switching between tabs works on `Cmd+[1...9]`.
  - [ ] Verify that other shortcuts are shown after you close all tabs.
  - [ ] Verify that the other shortcuts work and each of them is shown on hover on relevant UI
        elements.
- Workspaces
  - [ ] Verify that logging in to a new cluster adds it to the identity switcher and switches to the
        workspace of that cluster automatically.
  - [ ] Verify that the state of the current workspace is preserved when you change the workspace (by
        switching to another cluster) and return to the previous workspace.
- Command bar & autocomplete
  - Do the steps for the root cluster, then switch to a leaf cluster and repeat them.
  - [ ] Verify that the autocomplete for tsh ssh filters SSH logins and autocompletes them.
  - [ ] Verify that the autocomplete for tsh ssh filters SSH hosts by name and label and
        autocompletes them.
  - [ ] Verify that launching an invalid tsh ssh command shows the error in a new tab.
  - [ ] Verify that launching a valid tsh ssh command opens a new tab with the session opened.
  - [ ] Verify that the autocomplete for tsh proxy db filters databases by name and label and
        autocompletes them.
  - [ ] Verify that launching a tsh proxy db command opens a new local shell with the command
        running.
  - [ ] Verify that the autocomplete for tsh ssh doesn't break when you cut/paste commands in
        various points.
  - [ ] Verify that manually typing out what the autocomplete would suggest doesn't break the
        command bar.
  - [ ] Verify that launching any other command that's not supported by the autocomplete opens a new
        local shell with that command running.
- Resilience when resources become unavailable
  - For each scenario, create at least one tab for each available kind (minus k8s for now).
  - For each scenario, first do the external action, then click "Sync" on the relevant cluster tab.
    Verify that no unrecoverable error was raised. Then restart the app and verify that it was
    restarted gracefully (no unrecoverable error on restart, the user can continue using the app).
    * [ ] Stop the root cluster.
    * [ ] Stop a leaf cluster.
    * [ ] Disconnect your device from the internet.
- Refreshing certs
  - To test scenarios from this section, create a user with a role that has TTL of `1m`
    (`spec.options.max_session_ttl`).
  - Log in, create a db connection and run the CLI command; wait for the cert to expire, click
    "Sync" on the cluster tab.
    - Verify that after successfully logging in:
      - [ ] the cluster info is synced
      - [ ] the connection in the running CLI db client wasn't dropped; try executing `select
            now();`, the client should be able to automatically reinstantiate the connection.
      - [ ] the database proxy is able to handle new connections; click "Run" in the db tab and see
            if it connects without problems. You might need to resync the cluster again in case they
            managed to expire.
    - [ ] Verify that closing the login modal without logging in shows an error related to syncing
      the cluster.
  - Log in; wait for the cert to expire, click "Connect" next to a db in the cluster tab.
    - [ ] Verify that clicking "Connect" and then navigating to a different tab before the request
          completes doesn't show the login modal and instead immediately shows the error.
    - For this one, you might want to use a sever in our Cloud if the introduced latency is high
      enough. Perhaps enabling throttling in dev tools can help too.
  - [ ] Log in; create two db connections, then remove access to one of the db servers for that
    user; wait for the cert to expire, click "Sync", verify that the db tab with no access shows an
    appropriate error and that the other db tab still handles old and new connections.
- [ ] Verify that logs are collected for all processes (main, renderer, shared, tshd) under
  `~/Library/Application\ Support/Teleport\ Connect/logs`.
- [ ] Verify that the password from the login form is not saved in the renderer log.
- [ ] Log in to a cluster, then log out and log in again as a different user. Verify that the app
  works properly after that.

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
  - [ ] Application access through curl with `tsh app login`
  - [ ] `kubectl get po` after `tsh kube login`
  - [ ] Database access (no configuration change should be necessary if the database CA isn't rotated, other Teleport functionality should not be affected if only the database CA is rotated)

## EC2 Discovery

[EC2 Discovery docs](https://goteleport.com/docs/ver/11.0/server-access/guides/ec2-discovery/)

- Verify EC2 instance discovery
  - [ ]  Only EC2 instances matching given AWS tags have the installer executed on them
  - [ ]  Only the IAM permissions mentioned in the discovery docs are required for operation
  - [ ]  Custom scripts specified in different matchers are executed
  - [ ] Custom SSM documents specified in different matchers are executed
  - [ ] New EC2 instances with matching AWS tags are discovered and added to the teleport cluster
    - [ ] Large numbers of EC2 instances (51+) are all successfully added to the cluster
  - [ ] Nodes that have been discovered do not have the install script run on the node multiple times


## Resources

[Quick GitHub/SAML/OIDC Setup Tips]

<!---
reference style links
-->
[Quick GitHub/SAML/OIDC Setup Tips]: https://gravitational.slab.com/posts/quick-git-hub-saml-oidc-setup-6dfp292a
