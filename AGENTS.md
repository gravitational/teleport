# Teleport

Teleport is an identity-based access platform for humans, machines, and AI that
secures infrastructure using short-lived certificates, detailed audit logging,
and fine-grained role-based access controls.

## Repo structure

```
teleport/
├── api/                     # Public API module: proto, clients, types, etc - see `api/README.md`
├── docs/                    # Official documentation: how-tos, references
├── e/                       # Teleport Enterprise code base (git submodule to private repo)
├── e2e/                     # End-to-end tests (usually only run in CI, requires real K8s/RDS)
├── examples/                # Examples and Helm chart
├── integration/             # Integration tests (can run locally)
├── integrations/            # External integrations (e.g. Slack plugins, event handler, IaC providers/modules) - see `integrations/README.md`
├── lib/                     # Core library code (Go, some Rust and C)
├── rfd/                     # Design docs and guidelines
├── tool/                    # Teleport binaries (Go) - see `tool/README.md`
├── web/                     # Web UI (Typescript) and Teleport Connect (Electron) - see `web/README.md`
├── CHANGELOG.md             # All release notes
├── .github/workflows/       # CI workflows like lints
└── .github/ISSUE_TEMPLATE/  # Issue templates including test plans
```

## Dev tips

- Many dirs above have their own `README.md` with area-specific guidance.
  Commands (e.g. `make`, `pnpm`) and file paths are usually referenced from
  this repo root.
- `docs/pages` and `rfd/` are good sources to learn how things work. Prefer
  reading docs locally over fetching `https://goteleport.com/docs/` from the
  internet.
- Check `Makefile` for build, lint, test targets.

### Go dev

- See `rfd/0153-resource-guidelines.md` when adding new resources.
- Prefer internal test helper packages (e.g. `lib/events/eventstest`) or local
  mocks over adding new dependencies to tests.
- Avoid `time.Sleep` and `require.Eventually` in tests as CI is slow and flaky.
  Where possible, prefer `clockwork` fake clock or `synctest`. If unavoidable,
  use a generous timeout like 5s.

## Pull requests

Most PRs need a changelog entry in the PR description. The changelog will be
included in the release notes (see `CHANGELOG.md`). Use the `no-changelog`
label to exclude test, doc, or other non-user-facing changes.
```
changelog: Fixed crash when gadget name is longer than 32 characters
```

### Manual test plan

Features and bug fixes should have a manual test plan in the PR description
using the following format:
```
## Manual Test Plan

### Test Environment

Detail where the tests were performed and what configurations were used. 

### Test Cases

Exhaustive list of various tests that were run to validate the change.

- [ ] Foo
- [ ] Bar
```

For testing environment, prefer using Cloud to test as much as possible (e.g.
`make deploy-cloud` from `e`).

### Backporting

`master` is the next unreleased major. Features backport to the current release
only, and bug fixes and security patches go to current and previous. To find current
release branches: `git branch -r | grep -E "origin/branch/v[0-9]+$" | sort -V | tail -2`

Add `backport/branch/<version>` (e.g. `backport/branch/v18`) labels when
opening a PR to master to trigger bot for automatic backporting. Manual
backports should have branch name `<username>/backport-<PR#>-branch/<version>`
and title `[<version>] <original title>`.
