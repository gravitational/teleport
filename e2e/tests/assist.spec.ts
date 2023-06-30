import { expect, test } from '@playwright/test';

test.beforeEach(async ({ page }) => {
  await page.goto('/');

  await page.getByPlaceholder('Username').fill('bob');
  await page.getByPlaceholder('Password').fill('secret');

  await page.getByRole('button', { name: 'Sign In' }).click();

  // Close the Assist modal
  // await page.getByText('Close').click();
});

test('nodes should be visible', async ({ page }) => {
  await expect(page.getByText(/^teleport-e2e$/).first()).toBeVisible();
})

test.skip('should SSH terminal works', async ({ page }) => {
  await page.getByRole('button', { name: 'CONNECT ' }).click();
  const page1Promise = page.waitForEvent('popup');
  await page.getByRole('link', { name: 'jnyckowski' }).click();

  const page1 = await page1Promise;
  await page1.getByRole('textbox', { name: 'Terminal input' }).fill('ls -la');
  await page1.getByRole('textbox', { name: 'Terminal input' }).press('Enter');
  await page1.getByRole('textbox', { name: 'Terminal input' }).fill('exit 0');
  await page1.getByRole('textbox', { name: 'Terminal input' }).press('Enter');

  await expect(
    page1.getByRole('button', { name: 'Start a New Session' })
  ).toBeVisible();
});

test.skip('should summary be generated', async ({ page }) => {
  // Open the assist modal
  await page
    .getByRole('navigation')
    .filter({
      hasText: 'Account Settings',
    })
    .getByTestId('svg')
    .first()
    .click();
  await page.getByText('Start a new conversation').click();
  // Ask for free memory and hope for getting the command back.
  await page
    .getByPlaceholder('Reply to Teleport')
    .fill('show free memory on all nodes');
  await page.getByPlaceholder('Reply to Teleport').press('Enter');
  await page.getByRole('button', { name: 'Run' }).click();

  // Wait for the summary to appear
  const summary = page.getByText('Summary of command execution');
  await expect(summary).toBeVisible({ timeout: 30_000 });
});

test.skip('should be able to remove conversation', async ({ page }) => {
  // skip
  await page.locator('path:nth-child(2)').click();
  await page.getByText('Start a new conversation').click();
  // Show conversations
  await page.locator('header div').first().click();
  // Get current conversation count
  const conversationCount = await page.getByRole('listitem').count();
  // Make suer it's sane
  expect(conversationCount).toBeGreaterThan(0);

  // Click the remove button
  await page
    .getByRole('listitem')
    .filter({ hasText: /^New conversation$/ })
    .nth(1)
    .hover()
    .then(async () => {
      await page
        .getByRole('listitem')
        .filter({ hasText: /^New conversation$/ })
        .nth(1)
        .locator('svg')
        .click();
    });
  // Confirm the remove
  await page.getByRole('button', { name: 'Delete' }).click();

  // Check if the conversation was removed
  await expect(page.getByRole('listitem')).toHaveCount(conversationCount - 1);
});
