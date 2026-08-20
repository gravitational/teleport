import type { Page } from '@playwright/test';

import { generateInviteURL } from './tctl';
import { mockWebAuthn } from './webauthn';

export const defaultPassword = 'passwordtest123';

export async function signup(
  page: Page,
  username: string,
  password = defaultPassword
) {
  await page.addInitScript(() =>
    localStorage.setItem('grv_teleport_license_acknowledged', 'true')
  );

  await mockWebAuthn(page, username);

  const inviteURL = generateInviteURL(username);
  await page.goto(inviteURL);

  await page.getByRole('button', { name: 'Get started' }).click();
  await page.getByRole('textbox', { name: 'Password', exact: true }).click();
  await page
    .getByRole('textbox', { name: 'Password', exact: true })
    .fill(password);
  await page
    .getByRole('textbox', { name: 'Password', exact: true })
    .press('Tab');
  await page.getByRole('textbox', { name: 'Confirm Password' }).fill(password);
  await page.getByRole('button', { name: 'Next' }).click();
  await page.getByRole('button', { name: 'Create an MFA Method' }).click();
  await page.getByRole('button', { name: 'Submit' }).click();
  await page.getByRole('button', { name: 'Go to Cluster' }).click();
}
