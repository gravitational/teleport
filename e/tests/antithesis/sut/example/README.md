# SUT: Example

This is an example System Under Test for Antithesis.

## Topology

The example runs:

- Two Teleport Auth Service containers backed by PostgreSQL.
- Two Teleport Proxy Service containers.
- An nginx TCP load balancer.
- Simple workload container with one command.
