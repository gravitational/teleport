# control-plane

A collection of scripts used to automate the process of setting up a teleport control plane using the
`teleport-cluster` helm chart in `aws`.

*note:* This is an experimental tool used for internal testing and may be buggy/brittle.

## Quickstart

- Ensure aws cli is authenticated.

- Set up eks cluster and load credentials into kubectl.

- Create a `vars.env` in this directory (see `example-vars.env` for required contents and explanation).
Note that the name of the eks cluster must match the `CLUSTER_NAME` variable.

- If needed, run `init.sh` to add/update all necessary helm repos.

- Invoke `./apply.sh` to spin up the teleport control plane and monitoring stack.

- Use `./monitoring/port-forward.sh` to forward the grafana ui.

- When finished, first invoke `./clean-non-kube.sh`, then destroy the eks cluster (this ordering
is important, as some of the resources created by these scripts interfere with eks cluster teardown).

### Database access

You can also deploy database access agents. To setup the databases prior to testing
check the `databases` folder.

With the infrastructure in place, you can deploy the agents with
`deploy-database-agents` target, here is an example:

```shell
make create-token

make deploy-database-agents \
  TELEPORT_VERSION=13.3.5 \
  NODE_TOKEN=00000000000000000000000000000000 \
  PROXY_SERVER=yourcluster.teleportdemo.net:443 \
  NODE_REPLICAS=3 \
  DATABASE_ROLE_ARN=arn:aws:iam::000000000000:role/loadtest-database-access \
  POSTGRES_URL=loadtest-00000000000000000000000000.aaaaaaaaaaaa.us-east-1.rds.amazonaws.com:5432 \
  MYSQL_URL=loadtest-00000000000000000000000000.aaaaaaaaaaaa.us-east-1.rds.amazonaws.com:3306
```

The `DATABASE_ROLE_ARN`, `POSTGRES_URL` and `MYSQL_URL` can be retrieved from
databases Terraform output.
