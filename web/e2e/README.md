# E2E Testing with Playwright

This directory contains the configuration and tests for end-to-end testing against a real Teleport instance using Playwright.

E2E tests should be run by a corresponding Go test in `integration/web-e2e_test.go` and run
using `make test-web-e2e`. To run only the Playwright test directly, you'll need a Teleport instance running locally.

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
# Run all tests with the default START_URL (https://localhost:3080/web/login)
pnpm test

# Run a specific test file.
pnpm test signup.spec.ts

# Run tests against a specific START_URL.
START_URL=https://teleport.dev pnpm test

# Run tests with the Playwright UI, useful for debugging.
pnpm test --ui

# Start the Playwright codegen to generate tests by recording your browser interactions.
pnpm exec playwright codegen

# View past test reports.
pnpm report
```
