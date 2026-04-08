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
  rolesPage,
}) => {
  await mockWebAuthn(page);
  await rolesPage.goto();

  await test.step('Create a new role', async () => {
    await rolesPage.createRole('test-role');

    await expect(page.getByRole('cell', { name: 'test-role' })).toBeVisible();
  });

  await test.step('Edit the role via YAML to add a description', async () => {
    await rolesPage.editRole('test-role');

    await expect(page.getByText('Edit Role test-role')).toBeVisible();

    await rolesPage.switchToYamlEditor();

    await rolesPage.replaceYaml(
      'name: test-role',
      'name: test-role\n  description: test description'
    );

    await rolesPage.saveChangesButton.click();

    await expect(
      page.getByRole('cell', { name: 'test description' })
    ).toBeVisible();
  });

  await test.step('Delete the role', async () => {
    await rolesPage.deleteRole('test-role');

    await expect(
      page.getByRole('cell', { name: 'test-role' })
    ).not.toBeVisible();
  });
});

test('verify that an error is shown when attempting to save an invalid YAML', async ({
  rolesPage,
  page,
}) => {
  await rolesPage.goto();

  await rolesPage.createNewRoleButton.click();

  await rolesPage.switchToYamlEditor();
  await rolesPage.setYaml('adsafahlkj');

  await expect(rolesPage.createRoleButton).toBeEnabled();

  await rolesPage.createRoleButton.click();

  await expect(page.getByText('not a valid resource declaration')).toBeVisible();
});

test('verify that info guide works and has valid docs links', async ({
  rolesPage,
  page,
}) => {
  await rolesPage.goto();

  await rolesPage.infoGuideButton.click();

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
