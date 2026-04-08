import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

import { login } from '@gravitational/e2e/helpers/login';
import { test as setup } from '@gravitational/e2e/helpers/test';
import { type UserCredentials } from '@gravitational/e2e/helpers/env';

const authDir = join(dirname(fileURLToPath(import.meta.url)), '../../.auth');

const usersJSON: Record<string, UserCredentials> = JSON.parse(
  process.env.E2E_USERS_JSON || '{}'
);

for (const username of Object.keys(usersJSON)) {
  setup(`authenticate as ${username}`, async ({ page }, testInfo) => {
    await login(page, username);

    const browser = testInfo.project.name.split(':')[0];
    await page
      .context()
      .storageState({ path: join(authDir, `${browser}-${username}.json`) });
  });
}
