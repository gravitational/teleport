/*
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

import { type Page } from '@playwright/test';

import { password as e2ePassword } from './env';
import { expect } from './test';
import { mockWebAuthn } from './webauthn';

export async function login(
  page: Page,
  username = 'bob',
  password = e2ePassword
) {
  await page.addInitScript(() =>
    localStorage.setItem('grv_teleport_license_acknowledged', 'true')
  );

  await mockWebAuthn(page);

  await page.goto('/');

  await page.getByPlaceholder('Username').fill(username);
  await page.getByPlaceholder('Password').fill(password);

  await page
    .getByTestId('userpassword')
    .getByRole('button', { name: 'Sign In' })
    .click();

  await page.waitForLoadState('networkidle');

  await expect(page.getByText(/^Resources$/).first()).toBeVisible();
}
