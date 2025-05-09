import { Page } from '@playwright/test';

import { mockWebAuthn } from './mockWebAuthn';

/**
 * signup completes the signup flow for a new user. This is typically run at the beginning of every test.
 */
export async function signup(page: Page) {
  const { cleanup } = await mockWebAuthn(page);

  await page.goto('');

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

  await page.getByRole('button', { name: "I'll do that later" }).click();

  return { cleanup };
}
