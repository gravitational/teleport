/**
 * Teleport
 * Copyright (C) 2026 Gravitational, Inc.
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

import fs from 'node:fs/promises';
import module from 'node:module';
import os from 'node:os';
import path from 'node:path';

import {
  _electron as electron,
  expect,
  test as base,
  type Page,
} from '@playwright/test';

import { connectTshBin, connectAppDir, password, inviteUrl } from './env';

export async function launchApp(homeDir: string) {
  const requireFromApp = module.createRequire(
    path.join(connectAppDir, 'package.json')
  );
  const executablePath = requireFromApp('electron');

  const app = await electron.launch({
    executablePath,
    args: [connectAppDir, '--insecure'],
    env: {
      ...process.env,
      TELEPORT_TOOLS_VERSION: 'off',
      CONNECT_DATA_DIR: homeDir,
      CONNECT_TSH_BIN_PATH: connectTshBin,
    },
  });

  const page = await app.firstWindow();
  await page.waitForLoadState('domcontentloaded');

  const usageData = page.getByText('Anonymous usage data');
  await usageData.isVisible();
  const declineUsageData = page.getByRole('button', {
    name: 'Decline',
    exact: true,
  });
  await declineUsageData.click();

  return { app, page, [Symbol.asyncDispose]: async () => app.close() };
}

export async function login(page: Page): Promise<void> {
  await page.getByRole('button', { name: 'Connect', exact: true }).click();
  const clusterInput = page.getByPlaceholder('teleport.example.com');
  await expect(clusterInput).toBeVisible();

  await clusterInput.fill(new URL(inviteUrl).host);
  await expect(page.getByRole('button', { name: 'Next' })).toBeEnabled();
  await page.getByRole('button', { name: 'Next', exact: true }).click();

  await page.getByPlaceholder('Username').fill('bob');
  await page.getByPlaceholder('Password').fill(password);
  await page.getByRole('button', { name: 'Sign In' }).click();
  await expect(page.getByPlaceholder('Search or jump to')).toBeVisible();
}

type App = Awaited<ReturnType<typeof launchApp>>;

export const test = base.extend<{
  autoLogin: boolean;
  app: App;
}>({
  autoLogin: [false, { option: true }],
  app: async ({ autoLogin }, use, testInfo) => {
    await using temp = await fs.mkdtempDisposable(
      path.join(os.tmpdir(), 'connect-e2e-test-')
    );
    await using app = await launchApp(temp.path);
    if (autoLogin) {
      await login(app.page);
    }
    await use(app);
  },
});

export { expect };
