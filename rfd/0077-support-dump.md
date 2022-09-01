---
authors: Jeff Anderson <jeff@goteleport.com>
state: draft
---

# RFD 77 - Support Dump

## What

A support dump process should be able to automatically gather relevant information about a given teleport instance or cluster. This information should be programmatically discovered, and package it up to be shared easily.

It should be human friendly, but also machine parseable. It should strike a balance between the competing concerns of gathering as many types of information, completing quickly, being error resilient, and not so disruptive that it would exacerbate an issue on a stricken cluster.

## Why

This will primarily assist the interactions between the Teleport support team and customers requesting help. The Teleport development team should also greatly benefit from a standardized set of information about a problem cluster.

Currently, information gathering can be one of the most time intensive phases of finding a solution to a support request. Automating and standardizing this will have several benefits:

* It reduces the time required to solve a case.
* It decreases the burden on both the end user and the support agent.
* It will reduce the need for zoom troubleshooting sessions.
* It opens the door to automated analysis of a cluster to find common issues quickly.
* It will make it easier to reproduce an issue for both support and engineering.

## Details

### End User Experience

Because teleport has many deployment methods, the support dump procedure should be smart enough to detect and gather information in the correct way. This may include an entire cluster, or a single agent. It should also be able to gather logs from standard locations wherever possible. Log collection is often the biggest hurdle during the troubleshooting and support process in general. It should advise the end user if it is unable to gather critical information for any reason.

It should be built-in to one (or more) of the Teleport binaries so that it can be invoked anywhere Teleport is running.  It should be possible to initiate a support dump from both the command line or from the web interface. A dump might be at the node/agent level, at the cluster level, or an aggregate of the two types.  Provisions should be made to advise about (or redact) possible sensitive information where applicable. Considerations about RBAC should be handled gracefully.

File size limits will also need to be considered because the resulting files
may be transmitted over systems with extremely limited file sizes.

The end user should be able to run one command or click on one web gui button to generate a dump file. This file should be in a format that allows the user to easily inspect its contents. This should be doable by someone with average technical capabilities among our customers that handle Teleport cluster deploys or troubleshooting.

### Contents of the Dump File

* There should be an intuitive and well defined structure so data can be quickly found or discovered.
* It should be both user-readable and machine-readable.
* It should have timestamped log output.
* When created for an entire cluster, it should be gathering information from every teleport agent/node/service that is connected to the cluster.
* It should include relevant objects such as users and roles to help troubleshoot rbac options.
* any redactions should be done in a one way consistent obfuscation so that objects can still be uniquely differentiated.

### Structure

This is a faux/mock directory structure to convey organizational requirements and specifics about desired information to include.

#### Nodes / agents

In this context, each instance of a teleport process should be represented 1:1 in the data structure of the support dump.

A single node dump should include information about only that agent. That should include:

* the teleport configuration (file or CLI)
* any auth and/or proxy nodes it is aware of
* roles
* metrics scrape (if enabled)
* and tunnel statuses (if applicable)
* logs
* The command line used (/proc/pid/cmdline)

For a multiple node dump file, it should include that same information gathered by each node included in the dump. Nodes that are attempted but failed should also be indicated with a reason why the dump failed. (no tunnel, connection refused, etc)

```
├── nodes
│   ├── 6204f089-7da7-4381-b28b-d247d5122a2e
│   │   ├── teleport.yaml
│   │   ├── role.auth
│   │   ├── role.node
│   │   ├── cmdline
│   │   └── proc
│   │       └── sqlite.sql
│   └── dcefe26d-fe45-464c-99ab-d8a1ab044fa7
│       ├── teleport.yaml
│       ├── role.proxy
│       ├── role.node
│       ├── cmdline
│       └── proc
│           └── sqlite.sql
```
 
#### Storage Backend Dynamic Objects

It is very common for users to need help with one or more of their dynamic objects. This might include anything from users, roles, connectors, and really anything else that can be created via `tctl create`. The support dump should have the _capability_ to collect every type of object that is in the backend storage. The storage backend is not often a major concern for file size, but it might be appropriate to have provisions to select or deselect certain objects.

```
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
```

#### Non-Teleport-Native Data

The support dump should be able to gather relevant from non-teleport systems such as systemd and kubernetes.

##### Systemd

If the support dump detects that teleport is running inside systemd, it should include systemd-specific information:

* journald logs
  * journalctl -u teleport
  * journalctl -u teleport -f json
* the relevant teleport systemd unit(s).

The json format journald logs have full information, including timestamps. The default output format for journald only includes the journald MESSAGE field, but no other metadata.

```
└── nodes
    ├── 6204f089-7da7-4381-b28b-d247d5122a2e
    │   └── teleport.service
    │       ├── log.txt
    │       ├── log.json
    │       ├── teleport.timer
    │       └── teleport.service
    └── dcefe26d-fe45-464c-99ab-d8a1ab044fa7
        └── teleport.service
            ├── log.txt
            ├── log.json
            └── teleport.service
```

##### Kubernetes-Aware Information

Because kubernetes is a very common deployment mechanism, it would be ideal to have enough intelligence in the support dump to gather kubernetes information.

The most critical part of this will be pod logs with timestamps, but once it is possible to grab those, any number of other kubernetes objects that are available should be retrieved. Secrets and other sensitive information should be redacted.

Grabbing all possible resources in the relevant namespace is ideal because the exact methods used to deploy teleport in kubernetes vary. Some people use our helm charts, some people use their own method.

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

##### Supporting info about the dump

The support dump file on its own should be somewhat self contained to reduce the need for other tools or processes. The customers themselves are part of the audience for this file format, and should have some information to go with when they get this mysterious file.

There should be something like a readme with some short info about what the dump is, what was gathered, and how the file may be used.

Since the support dump process is doing many many things, there should be A status report of what was attempted, whether it succeeded, and whether the dump is considered "complete" or not.

```
├── readme.html
├── report.txt
```

## Miscellaneous Considerations

When a support dump is invoked, it should print minimal progress information.  There should be no surprises such as timing out with a failure after five minutes. It should instead be obvious that the support dump is working on something and fail quickly if it is going to fail. Users can be told to opt in to a more verbose mode, or bump up the timeout thresholds if desired.

Concise error/warning messages with a link to a KB or doc URL with more information are quite user friendly. For example, if it cannot find the teleport logs by looking in the typical places, the content at the URL should have more information about gathering logs and why it might have failed.

Even if the support dump fails to gather many values, it should still create an output file. It should be abundantly clear to the end user that something did not complete properly and that they should inspect the output and try to rerun it so that it isn't a surprise when the file is missing key information when it gets transmitted to the support agent.

In cluster mode, errors should be aggregated and reported in a concise manner.

The support dump process should have the capability to be invoked to optimize for speed, completeness, or file size. This will be especially helpful when invoking it via the web ui.

The support dump should optimize the normal operation of the cluster wherever possible. When an end user is experiencing difficulties with the product, the last thing that a support dump should do is aggravate the problem. Lazily precaching support dump data might be a feasible optimization to alleviate this concern.
