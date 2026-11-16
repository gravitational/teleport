---
authors: FrenchBen (@fbxai, 186336021+fbxai@users.noreply.github.com)
state: draft
---

# RFD 252 - tsh Environment Configuration Presets

## Required Approvers

Proposed reviewers; these assignments require maintainer agreement:

- Engineering: @zmb3 && (@r0mant || @ravicious)
- Security: @rjones || @klizhentas
- Product: @klizhentas

## UX

### An agent discovers and uses a configured environment

An operator asks an agent to inspect resources in staging. The operator has already supplied the configuration and an authorized identity; presets do not grant access or automate approval of authentication challenges.

The agent discovers the interface through `tsh --help` and `tsh preset --help`, then lists the available environments without connecting to a cluster:

```console
$ tsh preset ls --format=json
[
  {
    "name": "staging",
    "default": true,
    "proxy": "teleport.staging.example.com:443",
    "user": "alice"
  }
]
$ tsh --preset staging status --format=json
```

The status response retains its existing structure and reports the real proxy, user, cluster, and certificate validity. An additive top-level `active_config_preset` field identifies the selected configuration preset and is omitted when none is applied. Selecting a preset does not imply that credentials exist. If status reports missing or expired credentials, the agent follows the existing login flow or reports that authentication is required, rather than trying another environment.

Once authenticated, the agent runs `tsh --preset staging ls --format=json`. For repeated commands it can set `TELEPORT_PRESET=staging` in its process environment. An explicit selection avoids depending on a mutable default. Unknown presets return a nonzero exit status and an actionable error on stderr:

```text
preset "stage" not found; available presets: prod, staging. Run tsh preset ls.
```

### A new user saves a cluster address

Alice works with staging and production. She creates a reusable name without needing credentials or contacting the cluster:

```console
$ tsh preset add staging https://teleport.staging.example.com:443
Preset "staging" added for proxy "teleport.staging.example.com:443".
$ tsh --preset staging login
```

Login uses the existing authentication ceremony and output. `preset add` writes only the proxy address to the user configuration, never credentials or values inferred from a different active login. Reusing an existing name fails without overwriting it. Alice can use the same commands with a Teleport Cloud proxy; no server configuration or administrative access is required.

### An experienced user switches environments and overrides a setting

Alice edits her existing `~/.tsh/config/config.yaml`:

```yaml
presets:
  prod:
    proxy: teleport.prod.example.com:443
    cluster: prod
    user: alice
    mfa_mode: sso
    add_keys_to_agent: "no"
  staging:
    proxy: teleport.staging.example.com:443
    user: alice
default_preset: staging
```

```console
$ tsh --preset prod login
$ tsh --preset prod --login ubuntu ssh node01
$ tsh preset use prod
Default preset set to "prod".
$ tsh preset ls
```

`preset use` changes the default for future invocations. It neither logs in nor changes the shell environment or active credential profile. Alice checks the resolved target with `tsh status`; text output includes `Active Config Preset: prod` in addition to the existing connection details.

Explicit flags and existing environment variables override individual preset values. For example, `TELEPORT_ADD_KEYS_TO_AGENT=no` takes priority even when a preset requests `yes`. Variables previously exported by `tsh env` also take priority. If they point at staging, selecting `prod` alone will not replace them; Alice unsets the conflicting variables or passes explicit target flags.

Alice can use `tsh --preset= status` to disable preset application for that invocation. An explicitly empty flag disables both `TELEPORT_PRESET` and the configured default. An empty or absent `TELEPORT_PRESET` uses the default.

### Alice logs out and reconnects

`tsh --preset prod logout` follows the existing logout behavior for the resolved proxy and user. It removes login state as appropriate but preserves the authored configuration. Alice can subsequently run `tsh --preset prod login` without remembering the address. Removing the entire `~/.tsh` directory still deletes both credentials and configuration; documentation must not recommend this as routine logout or session refresh.

