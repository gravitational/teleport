# Load test Teleport SCIM integration using Grafana k6

## Setup

### Prerequisites

- Teleport cluster and an admin user with preset `editor` role or equivalent.
- `tsh` and `tctl`.
- `terraform`.

### Install Grafana k6

You can install k6 on macOS using brew:
```sh
brew install k6
```

### Set up Teleport Terraform provider.

Teleport Terraform provider expects a valid Teleport user session
and proxy address. To obtain these values, log in to Teleport cluster using `tsh`.
```sh
tsh login --proxy=<proxy-address> --user=<teleport-user>
```

Now export your Teleport credential for Terraform.
```sh
eval "$(tctl terraform env)"
```

### Install the SAML connector

SCIM plugin requires an SAML connector.

If you already have an SAML connector set up in your cluster, skip this step.

Otherwise, use the `make connector` command to create a new SAML connector named
`loadtest` which is configured with a fake SAML IdP metadata.

If you want to set up a valid SAML connector like Okta, first, refer to the official 
[Okta SSO](https://goteleport.com/docs/zero-trust-access/sso/integrate-idp/okta/) guide to learn how 
to configure Okta and obtain a metadata file to configure Teleport.
Once you have your Okta instance set up, obtain the SAML metadata file
and create a new Auth Connector resource using Terraform:
```sh
make connector CONNECTOR=okta IDP_METADATA=path-to-okta-metadata-file
```

### Install the SCIM plugin

Install the plugin using `make plugin` command. You will need to reference the 
name of the connector you set up in the step above.

```sh
make plugin CONNECTOR=loadtest
```

A successful plugin installation prints SCIM metadata needed to configure SCIM client.
```
SCIM Plugin Installed Successfully
 Base URL:               https://example.teleport.sh:443/v1/webapi/scim/scim
 OAuth Token URL:        https://example.teleport.sh:443/v1/webapi/plugin/scim/token
 OAuth Client ID:        abc123-id
 OAuth Client Secret:    abc123-secret
```

Export these credentials so that the SCIM client can discover them to load test
the SCIM integration.
```sh
export BASE_URL=<Base URL> TOKEN_URL=<OAuth Token URL> CLIENT_ID=<OAuth Client ID> CLIENT_SECRET=<OAuth Client Secret>
```

## Usage

The makefile exposes following targets to run load tests based on k6:
| Target | Runs |
|---|---|
| `smoke` | `scim-smoke.js`. Minimally test k6 client and SCIM integration |
| `users` | `scim-users.js`. Generate SCIM user requests. |
| `members` | `scim-members.js`. Generate SCIM member requests. |
| `groups` | `scim-groups.js`. Generate SCIM get group requests. |

The following environment variables are available to all the load test scripts:

| Variable | Required | Description |
|---|---|---|
| `BASE_URL` | yes | The `Base URL` printed by `make plugin`. E.g., `https://example.teleport.sh/v1/webapi/scim/scim` |
| `TOKEN_URL` | yes | The `OAuth Token URL` printed by `make plugin`. E.g., `https://example.teleport.sh/v1/webapi/plugin/scim/token` |
| `CLIENT_ID` + `CLIENT_SECRET` | yes | Plugin OAuth client credentials |
| `TIMEOUT` | no | Per-request timeout. Default `5m` (e.g. `30m` for long waits) |
| `MAX_RETRIES` | no | Retry `429` responses up to N times after `Retry-After`. Default `0`. Retries are counted in `scim_retries` |
| `VUS` | no | k6 virtual users, defaults to `1`. `scim-smoke.js` always uses `1`. |

Each k6 run writes `results/<NAME>.json` (summary), `results/<NAME>.log` and 
`results/<NAME>.html` (k6 web dashboard report). 
`NAME` defaults to the target, plus `-<OP>` when `OP` is set (e.g. `make users OP=delete`
writes `results/users-delete.*`).

### Run smoke test

To ensure you have correct k6 settings, run the smoke test.

```sh
make smoke
```

The command above creates an Access List in Teleport named `k6-group-0-smoke`,
creates a user named `k6-user-0-smoke@example.com` and adds that user to the 
Access List.

Then it performs cleanup by removing the group membership, deleting the user
and deleting the Access List.


## User load test

Create 1k users, with 10 k6 virtual users (`VUs`).
```sh
make users USER_COUNT=1000 VUS=10 MAX_RETRIES=10
```

To read users, use `OP=get` env var. To delete users, use `OP=delete` env var.

User specific env vars:
| Variable | Default | Meaning |
|---|---|---|
| `USER_START` | `0` | First user index (`k6-user-<n>@example.com`) |
| `USER_COUNT` | `1` | Number of users |
| `OP` | `create` | `create`, `get` or `delete` |

Only users created through SCIM will be deleted.


## Member load test

Member specific env vars:
| Variable | Default | Meaning |
|---|---|---|
| `GROUP_START` | `0` | First group index (`k6-group-<n>`) |
| `GROUP_COUNT` | `1` | Number of groups |
| `MEMBERS_MIN`, `MEMBERS_MAX` | `5` | Member range to populate groups |
| `USER_POOL` | `MEMBERS_MAX` | Users `k6-user-0` to `k6-user-<USER_POOL - 1>` that members are drawn from |
| `OP` | `add` | `add` or `remove` |

Before running this member load test, ensure:
- The users exist in Teleport. Run `make users` with `USER_COUNT` matching with 
  the `USER_POOL` to be used for members.
- Access Lists exist in Teleport. Teleport does not support creating Access List 
  from SCIM group, so you need to create an Access List of `scim` type first, and 
  use its name to link with the SCIM group. Group to Access List linking is 
  automatic because the generated Access List and group names are deterministic.

**Example 1**: 
To patch 100 members to 100 groups each, starting from `k6-group-0` 
to `k6-group-99`, first, ensure users:
```sh
make users USER_COUNT=1000 VUS=10 MAX_RETRIES=10
```

Next, create Access Lists:
```sh
make acl ACL_COUNT=100
```

Now patch the groups, each with 100 members, spread across 1k user pool.

```sh
make members NAME=members-10k GROUP_COUNT=100 MEMBERS_MIN=100 MEMBERS_MAX=100 USER_POOL=1000 VUS=50 MAX_RETRIES=10
```

By default, each Auth instance allows 600 SCIM requests per minute, with bursts 
of up to 1,200, and at most 300 concurrent writes. The rate limit can be [increased](https://goteleport.com/docs/identity-governance/integrations/scim/#rate-limiting) 
by updating `spec.scim.rate_limit` on the plugin.

**Example 2**: 
Patch single group (e.g., k6-group-99 created above) with 50k members sourced from 
existing pool of 60k users. 

First ensure users:
```sh
make users USER_COUNT=60000 VUS=10 MAX_RETRIES=10
```

Now patch members:
```sh
make members NAME=members-50k GROUP_START=99 GROUP_COUNT=1 MEMBERS_MIN=50000 MEMBERS_MAX=50000 USER_POOL=60000 VUS=50 MAX_RETRIES=10
```

Note: Since the Access Lists are created using Terraform, if you create 50 Access Lists
`make acl ACL_COUNT=50` in the earlier test and then perform another test with 10 Access Lists
`make acl ACL_COUNT=10`, Terraform will delete `k6-group-10` to `k6-group-49` and you
will need to create them again if you want to test in a larger group pool. 

To remove members, add `OP=remove` env var. It works similar to how members are
picked up when adding them, except that the request is sent as `remove` op.

## Read group

Implements groups read `GET /Groups/<id>`. 

Group specific env vars:
| Variable | Default | Meaning |
|---|---|---|
| `GROUP_START` | `0` | First group index (`k6-group-<n>`) |
| `GROUP_COUNT` | `1` | Number of groups |

For example, read `k6-group-99`:
```sh
make groups GROUP_START=99
```

## Cleanup

```
make destroy USER_COUNT=<num_created_users> ACL_COUNT=<num_created_acls>
```

The command will delete SCIM users, Access Lists, SCIM plugin and connector. 


## Contributing

If you are new to Grafana k6, start with the official [getting started](https://grafana.com/docs/k6/latest/get-started/write-your-first-test/) guide.

This project contains four k6 load test scripts:
- `scim-smoke.js`: minimally test k6 configuration for SCIM load testing.
- `scim-users.js`: create, get or delete SCIM users. Update this file to implement a new load test logic that covers user operations.
- `scim-members.js`: add or remove SCIM group members. Update this file to implement a new load test logic that covers member operations.
- `scim-groups.js`: get SCIM groups with all their members.

`client.js`: connects k6 internals with `lib` and SCIM client implementation.

`lib` contains JS helpers and SCIM client.
- `scim.js` implements minimal SCIM 2.0 client. Update this script to make changes to core SCIM client implementation. 
- `auth.js` implements OAuth token exchange.
- `config.js` defines client, user, member and group configuration.
- `fixture.js` defines the deterministic user and group names, and how members are spread across groups.

