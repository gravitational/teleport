# Kubernetes and SSH Integration Guide

Teleport v.3.0+ has the ability to act as a compliance gateway for managing privileged access to Kubernetes clusters. This enables the following capabilities:

* A Teleport Proxy can act as a single authentication endpoint for both SSH and
  Kubernetes. Users can authenticate against a Teleport proxy using Teleport's `tsh login` command and retrieve credentials for both SSH and Kubernetes API.
* Users RBAC roles are always synchronized between SSH and Kubernetes, making
  it easier to implement policies like _developers must not access production
  data_.
* Teleport's session recording and audit log extend to Kubernetes, as well.
  Regular `kubectl exec` commands are logged into the audit log and the
  interactive commands are recorded as regular sessions that can be stored and replayed in the
  future.

![ssh-kubernetes-integration](img/teleport-kube.png)

This guide will walk you through the steps required to configure Teleport to
work as a unified gateway for both SSH and Kubernetes. We will cover both the open
source and enterprise editions of Teleport.

For this guide, we'll be using an instance of Kubernetes running on [Google's GKE](https://cloud.google.com/kubernetes-engine/)
but this guide should apply with any upstream Kubernetes instance.

## Teleport Proxy Service

By default, the Kubernetes integration is turned off in Teleport. The configuration setting to enable the integration is the `proxy_service/kubernetes/enabled` setting which can be found in the proxy service section in the `/etc/teleport.yaml` file, as shown below:

```yaml
# snippet from /etc/teleport.yaml on the Teleport proxy service:
proxy_service:
    # create the 'kubernetes' section and set 'enabled' to 'yes':
    kubernetes:
        enabled: yes
        public_addr: [teleport.example.com:3026]
        listen_addr: 0.0.0.0:3026
```

Let's take a closer look at the available Kubernetes settings:

* `public_addr` defines the publicly accessible address which Kubernetes API
  clients like `kubectl` will connect to. This address will be placed inside of
  `kubeconfig` on a client's machine when a client executes `tsh login` command
  to retrieve its certificate. If you intend to run multiple Teleport proxies behind
  a load balancer, this must be the load balancer's public address.

* `listen_addr` defines which network interface and port the Teleport proxy server
  should bind to. It defaults to port 3026 on all NICs.

## Connecting the Teleport proxy to Kubernetes

There are two ways this can be done:

1. Deploy Teleport Proxy service as a Kubernetes pod inside the Kubernetes cluster you want the proxy to have access to.
   No Teleport configuration changes are required in this case.
2. Deploy the Teleport proxy service outside of Kubernetes and update the Teleport Proxy configuration with Kubernetes
   credentials. In this case, we need to update `/etc/teleport.yaml` for the proxy service as shown below:

```yaml
# snippet from /etc/teleport.yaml on the proxy service deployed outside k8s:
proxy_service:
  kubernetes:
    kubeconfig_file: /path/to/kubeconfig
```

To retrieve the Kubernetes credentials for the Teleport proxy service, you have to authenticate against your Kubernetes
cluster directly then copy the file to `/path/to/kubeconfig` on the Teleport proxy server.

Unfortunately for GKE users, GKE requires its own client-side extensions to authenticate, so we've created a
[simple script](https://github.com/gravitational/teleport/blob/master/examples/gke-auth/get-kubeconfig.sh) you can run
to generate a `kubeconfig` file for the Teleport proxy service.

## Impersonation

The next step is to configure the Teleport Proxy to be able to impersonate Kubernetes principals within a given group
using [Kubernetes Impersonation Headers](https://kubernetes.io/docs/reference/access-authn-authz/authentication/#user-impersonation).

If Teleport is running inside the cluster using a Kubernetes `ServiceAccount`, here's an example of the permissions that
the `ServiceAccount` will need to be able to use impersonation (change `teleport-serviceaccount` to the name of the
`ServiceAccount` that's being used):

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: teleport-impersonation
rules:
- apiGroups:
  - ""
  resources:
  - users
  - groups
  - serviceaccounts
  verbs:
  - impersonate
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: teleport
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: teleport-impersonation
subjects:
- kind: ServiceAccount
  # this should be changed to the name of the Kubernetes ServiceAccount being used
  name: teleport-serviceaccount
  namespace: default
```

There is also an [example of this usage](https://github.com/gravitational/teleport/blob/master/examples/chart/teleport/templates/clusterrole.yaml)
within the [example Teleport Helm chart](https://github.com/gravitational/teleport/blob/master/examples/chart/teleport/).

If Teleport is running outside of the Kubernetes cluster, you will need to ensure that the principal used to connect to
Kubernetes via the `kubeconfig` file has the same impersonation permissions as are described in the `ClusterRole` above.

## Kubernetes RBAC

Once you perform the steps above, your Teleport instance should become a fully functional Kubernetes API
proxy. The next step is to configure Teleport to assign the correct Kubernetes groups to Teleport users.

Mapping Kubernetes groups to Teleport users depends on how Teleport is
configured. In this guide we'll look at two common configurations:

* **Open source, Teleport Community edition** configured to authenticate users via [Github](admin-guide.md#github-oauth-20).
  In this case, we'll need to map Github teams to Kubernetes groups.

* **Commercial, Teleport Enterprise edition** configured to authenticate users via [Okta SSO](ssh_okta.md).
  In this case, we'll need to map users' groups that come from Okta to Kubernetes
  groups.

### Github Auth

When configuring Teleport to authenticate against Github, you have to create a
Teleport connector for Github, like the one shown below. Notice the `kubernetes_groups`
setting which assigns Kubernetes groups to a given Github team:

```yaml
kind: github
version: v3
metadata:
  # connector name that will be used with `tsh --auth=github login`
  name: github
spec:
  # client ID of Github OAuth app
  client_id: <client-id>
  # client secret of Github OAuth app
  client_secret: <client-secret>
  # connector display name that will be shown on web UI login screen
  display: Github
  # callback URL that will be called after successful authentication
  redirect_url: https://teleport.example.com:3080/v1/webapi/github/callback
  # mapping of org/team memberships onto allowed logins and roles
  teams_to_logins:
    - organization: octocats # Github organization name
      team: admins           # Github team name within that organization
      # allowed UNIX logins for team octocats/admins:
      logins:
        - root
      # list of Kubernetes groups this Github team is allowed to connect to
      kubernetes_groups: ["system:masters"]
```
To obtain client ID and client secret from Github, please follow [Github documentation](https://developer.github.com/apps/building-oauth-apps/creating-an-oauth-app/) on how to create and register an OAuth app. Be sure to set the "Authorization callback URL" to the same value as redirect_url in the resource spec.

Finally, create the Github connector with the command: `tctl create -f github.yaml`. Now, when Teleport users execute the Teleport's `tsh login` command, they will be prompted to login through the Github SSO and upon successful authentication, they have access to Kubernetes.

```bsh
# Login via Github SSO and retrieve SSH+Kubernetes certificates:
$ tsh login --proxy=teleport.example.com --auth=github login

# Use Kubernetes API!
$ kubectl exec -ti <pod-name>
```

The `kubectl exec` request will be routed through the Teleport proxy and
Teleport will log the audit record and record the session.

!!! note:
    For more information on integrating Teleport with Github SSO, please see the [Github section in the Admin Manual](admin-guide/#github-oauth-20).

### Okta Auth

With Okta (or any other SAML/OIDC/Active Directory provider), you must update
Teleport's roles to include the mapping to Kubernetes groups.

Let's assume you have the Teleport role called "admin". Add `kubernetes_groups`
setting to it as shown below:

```yaml
# NOTE: the role definition is edited to remove the unnecessary fields
kind: role
version: v3
metadata:
  name: admin
spec:
  allow:
    # if kubernetes integration is enabled, this setting configures which
    # kubernetes groups the users of this role will be assigned to.
    # note that you can refer to a SAML/OIDC trait via the "external" property bag,
    # this allows you to specify Kubernetes group membership in an identity manager:
    kubernetes_groups: ["system:masters", "{{external.trait_name}}"]]
```

To add `kubernetes_groups` setting to an existing Teleport role, you can either
use the Web UI or `tctl`:

```bsh
# Dump the "admin" role into a file:
$ tctl get roles/admin > admin.yaml
# Edit the file, add kubernetes_groups setting
# and then execute:
$ tctl create -f admin.yaml
```

!!! tip "Advanced Usage":
    `{{ external.trait_name }}` example is shown to demonstrate how to fetch
    the Kubernetes groups dynamically from Okta during login. In this case, you
    need to define Kubernetes group membership in Okta (as a trait) and use
    that trait name in the Teleport role.

Once this is complete, when users execute `tsh login` and go through the usual Okta login
sequence, their `kubeconfig` will be updated with their Kubernetes credentials.

!!! note:
    For more information on integrating Teleport with Okta, please see the [Okta integration guide](/ssh_okta/).
