/**
 * Teleport
 * Copyright (C) 2026 Gravitational, Inc.
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

import { env } from 'node:process';

import { expect, test } from '@gravitational/e2e/helpers/test';

test.describe('help and support page', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/');
    await page.getByRole('button', { name: 'User Menu' }).click();
    await page.getByRole('link', { name: 'Help & Support' }).click();
  });

  test('contains support and resource links', async ({ page }) => {
    const version = `oss_${env['E2E_TELEPORT_VERSION']}`;
    const expectedFetchableUrls: Record<string, string> = {
      'Ask the Community Questions':
        'https://github.com/gravitational/teleport/discussions',
      'Request a New Feature':
        'https://github.com/gravitational/teleport/issues/new/choose',
      'Get Started Guide': `https://goteleport.com/docs/get-started/?product=teleport&version=${version}`,
      'tsh User Guide': `https://goteleport.com/docs/connect-your-client/tsh/?product=teleport&version=${version}`,
      'Admin Guides': `https://goteleport.com/docs/admin-guides/management/admin/?product=teleport&version=${version}`,
      'Troubleshooting Guide': `https://goteleport.com/docs/admin-guides/management/admin/troubleshooting/?product=teleport&version=${version}`,
      'Download Page': `https://goteleport.com/download/`,
      FAQ: `https://goteleport.com/docs/faq?product=teleport&version=${version}`,
      'Product Changelog': `https://goteleport.com/docs/changelog?product=teleport&version=${version}`,
      'Upcoming Releases': `https://goteleport.com/docs/upcoming-releases?product=teleport&version=${version}`,
      'Teleport Blog': `https://goteleport.com/blog/`,
    };

    const expectedUrls: Record<string, string> = {
      ...expectedFetchableUrls,
      'Send Product Feedback': 'mailto:support@goteleport.com',
    };

    // Start fetching links that can be fetched; we'll need it later.
    const fetchPromises = Object.values(expectedFetchableUrls).map(url =>
      fetch(url)
    );

    // The links are all external, so to make sure our tests are not brittle,
    // we don't perform any assertions on the pages that they lead to. Instead,
    // just verify that the link href is as expected and perform a "dry run"
    // link actionability test.
    for (const name in expectedUrls) {
      const link = page.getByRole('link', { name });
      await expect(link).toHaveAttribute('href', expectedUrls[name]);
      await link.click({ trial: true });
    }

    // Make sure that all fetchable URLs are actually reachable.
    const responses = await Promise.all(fetchPromises);
    for (const resp of responses) {
      expect(resp.ok, `Expecting a successful response for ${resp.url}`).toBe(
        true
      );
    }
  });

  test('contains cluster information', async ({ page }) => {
    const clusterInformationSection = page.getByRole('region', {
      name: 'Cluster Information',
    });
    await expect(clusterInformationSection).toContainText(
      'Cluster Name: teleport-e2e'
    );
    await expect(clusterInformationSection).toContainText(
      `Teleport Version: ${env['E2E_TELEPORT_VERSION']}`
    );
    const host = URL.parse(page.url())?.host;
    await expect(clusterInformationSection).toContainText(
      `Public Address: ${host}`
    );
  });

  test('button to unlock premium support leads to the sales page', async ({
    page,
  }) => {
    const salesPromise = page.waitForEvent('popup');
    await page
      .getByRole('link', { name: 'Unlock Premium Support with Enterprise' })
      .click();
    const salesPage = await salesPromise;
    await expect(salesPage.getByRole('heading')).toContainText(
      'Schedule a sales conversation'
    );
    await salesPage.close();
  });
});
