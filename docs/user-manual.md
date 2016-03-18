# User Manual

## Introduction

The Teleport User Manual covers usage of the Teleport client tool `tsh`. In this 
document you will learn how to:

* Securely login into interactive shell on remote cluster nodes.
* Execute commands on cluster nodes.
* Securely copy files to and from cluster nodes.
* Explore a cluster and execute commands on those nodes in a cluster that match your criteria.
* Share interactive shell sessions with colleagues or join someone else's session.
* Replay recorded interactive sessions.
* Use Teleport with OpenSSH client: `ssh` or with other tools that use SSH under the hood like Chef and Ansible.

## Difference vs OpenSSH

There are many differences between Teleport's `tsh` and OpenSSH's `ssh` but the 
most obvious two are:

* `tsh` always requires `--proxy` flag because `tsh` needs to know which cluster
  you are connecting to. 

* `tsh` needs _two_ usernames: one for the cluster and another for the node you
  are trying to login into. See "Teleport Identity" section below. For convenience, 
  `tsh assumes `$USER` for both logins by default.

While it may appear less convenient than `ssh`, we hope that the default behavior
and techniques like bash aliases will help to minimize the amount of typing.

## Your Teleport Identity

A user identity in Teleport exists in the scope of a cluster. The member nodes
of a cluster may have multiple OS users on them. A Teleport administrator assigns
"user mappings" to every Teleport user account.

When logging into a remote node, you will have to specify both logins. Teleport
identity will have to be passed as `--user` flag, while the node login will be
passed as `login@host`, using syntax compatible with traditional `ssh`.

These examples assume your localhost username is 'joe':

```bash
# Authenticate against cluster 'work' as 'joe' and then login into 'node'
# as root:
> tsh --proxy=work.example.com --user=joe root@node

# Authenticate against cluster 'work' as 'joe' and then login into 'node'
# as joe (by default tsh uses $USER for both):
> tsh --proxy=work.example.com node
```

## Exploring the Cluster

In a Teleport cluster all nodes periodically ping the cluster's auth server and
update their statuses. This allows Teleport users to see which nodes are online:

```bash
# Connect to cluster 'work' as $USER and list all nodes in 
# a cluster:
> tsh --proxy=work ls

# Output:
Node Name     Node ID                Address            Labels
---------     -------                -------            ------
turing        11111111-dddd-4132     10.1.0.5:3022     os:linux
turing        22222222-cccc-8274     10.1.0.6:3022     os:linux
graviton      33333333-aaaa-1284     10.1.0.7:3022     os:osx
```

You can filter out nodes based on their labels. Lets only list OSX machines:

```
> tsh --proxy=work ls os=osx

Node Name     Node ID                Address            Labels
---------     -------                -------            ------
graviton      33333333-aaaa-1284     10.1.0.7:3022     os:osx
```

## Interactive Shell

TBD

## Copying Files

TBD

## Sharing Sessions

TBD

## Integration with OpenSSH

TBD

## Troubleshooting

If you encounter strange behaviour, you may want to try to solve it by enabling
the verbose logging by specifying `-d` flag when launching `tsh`.

Also you may want to reset it to a clean state by deleting temporary keys and 
other data from `~/.tsh`

## Getting Help

Please stop by and say hello in our mailing list: [teleport-dev@xxxxxx.com]() or open
an [issue on Github](https://github.com/gravitational/teleport/issues).

For commercial support, custom features or to try our multi-cluster edition of Teleport,
please reach out to us: `sales@gravitational.com`. 
