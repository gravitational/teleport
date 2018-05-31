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
tsh --proxy=proxy.example.com --user=<username> --insecure --cluster=foo.com ssh -p 22 node.foo.com
```

## Web UI

- [ ] Creating a new SSH session.
  
    Open a terminal to existing server    
    - [ ] Verify that input/output works as expected by typing on the keyboard.
    - [ ] Verify that participant indicator is shown.
    - [ ] Verify that window resize works.
    - [ ] Verify that terminal is automatically closed after session ends (by typing `exit`)          

    Open a terminal to non-existing server
    - [ ] Verify that connection error is shown to the user.
   
- [ ] Joining an existing SSH session.

    Start a new session then copy the session URL and open it in another tab.

    - [ ] Should be able to join an existing session.
    - [ ] Should be able to see what another participant is typing.
    - [ ] Should display the correct number of participants.
    - [ ] Verify that resize works in both terminals. Use midnight commander (`apt-get install mc`) as it clearly shows the actual screen of the terminal.

    Start a new session and then open the Sessions Page in another tab:

    - [ ] Should see 1 active session and participant.
    - [ ] Should be able to join this session by clicking on "Join" button.

    Join a session that has ended:

    - [ ] Should see a message saying that this session has ended.
    - [ ] Should see 2 buttons: "Start new" and "Replay". Verify that both work as expected.

- [ ] Node List.

    Open the Node List page.
    
    - [ ] Verify that sorting works.
    - [ ] Verify that dynamic labels are getting updated.
    - [ ] Verify that login dropdown control shows the correct list of available logins.
    - [ ] Verify that search works.
    - [ ] Verify that ssh control works (input validation).
    - [ ] Verify that cluster dropdown control shows the correct list of clusters.


- [ ] Session List (stored sessions).

    Open the Sessions page.

    - [ ] Verify that it displays the list of stored sessions.
    - [ ] Should be able to open a session player by clicking on "Play" button.
    - [ ] Verify that all columns display correct information.
    - [ ] Verify that date-picker works.
    - [ ] Verify that search works.

    Create a new session and open the Session List in another browser window.

    - [ ] Verify that this session is shown as active in the Session List.
    - [ ] Verify that "Join" button works.


- [ ] Invite

    Create user invite URL and use this URL to:

    - [ ] Verify that invite works with OTP enabled.
    - [ ] Verify that invite works with U2F enabled.
    - [ ] Verify that invite works with 2nd factor disabled.


- [ ] Expired Invite

    Create invalid or expired invite URL.

    - [ ] Verify that "Invite code has expired" message is displayed.


- [ ] User lock

    Open a login screen.

    - [ ] Verify that a user gets locked after several unsuccessful attempts.


- [ ] Login redirects.

    Log out and then navigate to Nodes list.

    - [ ] Verify that a user is redirected to the login page.
    - [ ] Verify that after successful login, a user is redirected to the Node List.
