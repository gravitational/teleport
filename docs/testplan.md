## Manual Testing Plan

Below are the items that should be manually tested with each release of Teleport.
These tests should be run on both a fresh install of the version to be released
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

- [ ] Verify that custom PAM environment variables are available as expected.

- [ ] Users
With every user combination, try to login and signup with invalid second factor, invalid password to see how the system reacts.

  - [ ] Adding Users Password Only
  - [ ] Adding Users OTP
  - [ ] Adding Users U2F
  - [ ] Adding Users WebAuthn
  - [ ] Managing MFA devices
    - [ ] Add an OTP device with `tsh mfa add`
    - [ ] Add a U2F device with `tsh mfa add`
    - [ ] Verify that the U2F device works under WebAuthn
    - [ ] Add a WebAuthn device with `tsh mfa add`
    - [ ] List MFA devices with `tsh mfa ls`
    - [ ] Remove an OTP device with `tsh mfa rm`
    - [ ] Remove a U2F device with `tsh mfa rm`
    - [ ] Remove a WebAuthn device with `tsh mfa rm`
    - [ ] Attempt removing the last MFA device on the user
      - [ ] with `second_factor: on` in `auth_service`, should fail
      - [ ] with `second_factor: optional` in `auth_service`, should succeed
  - [ ] Login Password Only
  - [ ] Login with MFA
    - [ ] Add 2 OTP and 2 WebAuthn devices with `tsh mfa add`
    - [ ] Login via OTP
    - [ ] Login via WebAuthn
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
  - [ ] Sessions can be recorded at the proxy
    - [ ] Sessions on remote clusters are recorded in the local cluster
    - [ ] Enable/disable host key checking.

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

