---
authors: Anton Miniailo (anton@goteleport.com)
state: implemented (v14.0)
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
Teleport will be able to discover Kubernetes service and automatically register them as
Teleport apps and keep them in up to date state, simplifying process and making user experience much better.

## Scope

In this document we focus on dynamically creating Teleport apps based on discovery of services in Kubernetes clusters that are running
Teleport agents. Discovery service and app service should be running inside the Kubernetes cluster whose services are being exposed.

## Details

Kubernetes app discovery will build upon existing infrastructure for other discovery capabilities that are already there. We will add another
type of matchers to the discovery service configuration - `kubernetes`. If these matchers are present, every 5 minutes discovery service will 
poll Kubernetes cluster, inside which it is running, for a list of services. Based on this list, it will form a list of Teleport apps resources
and update it on the backend, constantly keeping the dynamic apps state up to date. Writing to the backend will be handled
by already existing mechanism in our code - reconciler. Reconciler will compare given fresh list of resources and resources we currently have
on the backend, and then will decide on appropriate actions (create/update/delete) that are required on
the backend to change resources to the desired state.
The Teleport app service that runs in the same Kubernetes cluster will then react to those changes, and will start/stop proxying
apps that are selected by appropriate labels.

Name of the created Teleport app will consist of Kubernetes service name, namespace and Kubernetes cluster name:
`$SERVICE_NAME-$NAMESPACE-$KUBE_CLUSTER_NAME`. This is done to avoid naming collisions in discovered apps.
Dots in the cluster name will be replaces with hyphens "-", remaining parts of the name should already only contain alphanumeric
symbols and hyphens. There's still corner cases with small possibility of collisions if service name and namespace contain
hyphens in specific manner - if we detect naming collision we will not import both apps, logging an error instead. Though app names
will be long in that case, there's ongoing work to improve UX when working with long resources names in Teleport, 
see [RFD 129](https://github.com/gravitational/teleport/pull/27258).

We will add capability to use optional annotations to better control transformation of Kubernetes services into Teleport apps.
By default we will be exposing ports on a service that serve HTTP/HTTPS, using different heuristics to determine if it does (described later in
annotations section). But annotation can be used to specify which protocol to use for the app's URI, including `tcp://...` URIs. 

If Kubernetes service has multiple ports in use, we will expose each port as a separate app, with name of the app including the port's name
specified in the service. For example, if a service has port 443 exposed with name `tls` and also port `4004` with name `metrics`
we will create two apps: `testApp-tls-main-cluster-prod`, `testApp-metrics-main-cluster-prod`. Names for ports are mandatory in Kubernetes if more
than one port is exposed for the service, so we always will be able to use it if we need it. User will be able to use annotation to specify
preferred port, in that case only app with this port will be created, without port info in the app name.

In order for discovery service to be able to dynamically create/update/delete apps we will add Read/Write permissions 
for the apps to the built-in system role `Discovery`.

Labels from the Kubernetes service will be copied to the corresponding Teleport app.

## UX

### Discovery

Discovery service will use new `kubernetes` matchers to periodically poll list of services inside the Kubernetes cluster.
Labels specified in the config will be used to filter out services that should be exposed. Also users will be able to
specify namespaces to process. 

```yaml
## This section configures the Discovery Service
discovery_service:
  enabled: yes
  discovery_group: main-cluster
  kubernetes:
    - types: ["app"] # in the future "db" will be possible
      namespaces: [ "toronto", "porto" ]
      labels: # List of labels to select desired Kubernetes services
        env: staging
    - types: ["app"]
      namespaces: [ "seattle", "oakland" ]
      labels:
        env: testing
```

`discovery_group` field in the config will be translated into `teleport.dev/kubernetes-cluster` label on the Teleport app resource, so
later it's be easier to target it in the corresponding app service. If `kubernetes` matchers are
specified in the config `discovery_group` field is required to be set.

#### Annotations

Kubernetes annotations will allow users to fine tune transformation of services into Teleport apps. All annotations are optional - 
they will override default behaviour, but they are not required for import of services. By default services without any 
annotations will also be imported.

Annotation `teleport.dev/discovery-type` controls what Teleport entity this service is considered to be. If matchers in discovery service
config match service type it will be imported. Now we will only support value 
`app` which means Teleport application will be imported from this service. In the future there are plans to expand to database importing.
If annotation is not set service is treated as an app.

Annotation `teleport.dev/protocol` controls protocol for the access of the Teleport app we create. If annotation is not set,
additional attempts will be performed to try to determine protocol of an exposed port:
-  if kubernetes service port definition has `appProtocol` field, and it contains
values `http`/`https` we will use it in the URI.
- if exposed port's name is `https` or it has numeric value 443, `https` will be used.
- We'll perform HTTP request to the port to see if it serves HTTP/HTTPS requests
- if exposed port's name is `http` or it has numeric value 80 or 8080, `http` will be used.

If all heuristics mentioned above didn't work, the port will be skipped. For app to be imported with `tcp` protocol, the
service should have explicit annotation `teleport.dev/protocl: tcp`

Annotation `teleport.dev/port` controls preferred port for the Kubernetes service, only this one will be used even if service
has multiple exposed ports. Its value should be one of the exposed service ports; otherwise, the app will not be imported. 
Value can be matched either by numeric value or by the name of the port defined on the service.

Annotation `teleport.dev/app-name` controls resulting app name. If present it will override default app name pattern
`$SERVICE_NAME-$NAMESPACE-$KUBE_CLUSTER_NAME`. If multiple ports are exposed, resulting apps will have port names added
as a suffix to the annotation value, as `$APP_NAME-$PORT1_NAME`, `$APP_NAME-$PORT2_NAME` etc, where `$APP_NAME` is the name
set by annotation.

Annotation `teleport.dev/app-rewrite` controls rewrite configuration for Teleport app, if needed. It should
contain full rewrite configuration in YAML format, same as one would put into `rewrite` config section of an 
app (see [documentation](https://goteleport.com/docs/application-access/guides/connecting-apps/#rewrite-redirect)).
```yaml
annotations:
  teleport.dev/app-rewrite: |
    redirect:
    - "localhost"
    - "jenkins.internal.dev"
    headers:
    - "X-Custom-Header: example"
    - "Authorization: Bearer {{internal.jwt}}"
```

Example Kubernetes service configs using annotations can look like this:
```yaml
apiVersion: v1
kind: Service
metadata:
  name: first-service
  labels:
    app: first-service
  annotations:
    teleport.dev/discovery-type: app # Allowed values are [`app`]
    teleport.dev/protocol: http # Allowed values are [`http`, `https`, `tcp`]
    teleport.dev/port: 80
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
    teleport.dev/protocol: tcp 
    teleport.dev/port: fluentd
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
        "teleport.dev/kubernetes-cluster": "main-cluster"
        "env": "staging"
```

### Helm chart

We will update helm chart `teleport-kube-agent` to support configuring Kubernetes apps discovery. Ability to configure
and deploy discovery service will be added by using helm chart parameter `kubernetesDiscovery`. Kubernetes discovery
will be enabled by default when users installs kube agent chart and it will try to discover HTTP apps using heuristics 
described above.

```yaml
kubernetesDiscovery:
  - types: ["app"]
    namespaces: [ "toronto", "porto" ]
    labels:
      env: staging
  - types: ["app"]
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