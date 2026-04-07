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
