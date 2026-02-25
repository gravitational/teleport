import { join } from 'node:path';

import { test as setup } from '@playwright/test';

import { login } from '../helpers/login';

const authFile = join(__dirname, '../.auth/user.json');

setup('authenticate', async ({ page }) => {
  await login(page);

  await page.context().storageState({ path: authFile });
});
