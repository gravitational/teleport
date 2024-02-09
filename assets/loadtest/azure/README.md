# Terraform automation for Teleport load tests on Azure

This Terraform module sets up:
- a resource group with a random name prefixed by `loadtest-`
- an AKS cluster (parameters in `aks.tf`, random name prefixed by `loadtest-`)
- cert-manager with contributor access to the az.teleportdemo.net zone
- kube-prom-stack
- an Azure Database for PostgreSQL database (random name prefixed by `loadtest-`) with
  - logical replication
  - public network access
  - an admin user `adminuser` accessible from the local Azure AD credentials
  - an admin user `teleport` usable from the Teleport managed identity
- an Azure Blob Storage account (random name prefixed by `lt`), usable by the Teleport managed identity
- a `teleport` namespace with security annotations and restricted egress to the Azure IDMS
- Teleport from the `teleport-cluster` chart (optionally, see the `deploy_teleport` var)
- an `agents` namespace with restricted egress to the Azure IDMS, to deploy agents in by hand
- a public IP for the Teleport proxy service
- DNS entries (`cluster_prefix.dns_zone` and `*.cluster_prefix.dns_zone`)

## Deployment

To initialize the Terraform providers, run `terraform init`. To deploy, if `az account show` returns an error, run `az login`, then edit `terraform.tfvars`, then `terraform apply`.

After deployment, `make create-user` will create a `joe` Teleport account (outputting the invite link on the terminal), `make grafana` will port forward the grafana instance at [http://127.0.0.1:8080/](http://127.0.0.1:8080/), `make psql` will open a `psql` client connected to the backend database.

As a result of some of these commands, or manually with `make aks`, the local kube config should be pointed at the newly created AKS cluster.

## Recommendations

Parameters are sprinkled throughout the module, the main pgbk tunable is `pool_max_conns`, exposed in the chart as `databasePoolMaxConnections` (the current value of 50, in `teleport_kube.tf`, was good enough).

The size of the node pool can be scaled up and down by tweaking it in `aks.tf` and running `terraform apply` again; 10k reverse tunnel nodes required about 9 Standard_D16s_v3 nodes (576GiB of total ram).

## Clean up

To clean everything up, run `make destroy`. It's possible to delete just the teleport deployment (to create it again manually, say) by disabling it in `terraform.tfvars` and then running `terraform apply` again. Selectively destroying other resources is not recommended, as Terraform might get confused.
