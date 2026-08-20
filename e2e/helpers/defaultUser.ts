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
