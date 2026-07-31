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

import {
  expect,
  test,
  type Locator,
  type Page,
} from '@gravitational/e-e2e/helpers/test';

// openResourcesPanel opens the Resources side-nav drawer and returns its panel
async function openResourcesPanel(page: Page): Promise<Locator> {
  const button = page.getByRole('button', { name: 'Resources', exact: true });
  await button.hover();
  await button.click();

  const panel = page.locator('#panel-resources');
  await panel.waitFor({ state: 'visible' });

  return panel;
}

test.describe('feature hiding with a limited-access user', () => {
  // Run these tests against a Teleport cluster with a license that has feature hiding enabled.
  test.use({
    teleport: {
      config: {
        auth_service: {
          license_file:
            '${E2E_DIR}/testdata/licenses/license-featurehiding.pem',
        },
      },
    },
    user: {
      roles: [{ file: '@gravitational/e2e/roles/featurehiding-limited.yaml' }],
    },
  });

  test('hides management sections that the user cannot access', async ({
    page,
  }) => {
    await page.goto('/');

    await expect(
      page.getByRole('button', { name: 'Zero Trust Access' })
    ).not.toBeVisible();
    await expect(
      page.getByRole('button', { name: 'Machine & Workload ID' })
    ).not.toBeVisible();
  });

  test('hides identity governance features the user cannot access', async ({
    page,
    sideNavPage,
  }) => {
    await page.goto('/');

    const panel = await sideNavPage.openSection('Identity Governance');

    // Access Lists is the only feature in this section the role can access.
    await expect(panel.getByText('Access Lists')).toBeVisible();

    await expect(panel.getByText('Access Requests')).not.toBeVisible();
    await expect(panel.getByText('Access Automations')).not.toBeVisible();
    await expect(panel.getByText('Access Monitoring')).not.toBeVisible();
    await expect(panel.getByText('Session & Identity Locks')).not.toBeVisible();
  });

  test('only shows resource shortcuts for accessible kinds', async ({
    page,
  }) => {
    await page.goto('/');

    const panel = await openResourcesPanel(page);

    await expect(panel.getByText('Applications')).toBeVisible();
    await expect(panel.getByText('SSH Resources')).toBeVisible();

    // Should not see the kinds the role is denied access to.
    await expect(panel.getByText('Databases')).not.toBeVisible();
    await expect(panel.getByText('Kubernetes Clusters')).not.toBeVisible();
    await expect(panel.getByText('Desktops')).not.toBeVisible();
    await expect(panel.getByText('Git Servers')).not.toBeVisible();
  });

  test('resource type filter only lists accessible kinds', async ({
    page,
    unifiedResourcesPage,
  }) => {
    await unifiedResourcesPage.goto();

    await page.getByRole('button', { name: 'Types' }).click();

    const menu = page.getByRole('menu');

    await expect(menu.getByText('Applications')).toBeVisible();
    await expect(menu.getByText('SSH Resources')).toBeVisible();

    // Should not see filter options at all for denied kinds.
    await expect(menu.getByText('Databases')).not.toBeVisible();
    await expect(menu.getByText('Kubernetes Clusters')).not.toBeVisible();
    await expect(menu.getByText('Desktops')).not.toBeVisible();
  });
});

test.describe('feature hiding with a partial-access user', () => {
  // Run these tests against a Teleport cluster with a license that has feature hiding enabled.
  test.use({
    teleport: {
      config: {
        auth_service: {
          license_file:
            '${E2E_DIR}/testdata/licenses/license-featurehiding.pem',
        },
      },
    },
    user: {
      roles: [
        { file: '@gravitational/e2e/roles/featurehiding-bots-only.yaml' },
      ],
    },
  });

  test('shows only the accessible features within a section', async ({
    page,
    sideNavPage,
  }) => {
    await page.goto('/');

    const panel = await sideNavPage.openSection('Machine & Workload ID');

    // Bots is accessible for this role, so it should be visible.
    await expect(panel.getByText('Bots', { exact: true })).toBeVisible();
    // Other features in this section shouldn't be visible.
    await expect(panel.getByText('Workload Identity')).not.toBeVisible();
    await expect(panel.getByText('Bot Instances')).not.toBeVisible();

    // A section with no accessible features should be hidden entirely.
    await expect(
      page.getByRole('button', { name: 'Zero Trust Access' })
    ).not.toBeVisible();
  });

  test('nav search surfaces accessible features but not hidden ones', async ({
    page,
  }) => {
    await page.goto('/');

    const searchButton = page.getByRole('button', {
      name: 'Search',
      exact: true,
    });
    await searchButton.hover();
    await searchButton.click();

    const searchPanel = page.locator('#panel-Search');
    await searchPanel.waitFor({ state: 'visible' });
    const input = searchPanel.getByPlaceholder('Search for a page...');

    await input.fill('Bots');
    await expect(searchPanel.getByText('Bots', { exact: true })).toBeVisible();

    // Hidden features shouldn't appear in the results
    await input.fill('Integrations');
    await expect(searchPanel.getByText('Integrations')).not.toBeVisible();
  });
});

test.describe("feature hiding with a full-access user shouldn't hide any features", () => {
  // Run these tests against a Teleport cluster with a license that has feature hiding enabled.
  test.use({
    teleport: {
      config: {
        auth_service: {
          license_file:
            '${E2E_DIR}/testdata/licenses/license-featurehiding.pem',
        },
      },
    },
    user: { roles: ['access', 'editor'] },
  });

  test('does not hide features the user can access', async ({
    page,
    sideNavPage,
  }) => {
    await page.goto('/');

    await test.step('accessible management features remain visible', async () => {
      const panel = await sideNavPage.openSection('Zero Trust Access');

      await expect(panel.getByText('Integrations')).toBeVisible();
      await expect(panel.getByText('Trusted Devices')).toBeVisible();
    });

    await test.step('all resource shortcuts remain visible', async () => {
      const panel = await openResourcesPanel(page);

      await expect(panel.getByText('Applications')).toBeVisible();
      await expect(panel.getByText('Databases')).toBeVisible();
      await expect(panel.getByText('Kubernetes Clusters')).toBeVisible();
      await expect(panel.getByText('Desktops')).toBeVisible();
    });
  });
});
