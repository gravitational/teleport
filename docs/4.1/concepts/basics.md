# Teleport Basics

This doc will introduce the basic concepts of teleport so you can get started managing access!

First, get to know the components of Teleport. Teleport uses three different services which work together to make Teleport go: **Nodes**, **Auth**, and **Proxy**. It will be important to understand what each of these services do.

- **Teleport Nodes** are stateless servers which can be accessed remotely via Teleport Auth. The Teleport Node service runs on a machine and is similar to the `sshd` daemon you may be familiar with. In some setups it might make sense to replace `sshd` entirely. Users can log in to a Node with the regular ol' `ssh` or `tsh ssh` or via a web browser through the Teleport Proxy UI.
- **Teleport Auth** stores user account data and does authentication and authorization for every Node and every user in a cluster. The Auth Service also acts as the certificate authority (CA) of the cluster. The Auth service maintains state using a database of users, credentials, certificates, and audit logs. The default storage location is `/var/lib/teleport` or an [admin-configured storage destination](../guides/production#storage).
- **Teleport Proxy** receives all user requests and forwards data to the other two Teleport services: Node and Auth. The Teleport Proxy forwards user credentials to the Auth Service and creates connections to a requested Node after successful authentication. The Proxy also serves a Web UI which can be used to view Nodes and Users,and interact with recorded and active sessions.

## **Teleport Overview**

![Teleport Overview](img/overview.svg)

The teleport daemon calls services "roles" on the CLI.

_Note: the `--roles` flag has no relationship to concept of User Roles or permissions._

## Concepts

Next let's define the key concepts you will use in teleport.

|Concept                  | Description
|------------------|------------
| Node             | A node is a "server", "host" or "computer". Users can create shell sessions to access nodes remotely.
| User             | A user represents someone (a person) or something (a machine) who can perform a set of operations on a node.
| Cluster          | A cluster is a group of nodes that work together and can be considered a single system. Cluster nodes can create connections to each other, often over a private network. Cluster nodes often require TLS authentication to ensure that communication between nodes remains secure and comes from a trusted source.
| Certificate Authority (CA) | A Certificate Authority issues digital certificates in the form of public/private keypairs.
| Teleport's CA | Teleport operates two internal CA as a part of the Auth service. One is used to sign Teleport User public keys and the other signs Teleport Node public key. Each certificate is used to prove identity and manage access to Teleport Nodes.
| Teleport Node    | A Teleport Node is a regular node that is running the Teleport Node service. Teleport Nodes can be accessed by authorized Teleport Users via SSH. A Teleport Node is also considered a member of a Teleport Cluster.
| Teleport User    | A Teleport User is an account representing a someone who needs access to a Teleport Cluster. User data is either stored locally or in an external store.
| Teleport Cluster | A Teleport Cluster is comprised of one or more nodes which hold public keys signed by the same Teleport CA. The Teleport CA cryptographically signs the public key of a node, establishing cluster membership.


<!--| Cluster Name     | Every Teleport cluster must have a name. If a name is not supplied via `teleport.yaml` configuration file, a GUID will be generated. **IMPORTANT:** renaming a cluster invalidates its keys and all certificates it had created.
| Trusted Cluster | Teleport Auth Service can allow 3rd party users or nodes to connect if their public keys are signed by a trusted CA. A "trusted cluster" is a pair of public keys of the trusted CA. It can be configured via `teleport.yaml` file.-->

<!--Teleport Users are defined for all no level Every Teleport User must be associated with a list of machine-level OS usernames it can authenticate as during a login. This list is called "user mappings".-->

TODO: Closing Remarks, TBD add more content here
