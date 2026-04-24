/**
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

import { readFileSync } from 'node:fs';
import { dirname, join, relative } from 'node:path';
import { fileURLToPath } from 'node:url';

import { canonicalUserKey } from './canonicalKey';
import { test as base } from './fixtures';
import type { StorageState } from './login';
import { PlayerPage } from './pages/Player';
import { UnifiedResourcesPage } from './pages/UnifiedResources';
import { mockWebAuthn } from './webauthn';

export const CLUSTER_NAME = 'teleport-e2e';

export type UserRole = string | { file: string };

export interface UserTraits {
  logins?: string[];
  windows_logins?: string[];
  kubernetes_groups?: string[];
  kubernetes_users?: string[];
  db_names?: string[];
  db_users?: string[];
  db_roles?: string[];
  aws_role_arns?: string[];
  azure_identities?: string[];
  gcp_service_accounts?: string[];
  host_user_uid?: string[];
  host_user_gid?: string[];
  [key: string]: string[] | undefined;
}

export interface UserDefinition {
  roles: UserRole[];
  traits?: UserTraits;
  recordings?: string[];
  loginAs?: boolean;
}

const e2eDir = join(dirname(fileURLToPath(import.meta.url)), '..');
const authDir = join(e2eDir, '.auth');

const tryLoadUserMapping =
  cachedJSONLoader<Record<string, string>>('user-mapping.json');

const tryLoadRecordingMapping = cachedJSONLoader<
  Record<string, Record<string, string>>
>('recording-mapping.json');

const defaultUser: UserDefinition = { roles: ['access', 'editor'] };

interface E2EFixtures {
  recordings: string[];
  user: UserDefinition;
  users: UserDefinition[];
  username: string;
  loginAs: (index: number) => Promise<LoginAsResult>;
  recordingIds: Record<string, string>;
  unifiedResourcesPage: UnifiedResourcesPage;
  playerPage: PlayerPage;
}

export interface LoginAsResult {
  name: string;
  recordingIds: Record<string, string>;
}

export const test = base.extend<E2EFixtures>({
  recordings: [[], { option: true }],
  user: [undefined as unknown as UserDefinition, { option: true }],
  users: [[], { option: true }],
  username: async ({ user, users }, use, testInfo) => {
    const mapping = tryLoadUserMapping() ?? {};

    const picked = pickLoginDefinition(user, users);
    if (picked) {
      const source = relSpecPath(testInfo);
      const name =
        mapping[
          canonicalUserKey(picked.def, { index: picked.index, source })
        ] ?? mapping[canonicalUserKey(picked.def, { index: picked.index })];
      await use(name ?? '');
      return;
    }

    if (wantsAuthenticatedUser(testInfo.project.name)) {
      await use(
        mapping[canonicalUserKey(defaultUser, { isDefault: true })] ?? ''
      );
      return;
    }

    await use('');
  },
  loginAs: async ({ page, users }, use, testInfo) => {
    const mapping = tryLoadUserMapping();
    if (!mapping) {
      throw new Error(
        `failed to read user mapping at ${join(authDir, 'user-mapping.json')} — was the runner started?`
      );
    }

    const browser = testInfo.project.name.split(':')[0];

    await use(async (index: number): Promise<LoginAsResult> => {
      const definition = users[index];
      if (!definition) {
        throw new Error(
          `users[${index}] is undefined — declare ${index + 1} users in test.use()`
        );
      }

      const source = relSpecPath(testInfo);
      const name =
        mapping[canonicalUserKey(definition, { index, source })] ??
        mapping[canonicalUserKey(definition, { index })];
      if (!name) {
        throw new Error(
          `no generated username for users[${index}]: ${JSON.stringify(definition)}`
        );
      }

      const state: StorageState = JSON.parse(
        readFileSync(join(authDir, `${browser}-${name}.json`), 'utf-8')
      );

      const ctx = page.context();
      await ctx.clearCookies();
      await ctx.addCookies(state.cookies);

      // The page's WebAuthn mock is bound to the user the page fixture
      // started with; rebind it so subsequent MFA prompts answer with the
      // switched user's credential.
      await mockWebAuthn(page, name);

      // localStorage is origin-scoped; inject per origin after navigation.
      await page.goto('/');

      for (const { origin, localStorage: items } of state.origins) {
        await page.evaluate(
          ({ expected, entries }) => {
            if (location.origin !== expected) return;
            window.localStorage.clear();
            for (const { name, value } of entries) {
              window.localStorage.setItem(name, value);
            }
          },
          { expected: origin, entries: items }
        );
      }

      await page.reload();

      const recordingMapping = tryLoadRecordingMapping() ?? {};
      return { name, recordingIds: recordingMapping[name] ?? {} };
    });
  },
  recordingIds: async ({ username }, use) => {
    if (!username) {
      await use({});
      return;
    }

    const mapping = tryLoadRecordingMapping() ?? {};
    await use(mapping[username] ?? {});
  },
  page: async ({ page, username }, use) => {
    if (username) {
      await mockWebAuthn(page, username);
    }

    await use(page);
  },
  storageState: async ({ username }, use, testInfo) => {
    // Connect tests drive login through the Electron app and don't have a
    // setup project that writes shared storage state files, so leave it unset.
    if (!username || testInfo.project.name === 'connect') {
      await use(undefined as unknown as string);
      return;
    }

    const browser = testInfo.project.name.split(':')[0];

    await use(join(authDir, `${browser}-${username}.json`));
  },
  unifiedResourcesPage: async ({ page }, use) => {
    await use(new UnifiedResourcesPage(page));
  },
  playerPage: async ({ page }, use) => {
    await use(new PlayerPage(page));
  },
});

export { expect } from '@playwright/test';

function pickLoginDefinition(
  user: UserDefinition | undefined,
  users: UserDefinition[]
): { def: UserDefinition; index?: number } | undefined {
  if (user) {
    return { def: user };
  }

  if (users.length === 0) {
    return undefined;
  }

  const loginIdx = users.findIndex(u => u.loginAs);
  const i = loginIdx >= 0 ? loginIdx : 0;
  return { def: users[i], index: i };
}

// wantsAuthenticatedUser returns true for projects whose tests need a logged-in
// user even when test.use() doesn't declare one — `:authenticated` web tests
// and Connect tests both fall through to the default access/editor user.
function wantsAuthenticatedUser(projectName: string): boolean {
  return projectName.endsWith(':authenticated') || projectName === 'connect';
}

// relSpecPath returns the spec file path relative to e2e/, matching the
// `source` field the Go scanner emits for inline declarations.
function relSpecPath(testInfo: { file: string }) {
  return relative(e2eDir, testInfo.file);
}

// Mapping files are written once at runner startup and don't change during a
// test run, so each worker can cache them after first read.
function cachedJSONLoader<T>(filename: string): () => T | undefined {
  let cached: T | undefined;
  let loaded = false;

  return () => {
    if (!loaded) {
      loaded = true;

      try {
        cached = JSON.parse(readFileSync(join(authDir, filename), 'utf-8'));
      } catch {
        cached = undefined;
      }
    }

    return cached;
  };
}
