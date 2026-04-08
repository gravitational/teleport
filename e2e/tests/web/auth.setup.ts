import { mkdirSync, writeFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

import { test as setup } from '@playwright/test';

import { startUrl, users } from '@gravitational/e2e/helpers/env';
import { directLogin } from '@gravitational/e2e/helpers/login';

const authDir = join(dirname(fileURLToPath(import.meta.url)), '../../.auth');

mkdirSync(authDir, { recursive: true });

for (const [username, creds] of Object.entries(users)) {
  setup(`authenticate as ${username}`, async (_, testInfo) => {
    const state = await directLogin(startUrl, username, creds.password);

    const browser = testInfo.project.name.split(':')[0];

    writeFileSync(
      join(authDir, `${browser}-${username}.json`),
      JSON.stringify(state, null, 2)
    );
  });
}
