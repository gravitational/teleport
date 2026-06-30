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
