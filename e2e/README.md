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

By default, the runner creates a single user `bob` with the `access` and `editor` roles. Tests that need
additional users or custom roles can declare them via `test.use()`:

```ts
test.use({
  users: [
    { name: 'restricted-user', roles: ['access'] },
  ],
  loginAs: 'restricted-user',
});
```

The runner scans test files for these declarations and automatically bootstraps the users with WebAuthn
credentials before Teleport starts.

**Custom roles from YAML files:**

For roles that don't exist as built-in Teleport roles, reference a YAML file in `e2e/testdata/roles/`:

```ts
test.use({
  users: [
    {
      name: 'read-only-user',
      roles: [{ file: '@gravitational/e2e/roles/rbac-read-access.yaml' }],
    },
  ],
  loginAs: 'read-only-user',
});
```

The `@gravitational/e2e/roles/` prefix is stripped and the file is loaded from `e2e/testdata/roles/`. The role
name is extracted from `metadata.name` in the YAML. Custom role files are deduplicated, so multiple users can
reference the same file.

**`loginAs`:** Sets which user the test authenticates as. When omitted, the test runs as `bob`.

**Scoping:** `test.use()` follows Playwright's normal scoping rules — place it inside a `test.describe()` block
to limit it to that group, or at the top level of a file to apply to all tests in the file.

### Session Recordings

The runner automatically seeds session recordings into Teleport's data directory at startup so the Web UI's
session recordings page has content immediately. Recordings are stored in `e2e/testdata/recordings/` organized
by session type:

```
e2e/testdata/recordings/
├── events.jsonl          # generated - do not edit
├── ssh/
│   ├── <session-id>.tar
│   ├── <session-id>.metadata
│   └── <session-id>.thumbnail
├── k8s/
│   └── ...
└── desktop/
    └── ...
```

Each recording consists of a `.tar` file (required) and optional `.metadata` and `.thumbnail` sidecar files.
The `events.jsonl` file contains the session end audit events and is auto-generated from the `.tar` files.

**Adding a new recording:**

To add a new recording, place the `.tar` file (and any `.metadata`/`.thumbnail` files) in the appropriate subdirectory
(`ssh/`, `k8s/`, or `desktop/`).

By default, all recordings are associated with the `bob` user. To assign a recording to a different user,
add the session ID and username to the `recordingUserMap` in `e2e/runner/recordings.go`.

At runtime, the runner copies recording files into Teleport's records directory and appends the audit events
to the audit log with adjusted timestamps so that sessions appear recent in the UI.

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
