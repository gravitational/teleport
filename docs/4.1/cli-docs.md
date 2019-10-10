# CLI Docs

## `teleport`

TODO: This section will be overhauled

The Teleport daemon is called `teleport` and it supports the following commands:

| Command     | Description
|-------------|-------------------------------------------------------
| start       | Starts the Teleport daemon.
| status      | Shows the status of a Teleport connection. This command is only available from inside of an active SSH session.
| configure   | Dumps a sample configuration file in YAML format into standard output.
| version     | Shows the Teleport version.
| help        | Shows help.

When experimenting, you can quickly start `teleport` with verbose logging by typing
`teleport start -d`.

!!! danger "WARNING":
    Teleport stores data in `/var/lib/teleport`. Make sure that regular/non-admin users do not
    have access to this folder on the Auth server.

## `tsh`

## `tctl`