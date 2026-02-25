import { expect, type Page } from '@playwright/test';

import { mockWebAuthn } from '../utils/mockWebAuthn';

export async function login(page: Page, username = 'bob', password = 'secret') {
  const { cleanup } = await mockWebAuthn(page);

  await page.goto('/');

  await page.getByPlaceholder('Username').fill(username);
  await page.getByPlaceholder('Password').fill(password);

  await page.getByRole('button', { name: 'Sign In' }).click();

  await page.waitForLoadState('networkidle');

  await expect(page.getByText(/^Resources$/).first()).toBeVisible();

  return { cleanup };
}
