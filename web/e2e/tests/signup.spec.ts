import { expect, test } from '@playwright/test';

import { signup } from '../utils/signup';

test('verify that a user can sign up with webauthn and login', async ({
  page,
}) => {
  const { cleanup } = await signup(page);

  await page.getByRole('button', { name: 'User Menu' }).click();
  await page.getByText('Logout').click();
  await page.getByRole('textbox', { name: 'Username' }).fill('testuser');
  await page.getByRole('textbox', { name: 'Username' }).press('Tab');
  await page.getByRole('textbox', { name: 'Password' }).fill('passwordtest123');
  await page
    .getByTestId('userpassword')
    .getByRole('button', { name: 'Sign In' })
    .click();

  await expect(page.getByRole('heading', { name: 'Resources' })).toBeVisible();

  await cleanup();
});
