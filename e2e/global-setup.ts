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
import { fileURLToPath, pathToFileURL } from 'node:url';

const authDir = join(dirname(fileURLToPath(import.meta.url)), '.auth');

async function globalSetup() {
  const browser = process.env.E2E_BROWSER;
  if (!browser) {
    throw new Error('E2E_BROWSER must be set by the runner');
  }

  // No bootstrapped users — e.g. runs against an existing cluster via
  // --teleport-url, or unauthenticated/connect-only runs. There's nothing to
  // set up, and helpers/env throws on the missing E2E_USERS_FILE, so bail
  // before importing it.
  if (!process.env.E2E_USERS_FILE) {
    return;
  }

  // Imported lazily so reading E2E_USERS_FILE only happens once we know there
  // are credentials to generate auth state from.
  const { startUrl, users } = await import('./helpers/env');
  const { directLogin } = await import('./helpers/login');

  mkdirSync(authDir, { recursive: true });

  for (const [username, creds] of Object.entries(users)) {
    const state = await directLogin(startUrl, username, creds.password);
    writeFileSync(
      join(authDir, `${browser}-${username}.json`),
      JSON.stringify(state, null, 2)
    );
  }
}

export { globalSetup as default };

// When executed directly (e.g. `tsx global-setup.ts`) rather than imported as
// Playwright's configured globalSetup, run immediately. The runner's browse and
// codegen modes use this to generate auth state without a full test run.
if (
  process.argv[1] &&
  import.meta.url === pathToFileURL(process.argv[1]).href
) {
  await globalSetup();
}
