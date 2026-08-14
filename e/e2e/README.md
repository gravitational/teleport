# Enterprise E2E Tests

Playwright end-to-end tests that run against an **enterprise** Teleport instance.

This is not a separate test framework. It's the [OSS e2e](../../e2e) runner pointed at a different directory: `run.sh` sets `E2E_DIR=e/e2e` and execs the OSS Go runner, which then discovers tests, applies startup resources, and writes auth state under `e/e2e/` instead of `e2e/`. Everything else — runner flags, helpers, fixtures, user declarations, recordings — is identical to OSS. See [`../../e2e/README.md`](../../e2e/README.md) for the full reference.

## Running

```bash
./e/e2e/run.sh [flags] [test files...]
```

All OSS flags work (`--no-build`, `--ui`, `--debug`, `--browse`, etc.) — see the [OSS README](../../e2e/README.md#flags).

`run.sh` defaults `--license-file` to `e/fixtures/license-eub.pem` so enterprise-gated features are available without flags. Override with `--license-file` if you need a different license.

## Startup resources

YAML files in `config/resources/` are `tctl create`d after each Teleport instance is ready. Use this to seed enterprise-only resources (SAML/OIDC connectors, Access Lists, plugin configs, etc.) before tests run. Files are applied in lexicographic order.

Pass `--no-resource-setup` to skip this step entirely. This is useful when the run's license doesn't grant the features those resources need.

## Cloud Panel test

`tests/web/authenticated/cloudPanel.spec.ts` asserts the panel bundle (served by the Cloud API) mounts and renders. It catches Teleport changes that break how the panel is loaded/mounted or how Teleport calls the Cloud API (GetFile / GetFeatures / GetUsage).

Unlike the rest of the suite it needs a live Cloud backend and a cloud tenant license. It declares its own Teleport config and its own `TELEPORT_CLOUD_HOSTPORT` (see [`../../e2e/README.md`](../../e2e/README.md#teleport-config) for how declared configs work), so the e2e runner restarts Teleport with an active cloud license and the staging cloud API just for this test.

To point it at a different Cloud API host locally, edit both the `TELEPORT_CLOUD_HOSTPORT` value and the `license_file` in the test's `test.use({ teleport: { config, env } } })` block.
