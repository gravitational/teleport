/**
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

import { expect, test } from '@playwright/test';

test.beforeEach(async ({ page }) => {
  await page.goto('/');

  await page.getByPlaceholder('Username').fill('bob');
  await page.getByPlaceholder('Password').fill('secret');

  await page.getByRole('button', { name: 'Sign In' }).click();
});

test('nodes should be visible', async ({ page }) => {
  await expect(page.getByText(/^teleport-e2e$/).first()).toBeVisible();
});
