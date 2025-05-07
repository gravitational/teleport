import { expect, test } from '@playwright/test';

import { mockWebAuthn } from '../utils/mockWebAuthn';

test('verify that a user can create and delete a role', async ({ page }) => {
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

  await page.getByRole('button', { name: 'Zero Trust Access' }).click();
  await page.getByRole('link', { name: 'Roles' }).click();
  await page.getByTestId('create_new_role_button').click();
  await page.getByRole('textbox', { name: 'Role Name(required)' }).click();
  await page
    .getByRole('textbox', { name: 'Role Name(required)' })
    .press('ControlOrMeta+a');
  await page
    .getByRole('textbox', { name: 'Role Name(required)' })
    .fill('testrole');
  await page.getByRole('button', { name: 'Next: Resources' }).click();
  await page
    .getByRole('button', { name: 'Add Teleport Resource Access' })
    .click();
  await page.getByRole('menuitem', { name: 'SSH Server Access' }).click();
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

  cleanup();
});
