import { signup } from '@gravitational/e2e/helpers/signup';
import { deleteUserIfExists } from '@gravitational/e2e/helpers/tctl';
import { expect, test } from '@gravitational/e2e/helpers/test';
import { TestInfo } from '@playwright/test';

function username(testInfo: TestInfo) {
  return `testuser-${testInfo.workerIndex}`;
}

test('verify that a user can sign up with webauthn and login', async ({
  page,
}, testInfo) => {
  await signup(page, username(testInfo));

  await page.getByRole('button', { name: 'User Menu' }).click();
  await page.getByText('Logout').click();
  await page
    .getByRole('textbox', { name: 'Username' })
    .fill(username(testInfo));
  await page.getByRole('textbox', { name: 'Username' }).press('Tab');
  await page.getByRole('textbox', { name: 'Password' }).fill('passwordtest123');

  await page
    .getByTestId('userpassword')
    .getByRole('button', { name: 'Sign In' })
    .click();

  await expect(page.getByRole('heading', { name: 'Resources' })).toBeVisible();
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
