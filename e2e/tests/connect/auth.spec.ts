import { test, expect } from '@gravitational/e2e/helpers/connect';
import { startUrl } from '@gravitational/e2e/helpers/env';

test.use({ autoLogin: true });

test('logging out', async ({ app }) => {
  const { page } = app;
  await page.getByTitle(/Open Profiles/).click();
  await page.getByTitle(/Log out/).click();
  await expect(
    page.getByText('Are you sure you want to log out?')
  ).toBeVisible();
  await page.getByRole('button', { name: 'Log Out', exact: true }).click();
  const proxyHostname = new URL(startUrl).hostname;
  const previouslyUsedCluster = page
    .getByRole('listitem')
    .filter({ hasText: proxyHostname });
  await expect(previouslyUsedCluster.getByText('Not logged in')).toBeVisible();
});
