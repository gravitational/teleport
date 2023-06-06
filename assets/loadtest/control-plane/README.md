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
