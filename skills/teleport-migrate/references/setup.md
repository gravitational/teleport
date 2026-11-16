# Setup

Three stages. Each yields an artifact the operator's pipeline applies
(`runner` = hand off, the default) or that this session applies with
`$TCTL create -f` (`runner` = run it for me). Ask `runner` once, in the
Setup question round, before writing anything.

Hand-off writes Terraform into `<dirname of config_path>/`: `versions.tf`,
`migrator.tf` (stage 1) and `scopes.tf` (stage 3). Run it for me writes the
same resources as YAML to the same directory and applies each file with
`$TCTL create -f <file>` after the user confirms that file. Either way the
files stay on disk for review.

After writing Terraform, run `terraform fmt -check -diff <dir>` and report a
failure verbatim; it parses the files without needing providers.

The Terraform header, once per directory:

```hcl
terraform {
  required_providers {
    teleport = {
      source  = "terraform.releases.teleport.dev/gravitational/teleport"
      version = ">= 18.11.0, < 19.0.0"
    }
  }
}

provider "teleport" {
  alias              = "source"
  addr               = "<source_proxy>"
  identity_file_path = "<admin identity for the source cluster>"
}

provider "teleport" {
  alias              = "target"
  addr               = "<target_proxy>"
  identity_file_path = "<admin identity for the target cluster>"
}
```

The provider authenticates however the operator's pipeline already does; the
identity-file form above is one option. See
https://goteleport.com/docs/reference/terraform-provider/ for the others.

## Stage 1: migrator roles and users

The tool reads both node inventories and SSHes to every source host as root.
Two layers gate every read and they fail differently: a missing rule verb
errors out, a missing label selector drops the resource silently. Every
wildcard below is load-bearing. Three rules serve
the commands after Apply: `rollback` deletes the target node and its minted
token (`delete` on `node` and `scoped_token` on the target) and `retire` the
source node (`delete` on `node` on the source), and `retire` and `promote`
reach hosts as root through the target (`logins` on the target reader).

`migrator.tf`:

```hcl
resource "teleport_role" "migrate_source_reader" {
  provider = teleport.source
  version  = "v7"
  metadata = { name = "migrate-source-reader" }
  spec = {
    allow = {
      logins      = ["root"]
      node_labels = { "*" = ["*"] }
      rules = [
        {
          resources = ["node"]
          verbs     = ["list", "read"]
        },
        {
          # retire deletes the source node.
          resources = ["node"]
          verbs     = ["delete"]
        },
      ]
    }
  }
}

resource "teleport_user" "migrate_source" {
  provider   = teleport.source
  version    = "v2"
  depends_on = [teleport_role.migrate_source_reader]
  metadata   = { name = "migrate-source" }
  spec       = { roles = ["migrate-source-reader"] }
}

resource "teleport_role" "source_migrate_impersonator" {
  provider = teleport.source
  version  = "v7"
  metadata = { name = "migrate-impersonator" }
  spec = {
    allow = {
      impersonate = {
        users = ["migrate-source"]
        roles = ["migrate-source-reader"]
      }
    }
  }
}

resource "teleport_role" "migrate_target_reader" {
  provider = teleport.target
  version  = "v7"
  metadata = { name = "migrate-target-reader" }
  spec = {
    allow = {
      logins      = ["root"]
      node_labels = { "*" = ["*"] }
      rules = [
        {
          resources = ["node"]
          verbs     = ["list", "read"]
        },
        {
          # rollback deletes the target node.
          resources = ["node"]
          verbs     = ["delete"]
        },
        {
          resources = ["scoped_token"]
          verbs     = ["list", "read", "create", "delete"]
        },
        {
          # The skill (SKILL.md "Resolving fields") reads
          # scoped_role_assignment to offer scope names; the tool itself does
          # not use these.
          resources = ["scoped_role", "scoped_role_assignment"]
          verbs     = ["list", "read"]
        },
      ]
    }
  }
}

resource "teleport_user" "migrate_target" {
  provider   = teleport.target
  version    = "v2"
  depends_on = [teleport_role.migrate_target_reader]
  metadata   = { name = "migrate-target" }
  spec       = { roles = ["migrate-target-reader"] }
}

resource "teleport_role" "target_migrate_impersonator" {
  provider = teleport.target
  version  = "v7"
  metadata = { name = "migrate-impersonator" }
  spec = {
    allow = {
      impersonate = {
        users = ["migrate-target"]
        roles = ["migrate-target-reader"]
      }
    }
  }
}
```

