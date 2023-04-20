# Desktop Registration

In some cases, you may wish to disable Teleport's LDAP-based discovery
and register Windows desktops manually. While you can register desktops
via a config file, this approach doesn't work well for ephemeral environments
where desktops can come and go.

This example shows how to use Teleport's API client to register desktops.
It is intended to be used as a starting point for developing your own
integrations in cases where LDAP discovery is not available or insufficient.

## Authentication

This example authenticates to Teleport by using the `tsh` profile from disk.
This means you must run a `tsh login` prior to running the example.
The Teleport API can also load credentials from identity files generated via
`tctl auth sign` or with Teleport Machine ID.

## RBAC

The example must run with a role that grants `create` and `update` permission
on the `windows_desktop` resource.

```yaml
kind: role
version: v6
metadata:
  name: heartbeat-desktops
spec:
  allow:
    rules:
    - resources:
      - windows_desktop
      verbs:
      - create
      - update
```