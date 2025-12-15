/**
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

import { signup } from '../utils/signup';

test('verify that a user can create and delete a role', async ({ page }) => {
  const { cleanup } = await signup(page);

  await page.getByRole('button', { name: 'Zero Trust Access' }).click();
  await page.getByRole('link', { name: 'Roles' }).click();
  await page.getByRole('button', { name: 'Create New Role' }).click();
  await page.getByRole('textbox', { name: 'Role Name(required)' }).click();
  await page
    .getByRole('textbox', { name: 'Role Name(required)' })
    .fill('testrole');
  await page.getByRole('button', { name: 'Next: Resources' }).click();
  await page
    .getByRole('button', { name: 'Add Teleport Resource Access' })
    .click();
  await page.getByRole('menuitem', { name: 'SSH Server Access' }).click();
  await page.getByRole('button', { name: 'Next: Admin Rules' }).click();
  await page.getByRole('button', { name: 'Next: Options' }).click();
  await page.getByRole('button', { name: 'Create Role' }).click();

  await expect(page.getByRole('cell', { name: 'testrole' })).toBeVisible();

  await page
    .getByRole('row', { name: 'testrole Options' })
    .getByRole('button')
    .click();
  await page.getByRole('menuitem', { name: 'Delete' }).click();
  await page.getByRole('button', { name: 'Yes, Remove Role' }).click();

  await expect(page.getByRole('cell', { name: 'testrole' })).not.toBeVisible();

  await cleanup();
});
