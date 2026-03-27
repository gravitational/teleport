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

import { expect, test } from '@gravitational/e2e/helpers/test';
import { signup } from '@gravitational/e2e/helpers/signup';
import {
  createResource,
  deleteResource,
  deleteUser,
} from '@gravitational/e2e/helpers/tctl';
import { openNavSection } from '@gravitational/e2e/helpers/sidenav';

// Role with no allow.rules defined
const noRulesRole = `kind: role
metadata:
  name: rbac-no-allow
spec:
  allow:
    app_labels:
      '*': '*'
    logins:
    - root
    node_labels:
      '*': '*'
  options:
    max_session_ttl: 8h0m0s
version: v3`;

// Role with only read access to audit, sessions, roles, auth connectors, users, and trusted clusters
const readAccessRole = `kind: role
metadata:
  name: rbac-read-access
spec:
  allow:
    app_labels:
      '*': '*'
    logins:
    - root
    node_labels:
      '*': '*'
    rules:
    - resources:
      - event
      verbs:
      - list
    - resources:
      - session
      verbs:
      - list
      - read
    - resources:
      - role
      verbs:
      - list
      - read
    - resources:
      - auth_connector
      verbs:
      - list
      - read
    - resources:
      - user
      verbs:
      - list
      - read
    - resources:
      - trusted_cluster
      verbs:
      - list
      - read
  options:
    max_session_ttl: 8h0m0s
version: v3`;


test('Verify navigation with no allow rules', async ({
  page,
}, testInfo) => {
  test.setTimeout(60_000);
  const username = `test-user-${testInfo.workerIndex}`;

  createResource(noRulesRole);
  await signup(page, username, 'rbac-no-allow');

  await test.step('Audit section only contains Active Sessions', async () => {
    const panel = await openNavSection(page, 'Audit');
    await expect(panel.getByText('Active Sessions')).toBeVisible();
    await expect(panel.getByText('Audit Log')).not.toBeVisible();
    await expect(panel.getByText('Session Recordings')).not.toBeVisible();
  });

  await test.step('Identity Governance contains Access Requests and Trusted Devices with Enterprise CTAs', async () => {
    const panel = await openNavSection(page, 'Identity Governance');
    await expect(panel.getByText('Access Requests')).toBeVisible();
    await expect(panel.getByText('Trusted Devices')).toBeVisible();
    await expect(
      panel.getByText('Session & Identity Locks')
    ).not.toBeVisible();

    await page.goto('/web/accessrequest');
    await expect(
      page.getByText('Unlock Access Requests With Teleport Enterprise').first()
    ).toBeVisible();

    await page.goto('/web/devices');
    await expect(
      page.getByText('Unlock Trusted Devices With Teleport Enterprise').first()
    ).toBeVisible();
  });

  await test.step('Add New section contains Resource and Integration', async () => {
    const panel = await openNavSection(page, 'Add New');
    await expect(panel.getByText('Resource')).toBeVisible();
    await expect(panel.getByText('Integration')).toBeVisible();
  });

  await test.step('No Identity Security section', async () => {
    await expect(
      page.getByRole('button', { name: 'Identity Security' })
    ).not.toBeVisible();
  });

  await test.step('Enroll New Resource button is disabled on the Resources screen', async () => {
    await page.getByRole('button', { name: 'Resources' }).click();
    const enrollButton = page.getByRole('button', {
      name: 'Enroll New Resource',
    });
    await expect(enrollButton).toBeVisible({ timeout: 10_000 });
    await expect(enrollButton).toBeDisabled();
  });

  deleteUser(username);
  deleteResource('role', 'rbac-no-allow');
});

test('Verify read-only access to resources', async ({
  page,
}, testInfo) => {
  test.setTimeout(60_000);
  const username = `test-user-${testInfo.workerIndex}`;

  createResource(readAccessRole);
  await signup(page, username, 'rbac-read-access');

  await test.step('Audit Log is accessible', async () => {
      const panel = await openNavSection(page, 'Audit');
      await expect(
        panel.getByRole('link', { name: 'Audit Log' })
      ).toBeVisible();
      await page.goto('/web/cluster/teleport-e2e/audit/events');
      await expect(page.locator('h1').getByText('Audit Log')).toBeVisible();
    });

    await test.step('Session Recordings is accessible', async () => {
      const panel = await openNavSection(page, 'Audit');
      await expect(
        panel.getByRole('link', { name: 'Session Recordings' })
      ).toBeVisible();
      await page.goto('/web/cluster/teleport-e2e/recordings');
      await expect(
        page.locator('h1').getByText('Session Recordings')
      ).toBeVisible();
    });

    await test.step('User can see roles but cant create/delete/update', async () => {
      const ztaPanel = await openNavSection(page, 'Zero Trust Access');
      await expect(ztaPanel.getByRole('link', { name: 'Roles' })).toBeVisible();
      await page.goto('/web/roles');
      await expect(page.getByText('Roles').first()).toBeVisible();

      // Create button should be disabled
      const createButton = page.getByTestId('create_new_role_button');
      await expect(createButton).toBeDisabled();

      // Options menu should only have "View Details"
      const firstOptionsButton = page
        .locator('table')
        .getByRole('button')
        .first();
      if (await firstOptionsButton.isVisible()) {
        await firstOptionsButton.click();
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
      }
    });

    await test.step('User can see auth connectors', async () => {
      const ztaPanel2 = await openNavSection(page, 'Zero Trust Access');
      await expect(
        ztaPanel2.getByRole('link', { name: 'Auth Connectors' })
      ).toBeVisible();
      await page.goto('/web/sso');
      await expect(page.getByText('Auth Connectors').first()).toBeVisible();
    });

    await test.step('User can access Users screen but cant create/delete/update', async () => {
      const ztaPanel3 = await openNavSection(page, 'Zero Trust Access');
      await expect(
        ztaPanel3.getByRole('link', { name: 'Users' })
      ).toBeVisible();
      await page.goto('/web/users');
      await expect(page.getByText('Users').first()).toBeVisible();

      // Create new user button should be disabled.
      await expect(
        page.getByRole('button', { name: 'Create New User' })
      ).toBeDisabled();
    });

    await test.step('User can access Trusted Root Clusters but cant create/delete/update', async () => {
      const ztaPanel4 = await openNavSection(page, 'Zero Trust Access');
      await expect(
        ztaPanel4.getByRole('link', { name: 'Trusted Root Clusters' })
      ).toBeVisible();
      await page.goto('/web/trust');
      await expect(
        page.getByText('Trusted Root Clusters').first()
      ).toBeVisible();

      // Verify that the trusted cluster is in the list
      await expect(
        page.getByText('dummy-trusted-cluster')
      ).toBeVisible();

      // The connect button should be disabled
      await expect(
        page.getByRole('button', { name: 'Connect to Root Cluster' })
      ).toBeDisabled();

      // Verify trying to edit it returns an error
      await page
        .getByRole('button', { name: 'Edit Trusted Cluster' })
        .click();
      await page.waitForSelector('.ace_editor', { state: 'visible' });
      await page.evaluate(() => {
        const editor = (window as any).ace.edit(
          document.querySelector('.ace_editor')
        );
        editor.session.setValue(editor.session.getValue() + '\n');
      });
      await page.getByRole('button', { name: 'Save changes' }).click();
      await expect(
        page.getByText('access denied').first()
      ).toBeVisible({ timeout: 10_000 });
    });

  deleteUser(username);
  deleteResource('role', 'rbac-read-access');
});
