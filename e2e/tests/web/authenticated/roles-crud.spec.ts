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

import { expect, test } from '@gravitational/e2e/helpers/test';
import { mockWebAuthn } from '@gravitational/e2e/helpers/webauthn';

test('verify creating, editing, and deleting a role works', async ({
  page,
}) => {
  await mockWebAuthn(page);
  await page.goto('/');

  await page.getByRole('button', { name: 'Zero Trust Access' }).click();
  await page.getByRole('link', { name: 'Roles' }).click();

  // Create a new role
  await page.getByRole('button', { name: 'Create New Role' }).click();
  await expect(page.getByText('Create a New Role')).toBeVisible();
  await page
    .getByRole('textbox', { name: 'Role Name(required)' })
    .fill('test-role');
  await page.getByRole('button', { name: 'Next: Resources' }).click();
  await page
    .getByRole('button', { name: 'Add Teleport Resource Access' })
    .click();
  await page.getByRole('menuitem', { name: 'SSH Server Access' }).click();
  await page.getByRole('button', { name: 'Next: Admin Rules' }).click();
  await page.getByRole('button', { name: 'Next: Options' }).click();
  await page.getByRole('button', { name: 'Create Role' }).click();
  await expect(
    page.getByRole('cell', { name: 'test-role' })
  ).toBeVisible();

  // Edit the role
  await page
    .getByRole('row', { name: 'test-role Options' })
    .getByRole('button')
    .click();
  await page.getByRole('menuitem', { name: 'Edit' }).click();
  await expect(page.getByText('Edit Role test-role')).toBeVisible();

  // Add a descripton to the yaml
  await page.getByRole('tab', { name: 'Switch to YAML editor' }).click();
  await expect(page.getByTestId('yaml-editor')).toBeVisible();
  await page.waitForSelector('.ace_editor', { state: 'visible' });
  await page.evaluate(() => {
    const editor = (window as any).ace.edit(
      document.querySelector('.ace_editor')
    );
    const content = editor.session.getValue();
    const updated = content.replace(
      'name: test-role',
      'name: test-role\n  description: test description'
    );
    editor.session.setValue(updated);
  });
  await page.getByRole('button', { name: 'Save Changes' }).click();

  // Verify the updated description appears
  await expect(
    page.getByRole('cell', { name: 'test description' })
  ).toBeVisible();

  // Delete the role
  await page
    .getByRole('row', { name: 'test-role' })
    .getByRole('button')
    .click();
  await page.getByRole('menuitem', { name: 'Delete' }).click();
  await page.getByRole('button', { name: 'Yes, Remove Role' }).click();
  await expect(
    page.getByRole('cell', { name: 'test-role' })
  ).not.toBeVisible();
});

test('verify that an error is shown when attempting to save an invalid YAML', async ({
  page,
}) => {
  await mockWebAuthn(page);
  await page.goto('/');

  await page.getByRole('button', { name: 'Zero Trust Access' }).click();
  await page.getByRole('link', { name: 'Roles' }).click();
  await page.getByRole('button', { name: 'Create New Role' }).click();

  // Switch to yaml editor
  await page.getByRole('tab', { name: 'Switch to YAML editor' }).click();
  await expect(page.getByTestId('yaml-editor')).toBeVisible();

  // Replace all the content with invalid babble and save
  await page.waitForSelector('.ace_editor', { state: 'visible' });
  await page.locator('.ace_editor').click();
  await page.keyboard.press('Meta+a');
  await page.keyboard.type('adsafahlkj', { delay: 5 });
  await page.waitForTimeout(500);
  const createBtn = page.getByRole('button', { name: 'Create Role' });
  await expect(createBtn).toBeEnabled({ timeout: 5_000 });
  await createBtn.click();

  await expect(page.getByText('not a valid resource declaration')).toBeVisible({ timeout: 10_000 });
});

test('verify that info guide works and has valid docs links', async ({
  page,
}) => {
  await mockWebAuthn(page);
  await page.goto('/');

  await page.getByRole('button', { name: 'Zero Trust Access' }).click();
  await page.getByRole('link', { name: 'Roles' }).click();

  await page.getByTestId('info-guide-btn-open').click();
  await expect(
    page.getByText(
      'Teleport Role-based access control (RBAC) provides fine-grained control'
    )
  ).toBeVisible();
  await expect(
    page.getByRole('link', { name: 'Teleport Preset Roles' })
  ).toHaveAttribute(
    'href',
    'https://goteleport.com/docs/reference/access-controls/roles/#preset-roles'
  );
  await expect(
    page.getByRole('link', { name: 'Teleport Role Templates' })
  ).toHaveAttribute(
    'href',
    'https://goteleport.com/docs/admin-guides/access-controls/guides/role-templates/'
  );
});
