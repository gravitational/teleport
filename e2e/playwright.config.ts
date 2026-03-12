/**
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

import { defineConfig, devices } from '@playwright/test';

// Default to localhost:3080/web/login if START_URL is not defined.
const baseURL = process.env.START_URL || 'http://localhost:3080/web/login';

const browserList = (process.env.E2E_BROWSERS || 'chromium').split(',');
const multiBrowser = browserList.length > 1;

const browserDevices: Record<string, object> = {
  chromium: { ...devices['Desktop Chrome'], channel: 'chromium' },
  firefox: { ...devices['Desktop Firefox'] },
  webkit: { ...devices['Desktop Safari'] },
};

function projectName(browser: string, name: string): string {
  return multiBrowser ? `${browser}:${name}` : name;
}

export default defineConfig({
  testDir: './tests',
  timeout: 15_000,
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 1 : 0,
  workers: process.env.CI ? 1 : undefined,
  reporter: [['html', { open: 'never' }]],

  use: {
    ignoreHTTPSErrors: true,
    baseURL,
    trace: 'on-first-retry',
    screenshot: 'only-on-failure',
  },

  projects: [
    // Setup runs once in Chromium and generates auth state shared by all browsers.
    {
      name: 'setup',
      testDir: './tests/web',
      testMatch: /.*\.setup\.ts/,
      use: browserDevices['chromium'],
    },

    ...browserList.flatMap(browser => [
      {
        name: projectName(browser, 'authenticated'),
        testDir: './tests/web/authenticated',
        use: { ...browserDevices[browser], storageState: '.auth/user.json' },
        dependencies: ['setup'],
      },
      {
        name: projectName(browser, 'unauthenticated'),
        testDir: './tests/web/unauthenticated',
        use: { ...browserDevices[browser] },
      },
      {
        name: projectName(browser, 'with-ssh-node'),
        testDir: './tests/web/with-ssh-node',
        use: { ...browserDevices[browser], storageState: '.auth/user.json' },
        dependencies: ['setup'],
      },
    ]),

    {
      name: 'connect',
      testDir: './tests/connect',
    },
  ],
});
