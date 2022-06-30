---
authors: Jeff Anderson <jeff@goteleport.com>
state: draft
---

# RFD 77 - Support Dump

## What

A support dump process should be able to gather as much relevant information
about a given teleport instance as possible. This information should be
programatically discovered and package it up to be shared easily.

## Why

It is necessary to collect a variety of basic information when assisting a
teleport administrator with an issue remotely. This information gathering step
can be high effort for the user requesting help, especially since they may not
be able to anticipate which data is relevant.

This will primarily assist the interactions between the Teleport support team
and customers requesting help. The Teleport development team should also
greatly benefit from a standardized set of information about a problem cluster.

Without a support dump process, this increases the burden on both ends of the
interaction as relevant information is requested and inspected. This adds
unnecessary time to solution with each subsequent requests for information. 

A standardized support dump would be able to provide a large amount of
information in one step, increasing visibility into the possible issues and
decreasing time to solution for a given request. It will also greatly decrease
the effort required to reproduce a particular issue, especially when there is a
less common configuration in play.

## Details

### End User Experience

Because teleport has many deployment methods, the support dump procedure should
be smart enough to detect and gather information in the correct way. This may
include an entire cluster, or a single agent. It should also be able to gather
logs from standard locations wherever possible. It should advise the end user
if it is unable to gather critical information for any reason.

It should be built in to the Teleport binary itself. It should be possible to
initiate a support dump from both the command line or from the web interface.
Provisions should be made to advise about (or redact) possible sensitive
information where applicable.

File size limits will also need to be considered because the resulting files
may be transmitted over systems with extremely limited file sizes.

The end user should be able to run one command or click on one web gui button
to generate a dump file. This file will likely need to be a compressed tar file.

### Contents of the Dump File

* The resulting compressed tar file should have a defined directory structure.
* It should be both user-readable and machine-readable.
* It should have timestamped log output.
* When created for an entire cluster, it should be gathering information from every teleport agent/node/service that it is connected to the cluster.
* It should include relevant objects such as users and roles to help troubleshoot rbac options.
* any redactions should be done in a one way consistent obfuscation so that objects can still be uniquely differentiated.

### Structure

The top level `nodes/` directory will have basic information about each node in
the cluster. If the node where the support dump is initiated, then it will
include information about only that agent, plus information about the auth
and/or proxy nodes it is aware of, along with tunnel status.

The `objects/` directory will gather any objects it possibly can. If a support
dump is initiated remotely with tsh credentials, it should indicate which
objects it was unabled to gather due to permission issues. This should collect
every object type it can possibly retrieve. The separate .error files would
include any stderr type output to highlight any problems during the support
dump process. For a single node agent process, it would be helpful to retrieve
whatever objects the node is allowed to retrieve with its node credentials.

The `systemd/` directory is an example as to what it might look like if
teleport is a systemd service and logs can be retrieved that way. This example
shows both a plain text `log.txt` and a `log.json` which should correspond to
the `journalctl -u teleport` and `journalctl -u teleport -f json` outputs. The
latter include much more information, but takes up more space. The json output
preserves timestamps and other relevant information but is less human readable.

The `index.html` is a hypothetical single page application that can present the
data to the end user in a user-friendly format. It should be Teleport purple
and have a similar look and feel to the web gui. This is not a requirement for
an MVP, but would be quite helpful to both the end user and the support agent
as the included data is analyzed. At minimum, it should have some readme type
explanations of the directory structure and some example commands to inspect
the data, along with any warnings about the issues that were encountered when
the support dump was generated.

```
├── index.html
├── nodes
│   ├── 6204f089-7da7-4381-b28b-d247d5122a2e
│   │   ├── teleport.yaml
│   │   ├── role.auth
│   │   ├── role.node
│   │   └── proc
│   │       └── sqlite.sql
│   └── dcefe26d-fe45-464c-99ab-d8a1ab044fa7
│       ├── teleport.yaml
│       ├── role.proxy
│       ├── role.node
│       └── proc
│           └── sqlite.sql
├── objects
│   ├── tctl_get_auth_server.error
│   ├── tctl_get_auth_server.yaml
│   ├── tctl_get_cert_authority.error
│   ├── tctl_get_cert_authority.yaml
│   ├── tctl_get_cluster_auth_preference.error
│   ├── tctl_get_cluster_auth_preference.yaml
│   ├── tctl_get_connectors.error
│   ├── tctl_get_connectors.yaml
│   ├── tctl_get_github.error
│   ├── tctl_get_github.yaml
│   ├── tctl_get_kube_service.error
│   ├── tctl_get_kube_service.yaml
│   ├── tctl_get_namespace.error
│   ├── tctl_get_namespace.yaml
│   ├── tctl_get_node.error
│   ├── tctl_get_node.yaml
│   ├── tctl_get_oidc.error
│   ├── tctl_get_oidc.yaml
│   ├── tctl_get_proxy.error
│   ├── tctl_get_proxy.yaml
│   ├── tctl_get_remote_cluster.error
│   ├── tctl_get_remote_cluster.yaml
│   ├── tctl_get_role.error
│   ├── tctl_get_role.yaml
│   ├── tctl_get_saml.error
│   ├── tctl_get_saml.yaml
│   ├── tctl_get_semaphore.error
│   ├── tctl_get_semaphore.yaml
│   ├── tctl_get_trusted_cluster.error
│   ├── tctl_get_trusted_cluster.yaml
│   ├── tctl_get_tunnel.error
│   ├── tctl_get_tunnel.yaml
│   ├── tctl_get_user.error
│   └── tctl_get_user.yaml
└── systemd
    ├── 6204f089-7da7-4381-b28b-d247d5122a2e
    │   └── teleport.service
    │       ├── log.txt
    │       └── log.json
    └── dcefe26d-fe45-464c-99ab-d8a1ab044fa7
        └── teleport.service
            ├── log.txt
            └── log.json
```

