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

import type { Page } from '@playwright/test';

import { generateInviteURL } from './tctl';
import { mockWebAuthn } from './webauthn';

export async function signup(page: Page, username: string) {
  await page.addInitScript(() =>
    localStorage.setItem('grv_teleport_license_acknowledged', 'true')
  );

  await mockWebAuthn(page);

  const inviteURL = generateInviteURL(username);
  await page.goto(inviteURL);

  await page.getByRole('button', { name: 'Get started' }).click();
  await page.getByRole('textbox', { name: 'Password', exact: true }).click();
  await page
    .getByRole('textbox', { name: 'Password', exact: true })
    .fill('passwordtest123');
  await page
    .getByRole('textbox', { name: 'Password', exact: true })
    .press('Tab');
  await page
    .getByRole('textbox', { name: 'Confirm Password' })
    .fill('passwordtest123');
  await page.getByRole('button', { name: 'Next' }).click();
  await page.getByRole('button', { name: 'Create an MFA Method' }).click();
  await page.getByRole('button', { name: 'Submit' }).click();
  await page.getByRole('button', { name: 'Go to Cluster' }).click();
}
