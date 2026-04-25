# E2E Testing with Playwright

This directory contains end-to-end tests that run against a real Teleport instance using Playwright.

## Setup

The runner will install the E2E dependencies and Playwright browsers for you on each run. You can also set up your
environment manually if you prefer:

```bash
pnpm install
pnpm exec playwright install chromium
```

## Running Tests

Tests are run via the `run.sh` script, which builds and executes the Go test runner:

```bash
./e2e/run.sh [flags] [test files...]
```

You can invoke this from either the `e2e/` directory or the root of the repository (or any other directory, really) -
the runner will rewrite any `test-result/` paths to be relative to the current working directory, so you can still
click through to screenshots and anything else from Playwright's test results.

### Modes

By default, the runner runs in test mode. Use one of the following flags to change the mode (mutually exclusive):

| Flag               | Description                                                                     |
|--------------------|---------------------------------------------------------------------------------|
| `--ui`             | Open Playwright UI mode                                                         |
| `--debug`          | Run tests with Playwright inspector (`PWDEBUG=1`)                               |
| `--codegen`        | Open Playwright codegen against running Teleport. Available only for web tests. |
| `--browse`         | Open a signed-in browser for manual web testing                                 |
| `--browse-connect` | Open a signed-in Teleport Connect app for manual testing                        |

### Flags

| Flag                   | Default          | Description                                                                             |
|------------------------|------------------|-----------------------------------------------------------------------------------------|
| `-v`                   | `false`          | Enable debug logging                                                                    |
| `--no-build`           | `false`          | Skip `make` binaries (useful during development)                                        |
| `--quiet`              | `false`          | Redirect Teleport logs to file instead of stdout                                        |
| `--replace-certs`      | `false`          | Generate new self-signed certificates                                                   |
| `--update-snapshots`   | `false`          | Update Playwright snapshot baselines                                                    |
| `--teleport-log-level` | `INFO`           | Teleport log severity (`DEBUG`, `INFO`, `WARN`, `ERROR`)                                |
| `--license-file`       |                  | Path to Teleport license file (required for Enterprise features)                        |
| `--teleport-bin`       | `build/teleport` | Override teleport binary path (env: `TELEPORT_BIN`)                                     |
| `--tctl-bin`           | `build/tctl`     | Override tctl binary path (env: `TCTL_BIN`)                                             |
| `--teleport-url`       |                  | Override Teleport URL (env: `TELEPORT_URL`). If set, the runner skips starting Teleport |

### Fixtures

Fixtures are optional pieces of test infrastructure (like an SSH node or Teleport Connect) that are
auto-detected from test files. When a test declares `test.use({ fixtures: ['ssh-node'] })`, the runner
automatically starts the required infrastructure.

Available fixtures:

| Fixture      | Description                                                                          |
|--------------|--------------------------------------------------------------------------------------|
| `ssh-node`   | Start and connect a Teleport SSH node (runs in Docker)                               |
| `connect`    | Build Teleport Connect. Auto-detected from Connect test helpers.                     |

Fixtures can also be enabled manually with `--with-<name>` flags (e.g. `--with-ssh-node`, `--with-connect`),
which is useful for modes like `--codegen` or `--browse` where auto-detection does not run.

### Users and Roles

By default, the runner creates a single user with the `access` and `editor` roles. Tests that need custom
roles or traits can declare them via `test.use()`. Usernames are auto-generated as human-readable IDs
(e.g. `brave-falcon`) — tests don't specify names.

**Single user (most common):**

```ts
test.use({
  user: { roles: ['access', 'editor'] },
});
```

The singular `user` form creates one user and automatically logs in as that user.

**Multiple users:**

For tests that need more than one user (e.g. RBAC tests), use the `users` array. At least one user must
have `loginAs: true` to indicate which user the test authenticates as:

```ts

test.use({
  users: [
    { roles: ['access', 'editor'], loginAs: true },
    { roles: [{ file: '@gravitational/e2e/roles/viewer.yaml' }] },
  ],
});
```

**Traits:**

Users can specify Teleport traits. Built-in trait keys (`logins`, `kubernetes_groups`, `db_names`, etc.)
are typed in the `UserTraits` interface, and custom keys are also supported:

```ts
test.use({
  user: {
    roles: ['access'],
    traits: { logins: ['root', 'alice'], kubernetes_groups: ['dev'] },
  },
});
```

