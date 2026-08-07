import { expect, test } from '@gravitational/e-e2e/helpers/test';

// This test asserts that the Cloud Panel bundle is served by a staging
// Cloud API and mounted by this Teleport build
test('cloud panel mounts and renders inside Teleport', async ({ page }) => {
  // The cloud panel only exists on a cloud cluster which can reach the Cloud API
  test.skip(
    !process.env.TELEPORT_CLOUD_HOSTPORT,
    'cloud panel requires a cloud backend + license (set TELEPORT_CLOUD_HOSTPORT)'
  );

  const assetResponses: number[] = [];
  page.on('response', res => {
    if (res.url().includes('/v1/enterprise/cloud/assets/')) {
      assetResponses.push(res.status());
    }
  });

  await page.goto('/web/cloud/summary');

  // Assert Summary Page loads
  await expect(
    page.getByRole('heading', { name: 'Usage Reporting' })
  ).toBeVisible({ timeout: 15_000 });
  await expect(page.getByText(/something went wrong/i)).toHaveCount(0);

  // Assert that Teleport fetched the bundle from the Cloud API and every asset served OK
  expect(assetResponses.length).toBeGreaterThan(0);
  expect(assetResponses.every(status => status < 400)).toBe(true);

  // Assert we can navigate to MAU Breakdown page and it loads
  await page.getByRole('button', { name: /Monthly Active Users/i }).click();
  await expect(page).toHaveURL(/\/web\/cloud\/mau/);
  await expect(
    page.getByRole('heading', { name: 'Monthly Active Users (MAU)' })
  ).toBeVisible({ timeout: 15_000 });
});
