import { expect, test } from '@playwright/test';

import { mockWebAuthn } from '../utils/mockWebAuthn';

test('verify that a user can create and delete an auth connector', async ({
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

  await page.getByRole('button', { name: "I'll do that later" }).click();

  await page.getByRole('button', { name: 'Zero Trust Access' }).click();
  await page.getByRole('link', { name: 'Auth Connectors' }).click();
  await page.getByRole('button', { name: 'New GitHub Connector' }).click();

  await page.waitForSelector('.ace_editor', { state: 'visible' });
  await page.evaluate(() => {
    const editor = (window as any).ace.edit(
      document.querySelector('.ace_editor')
    );

    const lines = editor.session.getDocument().getAllLines();

    lines[3] = '  name: testconnector';

    editor.session.setValue(lines.join('\n'));
  });

  await page.getByRole('button', { name: 'Save Changes' }).click();

  await expect(page.getByText('testconnector')).toBeVisible();

  const connectorTile = page.getByTestId('testconnector-tile');

  await connectorTile.getByRole('button').click();

  await page.getByRole('menuitem', { name: 'Delete' }).click();
  await page.getByRole('button', { name: 'Delete Connector' }).click();

  await expect(page.getByText('testconnector')).not.toBeVisible();

  cleanup();
});
