## Teleport Decision Service/PDP Prototype Testing

The Decision Service prototype allows querying teleport for simulated access-control decisions
as if teleport were a classical PDP. Invocation of the Decision Service API requires use of the
`tctl` command-line tool in "local admin" mode (i.e. running alongside the teleport process).
The easiest way to query scenarios with the Decision Service is by starting teleport in a docker container,
loading any relevant configuration, and invoking the relevent decision methods.

This directory contains an example that makes a few test queries against a simple teleport state defined in
[`example-config/bootstrap.yaml`](./example-config/bootstrap.yaml). See [`run-example.sh`](./run-example.sh)
for a basic example of how to invoke the Decision Service in a docker container. This example uses the
`--bootstrap` flag to pre-load teleport with the initial state (users/roles/nodes), then checks access
scenarios by exec'ing `tctl` inside of the container.

Currently, the only supported decision method is `tctl decision evaluate-ssh-access` which simulates a subset of
the access checks performed by teleport when a user attempts to gain ssh access to a node.

Invocation of a decision method returns a decision object with one of the two following fields:

- `permit`: A conditional allow decision with parameters describing the limitations/conditions of access. Access is only permissible if the caller is capable of enforcing all limitations described within the permit.
- `denial`: A deny decision with at least a message suitable for display to the user, and possibly with other additional information as appropriate.

Ex:

```shell
$ tctl decision evaluate-ssh-access --login=alice --username=alice --server-id="$prod_node_id"
{
    "denial":  {
        "metadata":  {
            "pdp_version":  "18.0.0-dev",
            "user_message":  "user alice@root is not authorized to login as alice@root: access to node denied. User does not have permissions. Confirm SSH login."
        }
    }
}

$ tctl decision evaluate-ssh-access --login=alice --username=alice --server-id="$staging_node_id"
{
    "permit":  {
        "metadata":  {
            "pdp_version":  "18.0.0-dev"
        },
        "forward_agent":  true,
        "port_forward_mode":  "SSH_PORT_FORWARD_MODE_LOCAL",
        "x11_forwarding":  true,
        "ssh_file_copy":  true,
        "session_recording_mode":  "best_effort"
    }
}
```

Note that the evalute ssh access method is a work in progress and the permit only contains a subset of
the parameters that will eventually need to be included in order for the Decision Service to fully replace
teleport's current access control logic.

An overview/reference document for Teleport Roles can be found [here](https://goteleport.com/docs/reference/access-controls/roles/).

Teleport SSH Access decisions are primarily decided by the `node_labels` role field. For more advanced usecases,
it is worth reviewing our docs on [interpolation](https://goteleport.com/docs/admin-guides/access-controls/guides/role-templates/#interpolation-rules).