The YAML equivalent for run it for me, `migrator-source.yaml` and
`migrator-target.yaml`, is the same three resources per cluster as
`kind: role` (`version: v7`) and `kind: user` (`version: v2`) documents
separated by `---`, applied with
`$TCTL -i <admin identity> --auth-server <proxy> create -f <file>`.

Then, in either mode, the admin user needs `migrate-impersonator` to sign the
identities in stage 2. Tell the user to add it to their own roles, or run
`$TCTL users update <admin> --set-roles <current roles>,migrate-impersonator`
on each cluster when they ask. Never guess the current roles: read them from
`$TSH status`.

## Stage 2: identities

Each cluster's migrator identity is signed by impersonation, which needs the
admin's credentials: an active `tsh login` to that cluster, or an admin
identity file passed with `-i`. Without either, ask the user to run, in a
separate terminal, for each cluster:

```bash
tsh login --proxy=<proxy>
```

Then sign, per cluster, into the directory of `config_path`. Like the rest of
Setup these are handed off unless `runner` is "Run it for me", and then each
is confirmed before it runs:

```bash
$TCTL --auth-server <source_proxy> auth sign --user=migrate-source --out=<dir>/source_id --ttl=8h --overwrite
$TCTL --auth-server <target_proxy> auth sign --user=migrate-target --out=<dir>/target_id --ttl=8h --overwrite
```

`source_identity` is the absolute path of `<dir>/source_id`, `target_identity`
of `<dir>/target_id`; every `-i` in this skill uses the absolute form. A role
edit on either migrator takes effect only after its identity is re-signed.
Identities expire with `--ttl`; re-run this stage when a command fails with an
expired certificate.

## Stage 3: scope tree, tokens, config

For every distinct `target_scope` in `migrations`, the target needs the scope to
exist and a scoped role assignable there that lets users reach the migrated
hosts. A scope exists once a scoped role or assignment names it, so
`scopes.tf` declares one role at the common ancestor and one assignment per
leaf. The role's `scope` is that ancestor (`/org` here), never a leaf, and the
leaves go in its `assignable_scopes`. The example is two leaves under `/org`:

```hcl
resource "teleport_scoped_role" "agent_ssh" {
  provider = teleport.target
  version  = "v1"
  metadata = { name = "agent-ssh" }
  scope    = "/org"
  spec = {
    assignable_scopes = ["/org/team-a/prod", "/org/team-a/dev"]
    ssh = {
      logins = ["root", "ubuntu", "ec2-user"]
      labels = [{ name = "*", values = ["*"] }]
    }
  }
}

resource "teleport_scoped_role_assignment" "operators" {
  provider   = teleport.target
  version    = "v1"
  sub_kind   = "dynamic"
  depends_on = [teleport_scoped_role.agent_ssh]
  metadata   = { name = "migration-operators" }
  scope      = "/org"
  spec = {
    user = "<operator username>"
    assignments = [
      { role = "/org::agent-ssh", scope = "/org/team-a/prod" },
      { role = "/org::agent-ssh", scope = "/org/team-a/dev" },
    ]
  }
}
```

Ask for the logins and the operator username in the Setup round; the scope
does the isolation, so the label selector stays a wildcard. Scopes are a
strict tree: a scoped permission covers its scope and descendants only. A
scope two teams share is a sibling of both, reached by assignment, not a child
of either.

YAML for run it for me is `kind: scoped_role` / `kind: scoped_role_assignment`
(`version: v1`, `sub_kind: dynamic` on the assignment) with the same fields,
role before assignment in one file; an assignment naming a role that does not
exist is rejected.

### Delegated-join tokens

When Plan reports `create scoped token <scope>::<name> (method <m>) in the
target, then run plan again`, the host joins by a delegated method and
the tool cannot mint its token. Add one `teleport_scoped_token` per distinct
reason, with the bare `<name>`, the mapping's scope, and the same allow rules
the source token has. Read those under the admin's `tsh` profile, not the
migrator identity, which has no `token` rule:
`$TCTL --auth-server <source_proxy> get token/<name> --format=json`, `.[0].spec.allow`.
Roles are `Node`, plus `App` or `Kube` for those services when the host also runs them. The iam
shape:

