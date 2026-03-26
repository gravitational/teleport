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

import { login } from '@gravitational/e2e/helpers/login';
import { expect, test } from '@gravitational/e2e/helpers/test';

const lightBody = 'rgb(241, 242, 244)';
const darkBody = 'rgb(12, 20, 61)';

test('switching between dark and light theme', async ({ page }) => {
  await page.goto('/');
  await expect(page.locator('body')).toHaveCSS('background-color', lightBody);

  // Switch to dark theme.
  await page.getByRole('button', { name: 'User Menu' }).click();
  await page.getByText('Switch to Dark Theme').click();
  await expect(page.locator('body')).toHaveCSS('background-color', darkBody);

  // Dark theme should be retained after logging out and in again.
  await page.getByRole('button', { name: 'User Menu' }).click();
  await page.getByText('Logout').click();
  await expect(page.locator('body')).toHaveCSS('background-color', darkBody);

  await login(page);
  await expect(page.locator('body')).toHaveCSS('background-color', darkBody);

  // Switch to light theme.
  await page.getByRole('button', { name: 'User Menu' }).click();
  await page.getByText('Switch to Light Theme').click();
  await expect(page.locator('body')).toHaveCSS('background-color', lightBody);
});
