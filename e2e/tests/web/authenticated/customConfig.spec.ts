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
