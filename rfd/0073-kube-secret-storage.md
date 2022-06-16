---
authors: Tiago Silva (tiago.silva@goteleport.com)
state: draft
---


# RFD 73 - Teleport Kube-Agent credential storage in Kubernetes Native Secrets

## Required Approvers

Engineering @r0mant 
Product:  @klizhentas && @xinding33 


## What

Teleport Kubernetes Agent support for dynamic short-lived tokens relying only on native Kubernetes Secrets for identity storage.

### Related issues

- [#5585](https://github.com/gravitational/teleport/issues/5585)

## Why

When a Teleport Agent wants to join a Teleport Cluster, it needs to share the invite token with the cluster for initial authentication. The invite token can be:

- Short-lived token (the token will expire after a low TTL and cannot be reused after that time).
- Long-lived/static token (the usage of long-lived tokens is discouraging for security reasons).

After sharing the invite token with the Teleport cluster, the agent receives its identity from the Auth service. Identity certificates are mandatory for accessing the Teleport Cluster and must be stored for accesses without reusing the invite token.

Kubernetes Pods are, by definition, expected to be stateless. This means that each time a Pod is recycled because it was restarted, deleted, upgraded, or moved to another node, the state that was written to its filesystem is lost.

One way to overcome this problem is to use Persistent Volumes. PV is a Kubernetes feature that mounts a storage volume in the container filesystem, whose lifecycle is independent of the Pod that mounts it. Kubernetes’ PV storage has its own drawbacks. Persistent Volumes are very distinct between different cloud vendors. Some providers lock volumes to be mounted in the same zone they were created, meaning that if Teleport agent Pod is recycled, it must be maintained in the same zone it was created. This creates operational issues and for on-premises deployments might be difficult to manage.

Another possibility is that the agent might use the invite token each time the pod is recycled, meaning that it issues a join request to the cluster every time it starts and receives a new identity from the Auth service. This means that the invite token must be a static/long-lived token, otherwise after a while the agent could not register himself in the cluster because the invite token expired. This approach is not recommended and might cause security flaws because the join token can be stolen or guessed.

One solution that might address all the issues referenced above is to allow the agent to use Kubernetes secrets as storage backend for process state. This allows not only that the agent is able to run stateless depending only on native objects generally available in any Kubernetes cluster, but also that the agent might support dynamic short-lived invite tokens with no dependency on external storage.

## Details

### Secret creation and lifecycle

The agent creates, reads and updates the Kubernetes Secrets.

The agent will never issue a delete request to destroy the secret. Helm’s `post-delete` hook achieves this once the operator executes `helm uninstall`.

#### Secret content

Once agent updates the secret, it will have the following structure:

```yaml
apiVersion: v1
stringData: 
        
            # {role} might be kube, app,... if multiple roles are defined for the agent then, 
            #multiple entries are created each one holding its identity
            /ids/{role}/current: {
                "kind": "identity",
                "version": "v2",
                "metadata": {
                    "name": "current"
                },
                "spec": {
                    "key": "{key_content}",
                    "ssh_cert": "{ssh_cert_content}",
                    "tls_cert": "{tls_cert_content}",
                    "tls_ca_certs": ["{tls_ca_certs}"],
                    "ssh_ca_certs": ["{ssh_ca_certs}"]
                }
            }

             # State is important if restart/rollback happens during the CA rotation phase.
             # it holds the current status of the rotation
            /states/{role}/state: {
                "kind": "state",
                "version": "v2",
                ...
            }
            
            # during CA rotation phase, the new keys are stored if agent is restarted or rotation has to rollback
            /ids/{role}/replacement: {
                "kind": "identity",
                "version": "v2",
                "metadata": {
                    "name": "replacement"
                },
                "spec": {
                    "key": "{key_content}",
                    "ssh_cert": "{ssh_cert_content}",
                    "tls_cert": "{tls_cert_content}",
                    "tls_ca_certs": ["{tls_ca_certs}"],
                    "ssh_ca_certs": ["{ssh_ca_certs}"]
                }
            }
    
kind: Secret
metadata:
  name: {$RELEASE_NAME}-state-{{$TELEPORT_REPLICA_NAME}}
  namespace: {$KUBE_NAMESPACE}
```

Where:

- `ssh_cert` is a PEM encoded SSH host cert.
- `key` is a PEM encoded private key.
- `tls_cert` is a PEM encoded x509 client certificate.
- `tls_ca_certs` is a list of PEM encoded x509 certificate of the certificate authority of the cluster.
- `ssh_ca_certs` is a list of SSH certificate authorities encoded in the authorized_keys format.
- `role` is the role the agent is operating. If agent is running with multiple roles, i.e. app, kube..., multiple entries will be added, one for each role.
- `TELEPORT_REPLICA_NAME` is the teleport agent replica name. In Kubernetes this variable exposes the POD name `TELEPORT_REPLICA_NAME=metadata.name`.
- `KUBE_NAMESPACE` is the Kubernetes namespace in which the agent was installed. It is exposed by an ENV variable.
- `RELEASE_NAME` is the Helm release name.

#### RBAC Changes

The Teleport agent service account must be able to create, read and update secrets within the namespace, therefore, Helm must create a new namespace role and attach it to the agent service account with the following content:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: { .Release.Name }-secrets
  namespace: { .Release.Namespace }
rules:
- apiGroups: [""]
  # objects is "secrets"
  resources: ["secrets"]
  verbs: ["create", "get", "update","watch", "list"]
```

The RBAC only allows the service account to read and update the secrets created under `{ .Release.Namespace }`.

### Teleport Changes

#### Kube Secret as storage backend

Teleport will automatically detect when it's running in Kubernetes and use Secrets to store the process state. Auth server data, audit events and session recording will be unaffected and keep using their configured backends.

The Kubernetes Secret backend storage is responsible for managing the Kubernetes secret, i.e. creating the secret if it does not exist, reading and updating it.

If the identity secret exists in Kubernetes and has the node identity on it, the storage engine will parse and return the keys to the Agent, so it can use them to authenticate in the Teleport Cluster. If the cluster access operation is successful, the agent will be available for usage, but if the access operation fails because the Teleport Auth does not validate the node credentials, the Agent will log an error providing insightful information about the failure cause.

In case of the identity secret is not present or is empty, the agent will try to join the cluster with the invite token provided. If the invite token is valid (has details in the Teleport Cluster and did not expire yet), Teleport Cluster will reply with the agent identity. Given the identity, the agent will write it in the secret `{$RELEASE_NAME}-state-{{$TELEPORT_REPLICA_NAME}}` for future usage.

Otherwise, if the invite token is not valid or has expired, the Agent could not join the cluster, and it will stop and log a meaningful error message.

The following diagram shows the behavior when using Kubernetes’ Secret backend storage.

```text
                                                              ┌─────────┐                                        ┌────────┐          ┌──────────┐                    
                                                              │KubeAgent│                                        │Teleport│          │Kubernetes│                    
                                                              └────┬────┘                                        └───┬────┘          └────┬─────┘                    
                                                                   ────┐                                             │                    │                          
                                                                       │ init procedure                              │                    │                          
                                                                   <───┘                                             │                    │                          
                                                                   │                                                 │                    │                          
                                                                   │                          Get Secret Data        │                   ┌┴┐                         
                                                                   │────────────────────────────────────────────────────────────────────>│ │                         
                                                                   │                                                 │                   │ │                         
                                                                   │                                                 │                   │ │                         
         ╔══════╤══════════════════════════════════════════════════╪═════════════════════════════════════════════════╪═══════════════════╪═╪════════════════════════╗
         ║ ALT  │  Identity data is present in Secret              │                                                 │                   │ │                        ║
         ╟──────┘                                                  │                                                 │                   │ │                        ║
         ║                                                         │                        returns secret data      │                   │ │                        ║
         ║                                                         │<─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ │ │                        ║
         ║                                                         │                                                 │                   │ │                        ║
         ║                                                         │ Joining the cluster with identity from secret  ┌┴┐                  │ │                        ║
         ║                                                         │───────────────────────────────────────────────>│ │                  │ │                        ║
         ║                                                         │                                                │ │                  │ │                        ║
         ║                                                         │                                                │ │                  │ │                        ║
         ║                   ╔══════╤══════════════════════════════╪════════════════════════════════════════════════╪═╪═════════════╗    │ │                        ║
         ║                   ║ ALT  │  successful case             │                                                │ │             ║    │ │                        ║
         ║                   ╟──────┘                              │                                                │ │             ║    │ │                        ║
         ║                   ║                                     │Node successfully authenticated and registered  │ │             ║    │ │                        ║
         ║                   ║                                     │in the cluster                                  │ │             ║    │ │                        ║
         ║                   ║                                     │<─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─│ │             ║    │ │                        ║
         ║                   ╠═════════════════════════════════════╪════════════════════════════════════════════════╪═╪═════════════╣    │ │                        ║
         ║                   ║ [identity signed by a different Auth server]                                         │ │             ║    │ │                        ║
         ║                   ║                                     │Node identity signed by a different Auth Server │ │             ║    │ │                        ║
         ║                   ║                                     │<─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─│ │             ║    │ │                        ║
         ║                   ║                                     │                                                │ │             ║    │ │                        ║
         ║                   ║      ╔════════════════════════════╗ ────┐                                            │ │             ║    │ │                        ║
         ║                   ║      ║unable to join the cluster ░║     │ failure state.                             │ │             ║    │ │                        ║
         ║                   ║      ║logs the error              ║ <───┘                                            │ │             ║    │ │                        ║
         ║                   ╚══════╚════════════════════════════╝═╪════════════════════════════════════════════════╪═╪═════════════╝    │ │                        ║
         ╠═════════════════════════════════════════════════════════╪═════════════════════════════════════════════════════════════════════╪═╪════════════════════════╣
         ║ [Identity data is not present in Secret]                │                                                 │                   │ │                        ║
         ║                                                         │                           returns error         │                   │ │                        ║
         ║                                                         │<─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ │ │                        ║
         ║                                                         │                                                 │                   │ │                        ║
         ║                                                         │               Sends invite code                ┌┴┐                  │ │                        ║
         ║                                                         │───────────────────────────────────────────────>│ │                  │ │                        ║
         ║                                                         │                                                │ │                  │ │                        ║
         ║                                                         │                                                │ │                  │ │                        ║
         ║         ╔══════╤════════════════════════════════════════╪════════════════════════════════════════════════╪═╪══════════════════╪═╪══════════════╗         ║
         ║         ║ ALT  │  successful case                       │                                                │ │                  │ │              ║         ║
         ║         ╟──────┘                                        │                                                │ │                  │ │              ║         ║
         ║         ║                                               │             returns node identity              │ │                  │ │              ║         ║
         ║         ║                                               │<─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─│ │                  │ │              ║         ║
         ║         ║                                               │                                                │ │                  └┬┘              ║         ║
         ║         ║                                               │                  Updates secret data with Identity                   │               ║         ║
         ║         ║                                               │─────────────────────────────────────────────────────────────────────>│               ║         ║
         ║         ║                                               │                                                │ │                   │               ║         ║
         ║         ║                                               │               joins the cluster                │ │                   │               ║         ║
         ║         ║                                               │───────────────────────────────────────────────>│ │                   │               ║         ║
         ║         ╠═══════════════════════════════════════════════╪════════════════════════════════════════════════╪═╪═══════════════════╪═══════════════╣         ║
         ║         ║ [invite code expired]                         │                                                │ │                   │               ║         ║
         ║         ║                                               │          invalid invite token error            │ │                   │               ║         ║
         ║         ║                                               │<─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─│ │                   │               ║         ║
         ║         ║                                               │                                                │ │                   │               ║         ║
         ║         ║      ╔══════════════════════════════════════╗ ────┐                                            │ │                   │               ║         ║
         ║         ║      ║unable to join the cluster           ░║     │ failure state.                             │ │                   │               ║         ║
         ║         ║      ║because the invite might be expired.  ║ <───┘                                            │ │                   │               ║         ║
         ║         ║      ║logs the error                        ║ │                                                │ │                   │               ║         ║
         ║         ╚══════╚══════════════════════════════════════╝═╪════════════════════════════════════════════════╪═╪═══════════════════╪═══════════════╝         ║
         ╚═════════════════════════════════════════════════════════╪══════════════════════════════════════════════════════════════════════╪═════════════════════════╝
                                                                   │                                                 │                    │                          
                                                                   │                                                 │                    │                          
                                                    ╔═══════╤══════╪═════════════════════════════════════════════════╪════════════════════╪═══════════════╗          
                                                    ║ LOOP  │  CA  rotation                                          │                    │               ║          
                                                    ╟───────┘      │                                                 │                    │               ║          
                                                    ║              │              Rotate certificates                │                    │               ║          
                                                    ║              │<───────────────────────────────────────────────>│                    │               ║          
                                                    ║              │                                                 │                    │               ║          
                                                    ║              │                New certificates                 │                    │               ║          
                                                    ║              │<─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ │                    │               ║          
                                                    ║              │                                                 │                    │               ║          
                                                    ║              │                     Update secret content       │                    │               ║          
                                                    ║              │─────────────────────────────────────────────────────────────────────>│               ║          
                                                    ╚══════════════╪═════════════════════════════════════════════════╪════════════════════╪═══════════════╝          
                                                              ┌────┴────┐                                        ┌───┴────┐          ┌────┴─────┐                    
                                                              │KubeAgent│                                        │Teleport│          │Kubernetes│                    
                                                              └─────────┘                                        └────────┘          └──────────┘                    
```

### Backend storage

Kubernetes Secret storage will be a transparent backend storage that can be plugged into [`ProcessStorage`](https://github.com/gravitational/teleport/blob/1aa38f4bc56997ba13b26a1ef1b4da7a3a078930/lib/auth/state.go#L35) structure. Currently, `ProcessStorage` is responsible for handling the Identity and State storage. It expects a [`backend.Backend`](https://github.com/gravitational/teleport/blob/cc27d91a4585b0d744a7bec110260c335b0007fd/lib/backend/backend.go#L42), but it only requires a subset of it. The idea is to define a new interface `IdentityBackend` with the required methods and use it only for backend storage.

```go
// ProcessStorage is a backend for local process state,
// it helps to manage rotation for certificate authorities
// and keeps local process credentials - x509 and SSH certs and keys.
type ProcessStorage struct {
	identityStorage IdentityBackend
  Backend backend.Backend
}

// IdentityBackend implements abstraction over local or remote storage backend methods
// required for Identity/State storage.
// As in backend.Backend, Item keys are assumed to be valid UTF8, which may be enforced by the
// various Backend implementations.
type IdentityBackend interface {
	// Create creates item if it does not exist
	Create(ctx context.Context, i backend.Item) (*backend.Lease, error)
	// Put puts value into backend (creates if it does not
	// exists, updates it otherwise)
	Put(ctx context.Context, i backend.Item) (*backend.Lease, error)
	// Get returns a single item or not found error
	Get(ctx context.Context, key []byte) (*backend.Item, error)
}
```

During the startup procedure, the agent identifies that it is running inside Kubernetes. The identification can be achieved by checking the presence of the service account mount path `/var/run/secrets/kubernetes.io`. Although Kubernetes has an option that can disable this mount path, `automountServiceAccountToken: false`, this option is always `true` in our helm chart since we require it for handling the secrets via Kubernetes API and if the service account is not present, Teleport must avoid using secrets and fallback to local storage.

If the agent detects that it is running in Kubernetes, it instantiates an identity backend for Kubernetes Secret. This backend creates a client with configuration provided by `restclient.InClusterConfig()`, which uses the service account token mounted in the pod. With this, the agent can operate the secret by creating, updating, and reading the secret data.

```go
// NewProcessStorage returns a new instance of the process storage.
func NewProcessStorage(ctx context.Context, path string) (*ProcessStorage, error) {
  var (
    identityStorage IdentityBackend
    err error
  )

  if path == "" {
    return nil, trace.BadParameter("missing parameter path")
  }

  litebk, err := lite.NewWithConfig(ctx, lite.Config{
    Path:      path,
    EventsOff: true,
    Sync:      lite.SyncFull,
  })

  if kubernetes.InKubeCluster() {
    identityStorage, err = kubernetes.New()
    if err != nil {
      return nil, trace.Wrap(err)
    }
    // TODO: add compatibility layer
  } else {
    identityStorage = litebk
  }

  return &ProcessStorage{Backend: litebk, identityStorage: identityStorage}, nil
}
```

For a compatibility layer, if the secret does not exist in Kubernetes, but locally we have the SQLite database, this means storage is enabled, and the agent had already joined the cluster in the past. Hereby, the agent can inherit the credentials stored in the database and write them in Kubernetes secret ([more details](#upgrade-plans-from-pre-rfd-to-pos-rfd)).

The storage must also store the events of type `kind=state` since they are used during the CA rotation phase. They are helpful if the pod restarts, and has to rollback to the previous identity or finish the process and replace the old identity with the new one.

### CA Rotation

CA rotation feature allows the cluster operator to force the cluster to recreate and re-issue certificates for users and agents.

The CA rotation process is independent of the agent role and storage and has the following behavior: during the procedure, agents and users receive new keys signed by the newest CA from Teleport Auth. While the cluster is transitioning, the keys signed by the old CA are considered valid in order for the agent to be able to receive its new identity certificates from Teleport Auth. During the whole process, the CA rotation can be rollbacked, and if this happens the new certificate becomes invalid. This action can happen because another agent has issues or by decision of the operator. Given this, the new identity keys and old identity keys are stored separately in the process storage until the rotation process finishes successfully.

Once the new identity is received by the agent, it will store its state, indicating it is under a rotation event, and the new identity in the storage. The state under `/states/{role}/state` key and the identity under `/ids/{role}/replacement`. This is mandatory so the agent, after restart, can resume the operation if not finished yet or rollback. If the process fails, the state is rewritten in order to inform the rotation has finished. Otherwise, if the process finishes successfully the process will replace the identity under `/states/{role}/current` with the new identity and replace the state content.

Switching from local to Kubernetes Secret storage will not change the behavior, it just changes the storage location and storage structure.  

### Limitations

High availability can only be achieved using Statefulsets and not by Deployments. This means the [Helm chart](https://github.com/gravitational/teleport/tree/master/examples/chart/teleport-kube-agent) has to switch to Statefulset objects even if previously was running as Deployment. This change is required because at least one invariant must be kept across restarts to correctly map each agent pod and its identity Secret. The invariant used is the Statefulset pod name `{{ .Release.Name }}-{0...replicas}}`.

Given this, it is required to expose the `$TELEPORT_REPLICA_NAME` environment variable to each pod in order to the backend storage be able to write the identity separately. The values for `$TELEPORT_REPLICA_NAME` are populated by Kubernetes with the pod name by setting it to use the `fieldPath: metadata.name` value.

If the action is an upgrade and previously the agent was running as Deployment, Helm chart will keep the Deployment (it upgrades it to the version the operator chooses) in order to maintain the cluster accessible and install, as well, the Statefulset. This means that after a successful installation we might end up with multiple replicas from both Deployment and Statefulset writing into different secrets. Since Pods created from Deployments have random names each time a pod is recreated a new secret is generated creating a lot of garbage. In order to prevent that, when running in Deployment mode, the agent will clean the secret it created using a `preStop` hook.

### Upgrade from PRE-RFD versions

When upgrading from PRE-RFD into versions that implement this RFD, we have two scenarios based on pre-existing configuration.

A description of each case's caveats is available below.

1. Storage was available:  `Statefulset -> Statefulset`
    
    When storage is available, it means that identity and state were previously stored in the local database in PV. This means that the agent is able to read the local file and store that information in a new Kubernetes Secret, deleting the local database once the operation was successful.
    
    Once the secret is stored, the agent no longer requires the PV storage since nothing is stored there. At this point the operator can upgrade the Helm Chart to have `storage.enabled=false`.
    
    A descriptive comment and deprecation must be added to the storage section detailing the upgrade process. First, it's required in order to read the agent identity from local storage, and later it can be disabled.


2. Storage was not available: `Deployment -> Statefulset`

    If storage was not available, Helm chart installed the assets as a Deployment. Due to [limitations](#limitations), the current RFD forces the usage of Statefulset. This means that for this case, if no action is taken Helm chart will switch from Deployment to a Statefulset. Helm manages this change by destroying the Deployment object and creating the new Statefulset. During this transition, even if the operator has the `PodDisruptionBudget` object enabled, the Kubernetes cluster might become inaccessible from Teleport for some time because there is no guarantee that the system has at least one agent replica running. So in order to prevent the downtime, Helm chart will keep the Deployment if it already exists in the cluster, but it will also install in parallel the Statefulset. In order to inform the user of following actions, it will render a custom message to the operator saying that once the Statefulset pods are running he can safely delete the Deployment.
    The message will also contain the two commands necessary to automatically delete the Deployment once the Statefulset pods are running.
    ```bash
        $ kubectl wait --for=condition=available statefulset/{ .Release.Name } -n { .Release.Namespace } --timeout=10m
        $ kubectl delete deployment/{ .Release.Name } -n { .Release.Namespace }
    ```

    Regarding the invite token, since it was running as deployment, it should be using a long-lived/static token as described in the case above, and it should not create any issue when joining the cluster with that invite token if it is still valid.


### Helm Chart Differences

#### File *templates/config.yaml*

```diff
{{- $logLevel := (coalesce .Values.logLevel .Values.log.level "INFO") -}}
{{- if .Values.teleportVersionOverride -}}
  {{- $_ := set . "teleportVersion" .Values.teleportVersionOverride -}}
{{- else -}}
  {{- $_ := set . "teleportVersion" .Chart.Version -}}
{{- end -}}
apiVersion: v1
kind: ConfigMap
metadata:
  name: {{ .Release.Name }}
  namespace: {{ .Release.Namespace }}
{{- if .Values.extraLabels.config }}
  labels:
  {{- toYaml .Values.extraLabels.config | nindent 4 }}
{{- end }}
  {{- if .Values.annotations.config }}
  annotations:
    {{- toYaml .Values.annotations.config | nindent 4 }}
  {{- end }}
