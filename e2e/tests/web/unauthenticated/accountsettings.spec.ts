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

import { login, logout } from '@gravitational/e2e/helpers/login';
import { defaultPassword, signup } from '@gravitational/e2e/helpers/signup';
import { expect, test } from '@gravitational/e2e/helpers/test';
import {
  setCurrentDevice,
  WebAuthnDevice,
} from '@gravitational/e2e/helpers/webauthn';

test('account settings', async ({ page }, testInfo) => {
  const username = `testuser-${testInfo.workerIndex}`;
  await signup(page, username, defaultPassword);

  await page.goto('/');
  await page.getByRole('button', { name: 'User Menu' }).click();
  await page.getByRole('link', { name: 'Account Settings' }).click();
  const passkeyDevices = page.getByTestId('passkey-list').locator('tbody tr');
  const mfaDevices = page.getByTestId('mfa-list').locator('tbody tr');
  await expect(passkeyDevices).toHaveCount(0);
  await expect(mfaDevices).toHaveCount(1);
  await expect(mfaDevices.first()).toContainText('Hardware Key');
  await expect(mfaDevices.first()).toContainText('webauthn-device');

  await page.getByRole('button', { name: 'Add a Passkey' }).click();
  await page.getByRole('button', { name: 'Verify my identity' }).click();

  setCurrentDevice(new WebAuthnDevice());
  await page.getByRole('button', { name: 'Create a Passkey' }).click();

  await page.getByLabel('Passkey Nickname').fill('new-passkey');
  await page.getByRole('button', { name: 'Save the Passkey' }).click();
  await expect(mfaDevices).toHaveCount(1);
  await expect(passkeyDevices).toHaveCount(1);
  await expect(passkeyDevices.first()).toContainText('new-passkey');

  await logout(page);
  await login(page, username, defaultPassword);
});
