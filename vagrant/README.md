## Vagrant 

This directory contains scripts to create multiple Vagrant machines for experimenting with 
Teleport on multiple nodes. 

There are two nearly identical Vagrantfiles: one for Virtualbox and another for KVM/Libvirt,
they both share `base.rb`

### Default Configuration

`data/var` contains pre-created contents of guest's `/var/lib/teleport` 
`data/opt` contains pre-created contents of guest's `/opt/teleport` (configuration)

Three machines are created, grouped in two clusters, `cluster_a` and `cluster_b`:

* a-auth: CA+node+proxy for "cluster_a"
* a-node: Standalone node for "cluster_a"
* b-auth: CA+node+proxy for "cluster_b"

A reverse tunnels from cluster_a to cluster_b is created. This allows users of
cluster_b to login into any machine of cluster_a.

### How to use

Easy:

```
~: vagrant up
```

Then you need to `vagrant ssh` into a-auth and b-auth and on both CAs you need
to create 'vagrant' user:

```
~: tctl users add
```