data:
  teleport.yaml: |
    teleport:
      auth_token: "/etc/teleport-secrets/auth-token"
      auth_servers: ["{{ required "proxyAddr is required in chart values" .Values.proxyAddr }}"]
```

#### File *templates/deployment.yaml*

```diff
#
# Warning to maintainers, any changes to this file that are not specific to the Deployment need to also be duplicated
# in the statefulset.yaml file.
#
-{{- if not .Values.storage.enabled }}
+{{- $deployment := lookup "v1" "Deployment" .Release.Namespace .Release.Name -}}
+{{- if ( $deployment ) }}
{{- $replicaCount := (coalesce .Values.replicaCount .Values.highAvailability.replicaCount "1") }}
...
-        {{- if .Values.extraEnv }}
-        env:
-          {{- toYaml .Values.extraEnv | nindent 8 }}
-        {{- end }}
+        env:
+          - name: TELEPORT_REPLICA_NAME
+            valueFrom:
+              fieldRef:
+                 fieldPath: metadata.name
+          - name: KUBE_NAMESPACE
+            valueFrom:
+              fieldRef:
+                 fieldPath: metadata.namespace
+          - name: RELEASE_NAME
+            value: {{ .Release.Name }}
+         {{- if .Values.extraEnv }}
+          {{- toYaml .Values.extraEnv | nindent 8 }}
+        {{- end }}
```

#### File *templates/statefulset.yaml*

```diff
#
# Warning to maintainers, any changes to this file that are not specific to the StatefulSet need to also be duplicated
# in the deployment.yaml file.
#
-{{- if .Values.storage.enabled }}
{{- $replicaCount := (coalesce .Values.replicaCount .Values.highAvailability.replicaCount "1") }}
...
-        {{- if .Values.extraEnv }}
-        env:
-          {{- toYaml .Values.extraEnv | nindent 8 }}
-        {{- end }}
+        env:
+          - name: TELEPORT_REPLICA_NAME
+            valueFrom:
+              fieldRef:
+                 fieldPath: metadata.name
+          - name: KUBE_NAMESPACE
+            valueFrom:
+              fieldRef:
+                 fieldPath: metadata.namespace
+          - name: RELEASE_NAME
+            value: {{ .Release.Name }}
+         {{- if .Values.extraEnv }}
+          {{- toYaml .Values.extraEnv | nindent 10 }}
+        {{- end }}

