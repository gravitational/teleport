import { expect, type Page } from '@playwright/test';
import { generate } from 'otplib';

const secret = '5F5OH3PDPWTQKZEGN44LX6R4STEPZWB7';

export async function login(page: Page, username = 'bob', password = 'secret') {
  await page.addInitScript(() =>
    localStorage.setItem('grv_teleport_license_acknowledged', 'true')
  );
  await page.goto('/');

  await page.getByPlaceholder('Username').fill(username);
  await page.getByPlaceholder('Password').fill(password);

  const token = await generate({ secret });

  await page.getByPlaceholder('123 456').fill(token);

  await page.getByRole('button', { name: 'Sign In' }).click();

  await page.waitForLoadState('networkidle');

  await expect(page.getByText(/^Resources$/).first()).toBeVisible();
}
