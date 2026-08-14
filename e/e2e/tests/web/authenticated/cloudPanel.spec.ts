import { expect, test } from '@gravitational/e-e2e/helpers/test';

// This test asserts that the Cloud Panel bundle is served by a staging
// Cloud API and mounted by this Teleport build
test.describe('cloud panel', () => {
  test.use({
    teleport: {
      config: {
        auth_service: {
          license_file: '${E2E_DIR}/../fixtures/license-cloud-ent-staging.pem',
        },
      },
      env: {
        TELEPORT_CLOUD_HOSTPORT: 'api.cloud.gravitational.io',
      },
    },
  });

  test('cloud panel mounts and renders inside Teleport', async ({ page }) => {
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
});
