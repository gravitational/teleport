import {
  expect,
  test,
  type Locator,
  type Page,
} from '@gravitational/e-e2e/helpers/test';

// visibleCategories returns the categories visible in the sidenav
async function visibleCategories(page: Page): Promise<Set<string>> {
  const titles = await page.getByTestId('side-nav-category').allInnerTexts();

  return new Set(titles.map(title => title.trim()));
}

// visibleNavItems returns the items visible in a sidenav panel
async function visibleNavItems(panel: Locator): Promise<Set<string>> {
  const titles = await panel.getByTestId('side-nav-item').allInnerTexts();

  return new Set(titles.map(title => title.trim()));
}

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
    teleport: { license: 'featurehiding' },
    user: {
      roles: [{ file: '@gravitational/e2e/roles/featurehiding-limited.yaml' }],
    },
  });

  test('shows only the nav categories the user can access', async ({
    page,
  }) => {
    await page.goto('/');

    await expect
      .poll(() => visibleCategories(page))
      .toEqual(
        new Set(['Audit', 'Identity Governance', 'Resources', 'Search'])
      );
  });

  test('hides identity governance features the user cannot access', async ({
    page,
    sideNavPage,
  }) => {
    await page.goto('/');

    const panel = await sideNavPage.openSection('Identity Governance');

    // Access Lists is the only feature in this section the role can access.
    await expect
      .poll(() => visibleNavItems(panel))
      .toEqual(new Set(['Access Lists']));
  });

  test('only shows resource shortcuts for accessible kinds', async ({
    page,
  }) => {
    await page.goto('/');

    const panel = await openResourcesPanel(page);

    // The role should only see nodes, apps, and mcp servers
    await expect
      .poll(() => visibleNavItems(panel))
      .toEqual(
        new Set([
          'All Resources',
          'Pinned Resources',
          'SSH Resources',
          'Applications',
          'MCP Servers',
        ])
      );
  });

  test('resource type filter only lists accessible kinds', async ({
    page,
    unifiedResourcesPage,
  }) => {
    await unifiedResourcesPage.goto();

    await page.getByRole('button', { name: 'Types' }).click();

    const menu = page.getByRole('menu');

    // The role should only see filter options for apps, nodes, and mcp servers
    await expect
      .poll(
        async () => new Set(await menu.getByRole('menuitem').allInnerTexts())
      )
      .toEqual(new Set(['Applications', 'MCP Servers', 'SSH Resources']));
  });
});

test.describe('feature hiding with a partial-access user', () => {
  // Run these tests against a Teleport cluster with a license that has feature hiding enabled.
  test.use({
    teleport: { license: 'featurehiding' },
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

    // Bots is the only thing this role should grant access to in this nav section
    await expect.poll(() => visibleNavItems(panel)).toEqual(new Set(['Bots']));

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
    teleport: { license: 'featurehiding' },
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
