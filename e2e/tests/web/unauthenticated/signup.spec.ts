/**
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

import { signup } from '@gravitational/e2e/helpers/signup';
import { deleteUserIfExists } from '@gravitational/e2e/helpers/tctl';
import { expect, test } from '@gravitational/e2e/helpers/test';
import { TestInfo } from '@playwright/test';

function username(testInfo: TestInfo) {
  return `testuser-${testInfo.workerIndex}`;
}

// TODO(ryan): re-enable this test once Firefox flakiness is resolved.
test.skip('verify that a user can sign up with webauthn and login', async ({
  page,
}, testInfo) => {
  // Signing up auto-logs-in and we then log straight back in; that burst can
  // trip Teleport's challenge-generation rate limiter, so allow time to retry.
  test.slow();

  await signup(page, username(testInfo));

  await page.getByRole('button', { name: 'User Menu' }).click();
  await page.getByText('Logout').click();
  await page
    .getByRole('textbox', { name: 'Username' })
    .fill(username(testInfo));
  await page.getByRole('textbox', { name: 'Username' }).press('Tab');
  await page.getByRole('textbox', { name: 'Password' }).fill('passwordtest123');

  // The web login challenge endpoint is rate limited; signing up then logging
  // back in immediately can trip it ("rate limit exceeded, try again in Ns").
  // Retry the sign-in until the limiter lets it through.
  await expect(async () => {
    await page
      .getByTestId('userpassword')
      .getByRole('button', { name: 'Sign In' })
      .click();
    await expect(page.getByRole('heading', { name: 'Resources' })).toBeVisible({
      timeout: 5_000,
    });
  }).toPass({ timeout: 30_000, intervals: [3_000] });
});

// Clean the user before each attempt (and after) so retries start clean rather
// than failing on an already-registered user / consumed invite.
// oxlint-disable-next-line no-empty-pattern
test.beforeEach(({}, testInfo) => {
  deleteUserIfExists(username(testInfo));
});

// oxlint-disable-next-line no-empty-pattern
test.afterEach(({}, testInfo) => {
  deleteUserIfExists(username(testInfo));
});
