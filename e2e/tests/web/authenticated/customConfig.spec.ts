import { expect, test } from '@gravitational/e2e/helpers/test';

// Verify that an e2e test can be run with a custom config
test.describe('custom config', () => {
  test.use({
    teleport: {
      config: {
        proxy_service: {
          ssh_public_addr: ['e2e-custom-config.example.com:3023'],
        },
      },
    },
  });

  test('runs against a test-declared custom Teleport config', async ({
    page,
  }) => {
    const response = await page.request.get('/webapi/ping');
    expect(response.ok()).toBeTruthy();

    const ping = await response.json();
    expect(ping.proxy.ssh.ssh_public_addr).toBe(
      'e2e-custom-config.example.com:3023'
    );
  });
});