When traits are omitted, the default is `{ logins: ['root'] }`.

**Custom roles from YAML files:**

For roles that don't exist as built-in Teleport roles, reference a YAML file in `e2e/testdata/roles/`:

```ts
test.use({
  user: {
    roles: [{ file: '@gravitational/e2e/roles/rbac-read-access.yaml' }],
  },
});
```

The `@gravitational/e2e/roles/` prefix is stripped and the file is loaded from `e2e/testdata/roles/`. The role
name is extracted from `metadata.name` in the YAML. Custom role files are deduplicated, so multiple users can
reference the same file.

**`username` fixture:** The generated username of the logged-in user is available as the `username`
fixture value, for use in test assertions.

**Switching users mid-test:** For RBAC-style tests that need to swap users partway through, use the `loginAs`
fixture. It takes an index into the `users` array and swaps the page's cached session state for that user
without running the UI login flow, so the switch is near-instant.

```ts
test.use({
  users: [
    { roles: ['access'], loginAs: true },
    { roles: ['editor'] },
  ],
});

test('switch users', async ({ page, loginAs }) => {
  // signed in as users[0] at test start
  // ...
  const editorName = await loginAs(1);
  // now signed in as users[1]; `editorName` is the generated username
});
```

**Scoping:** `test.use()` follows Playwright's normal scoping rules — place it inside a `test.describe()` block
to limit it to that group, or at the top level of a file to apply to all tests in the file.

**Account isolation:** the runner gives each spec its own bootstrapped account, even when two specs declare
the same `user`/`users`. Concretely:
- Tests with no `test.use({ user/users })` in a project that needs auth (web `:authenticated` projects and
  the `connect` project) fall through to a single shared `access`/`editor` default user.
- An explicit `test.use({ user: { roles: [...] } })` always gets a fresh account, distinct from the default
  even when the roles match.
- Two specs that declare identical `test.use({ user: ... })` get distinct accounts (keyed by spec path).
- Tests in the same spec share one account when their `test.use()` resolves to identical content. To force
  separate accounts, vary traits or use the `users: [...]` array (entries are distinguished by index).
- Helper-declared `test.use({ user/users })` stays shared across every spec that imports the helper.

**How it works (briefly):** The runner generates credentials server-side, logs each user in over HTTP
(`/v1/webapi/mfa/login/*`) during setup, and writes the resulting cookies + localStorage to
`e2e/.auth/<browser>-<username>.json`. Tests pick up that state via Playwright's `storageState`, so they
start already authenticated without running the UI login flow.

### Common Commands

Typically, you'll want to run with `--no-build` during test development to skip rebuilding Teleport binaries on every
run. `--quiet` is also useful to reduce the noise from Teleport logs. The logs are captured in `teleport.log` for
debugging purposes.

Connect is built automatically when running `tests/connect` paths or when using `--browse-connect`.

```bash
# Run a specific test, skip rebuilding (fastest iteration loop)
./e2e/run.sh --no-build e2e/tests/web/authenticated/roles.spec.ts

# Run only Connect tests, skip rebuilding of both Teleport and Connect
./e2e/run.sh --no-build e2e/tests/connect

# Open a browser with auth already set up for manual testing
./e2e/run.sh --browse

# Open Connect with auth already set up for manual testing
./e2e/run.sh --browse-connect

# Debug a failing test with the Playwright inspector
./e2e/run.sh --debug e2e/tests/web/authenticated/roles.spec.ts

# Open Playwright UI mode (pick and run tests interactively)
./e2e/run.sh --ui

# Record a new test by interacting with the browser
./e2e/run.sh --codegen

# Update snapshot baselines after a visual change
./e2e/run.sh --update-snapshots e2e/tests/web/authenticated/ssh.spec.ts
```

### More Examples

```bash
# Run all tests
./e2e/run.sh

# Run SSH node tests (fixture is auto-detected)
./e2e/run.sh e2e/tests/web/authenticated/ssh.spec.ts

# Run all tests, skipping the Teleport build
./e2e/run.sh --no-build

# Run against an existing Teleport instance (doesn't work yet as authentication is hardcoded to the e2e setup and we need to figure out auth for remote instances)
./e2e/run.sh --teleport-url https://localhost:3080

# Set the Teleport log level to DEBUG for more verbose output
./e2e/run.sh --teleport-log-level DEBUG
```
