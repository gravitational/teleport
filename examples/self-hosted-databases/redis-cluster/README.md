# Redis cluster

The make script manages a Redis cluster using `docker-compose` on the same host
of the database service.

The Redis servers use self-signed certficates for internal communications and
incoming client connections.

It is assumed that you already have a running Teleport cluster, a running
database service, and approriate roles for database access.

It is assumed that `tctl` can be run either with a logged in `tsh` profile or
has direct access to Teleport Auth through `/etc/teleport.yaml`.

The script was tested on Amazon Linux and MacOS.

## To setup
1. `make init` to generate self-hosted certs and export Teleport database client CA.
1. `make up` to launch the docker containers with `docker-compose`.
1. `make dump` to see a sample database definition. Add this to your database service.

## To connect
```bash
$ tsh db connect self-hosted-redis-cluster
localhost:65120> auth teleport
OK
localhost:65120> set name "self-hosted-cluster"
OK
localhost:65120> 

$ tsh db connect --db-user alice self-hosted-redis-cluster
localhost:65137> auth alice f7ca53a40f56ec7ff6a235d9136431c6104a1769eed13d18a2d68f388325f305
OK
localhost:65137> get name
"self-hosted-cluster"
localhost:65137> get a
(nil)
localhost:65137> get b
(nil)
localhost:65137> 
```

Note `teleport` is the password for the default user, `alice`'s password is
shown as above. `users.acl` contains the sha256 hash of `alice`'s password.

## To teardown
`make clean`

## Sample Teleport role

```yaml
kind: role
version: v5
metadata:
  name: redis-access
spec:
  allow:
    db_labels:
      env: ["teleport-examples"]

    db_users: ["default", "alice"]
```
