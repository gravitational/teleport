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

import { mkdirSync, writeFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

import { startUrl, users } from '@gravitational/e2e/helpers/env';
import { directLogin } from '@gravitational/e2e/helpers/login';
// oxlint-disable-next-line no-restricted-imports
import { test as setup } from '@playwright/test';

const authDir = join(dirname(fileURLToPath(import.meta.url)), '../../.auth');

mkdirSync(authDir, { recursive: true });

for (const [username, creds] of Object.entries(users)) {
  // oxlint-disable-next-line no-empty-pattern -- Playwright requires fixture argument to be destructured.
  setup(`authenticate as ${username}`, async ({}, testInfo) => {
    const state = await directLogin(startUrl, username, creds.password);

    const browser = testInfo.project.name.split(':')[0];

    writeFileSync(
      join(authDir, `${browser}-${username}.json`),
      JSON.stringify(state, null, 2)
    );
  });
}