#### Kubernetes-Aware Information

Because kubernetes is a very common deploy mechanism, it would be ideal to have
enough intelligence in the support dump to gather kubernetes information.

The most critical part of this will be pod logs with timestamps, but once it is
possible to grab those, any number of other kubernetes objects that are
available should be retrieved. Secrets and other sensitive information should
be redacted.

Grabbing all possible resources in the relevant namespace is ideal because the
exact methods used to deploy teleport in kubernetes vary. Some people use our
helm charts, some people use their own method.

This directory structure might look something like this:

```
└── kube
    ├── kubectl_get_all.yaml
    ├── kubectl_get_bindings.yaml
    ├── kubectl_get_certificaterequests.cert-manager.io.yaml
    ├── kubectl_get_certificates.cert-manager.io.yaml
    ├── kubectl_get_challenges.acme.cert-manager.io.yaml
    ├── kubectl_get_configmaps.yaml
    ├── kubectl_get_controllerrevisions.apps.yaml
    ├── kubectl_get_cronjobs.batch.yaml
    ├── kubectl_get_crontabs.stable.example.com.yaml
    ├── kubectl_get_daemonsets.apps.yaml
    ├── kubectl_get_deployments.apps.yaml
    ├── kubectl_get_endpoints.yaml
    ├── kubectl_get_endpointslices.discovery.k8s.io.yaml
    ├── kubectl_get_events.events.k8s.io.yaml
    ├── kubectl_get_events.yaml
    ├── kubectl_get_horizontalpodautoscalers.autoscaling.yaml
    ├── kubectl_get_ingresses.extensions.yaml
    ├── kubectl_get_ingresses.networking.k8s.io.yaml
    ├── kubectl_get_issuers.cert-manager.io.yaml
    ├── kubectl_get_jobs.batch.yaml
    ├── kubectl_get_leases.coordination.k8s.io.yaml
    ├── kubectl_get_limitranges.yaml
    ├── kubectl_get_localsubjectaccessreviews.authorization.k8s.io.yaml
    ├── kubectl_get_networkpolicies.networking.k8s.io.yaml
    ├── kubectl_get_orders.acme.cert-manager.io.yaml
    ├── kubectl_get_persistentvolumeclaims.yaml
    ├── kubectl_get_persistentvolumes.yaml
    ├── kubectl_get_poddisruptionbudgets.policy.yaml
    ├── kubectl_get_pods.metrics.k8s.io.yaml
    ├── kubectl_get_pods.yaml
    ├── kubectl_get_podtemplates.yaml
    ├── kubectl_get_replicasets.apps.yaml
    ├── kubectl_get_replicationcontrollers.yaml
    ├── kubectl_get_resourcequotas.yaml
    ├── kubectl_get_rolebindings.rbac.authorization.k8s.io.yaml
    ├── kubectl_get_roles.rbac.authorization.k8s.io.yaml
    ├── kubectl_get_secrets.yaml
    ├── kubectl_get_securitygrouppolicies.vpcresources.k8s.aws.yaml
    ├── kubectl_get_serviceaccounts.yaml
    ├── kubectl_get_services.yaml
    ├── kubectl_get_statefulsets.apps.yaml
    ├── kubectl_logs_teleport_ct-bb79b9bdb-xbsmq_alpine.log
    ├── kubectl_logs_teleport_etcd-865c899bf6-xlzcl_etcd.log
    └── kubectl_logs_teleport_teleport-8657cd5579-nv48j_teleport.log
```

#### Miscellaneous Considerations

When a support dump is invoked, it should print minimal progress information.
There should be no surprises such as timing out with a failure after five
minutes. It should instead be obvious that the support dump is working on
something and fail quickly if it is going to fail. Users can be told to opt in
to a more verbose mode, or bump up the timeout thresholds if desired.

Concise error/warning messages with a link to a KB or doc URL with more
information are quite user friendly. For example, if it cannot find the
teleport logs by looking in the typical places, the content at the URL should
have more information about gathering logs and why it might have failed.

Even if the support dump fails to gather many values, it should still create a
tar file. It should be abundantly clear to the end user that something did not
complete properly and that they should inspect the output and try to rerun it
so that it isn't a surprise when the file is missing key information when it
gets transmitted to the support agent.

In cluster mode, errors should be aggregated and reported in a concise manner.

The support dump process should be optimized for time, especially since it
should be possible to invoke over the web ui. Some products opt to stream the
tarball to the browser as it is built, which means that there can be a long web
request while the process is done. 

The support dump should optimize the normal operation of the cluster wherever
possible. When an end user is experiencing difficulties with the product, the
last thing that a support dump should do is aggrevate the problem. A creative
solution like lazily caching the data a support dump would collect ahead of
time could make the process as painless as possible (with an optional force
refresh when needed)
