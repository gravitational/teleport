## Manual Testing Plan

Below are the items that should be manually tested with each release of Teleport.
These tests should be run on both a fresh install of the version to be released
as well as an upgrade of the previous version of Teleport.

- [ ] Adding nodes to a cluster
  - [ ] Adding Nodes via Valid Static Token
  - [ ] Adding Nodes via Valid Short-lived Tokens
  - [ ] Adding Nodes via Invalid Static Token Fails
  - [ ] Adding Nodes via Invalid Short-lived Tokens Fails
  - [ ] Revoking Node Invitation

- [ ] Labels
  - [ ] Static Labels
  - [ ] Dynamic Labels

- [ ] Trusted Clusters
  - [ ] Adding Trusted Cluster Valid Static Token
  - [ ] Adding Trusted Cluster Valid Short-lived Token
  - [ ] Adding Trusted Cluster Invalid Static Token
  - [ ] Adding Trusted Cluster Invalid Short-lived Token
  - [ ] Removing Trusted Cluster

- [ ] RBAC
  - [ ] Successfully connect to node with correct role
  - [ ] Unsuccessfully connect to a role in an in-valid role
  - [ ] Allow/deny role option: SSH agent forwarding
  - [ ] Allow/deny role option: Port forwarding

- [ ] Users
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
    - [ ] Server ID is of the proxy for proxy mode
  - [ ] Exec commands are recorded
  - [ ] `scp` commands are recorded
  - [ ] Subsystem results are recorded

- [ ] Interact with a cluster using `tsh`
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
