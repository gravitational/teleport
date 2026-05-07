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

import { CLUSTER_NAME, expect, test } from '@gravitational/e2e/helpers/test';

test.describe('no allow rules', () => {
  test.use({
    user: {
      roles: [{ file: '@gravitational/e2e/roles/rbac-no-allow.yaml' }],
    },
  });

  test('verify navigation with no allow rules', async ({
    page,
    sideNavPage,
  }) => {
    await page.goto('/');

    await test.step('Audit section only contains Active Sessions', async () => {
      const panel = await sideNavPage.openSection('Audit');

      await expect(panel.getByText('Active Sessions')).toBeVisible();
      await expect(panel.getByText('Audit Log')).not.toBeVisible();
      await expect(panel.getByText('Session Recordings')).not.toBeVisible();
    });

    await test.step('Identity Governance contains Access Requests with Enterprise CTA', async () => {
      const panel = await sideNavPage.openSection('Identity Governance');

      await expect(panel.getByText('Access Requests')).toBeVisible();

      await expect(
        panel.getByText('Session & Identity Locks')
      ).not.toBeVisible();

      await page.goto('/web/accessrequest');

      await expect(
        page
          .getByText('Unlock Access Requests With Teleport Enterprise')
          .first()
      ).toBeVisible();
    });

    await test.step('Add New section contains Resource and Integration', async () => {
      const panel = await sideNavPage.openSection('Add New');

      await expect(panel.getByText('Resource')).toBeVisible();
      await expect(panel.getByText('Integration')).toBeVisible();
    });

    await test.step('No Identity Security section', async () => {
      await expect(
        page.getByRole('button', { name: 'Identity Security' })
      ).not.toBeVisible();
    });

    await test.step('Enroll New Resource button is disabled on the Resources screen', async () => {
      await page.goto('/');

      const enrollButton = page.getByRole('button', {
        name: 'Enroll New Resource',
      });

      await expect(enrollButton).toBeVisible();
      await expect(enrollButton).toBeDisabled();
    });
  });
});

test.describe('read-only access', () => {
  test.use({
    user: {
      roles: [{ file: '@gravitational/e2e/roles/rbac-read-access.yaml' }],
    },
  });

  test('verify read-only access to resources', async ({
    page,
    rolesPage,
    trustedClustersPage,
  }) => {
    await page.goto('/');

    await test.step('Audit Log is accessible', async () => {
      await page.goto(`/web/cluster/${CLUSTER_NAME}/audit/events`);

      await expect(page.locator('h1').getByText('Audit Log')).toBeVisible();
    });

    await test.step('Session Recordings is accessible', async () => {
      await page.goto(`/web/cluster/${CLUSTER_NAME}/recordings`);

      await expect(
        page.locator('h1').getByText('Session Recordings')
      ).toBeVisible();
    });

    await test.step('User can see roles but cant create/delete/update', async () => {
      await rolesPage.goto();

      await expect(page.getByText('Roles').first()).toBeVisible();

      await expect(rolesPage.createNewRoleButton).toBeDisabled();

      await expect(rolesPage.firstOptionsMenuButton).toBeVisible();
      await rolesPage.firstOptionsMenuButton.click();

      await expect(
        page.getByRole('menuitem', { name: 'View Details' })
      ).toBeVisible();

      await expect(
        page.getByRole('menuitem', { name: 'Edit' })
      ).not.toBeVisible();

      await expect(
        page.getByRole('menuitem', { name: 'Delete' })
      ).not.toBeVisible();

      await page.keyboard.press('Escape');
    });

    await test.step('User can see auth connectors', async () => {
      await page.goto('/web/sso');

      await expect(page.getByText('Auth Connectors').first()).toBeVisible();
    });

    await test.step('User can access Users screen but cant create/delete/update', async () => {
      await page.goto('/web/users');

      await expect(page.getByText('Users').first()).toBeVisible();

      await expect(
        page.getByRole('button', { name: 'Create New User' })
      ).toBeDisabled();
    });

    await test.step('User can access Trusted Root Clusters but cant create/delete/update', async () => {
      await trustedClustersPage.goto();

      await expect(
        page.getByText('Trusted Root Clusters').first()
      ).toBeVisible();

      await expect(page.getByText('dummy-trusted-cluster')).toBeVisible();

      await expect(trustedClustersPage.connectButton).toBeDisabled();

      await trustedClustersPage.appendToYamlAndSave();

      await expect(page.getByText('access denied').first()).toBeVisible();
    });
  });
});
