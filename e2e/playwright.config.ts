import { defineConfig, devices } from '@playwright/test';

// Default to localhost:3080/web/login if START_URL is not defined.
const baseURL = process.env.START_URL || 'http://localhost:3080/web/login';

const browserList = (process.env.E2E_BROWSERS || 'chromium').split(',');

// The enterprise specs live in their own tree because they need the enterprise Teleport build and
// a license. A run covers one tree or the other, never both; the Go runner picks via --enterprise.
const suiteDir = process.env.E2E_ENTERPRISE ? '../e/e2e' : '.';

const browserDevices: Record<string, object> = {
  chromium: { ...devices['Desktop Chrome'], channel: 'chromium' },
  firefox: { ...devices['Desktop Firefox'] },
  webkit: { ...devices['Desktop Safari'] },
};

export default defineConfig({
  testDir: `${suiteDir}/tests`,
  timeout: 20_000,
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 1 : 0,
  workers: process.env.CI ? 1 : undefined,
  // No globalSetup: sessions are minted per user on first use (helpers/authState.ts) so the logins spread
  // across the run instead of drawing the whole run's rate limiter budget at once.
  reporter: [
    ['html', { open: 'never' }],
    ['json', { outputFile: 'test-results/results.json' }],
  ],

  use: {
    ignoreHTTPSErrors: true,
    baseURL,
    trace: 'on-first-retry',
    screenshot: 'only-on-failure',
  },

  projects: [
    ...browserList.flatMap(browser => [
      {
        name: `${browser}:authenticated`,
        testDir: `${suiteDir}/tests/web/authenticated`,
        use: { ...browserDevices[browser] },
      },
      {
        name: `${browser}:unauthenticated`,
        testDir: `${suiteDir}/tests/web/unauthenticated`,
        use: { ...browserDevices[browser] },
      },
    ]),

    {
      name: 'connect',
      // Enables interacting with Web UI from Connect test flows.
      use: browserDevices.chromium,
      testDir: `${suiteDir}/tests/connect`,
      workers: 1,
    },
  ],
});
