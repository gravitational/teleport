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

import { login } from '@gravitational/e2e/helpers/login';
import { defaultPassword, signup } from '@gravitational/e2e/helpers/signup';
import { expect, test } from '@gravitational/e2e/helpers/test';
import {
  defaultDevice,
  setCurrentDevice,
  WebAuthnDevice,
} from '@gravitational/e2e/helpers/webauthn';

test.afterEach(() => {
  setCurrentDevice(defaultDevice);
});

test('adding a new passkey', async ({
  page,
  authenticatedPage,
  accountSettingsPage,
}, testInfo) => {
  const username = `testuser-passkey-${testInfo.workerIndex}`;
  await signup(page, username, defaultPassword);

  await authenticatedPage.goto();
  await authenticatedPage.openAccountSettings();
  await expect(accountSettingsPage.passkeyRows).toHaveCount(0);
  await expect(accountSettingsPage.mfaRows).toHaveCount(1);
  await expect(accountSettingsPage.mfaRows.first()).toContainText(
    'Hardware Key'
  );
  await expect(accountSettingsPage.mfaRows.first()).toContainText(
    'webauthn-device'
  );

  await accountSettingsPage.addPasskey();
  await accountSettingsPage.verifyIdentity();

  setCurrentDevice(new WebAuthnDevice());
  await accountSettingsPage.createPasskey();

  await accountSettingsPage.setPasskeyNickname('new-passkey');
  await accountSettingsPage.savePasskey();
  await expect(accountSettingsPage.mfaRows).toHaveCount(1);
  await expect(accountSettingsPage.passkeyRows).toHaveCount(1);
  await expect(accountSettingsPage.passkeyRows.first()).toContainText(
    'new-passkey'
  );

  // Log out and in again, this time with the new MFA device, to test that it's
  // actually working.
  await authenticatedPage.logout();
  await login(page, username, defaultPassword);
});