- [ ] Interact with a cluster using the Web UI
  - [ ] Connect to a Teleport node
  - [ ] Connect to a OpenSSH node
  - [ ] Check agent forwarding is correct based on role and proxy mode.

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
* [ ] Deploy Teleport Proxy outside of GKE cluster fronting connections to it (use [this script](https://github.com/gravitational/teleport/blob/master/examples/k8s-auth/get-kubeconfig.sh) to generate a kubeconfig)
* [ ] Deploy Teleport Proxy outside of EKS cluster fronting connections to it (use [this script](https://github.com/gravitational/teleport/blob/master/examples/k8s-auth/get-kubeconfig.sh) to generate a kubeconfig)

### Teleport with multiple Kubernetes clusters

Note: you can use GKE or EKS or minikube to run Kubernetes clusters.
Minikube is the only caveat - it's not reachable publicly so don't run a proxy there.

* [ ] Deploy combo auth/proxy/kubernetes_service outside of a Kubernetes cluster, using a kubeconfig
  * [ ] Login with `tsh login`, check that `tsh kube ls` has your cluster
  * [ ] Run `kubectl get nodes`, `kubectl exec -it $SOME_POD -- sh`
  * [ ] Verify that the audit log recorded the above request and session
* [ ] Deploy combo auth/proxy/kubernetes_service inside of a Kubernetes cluster
  * [ ] Login with `tsh login`, check that `tsh kube ls` has your cluster
  * [ ] Run `kubectl get nodes`, `kubectl exec -it $SOME_POD -- sh`
  * [ ] Verify that the audit log recorded the above request and session
* [ ] Deploy combo auth/proxy_service outside of the Kubernetes cluster and kubernetes_service inside of a Kubernetes cluster, connected over a reverse tunnel
  * [ ] Login with `tsh login`, check that `tsh kube ls` has your cluster
  * [ ] Run `kubectl get nodes`, `kubectl exec -it $SOME_POD -- sh`
  * [ ] Verify that the audit log recorded the above request and session
* [ ] Deploy a second kubernetes_service inside of another Kubernetes cluster, connected over a reverse tunnel
  * [ ] Login with `tsh login`, check that `tsh kube ls` has both clusters
  * [ ] Switch to a second cluster using `tsh kube login`
  * [ ] Run `kubectl get nodes`, `kubectl exec -it $SOME_POD -- sh` on the new cluster
  * [ ] Verify that the audit log recorded the above request and session
* [ ] Deploy combo auth/proxy/kubernetes_service outside of a Kubernetes cluster, using a kubeconfig with multiple clusters in it
  * [ ] Login with `tsh login`, check that `tsh kube ls` has all clusters
* [ ] Test Kubernetes screen in the web UI (tab is located on left side nav on dashboard):
  * [ ] Verify that all kubes registered are shown with correct `name` and `labels`
  * [ ] Verify that clicking on a rows connect button renders a dialogue on manual instructions with `Step 2` login value matching the rows `name` column
  * [ ] Verify searching for `name` or `labels` in the search bar works
  * [ ] Verify you can sort by `name` colum

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
    - [ ] G Suite Screenshots are up to date
- [ ] ActiveDirectoy install instructions work
    - [ ] Active Directoy Screenshots are up to date
- [ ] Okta install instructions work
    - [ ] Okta Screenshots are up to date
- [ ] OneLogin install instructions work
    - [ ] OneLogin Screenshots are up to date
- [ ] OIDC install instructions work
    - [ ] OIDC Screenshots are up to date


### Teleport Plugins

- [ ] Test receiving a message via Teleport Slackbot
- [ ] Test receiving a new Jira Ticket via Teleport Jira

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
- [ ] Verify when there are no connectors, empty state renders
- [ ] Verify that creating OIDC/SAML/GITHUB connectors works
- [ ] Verify that editing  OIDC/SAML/GITHUB connectors works
- [ ] Verify that error is shown when saving an invalid YAML
- [ ] Verify that correct hint text is shown on the right side
- [ ] Verify that encrypted SAML assertions work with an identity provider that supports it (Azure).
- [ ] Verify that created github, saml, oidc card has their icons
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

### Creating Access Rquests
1. Create a role with limited permissions (defined below as `allow-roles`). This role allows you to see the Role screen and ssh into all nodes.
1. Create another role with limited permissions (defined below as `allow-users`). This role session expires in 4 minutes, allows you to see Users screen, and denies access to all nodes.
1. Create another role with no permissions other than being able to create requests (defined below as `default`)
1. Create a user with role `default` assigned
1. Create a few requests under this user to test pending/approved/denied state.
```
kind: role
metadata:
  name: allow-roles
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
version: v3
```
```
kind: role
metadata:
  name: allow-users-short-ttl
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
version: v3
```
```
kind: role
metadata:
  name: default
spec:
  allow:
    request:
      roles:
      - allow-roles
      - allow-users
      suggested_reviewers:
      - random-user-1
      - random-user-2
  options:
    max_session_ttl: 8h0m0s
version: v3
```
- [ ] Verify that under requestable roles, only `allow-roles` and `allow-users` are listed
- [ ] Verify input validation requires at least one role to be selected
- [ ] Verify you can select/input/modify reviewers
- [ ] Verify after creating a request, requests are listed in pending states
- [ ] Verify you can't review own requests

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
- [ ] Verify there is list of reviewers you selected (empty list if none selected AND suggested_reviewers wasn't defined)
- [ ] Verify threshold name is there (it will be `default` if thresholds weren't defined in role, or blank if not named)
- [ ] Verify you can approve a request with message, and immediately see updated state with your review stamp (green checkmark) and message box
- [ ] Verify you can deny a request, and immediately see updated state with your review stamp (red cross)
- [ ] Verify deleting the denied request is removed from list

### Assuming Approved Requests
- [ ] Verify assume buttons are only present for approved request and for logged in user
- [ ] Verify that assuming `allow-roles` allows you to see roles screen and ssh into nodes
- [ ] Verify that after clicking on the assume button, it is disabled in both the list and in viewing
- [ ] After assuming `allow-roles`, verify that assuming `allow-users-short-ttl` allows you to see users screen, and denies access to nodes
  - [ ] Verify a switchback banner is rendered with roles assumed, and count down of when it expires
  - [ ] Verify `switching back` goes back to your default static role
  - [ ] Verify after re-assuming `allow-users-short-ttl` role, the user is automatically logged out after the expiry is met (4 minutes)
- [ ] Verify that after logging out (or getting logged out automatically) and relogging in, permissions are reset to `default`, and requests that are not expired and are approved are assumable again

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
- [ ] Verify that switching between tabs works on alt+[1...9]

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
- [ ] Verify that error message is displayed (enter a invalid SID in the URL)

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


## Performance/Soak Test

Using `tsh bench` tool, perform the soak tests and benchmark tests on the following configurations:

* Cluster with 10K nodes in normal (non-IOT) node mode with ETCD
* Cluster with 10K nodes in normal (non-IOT) mode with DynamoDB

* Cluster with 1K IOT nodes with ETCD
* Cluster with 1K IOT nodes with DynamoDB

* Cluster with 500 trusted clusters with ETCD
* Cluster with 500 trusted clusters with DynamoDB

**Soak Tests**

Run 4hour soak test with a mix of interactive/non-interactive sessions:

```
tsh bench --duration=4h user@teleport-monster-6757d7b487-x226b ls
tsh bench -i --duration=4h user@teleport-monster-6757d7b487-x226b ps uax
```

Observe prometheus metrics for goroutines, open files, RAM, CPU, Timers and make sure there are no leaks

- [ ] Verify that prometheus metrics are accurate.

**Breaking load tests**

Load system with tsh bench to the capacity and publish maximum numbers of concurrent sessions with interactive
and non interactive tsh bench loads.


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
  - [ ] Self-hosted MongoDB.
  - [ ] Self-hosted CockroachDB.
  - [ ] AWS Aurora Postgres.
  - [ ] AWS Aurora MySQL.
  - [ ] AWS Redshift.
  - [ ] GCP Cloud SQL Postgres.
  - [ ] GCP Cloud SQL MySQL.
- [ ] Connect to a database within a remote cluster via a trusted cluster.
  - [ ] Self-hosted Postgres.
  - [ ] Self-hosted MySQL.
  - [ ] Self-hosted MongoDB.
  - [ ] Self-hosted CockroachDB.
  - [ ] AWS Aurora Postgres.
  - [ ] AWS Aurora MySQL.
  - [ ] AWS Redshift.
  - [ ] GCP Cloud SQL Postgres.
  - [ ] GCP Cloud SQL MySQL.
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
  - [ ] Can detect and register RDS instances.
  - [ ] Can detect and register Aurora clusters, and their reader and custom endpoints.
- [ ] Test Databases screen in the web UI (tab is located on left side nav on dashboard):
  - [ ] Verify that all dbs registered are shown with correct `name`, `description`, `type`, and `labels`
  - [ ] Verify that clicking on a rows connect button renders a dialogue on manual instructions with `Step 2` login value matching the rows `name` column
  - [ ] Verify searching for all columns in the search bar works
  - [ ] Verify you can sort by all columns except `labels`

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
    - [ ] MongoDB
    - [ ] CockroachDB
  - [ ] Verify connecting to a database through TLS ALPN SNI local proxy `tsh db proxy` with a GUI client.
- [ ] Application Access
  - [ ] Verify app access through proxy running in `multiplex` mode
- [ ] SSH Access
  - [ ] Connect to a OpenSSH server through a local ssh proxy `ssh -o "ForwardAgent yes" -o "ProxyCommand tsh proxy ssh" user@host.example.com`
  - [ ] Connect to a OpenSSH server on leaf-cluster through a local ssh proxy`ssh -o "ForwardAgent yes" -o "ProxyCommand tsh proxy ssh --user=%r --cluster=leaf-cluster %h:%p" user@node.foo.com`
  - [ ] Verify `tsh ssh` access through proxy running in multiplex mode
- [ ] Kubernetes access:
  - [ ] Verify kubernetes access through proxy running in `multiplex` mode

## Desktop Access

- [ ] Can connect to desktop defined in static `hosts` section.
- [ ] Can connect to desktop discovered via LDAP
- [ ] Download [Keyboard Key Info](https://dennisbabkin.com/kbdkeyinfo/) and verify all keys are processed correctly in each supported browser. Known issues: F11 cannot be captured by the browser without
[special configuration](https://social.technet.microsoft.com/Forums/en-US/784b2bbe-353f-412e-ac9a-193d81f306b6/remote-desktop-for-mac-f11-key-not-working-on-macbook-pro-touchbar?forum=winRDc) on MacOS.
- [ ] Left click and right click register as Windows clicks. (Right click on the
  desktop should show a Windows menu, not a browser context menu)
- [ ] Vertical and horizontal scroll work. [Horizontal Scroll Test](https://codepen.io/jaemskyle/pen/inbmB)
- [ ] All desktops have `teleport.dev/origin` label.
- [ ] Dynamic desktops have additional `teleport.dev` labels for OS, OS Version,
  DNS hostname.
- [ ] Verify that placing a user lock terminates an active desktop session.
- [ ] Verify desktop session start/end audit events.
- [ ] Regexp-based host labeling applies across all desktops, regardless of
  origin.
- [ ] RBAC denies access to a Windows desktop due to labels
- [ ] RBAC denies access to a Windows desktop with the wrong OS-login.
- [ ] Multiple sessions as different users on the same desktop are allowed.
- [ ] Connect multiple `windows_desktop_service`s to the same Teleport cluster,
  verify that connections to desktops on different AD domains works.
