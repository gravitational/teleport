import { login, logout } from '@gravitational/e2e/helpers/login';
import { defaultPassword, signup } from '@gravitational/e2e/helpers/signup';
import { deleteUserIfExists } from '@gravitational/e2e/helpers/tctl';
import { expect, test } from '@gravitational/e2e/helpers/test';
import { TestInfo } from '@playwright/test';

const lightBody = 'rgb(241, 242, 244)';
const darkBody = 'rgb(12, 20, 61)';

function username(testInfo: TestInfo) {
  return `testuser-${testInfo.workerIndex}`;
}

test('switching between dark and light theme', async ({ page }, testInfo) => {
  await signup(page, username(testInfo), defaultPassword);
  await expect(page.locator('body')).toHaveCSS('background-color', lightBody);

  // Switch to dark theme. Make sure that the change gets persisted in user
  // preferences on the backend side before we log out.
  const prefsResponse = page.waitForResponse(
    response =>
      URL.parse(response.url())?.pathname === '/v1/webapi/user/preferences' &&
      response.request().method() === 'PUT'
  );
  await page.getByRole('button', { name: 'User Menu' }).click();
  await page.getByText('Switch to Dark Theme').click();
  await expect(page.locator('body')).toHaveCSS('background-color', darkBody);
  await prefsResponse;

  // Dark theme should be retained after logging out and in again.
  await logout(page);
  await expect(page.locator('body')).toHaveCSS('background-color', darkBody);

  // Make sure that the theme was actually saved in the backend. Simulate
  // signing in on a fresh browser.
  await page.context().clearCookies();
  await page.evaluate(() => localStorage.clear());
  await login(page, username(testInfo), defaultPassword);
  await expect(page.locator('body')).toHaveCSS('background-color', darkBody);

  // Switch to light theme.
  await page.getByRole('button', { name: 'User Menu' }).click();
  await page.getByText('Switch to Light Theme').click();
  await expect(page.locator('body')).toHaveCSS('background-color', lightBody);
});

// Clean the user before each attempt (and after) so retries start clean rather
// than failing on an already-registered user / consumed invite.
// oxlint-disable-next-line no-empty-pattern
test.beforeEach(({}, testInfo) => {
  deleteUserIfExists(username(testInfo));
});

// oxlint-disable-next-line no-empty-pattern
test.afterEach(({}, testInfo) => {
  deleteUserIfExists(username(testInfo));
});
