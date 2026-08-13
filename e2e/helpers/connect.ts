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
import type { DisposableTempDir } from 'node:fs/promises';
import module from 'node:module';
import os from 'node:os';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

import {
  _electron as electron,
  expect,
  type Page,
  TestInfo,
  ElectronApplication,
} from '@playwright/test';

import { defaultUsername } from './defaultUser';
import { connectTshBin, connectAppDir, users, startUrl } from './env';
import type { UserCredentials } from './env';
import { test as fixtureBase } from './test';

export async function launchApp(homeDir: string, creds?: UserCredentials) {
  const userCreds = creds ?? lookupCreds(defaultUsername());
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
      E2E_WEBAUTHN_PRIVATE_KEY: userCreds.webauthnPrivateKey,
      E2E_WEBAUTHN_CREDENTIAL_ID: userCreds.webauthnCredentialId,
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

function lookupCreds(username: string): UserCredentials {
  const creds = users[username];
  if (!creds) {
    throw new Error(`no credentials for user "${username}" in E2E_USERS_FILE`);
  }
  return creds;
}

export async function login(
  page: Page,
  username?: string,
  password?: string
): Promise<void> {
  if (!username) {
    username = defaultUsername();
    password = lookupCreds(username).password;
  }
  if (!password) {
    throw new Error('login: password required when username is provided');
  }
  const connectButton = page.getByRole('button', {
    name: 'Connect',
    exact: true,
  });
  const addClusterItem = page
    .getByRole('listitem')
    .filter({ hasText: 'Add Cluster' });

  // The 'Connect' button is visible only when no clusters are present.
  await expect(connectButton.or(addClusterItem)).toBeVisible();
  if (await connectButton.isVisible()) {
    await connectButton.click();
  } else {
    await addClusterItem.click();
  }
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
  userDataDir: string;
  appConfigPath: string;
}

// Connect's login goes through webapi/mfa/login/{begin,finish}, which are rate limited per client IP. tshd
// makes those requests itself, so it can't carry the per-test X-Forwarded-For the browser tests use, and every
// Connect login shares one bucket. Logging in once per user and copying the resulting data dir keeps the count
// to one regardless of how many specs need a session.
const snapshots = new Map<string, Promise<DisposableTempDir>>();

function loggedInSnapshot(username: string, creds: UserCredentials) {
  let snapshot = snapshots.get(username);
  if (snapshot) {
    return snapshot;
  }

  snapshot = createLoggedInSnapshot(username, creds);
  snapshots.set(username, snapshot);

  return snapshot;
}

async function createLoggedInSnapshot(
  username: string,
  creds: UserCredentials
) {
  const dir = await fs.mkdtempDisposable(
    path.join(os.tmpdir(), 'connect-e2e-snapshot-')
  );

  try {
    await initializeDataDir(dir.path, withDefaultAppConfig({}));

    // Scoped so the app closes, flushing the tsh profile and app state to disk, before anything copies the dir.
    {
      await using app = await launchApp(dir.path, creds);
      await login(app.page, username, creds.password);
    }
  } catch (error) {
    await dir.remove();
    throw error;
  }

  return dir;
}

/**
 * loggedInDataDir returns a disposable data dir already holding a logged-in session. Defaults to the same user
 * as `launchApp` and `login` when none is given.
 */
export async function loggedInDataDir(
  username?: string,
  creds?: UserCredentials
) {
  const name = username ?? defaultUsername();
  const snapshot = await loggedInSnapshot(name, creds ?? lookupCreds(name));

  const temp = await fs.mkdtempDisposable(
    path.join(os.tmpdir(), 'connect-e2e-test-')
  );

  try {
    await fs.cp(snapshot.path, temp.path, { recursive: true });
  } catch (error) {
    await temp.remove();
    throw error;
  }

  return temp;
}

export const test = fixtureBase.extend<
  {
    autoLogin: boolean;
    /**
     * Sets app config before launching the app.
     *
     * Use `withDefaultAppConfig` for normal config overrides.
     */
    appConfig: AppConfigSetup;
    app: App;
  },
  { snapshotCleanup: void }
>({
  // Snapshots hold live certs and keys, so drop them when the worker exits rather than leaving them behind in
  // the temp dir.
  snapshotCleanup: [
    // oxlint-disable-next-line no-empty-pattern
    async ({}, use) => {
      await use();

      const dirs = await Promise.allSettled(snapshots.values());
      snapshots.clear();

      await Promise.all(
        dirs.map(dir =>
          dir.status === 'fulfilled' ? dir.value.remove() : undefined
        )
      );
    },
    { scope: 'worker', auto: true },
  ],
  autoLogin: [false, { option: true }],
  appConfig: [withDefaultAppConfig({}), { option: true }],
  app: async ({ autoLogin, appConfig, username }, use, testInfo) => {
    if (!username) {
      throw new Error(
        'Connect app fixture: no username resolved — was the runner started with user-mapping.json?'
      );
    }
    const creds = lookupCreds(username);

    // Start from a copy of the user's logged-in session rather than driving the login UI per test. The test's
    // own appConfig is applied on top, since the snapshot carries the default one.
    await using temp = autoLogin
      ? await loggedInDataDir(username, creds)
      : await fs.mkdtempDisposable(path.join(os.tmpdir(), 'connect-e2e-test-'));

    const { appConfigPath } = await initializeDataDir(temp.path, appConfig);
    await using launchedApp = await launchApp(temp.path, creds);

    await use({
      electronApp: launchedApp.electronApp,
      userDataDir: temp.path,
      page: launchedApp.page,
      appConfigPath,
    });

    if (testInfo.status !== testInfo.expectedStatus) {
      await attachLogs(temp.path, testInfo);
    }
  },
});

test.use({ fixtures: ['connect'] });

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
