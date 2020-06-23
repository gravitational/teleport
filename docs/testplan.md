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

- [ ] Users
With every user combination, try to login and signup with invalid second factor, invalid password to see how the system reacts.

  - [ ] Adding Users Password Only
  - [ ] Adding Users OTP
  - [ ] Adding Users U2F
  - [ ] Login Password Only
  - [ ] Login OTP
  - [ ] Login U2F
  - [ ] Login OIDC
  - [ ] Login SAML
  - [ ] Login GitHub
  - [ ] Deleting Users

- [ ] Backends
  - [ ] Teleport runs with etcd
  - [ ] Teleport runs with dynamodb
  - [ ] Teleport runs with boltdb
  - [ ] Teleport runs with dir

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
    - [ ] Server ID is the ID of the node in regular mode
    - [ ] Server ID is randomly generated for proxy node
  - [ ] Exec commands are recorded
  - [ ] `scp` commands are recorded
  - [ ] Subsystem results are recorded

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
* [ ] Deploy Teleport Proxy outside of GKE cluster fronting connections to it (this feature is not yet supported for EKS)

### Teleport with FIPS mode

* [ ] Perform trusted clusters, Web and SSH sanity check with all teleport components deployed in FIPS mode.

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

### Dashboard

#### Top Nav
- [ ] Verify that user name is displayed.
- [ ] Verify that user menu shows a logout button.

#### Cluster List
- [ ] Verify that root cluster is displayed with the proper label.
- [ ] Verify that "Name", "Version", "Nodes" and "Public URL" shows the correct information.
- [ ] Verify that column sorting works.
- [ ] Verify that cluster "View" button works as a hyperlink by opening a new browser tab.
- [ ] Verify that search works.

### Cluster

#### Top Nav
- [ ] Verify that user name is displayed.
- [ ] Verify that Breadcrumb Navigation has a link to dashboard.
- [ ] Verify that cluster name is displayed.
- [ ] Verify that clicking on Teleport logo navigates to the Dashboard.

#### Side Nav
- [ ] Verify that "Nodes" item is highlighted.
- [ ] Verify that each item has an icon.

#### Nodes
- [ ] Verify that "Nodes" table shows all joined nodes.
- [ ] Verify that "Connect" button shows a list of available logins.
- [ ] Verify that "Hostname", "Address" and "Labels" columns show the current values.
- [ ] Verify that "Search" works.
- [ ] Verify that terminal opens when clicking on one of the available logins.

#### Active Sessions
- [ ] Verify that "empty" state is handled.
- [ ] Verify that it displays the session when session is active.
- [ ] Verify that "Description", "Session ID", "Users", "Nodes" and "Duration" columns show correct values.
- [ ] Verify that "OPTIONS" button allows to join a session.

#### Audit log
- [ ] Verify that Audit log has "Sessions" and "Events" tabs where "Sessions" is active by default.
- [ ] Verify that time range button is shown and works.

#### Audit log (Sessions)
- [ ] Verify that Sessions table shows the correct session values.
- [ ] Verify that Play button opens a session player.
- [ ] Verify that search works (it should do a search on all table cells).

#### Auth Connectors.
- [ ] Verify that creating OIDC/SAML/GITHUB connectors works.
- [ ] Verify that editing  OIDC/SAML/GITHUB connectors works.
- [ ] Verify that error is shown when saving an invalid YAML.
- [ ] Verify that correct hint text is shown on the right side.

  Card Icons
  - [ ] Verify that GITHUB card has github icon
  - [ ] Verify that SAML card has SAML icon
  - [ ] Verify that OIDC card has OIDC icon

#### Roles
- [ ] Verify that roles are shown.
- [ ] Verify that "Create New Role" dialog works.
- [ ] Verify that deleting and editing works.
- [ ] Verify that error is shown when saving an invalid YAML.
- [ ] Verify that correct hint text is shown on the right side.

#### Trusted Clusters
- [ ] Verify that adding/removing a trusted cluster works.
- [ ] Verify that correct hint text is shown on the right side.

#### Help&Support
- [ ] Verify that all URLs work and correct (no 404)

### Account
- [ ] Verify that Account screen is accessibly from the user menu for local users.
- [ ] Verify that changing a local password works (OTP, U2F)

### Terminal
- [ ] Verify that top nav has a user menu (Dashboard and Logout).
- [ ] Verify that switching between tabs works on alt+[1...9].

#### Node List Tab
- [ ] Verify that Cluster selector works (URL should change too)
- [ ] Verify that Quick access input works. It
- [ ] Verify that Quick access input handles input errors.
- [ ] Verify that "Connect" button shows a list of available logins.
- [ ] Verify that "Hostname", "Address" and "Labels" columns show the current values.
- [ ] Verify that "Search" works.
- [ ] Verify that new tab is created when starting a session.

#### Session Tab
- [ ] Verify that session and browser tabs both show the title with login and node name.
- [ ] Verify that terminal resize works (use midnight commander (sudo apt-get install mc))
- [ ] Verify that session tab shows a list of participants when a new user joins the session.
- [ ] Verify that tab automatically closes on "$ exit" command.
- [ ] Verify that SCP Upload works.
- [ ] Verify that SCP Upload handles invalid paths and network errors.
- [ ] Verify that SCP Download works.
- [ ] Verify that SCP Download handles invalid paths and network errors.

### Session Player
- [ ] Verify that it can replay a session.
- [ ] Verify that scrolling behavior.
- [ ] Verify that error message is displayed (use invalid SID)

### Invite Form
- [ ] Verify that input validation.
- [ ] Verify that invite works with 2FA disabled.
- [ ] Verify that invite works with OTP enabled.
- [ ] Verify that invite works with U2F enabled.
- [ ] Verify that error message is shown if an invite is expired/invalid.

### Login Form
- [ ] Verify that input validation.
- [ ] Verify that login works with 2FA disabled.
- [ ] Verify that login works with OTP enabled.
- [ ] Verify that login works with U2F enabled.
- [ ] Verify that login works for Github/SAML/OIDC.
- [ ] Verify that account is locked after several unsuccessful attempts.
- [ ] Verify that redirect to original URL works after successful login.

### RBAC
 Create the following role
```
  spec:
  allow:
    logins:
    - root
    - '{{internal.logins}}'
    node_labels:
      '*': '*'
  deny:
    logins: null
  options:
    cert_format: standard
    forward_agent: true
    max_session_ttl: 30h0m0s
    port_forwarding: true
```
  - [ ] Verify that a user has access only to: "Cluster List", "Nodes", and "Active Sessions".
  - [ ] Verify that a user is redirected to the login page.
  - [ ] Verify that after successful login, a user is redirected to the Node List.

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
tsh bench --duration=4h --threads=10 user@teleport-monster-6757d7b487-x226b ls
tsh bench -i --duration=4h --threads=10 user@teleport-monster-6757d7b487-x226b ps uax
```

Observe prometheus metrics for goroutines, open files, RAM, CPU, Timers and make sure there are no leaks

**Breaking load tests**

Load system with tsh bench to the capacity and publish maximum numbers of concurrent sessions with interactive
and non interactive tsh bench loads.
