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

Unlike the rest of the suite it needs a live Cloud backend and a cloud tenant license, so it is skipped unless `TELEPORT_CLOUD_HOSTPORT` is set.

To run locally, you can use:

```bash
TELEPORT_CLOUD_HOSTPORT=api.cloud.gravitational.io ./e/e2e/run.sh --no-build --no-resource-setup --license-file <cloud-license.pem> e/e2e/tests/web/authenticated/cloudPanel.spec.ts
```

Note that the panel test needs none of the `config/resources/` seeds, and skipping them with `--no-resource-setup` keeps the run independent of whether the cloud license grants those resources' features.
