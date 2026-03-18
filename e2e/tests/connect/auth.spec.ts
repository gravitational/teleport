/*
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

import { test, expect } from '@gravitational/e2e/helpers/connect';

test.use({ autoLogin: true });

test('logging out', async ({ app }) => {
  const { page } = app;
  await page.getByTitle(/Open Profiles/).click();
  await page.getByTitle(/Log out/).click();
  await expect(
    page.getByText('Are you sure you want to log out?')
  ).toBeVisible();
  await page.getByRole('button', { name: 'Log Out', exact: true }).click();
  await expect(page.getByText('Connect a Cluster')).toBeVisible();
});