```hcl
resource "teleport_scoped_token" "iam_prod_a" {
  provider = teleport.target
  version  = "v1"
  metadata = { name = "<name>" }
  scope    = "<scope>"
  spec = {
    assigned_scope = "<scope>"
    join_method    = "iam"
    usage_mode     = "unlimited"
    roles          = ["Node"]
    aws = {
      allow = [{
        aws_account = "<account>"
        aws_arn     = "<arn from the source token>"
      }]
    }
  }
}
```

YAML: `kind: scoped_token`, `version: v1`, top-level `scope`, and `spec` as
above with `aws.allow` nested under `spec`. After the token exists, run
`plan --check --resource 'node/<id>'`.

### config.yaml

Write `config_path`:

```yaml
source:
  proxy_server: <source_proxy>
  identity_file: source_id
target:
  proxy_server: <target_proxy>
  identity_file: target_id
migrations:
  - source_labels: team=team-a,env=prod
    target_scope: /org/team-a/prod
  - source_labels: team=team-a,env=dev
    target_scope: /org/team-a/dev
```

The keys are `source`, `target` and `migrations`, plus the optional
`state_dir`. The scoped install on every host uses the fixed suffix
`migrated`, so there is nothing to name.

Relative `identity_file` and `state_dir` paths resolve against the file's
directory, which is why the identities are written as bare names beside the
file. Leave `state_dir` out unless the user named one; the tool then uses
`~/.teleport-migrate`. When the user did, write it absolute: YAML does not
expand `~`. `source_labels` is `*` or a comma-separated `k=v` list where every
key must match. A host may match one mapping only: the tool rejects a config
where one mapping selects every host another selects (`migrations[i] (…)
selects every host that migrations[j] (…) selects; a host can only exist in a
single scope`), and decides partial overlaps per host at plan time. Unknown
keys are an error.

## Troubleshoot

| Symptom | Cause | Action |
|---------|-------|--------|
| The source inventory under `source_identity` is missing hosts the admin sees | a label selector is missing from `migrate-source-reader` | add the `node_labels` wildcard, re-sign the identity |
| `access denied to perform action "list" on "<resource>"` | a rule verb is missing | add the resource to the role's `rules`, re-sign |
| `$TCTL get scoped_role_assignment` prints `[]` on a cluster that has them | tctl older than the cluster, usually because client auto-update re-executed an older binary | run with `TELEPORT_TOOLS_VERSION=off` and a tctl at or above the cluster version |
| the Gate stops on the cluster's edition or FIPS value | `/webapi/ping` on the source proxy returned a value other than `oss`, `ent` or `community`, or a non-boolean `fips`, or no JSON at all (a captive portal or a proxy in front of it) | check `curl https://<source_proxy>/webapi/ping` from the workstation; never run the skill against a proxy whose ping is not Teleport's |
| the Gate stops with a `shasum` or `sha256sum` failure | the downloaded tarball does not match its `.sha256`, often a mirror or proxy rewriting the download | fix the network path to the CDN; nothing was extracted |
| the Gate stops naming a path under `state_dir` | the install directory, or one above it up to `state_dir`, is a symlink, owned by another user, or writable by group or others, so another account could swap the binary | point `state_dir` at a directory only you can write, such as the default `~/.teleport-migrate`, or fix its owner and mode |
| the Gate stops: no macOS FIPS build | the source cluster reports `fips: true` and the workstation is macOS | run the migration from a Linux workstation |
| `tctl auth sign` fails with access denied | the admin lacks `migrate-impersonator` | stage 1's last step |
| `rollback` reports `access denied to perform action "delete" on "node"`, or `retire` fails with `the source agent is already removed from the host; run retire again to delete source node <id>` | the reader role lacks stage 1's `delete` rule on `node`, or, for the minted token, on `scoped_token` in `migrate-target-reader` | add it, re-sign; a rolled-back host is already clean, so its target node can also expire with its heartbeat |
| `retire` or `promote` fails to reach a host through the target with access denied | `migrate-target-reader` lacks `logins: ["root"]` | add it, re-sign |
| a token whose roles are not purely `Node`/`Bot` is rejected at join | the target lacks agent scope pinning | set `TELEPORT_UNSTABLE_AGENT_SCOPE_PIN=yes` on the target auth service |
| `migrations[i] (…) selects every host that migrations[j] (…) selects; a host can only exist in a single scope` | one mapping's selector is a subset of another's | make the selectors disjoint |
