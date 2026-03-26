import { login } from '@gravitational/e2e/helpers/login';
import { expect, test } from '@gravitational/e2e/helpers/test';

const lightBody = 'rgb(241, 242, 244)';
const darkBody = 'rgb(12, 20, 61)';

test('switching between dark and light mode', async ({ page }) => {
  await page.goto('/');
  await expect(page.locator('body')).toHaveCSS('background-color', lightBody);

  // Switch to dark mode.
  await page.getByRole('button', { name: 'User Menu' }).click();
  await page.getByText('Switch to Dark Theme').click();
  await expect(page.locator('body')).toHaveCSS('background-color', darkBody);

  // Dark mode should be retained after logging in again.
  await page.getByRole('button', { name: 'User Menu' }).click();
  await page.getByText('Logout').click();
  await login(page);
  await expect(page.locator('body')).toHaveCSS('background-color', darkBody);

  // Switch to light mode.
  await page.getByRole('button', { name: 'User Menu' }).click();
  await page.getByText('Switch to Light Theme').click();
  await expect(page.locator('body')).toHaveCSS('background-color', lightBody);
});
