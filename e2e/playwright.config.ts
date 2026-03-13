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

const webUse = {
  ...devices['Desktop Chrome'],
  ignoreHTTPSErrors: true,
  baseURL,
};

export default defineConfig({
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 2 : 0,
  workers: process.env.CI ? 1 : undefined,
  reporter: [['html', { open: 'never' }]],

  use: {
    trace: 'on-first-retry',
    screenshot: 'only-on-failure',
  },

  projects: [
    {
      name: 'setup',
      testDir: './tests/web',
      testMatch: /.*\.setup\.ts/,
      use: webUse,
    },
    {
      name: 'authenticated',
      testDir: './tests/web/authenticated',
      use: { ...webUse, storageState: '.auth/user.json' },
      dependencies: ['setup'],
    },
    {
      name: 'unauthenticated',
      testDir: './tests/web/unauthenticated',
      use: webUse,
    },
    {
      name: 'with-ssh-node',
      testDir: './tests/web/with-ssh-node',
      use: { ...webUse, storageState: '.auth/user.json' },
      dependencies: ['setup'],
    },
  ],
});
