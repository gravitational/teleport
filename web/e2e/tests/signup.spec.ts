import { test } from '@playwright/test';

import { mockWebAuthn } from '../utils/mockWebAuthn';

test('verify that a user can sign up with webauthn and login', async ({
  page,
}) => {
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
  await page.getByRole('button', { name: 'User Menu' }).click();
  await page.getByText('Logout').click();
  await page.getByRole('textbox', { name: 'Username' }).fill('e2e-user');
  await page.getByRole('textbox', { name: 'Username' }).press('Tab');
  await page.getByRole('textbox', { name: 'Password' }).fill('passwordtest123');
  await page
    .getByTestId('userpassword')
    .getByRole('button', { name: 'Sign In' })
    .click();

  await page.getByText('Resources');

  cleanup();
});
