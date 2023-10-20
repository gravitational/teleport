# Database access with RDS databases

Terraform definition to deploy RDS databases configured for Teleport database
access. The default script will create two Database instances: one PostgreSQL
and one MySQL.

## Prerequisites

## Quickstart

You can use the `Makefile` target `apply` to create a database:

```shell
make apply PREFIX="myloadtest" EKS_CLUSTER_NAME="yourloadtestcluster"
```

To destroy the created resource just change execute the `destroy` target with
the same arguments: 

```shell
make destroy PREFIX="myloadtest" EKS_CLUSTER_NAME="yourloadtestcluster"
```