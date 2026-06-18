# Monitor and Troubleshoot Enrollment

Continues from `references/apply.md`, or starts directly for a status request.

## Start: read status

Resolve the config, then read its status. Use the `discovery_config_name` from the apply
outputs when known; otherwise list configs and pick the one whose matchers are `aws` when
`CLOUD` is aws, or `azure` when `CLOUD` is azure.

```bash
$TCTL get discovery_config --format=json          # only when the name is unknown
$TCTL get discovery_config/<name> --format=json
```

Read these fields, whose JSON keys are camelCase:
- `.status.state`: `DISCOVERY_CONFIG_STATE_SYNCING` while an iteration runs, `DISCOVERY_CONFIG_STATE_RUNNING` when idle with no error, `DISCOVERY_CONFIG_STATE_ERROR` with `.status.errorMessage` set.
- `.status.integrationDiscoveredResources.<integration>.<key>`, holding `found`, `enrolled`, and `failed`. `<key>` is `awsEc2` for EC2, `awsEks` for EKS, `azureVms` for Azure VM. `found` counts resources the matcher selected, `enrolled` counts those that joined Teleport, `failed` counts those whose enrollment failed.

Summarize for the reader; do not print raw JSON.

## Healthy state

`state` is `RUNNING`, `enrolled` equals `found`, and `failed` is `0`. Report the counts and
stop.

While `state` is `SYNCING`, counts are still settling. Offer, but do not auto-run, a poll:
re-read the config every 30 seconds and print `[<time>] found=<n> enrolled=<n> failed=<n>`
per line. Stop when `enrolled >= found` and `failed == 0`, when `failed > 0`, or after 15
minutes.

## Scenarios

### state = ERROR

Cause: the Discovery Service rejected the config, or an integration call failed. The reason
is in `.status.errorMessage`. Read it and act on it. An assume-role or trust error points to
the AWS OIDC trust scenario below.

### Counts never appear

Observed: `state` stays `SYNCING` or `RUNNING` but `.status.integrationDiscoveredResources`
is empty across reads.

Cause: the `discovery_group` matches no running Discovery Service, so nothing processes the
config.

Resolve: confirm a service runs.

```bash
$TCTL inventory list --services=discovery
```

For `cloud` the group must be `cloud-discovery-group`. For `self-hosted` it must equal a
running service's `discovery_group`. An empty list means no service is running.

### found = 0

Observed: `found` stays `0`.

Cause: the integration cannot see the resources, or the matcher is too narrow.

Resolve: confirm the right account or subscription, that the regions include the resources,
and that the tags are not too strict. Set tags to `{"*": ["*"]}` to confirm visibility, then
re-apply.

### found > 0, enrolled = 0, EC2

Observed: `awsEc2.found` is above `0` and `awsEc2.enrolled` is `0`.

Cause: instances cannot join. The provision token is missing, the instance lacks the
`AmazonSSMManagedInstanceCore` policy or SSM connectivity, or it has no network path to the
proxy on 443.

Resolve: confirm the token exists, then read the per-resource detail in the failed-resources
scenario below.

```bash
$TCTL tokens ls
```

### found > 0, enrolled = 0, Azure VM

Observed: `azureVms.found` is above `0` and `azureVms.enrolled` is `0`.

Cause: VMs cannot join. The VM lacks a managed identity with
`Microsoft.Compute/virtualMachines/read`, or it has no network path to the proxy.

Resolve: confirm the token exists, then read the per-resource detail in the failed-resources
scenario below.

```bash
$TCTL tokens ls
```

### failed > 0

Observed: `failed` is above `0` for any key.

Cause: matched resources failed to enroll; the per-resource reason is in user tasks.

Resolve: read the failures.

```bash
$TCTL get user_tasks
```

Report only tasks whose `state` is `OPEN`. `task_type` is `discover-ec2`, `discover-eks`, or
`discover-azure-vm`; `issue_type` names the cause. Remove a resolved task with
`$TCTL rm user_task/<name>`.

For EC2 and Azure VM, a per-instance view adds exit codes. It does not cover EKS, and it
requires Teleport v18.7.6+ for `--cloud=aws`, v19.0.0+ for `--cloud=azure`:

```bash
$TCTL discovery nodes --cloud=<CLOUD> --last=24h --format=json
```

Status values include `Online`, `Unknown`, `Installed (offline)`, `Failed (exit code=<n>)`,
and `Failed (API error)`. Render a table, not raw JSON.

For resolution steps on server install failures, fetch the troubleshooting guide and match
each `issue_type`:

```
WebFetch:
  URL: https://goteleport.com/docs/enroll-resources/auto-discovery/servers/troubleshooting
  Prompt: "Extract troubleshooting steps for <CLOUD> discovery: exit code meanings, status interpretations, common errors, and resolutions."
```

### AWS OIDC trust

Observed: `state = ERROR` with an assume-role error, or EC2 found with no enrollment and a
user task naming an OIDC or role error.

Cause: the AWS OIDC trust is broken.

Resolve: the provider URL must be the proxy host with no port, and the audience must be
`discover.teleport`. Re-apply after a proxy TLS certificate rotation to refresh the provider
thumbprint.
