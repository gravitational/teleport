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

import { expect, test } from '@playwright/test';

import { createIntegration, deleteIntegration } from '../utils/api';

const INTEGRATION_NAME = 'test-aws-integration';

let integrationCreated = false;

test.afterEach(async ({ page }) => {
  if (integrationCreated) {
    await deleteIntegration(page, INTEGRATION_NAME);
    integrationCreated = false;
  }
});

test('Create discovery AWS with Terraform integration', async ({ page }) => {
  await page.goto('/web/integrations/new/aws-cloud');
  await expect(
    page.getByRole('heading', { name: 'Connect Amazon Web Services' })
  ).toBeVisible();

  const integrationNameInput = page.getByPlaceholder('my-aws-integration');
  await expect(integrationNameInput).toBeVisible();
  await integrationNameInput.clear();
  await integrationNameInput.fill(INTEGRATION_NAME);

  await expect(page.getByText('EC2 Instances', { exact: true })).toBeVisible();
  await expect(page.getByText('All regions')).toBeVisible();

  await page.getByRole('button', { name: 'Add a tag' }).click();
  await page.getByRole('textbox', { name: 'Environment' }).click();
  await page
    .getByRole('textbox', { name: 'Environment' })
    .fill('teleport.dev/creator');
  await page.getByRole('textbox', { name: 'Environment' }).press('Tab');
  await page
    .getByRole('textbox', { name: 'production' })
    .fill('charles.bryan@goteleport.com');

  await expect(
    page.getByRole('radio', { name: 'Terraform Configuration' })
  ).toBeVisible();

  await page
    .getByRole('button', { name: 'Copy Terraform Module' })
    .first()
    .click();

  // Creating AWS resources with Terraform would be slow and
  // expensive. Instead we manually create the integration with an API
  // call to continue testing the flow.
  const resp = await createIntegration(
    page,
    INTEGRATION_NAME,
    'arn:aws:iam::123456789012:role/test'
  );
  expect(resp.ok()).toBe(true);
  integrationCreated = true;

  // Wait for the integration creation API call to succeed before
  // checking the integration. Avoids timeout issues.
  await Promise.all([
    page.waitForResponse(
      resp =>
        resp.url().includes(`/integrations/${INTEGRATION_NAME}`) && resp.ok()
    ),
    page.getByRole('button', { name: 'Check Integration' }).click(),
  ]);
  await expect(page.getByText('Integration Detected')).toBeVisible();

  await page.getByRole('link', { name: 'View Integration' }).nth(1).click();
  await expect(page.getByText('test-aws-integration')).toBeVisible();
  await expect(page.getByLabel('status')).toHaveText('Scanning');
});
