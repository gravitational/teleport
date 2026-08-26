# cluster

Automation for creating the `loadtest` kubernetes cluster.

## Usage

Confirm that everything is working correctly:

```bash
$ make plan
```

If you are running this automation for the first time, you may be asked to run
`terraform init` before continuing.


To create/resize the cluster, edit [`terraform.tfvars`](./terraform.tfvars)
as needed, then run:

```bash
$ make apply
```

*NOTE*: The node pool is spread across three regions, so the `nodes_per_zone` variable
should be set to 1/3 the total desired nodes.  Nodes run ~100 pods each, so for a `1k` test for example, 
you will need to set `nodes_per_zone` to `4`. Don't set this value larger than it needs to be.


Once the cluster is up, configure `kubectl` with the appropriate credentials:

```bash
$ make get-creds
```

You should now be ready to proceed with test-specific setup (e.g. [k8s](../k8s)).

---

When you are done, make sure that you destroy the cluster:

```bash
$ make destroy
```