`tsh --preset prod login --force` requests a fresh authentication ceremony. Changing `auth` or `mfa_mode` does not invalidate an otherwise valid certificate.

## What

Add named presets to the existing `tsh` configuration file, selectable through `--preset`, `TELEPORT_PRESET`, or a configured default. A preset supplies reusable connection settings for an environment while retaining existing authentication, credential storage, and explicit flag and environment-variable overrides.

## Why

Users who work across Teleport environments repeatedly specify proxy, cluster, user, authentication, and SSH agent settings. Shell aliases and manually exported variables make these bundles difficult to discover and share, and leave agents dependent on undocumented shell state.

[PR #67694](https://github.com/gravitational/teleport/pull/67694) implements a starting point. Its review identifies broader questions about terminology, environment-variable precedence, multiple users on one proxy, and integration with Teleport Connect. This RFD defines the contract before that implementation ships. It follows the [pinned RFD guidelines](https://github.com/gravitational/teleport/blob/e678215b2d0613843a355e28c5c40493de35da8a/rfd/0000-rfds.md).

## Details

### Scope and terminology

An **environment** is the user's grouping of connection settings, such as production or staging. A **preset** is an authored, named bundle of those settings. A **login profile** is the existing runtime connection state managed by `tsh` under its home directory. Presets contain no login state and can exist before the first login or after logout.

Use `presets`, `default_preset`, `--preset`, and `TELEPORT_PRESET` consistently. Do not introduce a second meaning for `profile` or Kubernetes `context`. The earlier `--profile`/`TELEPORT_PROFILE` names in the PR description are superseded. This feature does not change the existing `tsh env` command or add arbitrary environment-variable assignment, shell execution, or secret storage.

### Configuration schema

Extend `lib/client.TSHConfig` with `Presets` and `DefaultPreset`, and represent each bundle with `TSHConfigPreset`, as proposed in the PR. All preset fields are optional; a proxy-only preset is the usual starting point. Presets may instead supply common settings while explicit input or a login profile supplies the target.

| YAML field | Existing input | Meaning |
| --- | --- | --- |
| `proxy` | `--proxy`, `TELEPORT_PROXY` | Proxy Service address |
| `cluster` | `--cluster`, `TELEPORT_CLUSTER` | Root or leaf Teleport cluster |
| `user` | `--user`, `TELEPORT_USER` | Teleport username |
| `login` | `--login`, `TELEPORT_LOGIN` | Remote host login |
| `auth` | `--auth`, `TELEPORT_AUTH` | Authentication connector |
| `kube_cluster` | `--kube-cluster`, `TELEPORT_KUBE_CLUSTER` | Kubernetes cluster |
| `mfa_mode` | `--mfa-mode`, `TELEPORT_MFA_MODE` | Existing MFA mode |
| `headless` | `--headless`, `TELEPORT_HEADLESS` | Headless authentication preference |
| `add_keys_to_agent` | `--add-keys-to-agent`, `TELEPORT_ADD_KEYS_TO_AGENT` | Existing SSH agent key policy |
| `use_local_ssh_agent` | Deprecated `--use-local-ssh-agent`, `TELEPORT_USE_LOCAL_SSH_AGENT` | Existing compatibility setting |
| `home` | `TELEPORT_HOME` | Credential/profile storage directory |

Boolean fields must distinguish omission from explicit `false`. Validate enums using the existing CLI definitions: MFA modes are `auto`, `cross-platform`, `platform`, `otp`, and `sso`; agent modes are `auto`, `yes`, `no`, and `only`. Quote agent modes in YAML so YAML boolean coercion cannot change their types. Preserve the existing interpretation of the deprecated agent option when both agent fields are present; presets do not create another policy.

Names are case-sensitive, nonempty strings without control characters. Names are map keys, never filesystem paths. Apply the existing proxy parser and field validation to resolved values before any network connection. Configuration errors identify the file, preset, and field, without dumping the configuration.

### Loading and merging

Retain the existing shared configuration lookup: `/etc/tsh.yaml` by default on supported operating systems, or the path supplied by `TELEPORT_GLOBAL_TSH_CONFIG`. Windows retains its existing behavior of skipping the default shared file. Load user configuration from `<bootstrap tsh home>/config/config.yaml`, with `~/.tsh` as the default home. Missing files remain equivalent to empty configuration.

Merge shared configuration first and user configuration second:

- Combine preset maps by name. A user preset replaces the entire shared preset with the same name; there is no field-by-field inheritance.
- A nonempty user `default_preset` overrides the shared default. Omission or an empty string inherits the shared default, matching the PR's merge rule.
- Validate each file's field types and enum values, then validate the default reference against the merged map. A user default may name a shared preset.
- Preserve existing merging of `add_headers`, `proxy_templates`, and `aliases`.

Normal commands reject a dangling `default_preset` rather than silently choosing another target. Preset management commands skip default-reference validation so `preset ls`, `preset add`, and `preset use` can repair a dangling default. They still reject malformed YAML and invalid field values.

### Selection and value precedence

Select one preset using this order:

1. An explicit `--preset <name>`; an explicitly empty `--preset=` disables presets.
2. A nonempty `TELEPORT_PRESET`.
3. The merged `default_preset`.
4. No preset, retaining existing behavior.

For each supported setting, explicit command-line flags override the existing environment variable, which overrides the selected preset. A preset then overrides values recovered from the existing login profile and built-in defaults, wherever those sources apply. Existing programmatic `CliOption` overrides must also be preserved; callers must be able to supply explicit boolean and default-valued overrides.

Track whether input was supplied rather than comparing it with a default. Explicit `--headless=false`, an environment value of `false`, an agent mode of `auto`, and an MFA mode of `auto` must all survive preset application. Retain existing parsing behavior for empty values of other environment variables. Invalid explicit input must fail, rather than being replaced by a preset.

Supplying a preset username has the same intent as supplying `--user`: mark it explicit and do not silently fall back to a locally remembered account. No unknown preset or invalid configuration may fall back to the active profile.

Apply preset values after parsing and programmatic overrides, but before target-dependent validation, proxy normalization, identity-store creation, managed-update lookup, or connection setup. For example, a preset proxy must satisfy the existing proxy requirement when an identity file is supplied. Aliases and client-tools re-execution must retain the selection and explicit overrides. Do not let a cached client store keep the bootstrap home after a preset changes credential storage.

### Bootstrap home and credential isolation

Resolve the bootstrap home from existing explicit inputs, including `TELEPORT_HOME`, before loading configuration. A preset's `home` can change the effective credential/profile storage directory only when that home was not explicitly supplied.

Read configuration once from the bootstrap location. Do not reload configuration or aliases from a selected preset's `home`, and do not follow another `default_preset` there. This avoids circular selection and makes the source of settings predictable. For this initial schema, a configured `home` must be an absolute path; document OS-appropriate examples and perform no shell-variable or command expansion.

Preset management always reads and writes the bootstrap user configuration, even if the default preset selects a different credential home. An explicit `TELEPORT_HOME` selects a different bootstrap configuration and also overrides any preset `home`.

Two presets with the same proxy, username, cluster, and home reuse existing credentials. Different clusters may share proxy/user private keys while using cluster-specific certificates. An alternate `home` separates credentials and profile files, but does not isolate the operating system account, SSH agent, or browser session; it is not a security boundary.

### Multiple users on one proxy

The key store distinguishes users, but the remembered login profile for a proxy records its most recently active user. Named presets do not change that profile model or promise simultaneous independent accounts in one home.

For predictable separate account workflows, configure a distinct absolute `home` for each account and select presets from the bootstrap configuration. For a shared home, a preset username must behave exactly like an explicit `--user`: use matching credentials or require authentication. Never borrow a different account's certificates, automatically log it out, or delete its keys to implement a switch. Verify this against [issue #60892](https://github.com/gravitational/teleport/issues/60892); any prerequisite correction to existing login behavior should be reviewed separately rather than advertised as solved by presets.

For example, the bootstrap configuration on Alice's Linux workstation can contain:

```yaml
presets:
  prod-user:
    proxy: teleport.prod.example.com:443
    user: alice
    home: /home/alice/.tsh-prod-user
  prod-admin:
    proxy: teleport.prod.example.com:443
    user: alice-admin
    home: /home/alice/.tsh-prod-admin
```

Alice runs `tsh --preset prod-admin login` to authenticate in that store. Selecting `prod-user` uses its separate store without copying or deleting either account's credentials.

### Management commands and status

Expose `tsh preset ls`, `tsh preset add <name> <proxy>`, and `tsh preset use <name>`. These commands require no login or network access and do not apply any selected/default preset, including one supplied explicitly. They operate on the merged catalog and write only to the bootstrap user file.

`ls` lists names in stable sorted order and marks the configured default; `--format=json` and `--format=yaml` return structured entries. An empty catalog returns an empty array in JSON. `add` creates a proxy-only preset and rejects a name already present in either configuration. `use` validates the name against the merged catalog before updating only `default_preset` in the user file.

`add` accepts existing proxy address syntax or a profile URL from `tsh status`. Use the existing address parser to normalize hostnames, IPv6, and web ports. URL inputs accept HTTP/HTTPS and no user information, query, fragment, or path other than `/`; HTTP syntax does not disable TLS verification. An omitted URL port uses the scheme's port. Host-style proxy syntax keeps existing `tsh` port defaults. Reject invalid ports and addresses without writing anything.

Both mutating commands also support `--format=json` and `--format=yaml`. Successful responses describe the result without credentials, for example:

```console
$ tsh preset add staging https://teleport.staging.example.com:443 --format=json
{"name":"staging","proxy":"teleport.staging.example.com:443"}
$ tsh preset use staging --format=json
{"default_preset":"staging"}
```

Use the existing file-lock and atomic-write facilities. Hold the lock across read, validation, modification, and replacement; recheck duplicate names under the lock. Preserve unrelated fields and comments. Create directories with owner-only permissions and files with mode `0600` on Unix; use the existing platform facilities on Windows. A failed mutation leaves the original file intact, and concurrent mutations must not lose each other's changes.

`tsh status` continues to report actual login state. The text-only `Active Config Preset` line and additive structured `active_config_preset` field report the selected bundle even when individual values were overridden; they do not assert that every setting came from that bundle. Preserve existing status fields, exit behavior, and empty/expired credential handling.

The PR's per-profile `presets` array lists names with a matching proxy endpoint. Retain it as an informational list, sorted by name. Host-style presets without an explicit web port match the hostname; explicit ports and URL inputs match both host and port. Label these as proxy matches in help/documentation: they do not prove a match for user, cluster, credential home, authentication mode, or the preset selected for this invocation.

### Teleport Connect and alternatives

[The PR review](https://github.com/gravitational/teleport/pull/67694#discussion_r3404718002) asks whether retaining existing login profiles after logout could solve this problem without another configuration concept. [PR #66318](https://github.com/gravitational/teleport/pull/66318) makes Connect remember clusters through workspace state. Those mechanisms help users rediscover previously visited clusters, but do not provide authored names and global setting bundles before login. Presets address that separate need.

Keep Connect's workspace persistence and existing credential/profile lifecycle. Do not automatically import presets into Connect, change its active workspace when `preset use` runs, or treat the new CLI default as Connect policy. Shared configuration entries alone must not trigger a connection. Test that Connect's existing login/logout and profile-watcher behavior tolerates the new fields. Account switching and a shared preset picker can be designed separately with Connect maintainers if needed.

Shell aliases remain useful for command shortcuts, but lack a typed catalog and consistent precedence. Extending `tsh env` would require shell mutation and would conflate settings with its existing export of authenticated profile information. Presets keep selection local to an invocation.

### Implementation scope and differences from the PR

This proposal is grounded in PR #67694 at commit `1137b9eb364462ecc1615632019d4e672704656f`; it does not describe all proposed behavior as already implemented.

- `lib/client/tshconfig.go` and its tests own the schema, merging, and validation.
- `tool/tsh/common/tsh.go` and its tests own selection, input precedence, initialization order, and status output. Audit client construction and re-execution callers so resolution happens before they consume settings.
- `tool/tsh/common/preset.go` and its tests own management commands, address normalization, and safe user-file updates.
- `docs/pages/connect-your-client/teleport-clients/tsh.mdx` documents the schema, agent workflows, override rules, credential reuse, and bootstrap home behavior.

The PR already changes these areas and introduces the core model. Remaining design work includes explicit empty selection, structured mutation responses and selected preset status, repair of dangling defaults through management commands, and enforcement of the bootstrap-home and initialization-order rules. Test actual CLI parsing and environment inputs, in addition to the helper-level tests.

## AI Proofing

Discovery, listing, mutation, selection, and status all have CLI interfaces; management supports structured output without requiring credentials. JSON/YAML stdout contains only the response, while errors and diagnostics use stderr and return existing nonzero error statuses. Mutations do not prompt or contact a cluster. Authentication continues to obey existing interactive and headless requirements; agents receive no new MFA or access-policy bypass.

The documentation will include the first UX story as a short executable workflow. A separate agent skill is unnecessary for these few composable commands. Agents should prefer an explicit preset per invocation and inspect actual status before operating on resources; they should not persist a new default unless their task calls for it.

## Security

Presets are local configuration, not authorization policy. Existing RBAC, certificate validity, device trust, MFA, TLS verification, and access-request checks still apply. No preset field enables insecure TLS, supplies tokens, or changes server-side authorization.

The main risks are unintended target/account selection, unexpected credential export into an SSH agent, and unsafe modification of a configuration file that may already contain sensitive headers. Fail on invalid selection, preserve explicit environment and boolean overrides, and resolve all settings before loading credentials or choosing a managed client binary. Never copy credentials or auth headers into command responses, and never execute preset strings.

User-file updates must not broaden permissions or truncate other configuration. Use the existing platform handling for locks and atomic replacement, including its established symlink behavior; test the write path rather than introducing a general-purpose configuration editor. Escape user-controlled values in text errors and output so names cannot inject terminal control sequences.

There are no new listeners, server endpoints, cryptographic protocols, or server-side resource fan-outs. Management performs bounded local work relative to the configuration size; ordinary connections retain existing rate limits.

## Privacy

Preset names, proxy addresses, usernames, cluster names, and home paths can identify people or internal infrastructure. They remain in the existing local configuration and are included in local output only when requested. No new remote collection is proposed. Do not log complete presets, headers, identities, or credential paths. Removing an entry removes its authored settings; logout does not, and deleting configuration does not revoke existing credentials.

## Proto Specification

No protobuf or server API changes are required. The new interfaces are local YAML fields and additive CLI output fields.

## Scale

Load and merge configuration once per invocation, with linear cost in its size. Listing sorts preset names locally. The PR's status annotation compares profiles with presets, so its cost is proportional to their product; avoid network calls per preset and measure this with a large local catalog before considering indexing. No distributed pagination or UI changes are needed.

## Backward Compatibility

With no preset selected and no default configured, existing behavior remains. No credential files, backend data, remote clusters, or Connect workspaces need migration. Keep the existing `tsh env` exports and status response structure; new status properties are additive.

Older clients do not gain preset support and may ignore new YAML fields. They may therefore fall back to an old active profile. Automation that depends on presets must use a supporting client and an explicit `--preset` flag, which an unsupported binary rejects, rather than relying only on `TELEPORT_PRESET` or `default_preset`. Existing older clients can continue using explicit connection flags with the same credential store. Managed-update re-execution must preserve selection and fail clearly if the selected binary cannot support it.

The unmerged PR's earlier profile/context naming is not a released migration contract. Document the final names and have early testers rename their keys. Changing the default affects subsequent CLI invocations; it does not select a profile for another process already running. A `home` value does not move or copy credentials, and is deliberately limited to absolute paths in this design.

## Audit Events

No new server audit events are required for local configuration operations. Login and resource access continue emitting their existing events for the actual authenticated user and cluster. Preset names are not sent to the server.

## Observability

Reuse the existing debug message indicating that a preset was applied, without logging its values. Status shows both the selected preset and actual identity and target; configuration errors name the field and file to correct. The catalog remains inspectable without valid credentials or a valid default reference. Do not add a daemon, metrics endpoint, or distributed tracing solely for local preset selection.

## Product Usage

No new telemetry is proposed. Initial adoption feedback can come from the implementation PR, support reports, and voluntary user feedback. Any future usage measurement should be separately reviewed and avoid collecting preset names, addresses, usernames, or paths.

## Test Plan

Add the following cases to the implementation PR's [manual test plan](../.github/ISSUE_TEMPLATE/testplan.md), alongside focused unit and CLI integration coverage. These are acceptance criteria, not results claimed for the current draft.

1. Load old configurations unchanged, absent files, malformed YAML, every field type, enum failures, explicit false, and quoted agent policies. Verify shared and user map replacement and a user default referencing a shared preset.
2. Exercise real parsing of flag versus environment versus default selection, explicit empty `--preset=`, unknown names, and no-preset behavior. For every supported field, test flag/environment precedence and explicit values equal to defaults. Include `TELEPORT_ADD_KEYS_TO_AGENT=no` against a `yes` preset.
3. Verify presets override remembered connection values, preset users remain explicit, and alias execution and managed-update re-execution retain the selection. A preset proxy must be resolved before identity validation and client construction. Stale `tsh env` exports must remain visible overrides.
4. Verify alternate homes use the bootstrap catalog once, never load destination aliases/configuration, and instantiate stores in the effective home. Explicit `TELEPORT_HOME` wins. Reject relative preset homes. Management commands must still update the bootstrap file when the default selects another home.
5. Exercise `ls`, `add`, and `use` with no credentials and an unreachable proxy; assert no connection attempt. Test all output formats, empty catalogs, stable ordering, errors on stderr, a dangling default repaired by management, duplicate names in either file, IPv6, ports, and malformed profile URLs.
6. Verify writes retain unrelated fields, comments, and restrictive permissions. Concurrent updates must retain both edits; failed validation or replacement must leave the original intact. Cover existing platform symlink behavior.
7. Test text and structured status with no preset, an explicit selection, default selection, individual overrides, matching proxies, different ports, different users/homes, missing credentials, and expired credentials. Matching proxy names must never imply an authenticated account or selected preset.
8. Manually log in to staging and production, access a root and leaf cluster, force reauthentication, log out, and log back in. Confirm presets survive logout and share or separate credentials as documented. Test same-proxy different-user workflows, using isolated homes as the supported isolation option; no wrong-account reuse or implicit logout is acceptable.
9. Check Teleport Cloud, a self-hosted proxy with a custom port, Linux/macOS, and Windows. Check Connect running during CLI login/logout and default changes. Test a supporting client against an older supported server and failure of explicit preset selection with a client that lacks the feature.

For local manual tests against clusters advertising managed client versions, set `TELEPORT_TOOLS_VERSION=off` to exercise the intended development binary, then test managed-update behavior separately with it enabled.

## Decisions Needed Before Approval

Confirm a maintainer sponsor and required reviewers, particularly a Connect reviewer for shared profile behavior. Approve the preset/profile distinction, the explicit override contract, and bootstrap versus credential-home semantics before expanding the implementation.
