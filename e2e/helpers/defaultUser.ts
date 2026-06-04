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
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

import { canonicalUserKey, type UserKeyInput } from './canonicalKey';

const authDir = join(
  process.env.E2E_DIR ?? join(dirname(fileURLToPath(import.meta.url)), '..'),
  '.auth'
);

// Duplicated from the Go runner (defaultUsers() in scan.go) because this code
// runs before the runner produces any mapping — the TS side can't discover the
// default through the user-mapping.json file, it has to compute the key directly.
const DEFAULT_USER: UserKeyInput = { roles: ['access', 'editor'] };

let cached: string | undefined;

// defaultUsername returns the runner-generated username for the default
// access+editor user. Used by flows that don't declare users via test.use()
// (Connect tests, open-with-webauthn script).
export function defaultUsername() {
  if (cached) {
    return cached;
  }

  const mappingPath = join(authDir, 'user-mapping.json');
  const mapping = JSON.parse(readFileSync(mappingPath, 'utf-8')) as Record<
    string,
    string
  >;

  const name = mapping[canonicalUserKey(DEFAULT_USER, { isDefault: true })];
  if (!name) {
    throw new Error(`no default user in ${mappingPath}`);
  }

  cached = name;
  return name;
}
