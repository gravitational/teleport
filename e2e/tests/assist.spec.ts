/**
 * Copyright 2023 Gravitational, Inc
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import { expect, test } from '@playwright/test';

test.beforeEach(async ({ page }) => {
  await page.goto('/');

  await page.getByPlaceholder('Username').fill('bob');
  await page.getByPlaceholder('Password').fill('secret');

  await page.getByRole('button', { name: 'Sign In' }).click();
});

test('nodes should be visible', async ({ page }) => {
  await expect(page.getByText(/^teleport-e2e$/).first()).toBeVisible();
});
