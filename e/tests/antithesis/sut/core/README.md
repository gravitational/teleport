# SUT: Core

This system under test exercises core Teleport properties in a standard configuration of 2 auth servers and 2 proxy servers. 

## Topology

This SUT runs:

- Two Teleport Auth Service containers backed by PostgreSQL.
- Two Teleport Proxy Service containers.
- An nginx TCP load balancer.
- Agent node configured with SSH service


### Test Templates

* `crud`, tests focusing on properties of CRUD operations. 
* `ssh`, tests running commands against a Teleport SSH service agent.
* `apps`, issues user certificate and performs a http request using said cert.
* `db`, registers static and dynamic Postgres databases and runs Database Access queries.
