# Redis single instance

The make script manages a single Redis instance in a docker container on the
same host of the database service.

It is assumed that you already have a running Teleport cluster, a running
database service, and approriate roles for database access.

It is assumed that `tctl` can be run either with a logged in `tsh` profile or
has direct access to Teleport Auth through `/etc/teleport.yaml`.

The script was tested on Amazon Linux and MacOS.

## To setup
1. `make init` to generate Teleport database certs.
1. `make up` to launch the docker container.
1. `make dump` to see a sample database definition. Add this to your database service.

## To connect
```bash
$ tsh db connect self-hosted-redis 
localhost:63152> auth teleport
OK
localhost:63152> set name "self-hosted-redis"
OK
localhost:63152>

$ tsh db connect --db-user alice self-hosted-redis
localhost:63458> auth alice f7ca53a40f56ec7ff6a235d9136431c6104a1769eed13d18a2d68f388325f305
OK
localhost:63458> get name
"self-hosted-redis"
localhost:63458>
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
