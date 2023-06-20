---
    authors: Anton Miniailo (anton@goteleport.com)
    state: draft
---

# RFD 135 - Kubernetes App discovery

## Required Approvers

- Engineering: `@r0mant` && `@tigrato`
- Product: `@klizhentas` || `@xinding33`
- Security: `@reedloden` || `@jentfoo`

## What

Proposes discovery of apps that are hosted inside Kubernetes clusters and exposed as services.

Related issue: [#25538](https://github.com/gravitational/teleport/issues/25538)

## Why

Some users dynamically deploy apps into their Kubernetes clusters and expose those as Kubernetes services. When they actively
create/update/remove apps, manually trying to mirror these changes to Teleport may be time-consuming and prone to
errors, especially for organizations that have a large number of services running in their Kubernetes clusters.
With the changes described in this RFD, Teleport will be able to discover Kubernetes service and automatically register them as
Teleport apps and keep them in up to date state, greatly simplifying process and making user experience much better.

## Scope

In this document we focus on dynamically creating Teleport apps based on discovery of services in Kubernetes clusters that are running
Teleport agents. Discovery service and app service should be running inside the Kubernetes cluster whose services are being exposed.

## Details

Kubernetes app discovery will build upon existing infrastructure for other discovery capabilities that are already there. We will add another
type of matchers to the discovery service configuration - `kubernetes_apps`. When discovery service sees this, it will periodically
poll Kubernetes cluster, inside which it is running, for a list of services. Based on this list, it will form a list of Teleport apps resources
and update it on the backend, constantly keeping the dynamic apps state up to date. Writing to the backend will be handled
by already existing mechanism in our code - reconciler. Reconciler will compare given fresh list of resources and resources we currently have
on the backend, and then will decide on appropriate actions (create/update/delete) that are required on
the backend to change resources to the desired state.
The Teleport app service that runs in the same Kubernetes cluster will then react to those changes, and will start/stop proxying
apps that are selected by appropriate labels.

Name of the created Teleport app will consist of Kubernetes service name, namespace and cluster name, like this:
`$SERVICE_NAME-$CLUSTER_NAME-$NAMESPACE`. This is done to avoid naming collisions in discovered apps. Though app names
will be long in that case, there's ongoing work to improve UX when working with long resources names in Teleport, see [RFD 129](https://github.com/gravitational/teleport/pull/27258).

To allow for finer control over exposed service transformation into Teleport app Kubernetes annotations will be used.
By default we will be exposing services as `HTTP` apps, but annotation can be used to specify whether the service is a `TCP` application.

If Kubernetes service has multiple ports in use, we will expose each port as a separate app, with name of the app including the port's name
specified in the service. For example, if a service has port 443 exposed with name `tls` and also port `5432` with name `postgres`
we will create two apps: `testApp-tls-main-cluster-prod`, `testApp-postgres-main-cluster-prod`. But user will be able to use annotation to specify
preferred port, in that case only app with this port will be created, without port info in the app name.

Labels from the Kubernetes service will be copied to the corresponding Teleport app.

## UX

### Discovery

Discovery service will use new `kubernetes_apps` matchers to periodically poll list of services inside the Kubernetes cluster.
Labels specified in the config will be used to filter out services that should be exposed. Also users will be able to
specify namespaces to process.

```yaml
## This section configures the Discovery Service
discovery_service:
  enabled: yes
  discovery_group: kube-auto-apps
  kubernetes_apps:
    - cluster_name: main-cluster
      namespaces: [ "toronto", "porto" ]
      labels:
        env: staging
    - cluster_name: main-cluster
      namespaces: [ "seattle", "oakland" ]
      labels:
        env: testing
```

`cluster_name` field will be translated into `teleport.dev/kubeapp-cluster-name` label on the Teleport app resource, so
later it's be easier to target it in the corresponding app service.

Kubernetes annotation `teleport.dev/kubeapp-type` control type of the Teleport app we create. If annotation is missing
`HTTP` type will be used.

Annotation `teleport.dev/kubeapp-port` will control preferred port for the Kubernetes service. Its value should be one of the
exposed service ports; otherwise, the app will not be imported. Value can be matched either by numeric value or by the name of
the port defined on the service.

Example Kubernetes service configs using these annotations can look like this:
```yaml
apiVersion: v1
kind: Service
metadata:
  name: first-service
  labels:
    app: first-service
  annotations:
    teleport.dev/kubeapp-type: HTTP # Allowed values are [`HTTP`, `tcp`]
    teleport.dev/kubeapp-port: 80
spec:
  ports:
  - name: http
    port: 80
    protocol: TCP
    targetPort: 80
  - name: postgres
    port: 5432
    protocol: TCP
    targetPort: 5432
  selector:
    app: first-pod
---
apiVersion: v1
kind: Service
metadata:
  name: second-service
  labels:
    app: second-service
  annotations:
    teleport.dev/kubeapp-type: tcp # Allowed values are [`HTTP`, `tcp`]
    teleport.dev/kubeapp-port: fluentd
spec:
  ports:
    - name: http
      port: 80
      protocol: TCP
      targetPort: 80
    - name: fluentd
      port: 24224
      protocol: TCP
      targetPort: 24224
  selector:
    app: second-pod
```

Discovery for the Kubernetes apps will support dynamic discovery configuration, when it will be implemented - [RFD 125](https://github.com/gravitational/teleport/blob/master/rfd/0125-dynamic-auto-discovery-config.md).

### App service

App service will be running inside the Kubernetes cluster and proxy connection from remote users to local cluster IP. App service
already has capability to watch dynamically created app resources, users just need to correctly setup
`resources` field in the config:

```yaml
app_service:
  enabled: yes
  resources:
    - labels:
        "teleport.dev/kubeapp-cluster-name": "main-cluster"
        "env": "staging"
```

### Helm chart

We will update helm chart `teleport-kube-agent` to support configuring Kubernetes apps discovery. Ability to configure
and deploy discovery service will be added by using parameter `kubernetesApps`.

```yaml
kubernetesApps:
  - cluster_name: main-cluster
    namespaces: [ "toronto", "porto" ]
    labels:
      env: staging
  - cluster_name: main-cluster
    namespaces: [ "seattle", "oakland" ]
    labels:
      env: testing
```

## Security

Similar to the in-cluster kubernetes service, discovery will use service account of its pod as credentials to list services inside the cluster.
So Teleport service account needs to have permission to list services of the cluster. It is not a big expansion of security
permissions requirements by itself, but users need to be careful to not expose apps by accident - we will make a point in documentation
about importance of using correct labels to select cluster services.

## Future work

In the future we can improve on customization capabilities, using more annotations or CRDs, allowing for finer control of
how Kubernetes services are exposed in Teleport.
Also, if there will be demand, we can try to expand scope to allow discovery without requirement of running Teleport services
inside the Kubernetes clusters. For example exposing services that have external IP (in which case we
might be able to expose it without running Teleport app service inside the cluster). Or, we could expose services in dynamically discovered Kubernetes clusters.
That has its complications in usability and security, so the need for this expansion will be
evaluated when we have more usage patterns data.