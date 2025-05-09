import { expect, test } from '@playwright/test';

import { signup } from '../utils/signup';

test('verify that a user can create and delete a role', async ({ page }) => {
  const { cleanup } = await signup(page);

  await page.getByRole('button', { name: 'Zero Trust Access' }).click();
  await page.getByRole('link', { name: 'Roles' }).click();
  await page.getByTestId('create_new_role_button').click();
  await page.getByRole('textbox', { name: 'Role Name(required)' }).click();
  await page
    .getByRole('textbox', { name: 'Role Name(required)' })
    .fill('testrole');
  await page.getByRole('button', { name: 'Next: Resources' }).click();
  await page
    .getByRole('button', { name: 'Add Teleport Resource Access' })
    .click();
  await page.getByRole('menuitem', { name: 'Servers' }).click();
  await page.getByRole('button', { name: 'Next: Admin Rules' }).click();
  await page.getByRole('button', { name: 'Next: Options' }).click();
  await page.getByRole('button', { name: 'Create Role' }).click();

  await expect(page.getByRole('cell', { name: 'testrole' })).toBeVisible();

  await page
    .getByRole('row', { name: 'testrole Options' })
    .getByRole('button')
    .click();
  await page.getByRole('menuitem', { name: 'Delete' }).click();
  await page.getByRole('button', { name: 'Yes, Remove Role' }).click();

  await expect(page.getByRole('cell', { name: 'testrole' })).not.toBeVisible();

  await cleanup();
});
