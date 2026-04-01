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

import { login, logout } from '@gravitational/e2e/helpers/login';
import { defaultPassword, signup } from '@gravitational/e2e/helpers/signup';
import { expect, test } from '@gravitational/e2e/helpers/test';

const lightBody = 'rgb(241, 242, 244)';
const darkBody = 'rgb(12, 20, 61)';

test('switching between dark and light theme', async ({ page }, testInfo) => {
  const username = `testuser-${testInfo.workerIndex}`;

  await signup(page, username, defaultPassword);
  await expect(page.locator('body')).toHaveCSS('background-color', lightBody);

  // Switch to dark theme. Make sure that the change gets persisted in user
  // preferences on the backend side before we log out.
  const prefsResponse = page.waitForResponse(
    response =>
      URL.parse(response.url())?.pathname === '/v1/webapi/user/preferences' &&
      response.request().method() === 'PUT'
  );
  await page.getByRole('button', { name: 'User Menu' }).click();
  await page.getByText('Switch to Dark Theme').click();
  await expect(page.locator('body')).toHaveCSS('background-color', darkBody);
  await prefsResponse;

  // Dark theme should be retained after logging out and in again.
  await logout(page);
  await expect(page.locator('body')).toHaveCSS('background-color', darkBody);

  // Make sure that the theme was actually saved in the backend. Simulate
  // signing in on a fresh browser.
  // await page.context().clearCookies();
  await page.evaluate(() => localStorage.clear());
  await login(page, username, defaultPassword);
  await expect(page.locator('body')).toHaveCSS('background-color', darkBody);

  // Switch to light theme.
  await page.getByRole('button', { name: 'User Menu' }).click();
  await page.getByText('Switch to Light Theme').click();
  await expect(page.locator('body')).toHaveCSS('background-color', lightBody);
});