# remove PV storage if storage is not enabled
...
+{{- if .Values.storage.enabled }}
        - mountPath: /var/lib/teleport
          name: "{{ .Release.Name }}-teleport-data"
+{{- end }}
...
+{{- if .Values.storage.enabled }}
  volumeClaimTemplates:
  - metadata:
      name: "{{ .Release.Name }}-teleport-data"
    spec:
      accessModes: [ "ReadWriteOnce" ]
      storageClassName: {{ .Values.storage.storageClassName }}
      resources:
        requests:
          storage: {{ .Values.storage.requests }}
+{{- end }}
{{- end }}
```

#### File *values.yaml*

Change storage comments

#### File *templates/NOTES.txt*

```diff
+{{- $deployment := lookup "v1" "Deployment" .Release.Namespace .Release.Name -}}
+{{- if ( $deployment ) }}
+#################################################################################
+######   WARNING: Statefulset installed and Deployment not removed.         #####
+######            Manual Deployment deletion is required                    #####
+#################################################################################
+
+In order to delete the Deployment wait for statefulset pods to be ready:
+$ kubectl wait --for=condition=available statefulset/{ .Release.Name } -n { .Release.Namespace } --timeout=10m
+
+Finally, delete the deployment with:
+$ kubectl delete deployment/{ .Release.Name } -n { .Release.Namespace }
+
++{{- end}}
+For more information on running Teleport, visit:
+https://goteleport.com/docs
```

## UX

If the agent detects that it is running inside Kubernetes, it will enable, by default, the Kube secret storage instead of local storage. This means that it will not require any additional configuration or operation by the end-user.

To install the agent on Kubernetes using Helm, the command remains the same as before, as one can see below.

```bash
$ helm install/upgrade teleport-kube-agent . \
  --create-namespace \
  --namespace teleport \
  --set roles=kube \
  --set proxyAddr=${PROXY_ENDPOINT?} \
  --set authToken=${JOIN_TOKEN?} \
  --set kubeClusterName=${KUBERNETES_CLUSTER_NAME?}
