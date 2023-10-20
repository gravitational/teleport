---
authors: Tiago Silva (tiago.silva@goteleport.com)
state: draft
---

# RFD 0122 - Robust Access to Kubernetes Clusters

## Required Approvers

* Engineering: @r0mant && @rosstimothy

## What

Extend [RFD 93](./0093-offline-access.md) to support Kubernetes access when there is intermittent or no connectivity to Auth.

## Why

While performing an upgrade of the Auth server or during cluster downtime, there is a period of time when access is restricted.
During this time, users are unable to access any resources that require a connection to Auth. This includes Kubernetes clusters,
databases, and any other resources. This can be problematic for cluster admins who need to perform
an Auth upgrade or need to perform maintenance on the Auth server because they lose the ability to access the cluster/node
during the upgrade or maintenance. If during the upgrade an issue is found that requires a rollback or configuration change, they
are unable to access the cluster/node to perform the rollback or configuration change until the Auth server is back online if
no other authentication methods are configured.

## Details

In the following sections, we will discuss the different scenarios where a connection to Auth is required and the different ways
we can mitigate the impact of Auth downtime.

### Auth Connection Required

The following cluster settings or operations **must** have a connection to Auth server to establish a connection to a node:

#### Kubernetes cluster login `tsh kube login`

```bash
tsh kube login <kube_cluster>|--all
```

When a user attempts to login to a Kubernetes cluster - `tsh kube login <kube_cluster>` dials the Auth server to list the
available Kubernetes clusters using `proto.AuthService/ListResources` to ensure the selected cluster exists and to update
the kubeconfig for it.

#### Kubernetes cluster credentials

```bash
tsh kube credentials --kube-cluster=<kube_cluster> --teleport-cluster=<cluster_name> --proxy=<proxy_addr>
```

Teleport routes all Kubernetes traffic based on the cluster name present in the user's certificate. In order to
route traffic to the correct cluster, `tsh` needs to be able to request a new certificate with the requested cluster name.
Without a connection to Auth, `tsh` cannot request the new certificate and therefore Teleport cannot route traffic to the cluster.

Depending on the configuration of the cluster, roles and target cluster,
`tsh` may be able to use a cached certificate to connect to the cluster.
When a certificate is cached, `tsh` will not attempt to request a new certificate from Auth and will dial directly to the
Teleport proxy using the cached certificate. This is only possible when the following conditions are met:

- No MFA is required for the user or the user has already completed the MFA ceremony using `tsh proxy kube`
- The cached certificate is still valid
- The cached certificate includes the MFA payload (if required)

#### Sync Recording modes

Similarly to RFD 93, `proxy-sync` and `node-sync` recording modes require a persistent connection to Auth to upload session recordings. As a result,
even if the conditions above are met, `kubectl exec` interactive sessions cannot be created without being able to start a connection
for the recordings. This will prevent the connection to the Kubernetes cluster from being established successfully.

#### Per-Session MFA

When Per-Session MFA is enabled either at the cluster or role level, `tsh` requires a connection to Auth to perform the MFA ceremony.
By default, MFA certificates are valid for 1 minute and cached locally. This means that if a user has already completed the MFA ceremony
within the last minute, `tsh` will not need to connect to Auth to perform the MFA ceremony again. However, if the user has not completed
the MFA ceremony within the last minute, `tsh` will need to connect to Auth to perform the MFA ceremony.

MFA ceremonies are performed when a user invokes any `kubectl` command that requires a connection to the cluster.
If the user's certificate for the cluster does not exist or is expired, `tsh` will need to connect to Auth to request a new certificate.

#### Moderated Sessions

Similarly to RFD 93, Teleport Kubernetes access also requires a connection to Auth to create a moderated session.
It's required because the session tracker is created on the Auth server and the participants are appended to the session.
Kubernetes agents constantly update the session tracker to extend the TTL while the session is active. Finally,
once the session is closed, the session tracker is updated again to mark the session as closed.

This means that `kubectl exec -it` (interactive sessions) requires a permanent connection to Auth to create the session and
append the participants to the session. On the other hand, `kubectl exec` (non-interactive sessions) does not require a permanent
connection to Auth because the session is created and closed immediately after the command completes and the session tracker
is never created.

Without Auth connectivity, it is not possible to create or join a moderated session.

#### Web UI

Not applicable to this RFD as the Web UI is not supported for Kubernetes clusters.

#### Strict Locking mode

```yaml
kind: role
version: v5
metadata:
  name: lock-strict
spec:
  options:
    lock: strict
```

When Strict locking mode is enabled, Teleport requires a connection to Auth to ensure that the locks are up to date.
If the locks are not up to date, Teleport will terminate all connections to the cluster and prevent new connections from being established.

Strict locking isn't enabled by default.

### Auth Connection Not Required

### `kubectl`

Attempting to connect to a Kubernetes cluster when the user's certificate is cached locally and does not require MFA should not require a connection to Auth.

The exception to this is when the user tries to create an interactive session (`kubectl exec -it`) because the session is moderated and requires a connection to Auth.

###### Per-Session MFA

With Teleport 13, `tsh proxy kube` is now able to cache MFAs certificates for longer than 1 minute. This means that if a user has already
completed the MFA ceremony within the `max_session_ttl` time, `tsh proxy kube` will not need to connect to Auth to perform the MFA ceremony again
until the `max_session_ttl` has expired.

This feature is useful when Auth server downtime is expected and users can complete the MFA ceremony before Auth downtime.

###### Session Trackers

A new session tracker is created for every interactive session. Kubernetes Agent will attempt to persist the session tracker
but if it cannot, the session will be aborted.

We propose the same solution as RFD 93 to mitigate the impact of Auth downtime on interactive sessions.
If the session tracker cannot be persisted, the agent should check if moderated sessions are required.
If they aren't, the session should be allowed to proceed using a local session tracker resource.

While this does allow access to the node, it will prevent the following:

- the session from appearing in the active sessions list
- the session will not be joinable by other users
- the session recording will not be available until the UploadCompleter processes it

### Teleport Connect

Currently, Teleport Connect leverages the `tsh` client and kubectl to establish a connection to the cluster which means that
the MFA certificates are cached locally but only for 1 minute.

We have plans to move Teleport Connect to `tsh proxy kube` which will allow us to leverage the longer MFA certificate caching
and allow users to complete the MFA ceremony before Auth downtime.

## Testing

Similarly to RFD 93, tests will be added to verify that in each scenario described in this RFD that access to the Kube cluster is as expected.

An integration test will be added to verify that a user with a cached certificate can connect to a cluster when Auth is down. The test consists of the following steps:

1. Start a Teleport Auth server
2. Start a Teleport Proxy server
3. Start a Teleport Kubernetes Agent serving a Kubernetes KinD cluster
4. Create a user certificate for user without moderation requirements (ok)
5. Create a user certificate for user with moderation requirements (fail)
6. Create a user certificate for user with MFA requirements (ok if MFA is cached)
7. Shutdown the Auth server
8. Attempt to connect to the cluster using `kubectl exec`/`kubectl get pods` for each user mentioned

## Security

Allowing non-moderated sessions to continue without the session tracker in the backend means does pose some auditing risk.

