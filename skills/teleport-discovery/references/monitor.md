# Monitor and Troubleshoot Enrollment

Continues from `references/apply.md`, or starts directly for a status request.

Run **Read status** first. For a healthy result, follow **Healthy state**. Otherwise, follow
the matching **Diagnose** scenario.

## Read status

Resolve the config, then read its status. Use the `discovery_config_name` from the apply
outputs when known. Otherwise, list configs; if exactly one matches `CLOUD`, use it; if several
do, list them and ask the user which to check.

```bash
$TCTL get discovery_config --format=json          # only when the name is unknown
$TCTL get discovery_config/<name> --format=json
```

Read these fields:
- `.status.state`: `DISCOVERY_CONFIG_STATE_SYNCING` while an iteration runs, `DISCOVERY_CONFIG_STATE_RUNNING` when idle with no error, `DISCOVERY_CONFIG_STATE_ERROR` with `.status.error_message` set.
- `.status.integration_discovered_resources.<integration>.<key>`, holding `found`, `enrolled`, and `failed`. `<key>` is `awsEc2` for EC2, `awsEks` for EKS, `azureVms` for Azure VM. A missing count key means `0`. A `<key>` entry with `syncStart` and `syncEnd` but no counts means the last cycle found nothing.

Summarize for the reader instead of printing raw JSON.

## Healthy state

`state` is `RUNNING`, `enrolled` equals `found`, and `failed` is `0`. Report the counts and
stop.

While `state` is `SYNCING`, offer, but do not auto-run, a poll: re-read the config every 30
seconds and print `[<time>] found=<n> enrolled=<n> failed=<n>` per line. Stop when
`found > 0` and `enrolled >= found` and `failed == 0`, when `failed > 0`, or after 10 minutes.

## Diagnose

Evaluate the scenarios in order and run the first that matches. Each reports a diagnosis and
the fix it points to. Apply a fix only by following **Setup** and **Apply**.

### state = ERROR

Read `.status.error_message` and report it to the user. An assume-role or trust error means
the **AWS OIDC trust** requirement is broken: the OIDC provider must be corrected to satisfy
it and re-applied.

### Counts never appear

`.status.integration_discovered_resources` is empty across reads while `state` is `SYNCING`
or `RUNNING`. Run **List Discovery Services**. When none is connected, tell the user to
start a Discovery Service in the config's `discovery_group`. When services are connected
but no active group equals the config's `discovery_group`, inform the user that the
config's `discovery_group` matches no running service and list the active groups.

### found = 0

Confirm the integration targets the right account or subscription, the regions include the
resources, and the tags match the intended resources. A matcher that misses them must be
corrected to match only those resources and re-applied.

### found > 0, enrolled = 0

Run **Check the provision token**, then **Read per-resource failures**. State the matching
**EC2** or **Azure VM** requirement. A failure naming an OIDC or role error points to the
**AWS OIDC trust** requirement.

### failed > 0

Run **Read per-resource failures**, then state the matching requirement.

## Procedures

### List Discovery Services

```bash
$TCTL inventory list --services=discovery
```

An empty list means no Discovery Service is connected. The output does not show each
service's `discovery_group`, so derive the active groups from configs: in
`$TCTL get discovery_config --format=json`, a `discovery_group` is active when any of its
configs has `status.last_sync_time` within the last 15 minutes. For `cloud` the group is
`cloud-discovery-group`.

### Check the provision token

```bash
$TCTL tokens ls
```

Confirm a token exists for the matched resources.

### Read per-resource failures

```bash
$TCTL get user_tasks
```

Report only tasks whose `state` is `OPEN`. `task_type` is `discover-ec2`, `discover-eks`, or
`discover-azure-vm`; `issue_type` names the cause.

For EC2 and Azure VM, a per-instance view adds exit codes. It does not cover EKS, and it
requires Teleport v18.7.6+ for `--cloud=aws`, v19.0.0+ for `--cloud=azure`:

```bash
$TCTL discovery nodes --cloud=<CLOUD> --last=24h --format=json
```

Status values include `Online`, `Unknown`, `Installed (offline)`, `Failed (exit code=<n>)`,
and `Failed (API error)`. Render a table.

For resolution steps on server install failures, fetch the troubleshooting guide and match
each `issue_type`:

```
WebFetch:
  URL: https://goteleport.com/docs/enroll-resources/auto-discovery/servers/troubleshooting
  Prompt: "Extract troubleshooting steps for <CLOUD> discovery: exit code meanings, status interpretations, common errors, and resolutions."
```

## Requirements

State the matching requirement when a scenario points here.

- **EC2**: a provision token, the `AmazonSSMManagedInstanceCore` policy or SSM connectivity, and a network path to the proxy on 443.
- **Azure VM**: a managed identity with `Microsoft.Compute/virtualMachines/read`, and a network path to the proxy.
- **AWS OIDC trust**: the provider URL is the proxy host with no port, and the audience is `discover.teleport`. Re-apply after a proxy TLS certificate rotation to refresh the provider thumbprint.