```

If the Deployment is kept, Helm will show the following data in operator's console after the installation finishes. The message warns him that a manual action is required to delete the Deployment.

```text
#################################################################################
######   WARNING: Statefulset installed and Deployment not removed.         #####
######            Manual Deployment deletion is required                    #####
#################################################################################

In order to delete the Deployment wait for statefulset pods to be ready:
$ kubectl wait --for=condition=available statefulset/teleport-kube-agent -n teleport --timeout=10m

Finally, delete the deployment with:
$ kubectl delete deployment/teleport-kube-agent -n teleport

For more information on running Teleport, visit:
https://goteleport.com/docs
```

## Security

The major security change is that the Teleport agent Service account will have access to any secrets in the namespace it is installed in. This means that if the SA token is stolen or a hijack happens in the Teleport agent, the attacker can use the SA token to read any secrets in that namespace. It is recommended that Teleport is installed in a separate namespace to minimize this issue.

This issue happens because, currently, Kubernetes does not allow the specification of `resourceNames` in RBAC roles with [wildcards][wildcards]. If in the future if supported, we can limit the read/list/update actions when handling secrets for Teleport Service account. Otherwise, the possible solution might be to keep the identity stored in a single Secret instead of a secret per pod, and in this case, we can limit the SA actions on secrets immediately. 

# Links
[rbac]: https://kubernetes.io/docs/reference/access-authn-authz/rbac/
[wildcards]: https://github.com/kubernetes/kubernetes/issues/56582
- [1] https://kubernetes.io/docs/reference/access-authn-authz/rbac/
- [2] https://github.com/kubernetes/kubernetes/issues/56582



<!-- Plant UML diagrams -->
<!--

```plantuml
@startuml
participant KubeAgent 
participant Teleport
participant Kubernetes
KubeAgent -> KubeAgent: init procedure
KubeAgent -> Kubernetes: Get Secret Data
activate Kubernetes
alt Identity data is present in Secret
Kubernetes -> KubeAgent: returns secret data
    KubeAgent -> Teleport: Joining the cluster with identity from secret
    activate Teleport
    alt successful case
       Teleport->KubeAgent: Node successfully authenticated and registered\nin the cluster
       
    else identity signed by a different Auth server
        Teleport ->KubeAgent: Node identity signed by a different Auth Server 

    KubeAgent -> KubeAgent: failure state.
    note left
        unable to join the cluster
        logs the error
    end note

    end
deactivate Teleport
else Identity data is not present in Secret
Kubernetes -> KubeAgent: returns error
    KubeAgent -> Teleport: Sends invite code

activate Teleport
    alt successful case
        Teleport -> KubeAgent: returns node identity

        KubeAgent -> Kubernetes: Updates secret data with Identity
    deactivate Kubernetes
        KubeAgent -> Teleport: joins the cluster
    else invite code expired
    Teleport->KubeAgent: invalid invite token error

    KubeAgent -> KubeAgent: failure state.
    note left
        unable to join the cluster
        because the invite might be expired.
        logs the error
    end note
    end
       deactivate Teleport
       
end
  loop CA rotation
      KubeAgent<->Teleport: Rotate certificates
      Teleport -> KubeAgent: New certificates
      KubeAgent->Kubernetes: Update secret content
  end
@enduml
```
-->