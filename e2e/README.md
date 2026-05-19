# E2E Testing with Playwright

This directory contains the configuration and tests for end-to-end testing against a real Teleport instance using Playwright.

E2E tests should be run by a corresponding Go test in `web-e2e_test.go` and run
using `make test-web-e2e`. To run only the Playwright test directly, you'll need a Teleport instance running locally.

Any test that involves an authenticated user should be run with a `START_URL` env variable that contains an invite link for the test user, this should be generated and provided by the corresponding Go test.

### Setup

Before being able to run any tests, you'll need to install the playwright package and the chromium browser.

```bash
# Install packages
pnpm install

# Install the Chromium browser
pnpm exec playwright install chromium
```

## Running Tests

### Basic Commands

```bash
# Run a test  with the default START_URL (https://localhost:3080/web/login)
pnpm test signup.spec.ts

# Run a test with a specific START_URL.
START_URL=https://teleport.dev pnpm test signup.spec.ts

# Run tests with the Playwright UI, useful for debugging.
pnpm test --ui

# Start the Playwright codegen to generate tests by recording your browser interactions.
pnpm exec playwright codegen

# View past test reports.
pnpm report
```
