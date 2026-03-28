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
import { fileURLToPath } from 'node:url';

import {
  _electron as electron,
  expect,
  test as base,
  type Page,
  TestInfo,
  ElectronApplication,
} from '@playwright/test';

import { connectTshBin, connectAppDir, password, startUrl } from './env';

export async function launchApp(homeDir: string) {
  const requireFromApp = module.createRequire(
    path.join(connectAppDir, 'package.json')
  );
  const executablePath = requireFromApp('electron');

  const electronApp = await electron.launch({
    executablePath,
    args: [connectAppDir, '--insecure'],
    env: {
      ...process.env,
      TELEPORT_TOOLS_VERSION: 'off',
      CONNECT_DATA_DIR: homeDir,
      CONNECT_TSH_BIN_PATH: connectTshBin,
    },
  });

  try {
    const page = await electronApp.firstWindow();
    await page.waitForLoadState('domcontentloaded');

    return {
      electronApp,
      page,
      [Symbol.asyncDispose]: async () => electronApp.close(),
    };
  } catch (err) {
    await electronApp.close();
    throw err;
  }
}

export async function login(page: Page, username = 'bob'): Promise<void> {
  await page.getByRole('button', { name: 'Connect', exact: true }).click();
  const clusterInput = page.getByPlaceholder('teleport.example.com');
  await expect(clusterInput).toBeVisible();

  await clusterInput.fill(new URL(startUrl).host);
  await expect(page.getByRole('button', { name: 'Next' })).toBeEnabled();
  await page.getByRole('button', { name: 'Next', exact: true }).click();

  await page.getByPlaceholder('Username').fill(username);
  await page.getByPlaceholder('Password').fill(password);
  await page.getByRole('button', { name: 'Sign In' }).click();
  await expect(page.getByPlaceholder('Search or jump to')).toBeVisible();
}

export interface App {
  electronApp: ElectronApplication;
  page: Page;
  appConfigPath: string;
}

export const test = base.extend<{
  autoLogin: boolean;
  /**
   * Sets app config before launching the app.
   *
   * Use `withDefaultAppConfig` for normal config overrides.
   */
  appConfig: AppConfigSetup;
  app: App;
}>({
  autoLogin: [false, { option: true }],
  appConfig: [withDefaultAppConfig({}), { option: true }],
  app: async ({ autoLogin, appConfig }, use, testInfo) => {
    await using temp = await fs.mkdtempDisposable(
      path.join(os.tmpdir(), 'connect-e2e-test-')
    );
    const { appConfigPath } = await initializeDataDir(temp.path, appConfig);
    await using launchedApp = await launchApp(temp.path);
    if (autoLogin) {
      await login(launchedApp.page);
    }
    await use({
      electronApp: launchedApp.electronApp,
      page: launchedApp.page,
      appConfigPath,
    });

    if (testInfo.status !== testInfo.expectedStatus) {
      await attachLogs(temp.path, testInfo);
    }
  },
});

export type AppConfigSetup =
  | {
      kind: 'appConfigPatch';
      patch: Record<string, unknown>;
    }
  | {
      kind: 'appConfigRaw';
      rawConfig: string;
    };

/**
 * Writes the Connect app config to the data directory,
 * configuring the terminal to use a wrapper shell that disables history
 * and rc files so that tests don't pollute the user's shell history.
 */
export function withDefaultAppConfig(
  patch: Record<string, unknown>
): AppConfigSetup {
  return {
    kind: 'appConfigPatch',
    patch,
  };
}

export function withRawAppConfig(rawConfig: string): AppConfigSetup {
  return {
    kind: 'appConfigRaw',
    rawConfig,
  };
}

export async function initializeDataDir(
  dataDir: string,
  appConfig: AppConfigSetup
): Promise<{ appConfigPath: string }> {
  const userDataDir = path.join(dataDir, 'userData');
  await fs.mkdir(userDataDir, { recursive: true });
  const appConfigPath = path.join(userDataDir, 'app_config.json');

  await applyAppConfig({
    appConfigPath,
    action: appConfig,
  });

  return { appConfigPath };
}

async function applyAppConfig({
  appConfigPath,
  action,
}: {
  appConfigPath: string;
  action: AppConfigSetup;
}) {
  const shellWrapper = path.resolve(
    path.dirname(fileURLToPath(import.meta.url)),
    '../scripts/connect-e2e-shell.sh'
  );
  const defaultAppConfig: Record<string, unknown> = {
    'usageReporting.enabled': false,
    'terminal.shell': 'custom',
    'terminal.customShell': shellWrapper,
  };

  switch (action.kind) {
    case 'appConfigPatch':
      await fs.writeFile(
        appConfigPath,
        JSON.stringify({ ...defaultAppConfig, ...action.patch }),
        'utf8'
      );
      break;
    case 'appConfigRaw':
      await fs.writeFile(appConfigPath, action.rawConfig, 'utf8');
  }
}

const logFiles = ['main.log', 'renderer.log', 'shared.log', 'tshd.log'];
async function attachLogs(dataDir: string, testInfo: TestInfo) {
  const logsDir = path.join(dataDir, 'userData', 'logs');
  await Promise.all(
    logFiles.map(logFile =>
      testInfo.attach(logFile, {
        path: path.join(logsDir, logFile),
      })
    )
  );
}

export { expect };
