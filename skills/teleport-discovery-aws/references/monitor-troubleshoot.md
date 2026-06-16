# Verify, Monitor, and Troubleshoot

## Verify

```bash
tctl get integrations                       # AWS OIDC integration present (sub_kind: aws-oidc)
tctl get discovery_config/<config_name>     # matchers present
tctl inventory list --services=discovery    # a discovery service runs the config's group
```

For EC2 also confirm the provision token:

```bash
tctl tokens ls                              # the IAM join token present (requires MFA)
```

## Monitor first sync

Offer this. Never start it automatically.

Poll the discovery config and read the enrollment counts:

```bash
tctl get discovery_config/<config_name> --format=json
```

Counts live at:

```
.status.integration_discovered_resources.<integration_name>.<key>.{found,enrolled,failed}
```

`<key>` is camelCase in JSON output: `awsEc2` for EC2, `awsEks` for EKS.

- `found`: resources matched by the matcher.
- `enrolled`: resources that joined Teleport.
- `failed`: resources whose enrollment failed.

Poll every 30 seconds. Print one line per poll: `[<time>] found=<n> enrolled=<n> failed=<n>`.
Stop when `enrolled >= found` and `failed == 0` (success), when `failed > 0`, or
after 15 minutes.

## Troubleshoot

| Symptom | Cause | Action |
|---------|-------|--------|
| Config exists, nothing discovered, status never updates | `discovery_group` matches no running discovery service | `tctl inventory list --services=discovery`. For Cloud the group must be `cloud-discovery-group`. For self-hosted it must equal a running service's group. |
| `found = 0` | Integration cannot see the resources, or matcher too narrow | Wrong account, region list excludes them, or tags too strict. Broaden tags to `{"*": ["*"]}` to confirm visibility. |
| `found > 0`, `enrolled = 0` (EC2) | Instances cannot join | Token missing or misnamed (`tctl tokens ls`), instance lacks the `AmazonSSMManagedInstanceCore` policy or SSM connectivity, or no network path to the proxy on 443. |
| Integration shows errors, cannot assume role | OIDC trust broken | Provider URL must be the proxy host with no port. Audience must be `discover.teleport`. If the proxy TLS cert rotated, refresh the provider by re-applying Terraform. |
| Discovery failures reported as user tasks | Per-resource enrollment errors | `tctl get user_tasks` for detail. Remove a resolved one with `tctl rm user_task/<name>`. |
